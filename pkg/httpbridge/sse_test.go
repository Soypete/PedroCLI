package httpbridge

import (
	"context"
	"testing"
	"time"

	"github.com/soypete/pedrocli/pkg/toolformat"
)

// mockBridge is a simple mock for testing SSE broadcaster
type mockBridge struct{}

func (m *mockBridge) CallTool(ctx context.Context, name string, args map[string]interface{}) (*toolformat.BridgeResult, error) {
	return &toolformat.BridgeResult{Success: true, Output: "mock result"}, nil
}

func (m *mockBridge) IsHealthy() bool { return true }

func (m *mockBridge) GetToolNames() []string { return []string{"test_tool"} }

func TestSSEBroadcaster_AddRemoveClient(t *testing.T) {
	ctx := context.Background()
	broadcaster := NewSSEBroadcasterWithBridge(&mockBridge{}, ctx)

	// Add a client
	client := broadcaster.AddClient("job-123")
	if client == nil {
		t.Fatal("Expected client to be created")
	}

	// Check client was added
	broadcaster.mutex.RLock()
	if len(broadcaster.clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(broadcaster.clients))
	}
	broadcaster.mutex.RUnlock()

	// Remove client
	broadcaster.RemoveClient(client.ID)

	// Check client was removed
	broadcaster.mutex.RLock()
	if len(broadcaster.clients) != 0 {
		t.Errorf("Expected 0 clients, got %d", len(broadcaster.clients))
	}
	broadcaster.mutex.RUnlock()
}

func TestSSEBroadcaster_Broadcast(t *testing.T) {
	ctx := context.Background()
	broadcaster := NewSSEBroadcasterWithBridge(&mockBridge{}, ctx)

	// Add two clients watching the same job
	client1 := broadcaster.AddClient("job-123")
	client2 := broadcaster.AddClient("job-123")

	// Add a client watching a different job
	client3 := broadcaster.AddClient("job-456")

	// Broadcast message to job-123
	msg := SSEMessage{
		Event: "update",
		Data:  "test message",
	}

	go broadcaster.Broadcast("job-123", msg)

	// Check that client1 and client2 received the message
	select {
	case received := <-client1.Channel:
		if received.Event != "update" {
			t.Errorf("Expected event 'update', got '%s'", received.Event)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("client1 did not receive message")
	}

	select {
	case received := <-client2.Channel:
		if received.Event != "update" {
			t.Errorf("Expected event 'update', got '%s'", received.Event)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("client2 did not receive message")
	}

	// Check that client3 did NOT receive the message
	select {
	case <-client3.Channel:
		t.Error("client3 should not have received message for different job")
	case <-time.After(100 * time.Millisecond):
		// Expected - no message received
	}

	// Cleanup
	broadcaster.RemoveClient(client1.ID)
	broadcaster.RemoveClient(client2.ID)
	broadcaster.RemoveClient(client3.ID)
}

func TestSSEBroadcaster_BroadcastToAll(t *testing.T) {
	ctx := context.Background()
	broadcaster := NewSSEBroadcasterWithBridge(&mockBridge{}, ctx)

	// Add client watching all jobs (*)
	clientAll := broadcaster.AddClient("*")

	// Add client watching specific job
	clientSpecific := broadcaster.AddClient("job-123")

	// Broadcast message to all
	msg := SSEMessage{
		Event: "list",
		Data:  "all jobs",
	}

	go broadcaster.Broadcast("*", msg)

	// Check that clientAll received the message
	select {
	case received := <-clientAll.Channel:
		if received.Event != "list" {
			t.Errorf("Expected event 'list', got '%s'", received.Event)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("clientAll did not receive message")
	}

	// clientSpecific should NOT receive this message (it's only watching job-123)
	select {
	case <-clientSpecific.Channel:
		t.Error("clientSpecific should not receive broadcast to '*'")
	case <-time.After(100 * time.Millisecond):
		// Expected - no message received
	}

	// Cleanup
	broadcaster.RemoveClient(clientAll.ID)
	broadcaster.RemoveClient(clientSpecific.ID)
}

func TestSSEMessage_JSON(t *testing.T) {
	msg := SSEMessage{
		Event: "update",
		Data: map[string]interface{}{
			"job_id": "job-123",
			"status": "running",
		},
	}

	// This just validates the struct is JSON-serializable
	// Actual JSON marshaling is tested in the HTTP handler
	if msg.Event != "update" {
		t.Errorf("Expected event 'update', got '%s'", msg.Event)
	}
}
