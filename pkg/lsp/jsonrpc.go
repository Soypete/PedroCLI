package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// JSONRPCMessage represents a JSON-RPC 2.0 message.
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`     // Request/Response ID
	Method  string          `json:"method,omitempty"` // Request/Notification method
	Params  json.RawMessage `json:"params,omitempty"` // Request/Notification params
	Result  json.RawMessage `json:"result,omitempty"` // Response result
	Error   *JSONRPCError   `json:"error,omitempty"`  // Response error
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// JSONRPCConn provides JSON-RPC communication over stdio.
type JSONRPCConn struct {
	reader     *bufio.Reader
	writer     io.Writer
	writeMu    sync.Mutex
	idCounter  int64
	pending    map[int64]chan *JSONRPCMessage
	pendingMu  sync.Mutex
	handlers   map[string]NotificationHandler
	handlersMu sync.RWMutex
	closed     atomic.Bool
	closeCh    chan struct{}
}

// NotificationHandler handles incoming notifications.
type NotificationHandler func(method string, params json.RawMessage)

// NewJSONRPCConn creates a new JSON-RPC connection.
func NewJSONRPCConn(r io.Reader, w io.Writer) *JSONRPCConn {
	conn := &JSONRPCConn{
		reader:   bufio.NewReader(r),
		writer:   w,
		pending:  make(map[int64]chan *JSONRPCMessage),
		handlers: make(map[string]NotificationHandler),
		closeCh:  make(chan struct{}),
	}
	go conn.readLoop()
	return conn
}

// Close closes the connection.
func (c *JSONRPCConn) Close() {
	if c.closed.CompareAndSwap(false, true) {
		close(c.closeCh)
		// Cancel all pending requests
		c.pendingMu.Lock()
		for _, ch := range c.pending {
			close(ch)
		}
		c.pending = make(map[int64]chan *JSONRPCMessage)
		c.pendingMu.Unlock()
	}
}

// OnNotification registers a handler for notifications.
func (c *JSONRPCConn) OnNotification(method string, handler NotificationHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.handlers[method] = handler
}

// Call sends a request and waits for a response.
func (c *JSONRPCConn) Call(method string, params interface{}, result interface{}) error {
	if c.closed.Load() {
		return fmt.Errorf("connection closed")
	}

	id := atomic.AddInt64(&c.idCounter, 1)

	// Create response channel
	respCh := make(chan *JSONRPCMessage, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	// Marshal params
	var paramsJSON json.RawMessage
	if params != nil {
		var err error
		paramsJSON, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	// Send request
	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  paramsJSON,
	}

	if err := c.writeMessage(&msg); err != nil {
		return err
	}

	// Wait for response
	select {
	case resp, ok := <-respCh:
		if !ok {
			return fmt.Errorf("connection closed while waiting for response")
		}
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil && resp.Result != nil {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("failed to unmarshal result: %w", err)
			}
		}
		return nil
	case <-c.closeCh:
		return fmt.Errorf("connection closed")
	}
}

// Notify sends a notification (no response expected).
func (c *JSONRPCConn) Notify(method string, params interface{}) error {
	if c.closed.Load() {
		return fmt.Errorf("connection closed")
	}

	var paramsJSON json.RawMessage
	if params != nil {
		var err error
		paramsJSON, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsJSON,
	}

	return c.writeMessage(&msg)
}

// writeMessage writes a JSON-RPC message with Content-Length header.
func (c *JSONRPCConn) writeMessage(msg *JSONRPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := c.writer.Write([]byte(header)); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if _, err := c.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// readLoop reads messages from the connection.
func (c *JSONRPCConn) readLoop() {
	for !c.closed.Load() {
		msg, err := c.readMessage()
		if err != nil {
			// Skip read errors - connection may be closed or server sent malformed message
			continue
		}

		c.handleMessage(msg)
	}
}

// readMessage reads a single JSON-RPC message.
func (c *JSONRPCConn) readMessage() (*JSONRPCMessage, error) {
	// Read headers
	contentLength := 0
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break // End of headers
		}

		if strings.HasPrefix(line, "Content-Length:") {
			lenStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, err = strconv.Atoi(lenStr)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %s", lenStr)
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	// Read content
	content := make([]byte, contentLength)
	if _, err := io.ReadFull(c.reader, content); err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	var msg JSONRPCMessage
	if err := json.Unmarshal(content, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}

// handleMessage handles an incoming message.
func (c *JSONRPCConn) handleMessage(msg *JSONRPCMessage) {
	// Response
	if msg.ID != nil && (msg.Result != nil || msg.Error != nil) {
		c.pendingMu.Lock()
		ch, ok := c.pending[*msg.ID]
		c.pendingMu.Unlock()
		if ok {
			select {
			case ch <- msg:
			default:
			}
		}
		return
	}

	// Notification
	if msg.Method != "" && msg.ID == nil {
		c.handlersMu.RLock()
		handler, ok := c.handlers[msg.Method]
		c.handlersMu.RUnlock()
		if ok {
			go handler(msg.Method, msg.Params)
		}
	}
}
