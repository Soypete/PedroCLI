# Web UI Deployment Guide

PedroCLI includes a browser-based web UI for interacting with autonomous coding agents. This guide covers setup, configuration, and deployment.

## Overview

The web UI provides:
- **Agent selector**: Choose between Builder, Debugger, Reviewer, and Triager agents
- **Text & voice input**: Type requests or speak them using your microphone
- **Real-time progress**: Watch agents work with live updates
- **Job management**: View recent jobs and their results
- **Self-hosted STT/TTS**: Optional voice features using whisper.cpp and Piper

## Quick Start

### 1. Build the Web Server

```bash
# Build from source
cd PedroCLI
make build

# This creates two binaries:
# - pedrocli (CLI client)
# - pedrocli-server (MCP server)
# - web-server (Web UI server) - built separately
go build -o web-server ./cmd/web-server
```

### 2. Configure PedroCLI

Create `.pedroceli.json` in your project directory:

```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b",
    "ollama_url": "http://localhost:11434",
    "temperature": 0.2
  },
  "project": {
    "name": "My Project",
    "workdir": "/path/to/your/project",
    "tech_stack": ["Go", "Python"]
  },
  "tools": {
    "allowed_bash_commands": ["go", "git", "cat", "ls", "head", "tail"],
    "forbidden_commands": ["rm", "mv", "dd", "sudo"]
  }
}
```

### 3. Start Ollama (or llama.cpp)

```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Pull a model
ollama pull qwen2.5-coder:32b

# Ollama runs automatically
```

### 4. Run the Web Server

```bash
# Basic usage (text-only mode)
./web-server -port 8080 -config .pedroceli.json

# Open browser
open http://localhost:8080
```

The web UI is now running! You can select an agent and start building.

## Voice Input Setup (Optional)

To enable voice input, you need to set up whisper.cpp for speech-to-text.

### Install whisper.cpp

```bash
# Clone and build whisper.cpp
git clone https://github.com/ggerganov/whisper.cpp
cd whisper.cpp

# Build (with GPU support)
make WHISPER_CUDA=1  # For NVIDIA GPU
# or
make WHISPER_METAL=1  # For Apple Silicon

# Download a model
./models/download-ggml-model.sh base.en
# This downloads models/ggml-base.en.bin
```

### Run Web Server with Voice Support

```bash
./web-server \
  -port 8080 \
  -config .pedroceli.json \
  -whisper-bin /path/to/whisper.cpp/main \
  -whisper-model /path/to/whisper.cpp/models/ggml-base.en.bin
```

Now the voice input mode will be fully functional!

### Recommended Whisper Models

- **tiny.en** (75 MB): Fast, less accurate, good for testing
- **base.en** (142 MB): Good balance, **recommended for most users**
- **small.en** (466 MB): Higher accuracy, slower
- **medium.en** (1.5 GB): Very accurate, requires good hardware
- **large-v3** (3.1 GB): Best accuracy, GPU recommended

## Voice Output Setup (Optional)

To enable text-to-speech for agent responses, set up Piper.

### Install Piper

```bash
# Download Piper binary
curl -LO https://github.com/rhasspy/piper/releases/download/v1.2.0/piper_amd64.tar.gz
tar -xzf piper_amd64.tar.gz

# Download a voice model
curl -LO https://github.com/rhasspy/piper/releases/download/v1.2.0/en_US-lessac-medium.onnx
curl -LO https://github.com/rhasspy/piper/releases/download/v1.2.0/en_US-lessac-medium.onnx.json
```

### Run Web Server with TTS Support

```bash
./web-server \
  -port 8080 \
  -config .pedroceli.json \
  -whisper-bin /path/to/whisper.cpp/main \
  -whisper-model /path/to/whisper.cpp/models/ggml-base.en.bin \
  -piper-bin /path/to/piper/piper \
  -piper-model /path/to/piper/en_US-lessac-medium.onnx
```

### Recommended Piper Voices

- **en_US-lessac-medium**: Natural US English, **recommended**
- **en_US-libritts-high**: High quality, slower
- **en_GB-alan-medium**: British English
- **es_ES-sharvard-medium**: Spanish
- **fr_FR-siwis-medium**: French
- **de_DE-thorsten-medium**: German

See full list: https://github.com/rhasspy/piper/releases

## Docker Deployment

### Build Docker Image

```bash
# Build the image
docker build -t pedrocli-web .

# Or use pre-built image
docker pull ghcr.io/soypete/pedrocli:latest
```

