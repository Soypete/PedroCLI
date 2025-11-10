# Configuration Files

This directory contains example configuration files for PedroCLI. You can create multiple configuration files here and switch between them using the web UI.

## Using Multiple Configs

Start the web server with the `-configs-dir` flag:

```bash
web-server -port 8080 -configs-dir ./configs
```

The web UI will display a dropdown in the header allowing you to browse available configurations. The dropdown shows:
- Config filename
- Backend type (ollama, anthropic, llamacpp)
- Model name
- Project name
- Debug status

## Example Configs

### local-ollama-small.json
- **Purpose**: Local development with smaller Ollama model
- **Model**: qwen2.5-coder:7b
- **Debug**: Disabled (files cleaned up after jobs)

### debug-mode.json
- **Purpose**: Debugging agent behavior with full conversation logs
- **Model**: qwen2.5-coder:7b
- **Debug**: Enabled (keeps all prompts, responses, and tool calls in `/tmp/pedroceli-jobs/`)

## Creating Custom Configs

1. Copy one of the example files
2. Modify the settings as needed
3. Save with a descriptive name (e.g., `production-anthropic.json`)
4. Restart the web server
5. Select your config from the dropdown

## Config Structure

```json
{
  "model": {
    "type": "ollama|anthropic|llamacpp",
    "model_name": "model-name",
    "temperature": 0.2
  },
  "project": {
    "name": "ProjectName",
    "workdir": "/path/to/project",
    "tech_stack": ["Go", "Python"]
  },
  "tools": {
    "allowed_bash_commands": ["go", "git", "cat"],
    "forbidden_commands": ["rm", "mv", "sudo"]
  },
  "limits": {
    "max_task_duration_minutes": 30,
    "max_inference_runs": 20
  },
  "debug": {
    "enabled": false,
    "keep_temp_files": false,
    "log_level": "info"
  }
}
```

## Notes

- Changing configs requires a server restart
- The web UI shows restart instructions when you select a different config
- Debug mode is useful for understanding agent reasoning and tool usage
- All inference rounds are saved to `/tmp/pedroceli-jobs/{job-id}/` when debug is enabled
