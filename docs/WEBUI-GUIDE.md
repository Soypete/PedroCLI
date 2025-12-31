# PedroCLI Web UI Guide

A quick-start guide for using the PedroCLI web interface.

## Starting the Server

```bash
# Build (if needed)
make build

# Start the HTTP server
./pedrocli-http-server
```

You should see:
```
🚀 PedroCLI HTTP Server v0.2.0-dev
📡 Listening on http://0.0.0.0:8080
🔧 MCP Server: Running
```

Open **http://localhost:8080** in your browser.

## Using the Web UI

### Coding Tools

1. Click **Coding Tools** button (default)
2. Select job type:
   - **Builder** - Build new features from descriptions
   - **Debugger** - Fix issues (provide symptoms + logs)
   - **Reviewer** - Review code on a branch
   - **Triager** - Diagnose issues without fixing
3. Fill in the fields
4. Click **Create Job**

### Podcast Tools

1. Click **Podcast Tools** button
2. Select job type:
   - **Create Script** - Generate episode script & outline
   - **Add Link** - Add article/news to Notion for review
   - **Add Guest** - Add guest info to Notion database
   - **Review News** - Summarize news for episode prep
3. Fill in the fields
4. Click **Create Job**

**Note**: Podcast tools require Notion integration. See [Podcast HOWTO](podcast/HOWTO.md).

## Voice Dictation

The web UI supports voice-to-text for filling in text fields using whisper.cpp.

### Setup whisper.cpp

1. **Install whisper.cpp**:
   ```bash
   git clone https://github.com/ggerganov/whisper.cpp
   cd whisper.cpp
   make
   ```

2. **Download a model**:
   ```bash
   # base.en is recommended for English (good balance of speed/accuracy)
   bash ./models/download-ggml-model.sh base.en
   ```

3. **Start the whisper.cpp server**:
   ```bash
   ./server -m models/ggml-base.en.bin --port 9090 --host 0.0.0.0
   ```

### Enable Voice in PedroCLI

Add to your `.pedrocli.json`:

```json
{
  "voice": {
    "enabled": true,
    "whisper_url": "http://localhost:9090",
    "language": "auto"
  }
}
```

### Using Voice

1. Click the **Voice** button next to any text field
2. Allow microphone access when prompted
3. Speak your text
4. Click **Stop** - the text will be transcribed and filled in

**Model Options**:
- `tiny` - Fastest, less accurate (~50MB)
- `base` - Balanced (~150MB) **recommended**
- `small` - Better accuracy (~500MB)
- `medium`/`large` - Best accuracy, slower (>1GB)

## Configuration Reference

Full `.pedrocli.json` example:

```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b",
    "temperature": 0.2
  },
  "project": {
    "name": "MyProject",
    "workdir": "/path/to/project"
  },
  "web": {
    "enabled": true,
    "port": 8080,
    "host": "0.0.0.0"
  },
  "voice": {
    "enabled": true,
    "whisper_url": "http://localhost:9090",
    "language": "auto"
  },
  "podcast": {
    "enabled": true,
    "notion": {
      "enabled": true,
      "databases": {
        "scripts": "YOUR_EPISODE_DB_ID",
        "articles_review": "YOUR_ARTICLES_DB_ID",
        "guests": "YOUR_GUESTS_DB_ID"
      }
    }
  }
}
```

## Quick Start Checklist

### Coding Jobs Only
- [ ] Run `./pedrocli-http-server`
- [ ] Open http://localhost:8080
- [ ] Have Ollama running with your model

### Podcast Jobs
- [ ] Configure Notion databases in `.pedrocli.json`
- [ ] Store Notion token (see [Podcast HOWTO](podcast/HOWTO.md))
- [ ] Run `./pedrocli-http-server`
- [ ] Click **Podcast Tools** in the web UI

### Voice Dictation
- [ ] Install and build whisper.cpp
- [ ] Download a model (base.en recommended)
- [ ] Start whisper.cpp server on port 9090
- [ ] Add `voice` config to `.pedrocli.json`
- [ ] Restart `./pedrocli-http-server`
- [ ] Click **Voice** button to dictate

## Troubleshooting

### "Voice transcription is not enabled"
1. Check whisper.cpp is running: `curl http://localhost:9090/health`
2. Verify `voice.enabled: true` in config
3. Restart the HTTP server

### Jobs not appearing
1. Check MCP server started (look for "🔧 MCP Server: Running")
2. Click **Refresh** in Active Jobs
3. Check browser console for errors

### Notion errors
1. Verify token is stored correctly
2. Check database is shared with your integration
3. See [Podcast HOWTO](podcast/HOWTO.md) for detailed setup

## Remote Access (Tailscale)

The server binds to `0.0.0.0:8080` by default, making it accessible via Tailscale:

```
http://<your-tailscale-ip>:8080
```

This allows you to use the web UI from your phone or other devices on your Tailscale network.
