# Web Server API Reference

The PedroCLI web server provides a REST API and WebSocket interface for interacting with autonomous coding agents through a browser.

## Starting the Web Server

```bash
# Basic usage
./web-server -port 8080 -config .pedroceli.json

# With custom jobs directory
./web-server -port 8080 -config .pedroceli.json -jobs-dir /path/to/jobs

# With voice support (optional)
./web-server \
  -port 8080 \
  -config .pedroceli.json \
  -whisper-bin /path/to/whisper.cpp/main \
  -whisper-model /path/to/models/ggml-base.en.bin \
  -piper-bin /path/to/piper \
  -piper-model /path/to/models/en_US-lessac-medium.onnx
```

## Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8080` | HTTP server port |
| `-config` | `.pedroceli.json` | Path to configuration file |
| `-jobs-dir` | `/tmp/pedroceli-jobs` | Directory for job storage |
| `-whisper-bin` | `""` | Path to whisper.cpp binary (optional) |
| `-whisper-model` | `""` | Path to whisper model file (optional) |
| `-piper-bin` | `""` | Path to piper TTS binary (optional) |
| `-piper-model` | `""` | Path to piper model file (optional) |

## REST API Endpoints

### GET /api/agents

Returns a list of available agents.

**Response:**
```json
{
  "agents": [
    {
      "name": "builder",
      "description": "Builds new features autonomously"
    },
    {
      "name": "debugger",
      "description": "Debugs and fixes issues"
    },
    {
      "name": "reviewer",
      "description": "Reviews code changes"
    },
    {
      "name": "triager",
      "description": "Triages and diagnoses issues"
    }
  ]
}
```

### GET /api/jobs

Returns a list of all jobs.

**Response:**
```json
{
  "jobs": [
    {
      "id": "job-1234567890",
      "agent": "builder",
      "status": "completed",
      "created_at": "2025-11-10T05:00:00Z",
      "updated_at": "2025-11-10T05:15:00Z",
      "input": {
        "description": "Add rate limiting"
      },
      "result": "Feature implemented successfully"
    }
  ]
}
```

### GET /api/jobs/{jobID}

Returns details for a specific job.

**Response:**
```json
{
  "id": "job-1234567890",
  "agent": "builder",
  "status": "running",
  "created_at": "2025-11-10T05:00:00Z",
  "updated_at": "2025-11-10T05:05:00Z",
  "context_dir": "/tmp/pedroceli-jobs/job-1234567890",
  "input": {
    "description": "Add rate limiting"
  },
  "current_step": 3,
  "total_steps": 10
}
```

**Status codes:**
- `200 OK` - Job found
- `404 Not Found` - Job does not exist

### GET /api/debug/{jobID}

Returns the complete conversation history and debug information for a job. This endpoint is useful for understanding what the agent did, debugging failures, or reviewing the agent's reasoning process.

**Response:**
```json
{
  "job_id": "job-1234567890",
  "context_dir": "/tmp/pedroceli-jobs/job-1234567890",
  "files": [
    {
      "name": "1-prompt.txt",
      "type": "prompt",
      "content": "You are a builder agent. Build the following feature...",
      "step": 1
    },
    {
      "name": "1-tool-calls.json",
      "type": "tool-calls",
      "content": "[{\"tool\": \"search\", \"arguments\": {...}}]",
      "step": 1
    },
    {
      "name": "1-tool-results.json",
      "type": "tool-results",
      "content": "[{\"result\": \"Found 3 files matching pattern\"}]",
      "step": 1
    },
    {
      "name": "1-response.txt",
      "type": "response",
      "content": "I found the relevant files. Next I will...",
      "step": 1
    }
  ]
}
```

**File Types:**
- `prompt` - The prompt sent to the LLM
- `tool-calls` - Tools the agent called (JSON)
- `tool-results` - Results from tool execution (JSON)
- `response` - The agent's response text

**Status codes:**
- `200 OK` - Debug history found
- `404 Not Found` - Job not found or no debug context available
- `500 Internal Server Error` - Failed to read debug files

**Note:** Debug history is only available when debug mode is enabled in the configuration:

```json
{
  "debug": {
    "enabled": true,
    "keep_temp_files": true
  }
}
```

### POST /api/transcribe

Transcribes audio to text using whisper.cpp (if configured).

**Request:**
- Content-Type: `multipart/form-data`
- Field: `audio` (audio file)

**Response:**
```json
{
  "text": "Build a rate limiting feature for the API"
}
```

**Status codes:**
- `200 OK` - Transcription successful
- `400 Bad Request` - Invalid or missing audio file
- `503 Service Unavailable` - STT not configured (start server with `-whisper-bin` and `-whisper-model`)

### POST /api/speak

Converts text to speech using Piper (if configured).

**Request:**
```json
{
  "text": "The build completed successfully"
}
```

**Response:**
- Content-Type: `audio/wav`
- Body: WAV audio data

**Status codes:**
- `200 OK` - Speech synthesis successful
- `400 Bad Request` - Invalid or missing text
- `503 Service Unavailable` - TTS not configured (start server with `-piper-bin` and `-piper-model`)

## WebSocket API

### Endpoint: /ws

The WebSocket endpoint provides real-time bidirectional communication for running agents and monitoring job progress.

### Message Types

#### Client → Server

**Run Agent:**
```json
{
  "type": "run_agent",
  "agent": "builder",
  "input": {
    "description": "Add rate limiting to the API"
  }
}
```