### Run with Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  ollama:
    image: ollama/ollama:latest
    ports:
      - "11434:11434"
    volumes:
      - ollama-data:/root/.ollama
    # Uncomment for GPU support
    # deploy:
    #   resources:
    #     reservations:
    #       devices:
    #         - driver: nvidia
    #           count: 1
    #           capabilities: [gpu]

  pedrocli-web:
    image: ghcr.io/soypete/pedrocli:latest
    command: web-server -port 8080 -config /config/.pedroceli.json
    ports:
      - "8080:8080"
    volumes:
      - ./:/workspace
      - ./.pedroceli.json:/config/.pedroceli.json:ro
    environment:
      OLLAMA_URL: http://ollama:11434
    depends_on:
      - ollama

volumes:
  ollama-data:
```

Start the stack:

```bash
docker-compose up -d
```

Access at http://localhost:8080

## Production Deployment

### Using systemd (Linux)

Create `/etc/systemd/system/pedrocli-web.service`:

```ini
[Unit]
Description=PedroCLI Web UI
After=network.target

[Service]
Type=simple
User=pedrocli
WorkingDirectory=/opt/pedrocli
ExecStart=/usr/local/bin/web-server \
  -port 8080 \
  -config /opt/pedrocli/.pedroceli.json \
  -whisper-bin /usr/local/bin/whisper-cpp \
  -whisper-model /opt/pedrocli/models/ggml-base.en.bin
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable pedrocli-web
sudo systemctl start pedrocli-web
sudo systemctl status pedrocli-web
```

### Reverse Proxy with Caddy

`Caddyfile`:

```
pedrocli.yourdomain.com {
    reverse_proxy localhost:8080

    # WebSocket support
    @websocket {
        header Connection *Upgrade*
        header Upgrade websocket
    }
    reverse_proxy @websocket localhost:8080
}
```

Start Caddy:

```bash
caddy run --config Caddyfile
```

Caddy automatically handles HTTPS with Let's Encrypt.

### Reverse Proxy with nginx

`/etc/nginx/sites-available/pedrocli`:

```nginx
server {
    listen 80;
    server_name pedrocli.yourdomain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Enable and reload:

```bash
sudo ln -s /etc/nginx/sites-available/pedrocli /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

Add SSL with certbot:

```bash
sudo certbot --nginx -d pedrocli.yourdomain.com
```

## Kubernetes Deployment

Create `k8s/deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pedrocli-web
spec:
  replicas: 1
  selector:
    matchLabels:
      app: pedrocli-web
  template:
    metadata:
      labels:
        app: pedrocli-web
    spec:
      containers:
      - name: web
        image: ghcr.io/soypete/pedrocli:latest
        command: ["web-server"]
        args:
          - "-port=8080"
          - "-config=/config/.pedroceli.json"
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: config
          mountPath: /config
        - name: workspace
          mountPath: /workspace
      volumes:
      - name: config
        configMap:
          name: pedrocli-config
      - name: workspace
        persistentVolumeClaim:
          claimName: pedrocli-workspace
---
apiVersion: v1
kind: Service
metadata:
  name: pedrocli-web
spec:
  selector:
    app: pedrocli-web
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

Apply:

```bash
kubectl apply -f k8s/deployment.yaml
```

## Configuration Options

### Command-Line Flags

```bash
web-server [options]

Options:
  -port string
        HTTP server port (default "8080")
  -config string
        Config file path (default ".pedroceli.json")
  -whisper-bin string
        Path to whisper.cpp binary (optional)
  -whisper-model string
        Path to whisper model file (optional)
  -piper-bin string
        Path to piper binary (optional)
  -piper-model string
        Path to piper model file (optional)
```

### Environment Variables

You can also use environment variables:

```bash
export PEDROCLI_PORT=8080
export PEDROCLI_CONFIG=/path/to/.pedroceli.json
export WHISPER_BIN=/path/to/whisper.cpp/main
export WHISPER_MODEL=/path/to/models/ggml-base.en.bin
export PIPER_BIN=/path/to/piper/piper
export PIPER_MODEL=/path/to/models/en_US-lessac-medium.onnx

./web-server
```

## Security Considerations

### Authentication

The web server does NOT include authentication by default. For production:

1. **Use a reverse proxy** with authentication (Caddy, nginx, Traefik)
2. **Restrict network access** (firewall, VPN)
3. **Use HTTPS** for all connections

Example with Caddy basic auth:

```
pedrocli.yourdomain.com {
    basicauth {
        user $2a$14$...hashed_password...
    }
    reverse_proxy localhost:8080
}
```

### CORS and WebSocket

The web server allows all origins by default (for development). In production, configure the reverse proxy to restrict origins:

```nginx
# nginx example
add_header Access-Control-Allow-Origin "https://yourdomain.com";
```

### File Access

The web server has full file access within the configured `workdir`. Be careful:

- Don't expose sensitive directories
- Use project-specific configs
- Review agent changes before committing

## Monitoring

### Health Checks

```bash
# Check if server is running
curl http://localhost:8080/

# Check agents are available
curl http://localhost:8080/api/agents

# Check job status
curl http://localhost:8080/api/jobs
```

### Logs

The web server logs to stdout. In production:

```bash
# systemd
journalctl -u pedrocli-web -f

# Docker
docker logs -f pedrocli-web

# Kubernetes
kubectl logs -f deployment/pedrocli-web
```

### Resource Usage

Monitor:
- **CPU**: Depends on LLM backend (Ollama/llama.cpp)
- **Memory**: 8GB+ recommended for 32B models
- **Disk**: Workspace files + model files (1-5GB for models)
- **Network**: WebSocket connections (minimal)

## Troubleshooting

### "WebSocket connection failed"

Check that your reverse proxy supports WebSocket upgrades. Both Caddy and nginx examples above include WebSocket support.

### "Speech-to-text not available"

Make sure you're passing the `-whisper-bin` and `-whisper-model` flags and that the files exist:

```bash
ls -la /path/to/whisper.cpp/main
ls -la /path/to/models/ggml-base.en.bin
```

### "Job stays in running state"

The agent might be stuck. Check:

1. **LLM backend is running**: `ollama list` or check llama.cpp process
2. **Increase max iterations** in config: `"limits": {"max_inference_runs": 30}`
3. **Check job logs**: Look in `/tmp/pedroceli-jobs/` directory

### "Port already in use"

Another process is using port 8080:

```bash
# Find what's using the port
lsof -i :8080

# Use a different port
./web-server -port 8081
```

## Performance Optimization

### Model Selection

- **Development**: Use smaller models (7B-14B) for faster iteration
- **Production**: Use larger models (32B+) for better results

### Caching

Enable LLM response caching in your backend (Ollama has built-in caching).

### Horizontal Scaling

Run multiple web server instances behind a load balancer:

```yaml
# docker-compose.yml
services:
  pedrocli-web-1:
    image: ghcr.io/soypete/pedrocli:latest
    command: web-server
    ports: ["8081:8080"]

  pedrocli-web-2:
    image: ghcr.io/soypete/pedrocli:latest
    command: web-server
    ports: ["8082:8080"]

  load-balancer:
    image: nginx:latest
    ports: ["80:80"]
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
```

## Examples

### Complete Production Setup

```bash
#!/bin/bash
# setup.sh - Production deployment script

set -e

# Install dependencies
sudo apt-get update
sudo apt-get install -y curl git build-essential

# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh
ollama pull qwen2.5-coder:32b

# Install PedroCLI
curl -fsSL https://raw.githubusercontent.com/Soypete/PedroCLI/main/install.sh | sh

# Build web server
cd PedroCLI
go build -o /usr/local/bin/web-server ./cmd/web-server

# Install whisper.cpp
git clone https://github.com/ggerganov/whisper.cpp
cd whisper.cpp && make && cd ..
./whisper.cpp/models/download-ggml-model.sh base.en

# Install systemd service
sudo cp pedrocli-web.service /etc/systemd/system/
sudo systemctl enable pedrocli-web
sudo systemctl start pedrocli-web

# Install Caddy
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/caddy-stable-archive-keyring.gpg] https://dl.cloudsmith.io/public/caddy/stable/deb/debian any-version main" | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update && sudo apt install caddy

# Configure Caddy
cat > /etc/caddy/Caddyfile <<EOF
pedrocli.yourdomain.com {
    reverse_proxy localhost:8080
}
EOF

sudo systemctl restart caddy

echo "✅ PedroCLI Web UI deployed!"
echo "Access at: https://pedrocli.yourdomain.com"
```

## Further Reading

- [PedroCLI Main Documentation](../README.md)
- [MCP Server Integration](./MCP_SERVER.md)
- [Whisper.cpp Documentation](https://github.com/ggerganov/whisper.cpp)
- [Piper TTS Documentation](https://github.com/rhasspy/piper)
- [Model Context Protocol](https://modelcontextprotocol.io/)
