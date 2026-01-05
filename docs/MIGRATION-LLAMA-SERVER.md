# Migration Guide: llama-cli → llama-server

This guide helps you migrate from the old llama-cli (one-shot subprocess) backend to the new llama-server (HTTP API) backend.

## Why Migrate?

**llama-server benefits:**
- **Native tool calling** - More reliable tool execution via OpenAI-compatible API
- **Better performance** - Model stays loaded in memory (no reload on each request)
- **Cleaner architecture** - No subprocess overhead, pure HTTP communication

## Migration Steps

### 1. Update llama.cpp Installation

If you're using an old version of llama.cpp, update to the latest:

```bash
cd /path/to/llama.cpp
git pull
make clean
make LLAMA_CUDA=1  # or LLAMA_METAL=1 for Mac
```

Verify `llama-server` exists:
```bash
./llama-server --version
```

### 2. Start llama-server

Instead of PedroCLI invoking `llama-cli` for each request, you now start `llama-server` once and keep it running:

```bash
# Using Makefile (from PedroCLI directory)
make llama-server

# Or manually
llama-server \
  --model /path/to/your-model.gguf \
  --port 8082 \
  --ctx-size 32768 \
  --n-gpu-layers 35 \
  --jinja \
  --log-disable \
  --no-webui \
  --metrics
```

**Port Note:** Default port is 8082 (8081 is reserved for whisper-server)

Check health:
```bash
curl http://localhost:8082/health
# Should return: {"status":"ok"}
```

### 3. Update Your Config File

**Old config (.pedrocli.json):**
```json
{
  "model": {
    "type": "llamacpp",
    "model_path": "/path/to/model.gguf",
    "llamacpp_path": "/usr/local/bin/llama-cli",
    "context_size": 32768,
    "usable_context": 24576,
    "temperature": 0.2,
    "threads": 8,
    "n_gpu_layers": 35,
    "enable_grammar": true,
    "grammar_logging": true
  }
}
```

**New config (.pedrocli.json):**
```json
{
  "model": {
    "type": "llamacpp",
    "server_url": "http://localhost:8082",
    "model_name": "qwen2.5-coder-32b-instruct",
    "context_size": 32768,
    "temperature": 0.2,
    "enable_tools": true
  }
}
```

**Changes:**
- ❌ Remove: `model_path`, `llamacpp_path`, `usable_context`, `threads`, `n_gpu_layers`
- ❌ Remove: `enable_grammar`, `grammar_logging` (replaced by native tool calling)
- ✅ Add: `server_url` (where llama-server is running)
- ✅ Add: `model_name` (arbitrary identifier, can be anything)
- ✅ Add: `enable_tools: true` (enables native tool calling)

### 4. Test the Migration

```bash
# 1. Verify llama-server is running
curl http://localhost:8082/health

# 2. Run a simple build test
./pedrocli build -description "Create a test file called hello.txt with 'Hello, World!'"

# 3. Check job output
ls /tmp/pedrocli-jobs/  # Should see job directories with context files
```

## Configuration Reference

### Required Fields

| Field | Description | Example |
|-------|-------------|---------|
| `type` | Backend type | `"llamacpp"` |
| `server_url` | llama-server endpoint | `"http://localhost:8082"` |
| `model_name` | Model identifier | `"qwen2.5-coder-32b-instruct"` |
| `context_size` | Context window size | `32768` |
| `temperature` | Sampling temperature | `0.2` |
| `enable_tools` | Enable native tool calling | `true` |

### Removed Fields (No Longer Needed)

| Old Field | Why Removed |
|-----------|-------------|
| `model_path` | Model is loaded by llama-server |
| `llamacpp_path` | No longer invoke llama-cli subprocess |
| `usable_context` | Auto-calculated as 75% of `context_size` |
| `threads` | Configured when starting llama-server |
| `n_gpu_layers` | Configured when starting llama-server |
| `enable_grammar` | Replaced by `enable_tools` (native API) |
| `grammar_logging` | Not applicable with native tool calling |

## Starting llama-server Automatically