**Get Job Status:**
```json
{
  "type": "get_job_status",
  "job_id": "job-1234567890"
}
```

#### Server → Client

**Job Started:**
```json
{
  "type": "job_started",
  "job": {
    "id": "job-1234567890",
    "agent": "builder",
    "status": "running",
    "created_at": "2025-11-10T05:00:00Z"
  }
}
```

**Job Update:**
```json
{
  "type": "job_update",
  "job": {
    "id": "job-1234567890",
    "status": "running",
    "current_step": 5,
    "total_steps": 10,
    "updated_at": "2025-11-10T05:05:00Z"
  }
}
```

**Job Status:**
```json
{
  "type": "job_status",
  "job": {
    "id": "job-1234567890",
    "status": "completed",
    "result": "Feature implemented successfully"
  }
}
```

**Error:**
```json
{
  "type": "error",
  "error": "Job not found"
}
```

## Example: Using the Debug Viewer

The debug viewer allows you to see the complete conversation history between the agent and the LLM, including all tool calls and results.

### 1. Enable Debug Mode

Edit `.pedroceli.json`:
```json
{
  "debug": {
    "enabled": true,
    "keep_temp_files": true
  }
}
```

### 2. Run a Job

```bash
# Via CLI
./pedrocli build -description "Add health check endpoint"

# Or via Web UI - the job will be created automatically
```

### 3. Access Debug History

```bash
# Get the job ID from the jobs list
curl http://localhost:8080/api/jobs

# Fetch debug history
curl http://localhost:8080/api/debug/job-1234567890
```

The response will contain all prompts, tool calls, tool results, and responses for every step of the agent's execution. This is invaluable for:
- Understanding why an agent made certain decisions
- Debugging failed tasks
- Improving prompts
- Analyzing agent behavior

## Static Files

The server serves static files from the `web/static/` directory at the `/static/` path. The main HTML interface is served at `/`.

## CORS Policy

In development mode, the server accepts WebSocket connections from any origin. For production deployments, you should implement proper CORS restrictions.

## Job Storage

Jobs are persisted to disk in the jobs directory (`/tmp/pedroceli-jobs` by default). Each job gets its own subdirectory containing:
- `job.json` - Job metadata and state
- Conversation history files (if debug mode is enabled):
  - `{step}-prompt.txt` - Prompt sent to LLM
  - `{step}-tool-calls.json` - Tools called by agent
  - `{step}-tool-results.json` - Results from tool execution
  - `{step}-response.txt` - Agent's response

## Security Considerations

- The web server is designed for local development and trusted networks
- No authentication is implemented by default
- For remote access, use a VPN (like Tailscale) or implement authentication
- The bash tool is restricted to safe commands (configurable in `.pedroceli.json`)
- Git operations are sandboxed to the project directory

## Performance Tips

1. **Use Ollama for convenience**: Easier to set up, good performance
2. **Use llama.cpp for maximum speed**: Direct GPU access, lowest latency
3. **Enable GPU layers**: Set `n_gpu_layers: -1` to use all GPU
4. **Monitor job directory**: Large debug histories can consume disk space
5. **Set reasonable limits**: Configure `max_inference_runs` to prevent runaway agents

## Troubleshooting

### Port Already in Use

```bash
# Find and kill process using port 8080
lsof -ti:8080 | xargs kill -9

# Or use a different port
./web-server -port 8081
```

### WebSocket Connection Failed

- Check that the server is running
- Verify no firewall is blocking the connection
- Try accessing from `localhost` instead of an IP address

### Speech-to-Text Not Working

- Verify whisper.cpp is installed and accessible
- Check the model file path is correct
- Ensure you're using a compatible audio format (WAV recommended)
- Check server logs for detailed error messages

### Debug History Not Available

- Ensure `debug.enabled: true` in config
- Verify `debug.keep_temp_files: true` is set
- Check that the jobs directory is writable
- The job might have been cleaned up - run a new job with debug enabled

## Integration Examples

### Using with curl

```bash
# Get agents
curl http://localhost:8080/api/agents

# Get jobs
curl http://localhost:8080/api/jobs

# Get debug history
curl http://localhost:8080/api/debug/job-1234567890 | jq .
```

### Using with JavaScript

```javascript
// Connect to WebSocket
const ws = new WebSocket('ws://localhost:8080/ws');

// Run an agent
ws.send(JSON.stringify({
  type: 'run_agent',
  agent: 'builder',
  input: {
    description: 'Add logging to all API endpoints'
  }
}));

// Handle updates
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  console.log('Received:', msg.type, msg);

  if (msg.type === 'job_started') {
    console.log('Job ID:', msg.job.id);
  }
};
```

### Using with Python

```python
import requests
import json

# Get agents
response = requests.get('http://localhost:8080/api/agents')
agents = response.json()['agents']
print(f"Available agents: {[a['name'] for a in agents]}")

# Get debug history
job_id = 'job-1234567890'
response = requests.get(f'http://localhost:8080/api/debug/{job_id}')
debug_data = response.json()

# Print conversation history
for file in debug_data['files']:
    print(f"\n=== Step {file['step']}: {file['type']} ===")
    print(file['content'][:200] + '...' if len(file['content']) > 200 else file['content'])
```

## See Also

- [Main README](../README.md) - Installation and setup
- [MCP Server Documentation](MCP_SERVER.md) - Using with Claude Desktop
- [Configuration Reference](../README.md#configuration-reference) - Full config options