### Option 1: Makefile (Recommended)

Add to your shell startup (`~/.bashrc`, `~/.zshrc`):

```bash
# Start llama-server in the background (from PedroCLI directory)
alias start-pedro='cd /path/to/pedrocli && make llama-server &'
alias stop-pedro='make stop-llama'
```

### Option 2: systemd Service (Linux)

Create `/etc/systemd/system/llama-server.service`:

```ini
[Unit]
Description=llama-server for PedroCLI
After=network.target

[Service]
Type=simple
User=youruser
ExecStart=/usr/local/bin/llama-server \
  --model /path/to/model.gguf \
  --port 8082 \
  --ctx-size 32768 \
  --n-gpu-layers 35 \
  --jinja \
  --log-disable \
  --no-webui
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable llama-server
sudo systemctl start llama-server
sudo systemctl status llama-server
```

### Option 3: launchd (macOS)

Create `~/Library/LaunchAgents/com.pedrocli.llama-server.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.pedrocli.llama-server</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/llama-server</string>
        <string>--model</string>
        <string>/path/to/model.gguf</string>
        <string>--port</string>
        <string>8082</string>
        <string>--ctx-size</string>
        <string>32768</string>
        <string>--n-gpu-layers</string>
        <string>35</string>
        <string>--jinja</string>
        <string>--log-disable</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
```

Load and start:
```bash
launchctl load ~/Library/LaunchAgents/com.pedrocli.llama-server.plist
launchctl start com.pedrocli.llama-server
```

## Troubleshooting

### llama-server won't start

**Error: Port 8082 already in use**
```bash
# Check what's using the port
lsof -i :8082

# Kill it or use a different port
LLAMA_PORT=9090 make llama-server
```

**Error: Model not found**
```bash
# Verify model path
ls -lh /path/to/model.gguf

# Or let Makefile auto-detect
make llama-server  # Uses LLAMA_MODEL variable
```

### PedroCLI can't connect

**Error: "llama-server not reachable"**
```bash
# 1. Check server is running
curl http://localhost:8082/health

# 2. Check config has correct URL
grep server_url .pedrocli.json

# 3. Check firewall (if running remotely)
curl http://remote-host:8082/health
```

### Tool calls not working

**Symptoms: Agent doesn't call tools or gets stuck**

1. **Verify `enable_tools: true` in config**
   ```bash
   grep enable_tools .pedrocli.json
   ```

2. **Check llama-server was started with `--jinja` flag**
   ```bash
   ps aux | grep llama-server | grep jinja
   ```

3. **Verify model supports tool calling**
   - Qwen 2.5 Coder series: ✅ Yes
   - Llama 3.x series: ✅ Yes
   - Mistral series: ✅ Yes
   - Older models: ❌ May not support

### Performance issues

**Slow first request**
- This is normal - model loads into VRAM on first request
- Subsequent requests should be fast

**Slow every request**
- Check GPU layers: `ps aux | grep llama-server | grep n-gpu-layers`
- Increase if too low (e.g., `--n-gpu-layers 35` for 24GB VRAM)
- Monitor VRAM usage: `nvidia-smi` (NVIDIA) or `Activity Monitor` (Mac)

## Rollback Plan

If you need to rollback to llama-cli (not recommended):

1. **Stop llama-server:**
   ```bash
   make stop-llama
   ```

2. **Revert config:**
   ```bash
   git checkout .pedrocli.json  # If you committed old config
   ```

3. **Note:** llama-cli support is deprecated and will be removed in a future release

## Getting Help

- **Issue tracker:** https://github.com/Soypete/PedroCLI/issues
- **Documentation:** https://github.com/Soypete/PedroCLI/blob/main/docs/
- **Testing guide:** `docs/TESTING-LLAMA-SERVER.md`

## Version Compatibility

| PedroCLI Version | llama-server | Notes |
|------------------|--------------|-------|
| v0.3.0+ | Required | Native tool calling |
| v0.2.x | Optional | Falls back to llama-cli |
| v0.1.x | Not supported | llama-cli only |
