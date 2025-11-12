# MCP Server Integration

PedroCLI includes a Model Context Protocol (MCP) server that can be integrated with various MCP clients like Claude Desktop, Cline, and other compatible tools.

## What is the MCP Server?

The MCP server (`pedrocli-server`) exposes all of PedroCLI's tools through a standard JSON-RPC 2.0 interface over stdio. This allows any MCP-compatible client to use PedroCLI's capabilities:

- **File operations**: Read, write, edit files
- **Code editing**: Precise line-based code modifications
- **Search**: Grep, find files, find definitions
- **Navigate**: Code structure, outlines, imports
- **Git operations**: Branch, commit, PR management
- **Bash commands**: Controlled command execution
- **Testing**: Run tests and analyze failures

## Running the MCP Server

### Standalone Mode

```bash
# Run the MCP server directly
./pedrocli-server

# The server communicates via stdio (standard input/output)
# It's designed to be launched by MCP clients, not run directly
```

### Configuration

The MCP server reads configuration from `~/.pedroceli.json` or `.pedroceli.json` in the current directory.

Example configuration:

```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b",
    "ollama_url": "http://localhost:11434"
  },
  "project": {
    "name": "My Project",
    "workdir": "/path/to/your/project",
    "tech_stack": ["Go", "Python"]
  },
  "tools": {
    "allowed_bash_commands": ["go", "git", "cat", "ls"],
    "forbidden_commands": ["rm", "mv", "dd", "sudo"]
  }
}
```

## Integration with Claude Desktop

Claude Desktop supports MCP servers for extending Claude's capabilities.

### 1. Install PedroCLI

```bash
# Using install script
curl -fsSL https://raw.githubusercontent.com/Soypete/PedroCLI/main/install.sh | sh

# Or via Homebrew
brew install Soypete/tap/pedrocli
```

### 2. Configure Claude Desktop

Edit your Claude Desktop MCP configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`

**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

**Linux**: `~/.config/Claude/claude_desktop_config.json`

Add the PedroCLI server:

```json
{
  "mcpServers": {
    "pedrocli": {
      "command": "/path/to/pedrocli-server",
      "args": [],
      "env": {
        "PEDROCLI_CONFIG": "/path/to/your/.pedroceli.json"
      }
    }
  }
}
```

### 3. Restart Claude Desktop

Restart Claude Desktop to load the MCP server. You should now see PedroCLI tools available in Claude.

### 4. Using PedroCLI Tools in Claude

Once configured, you can ask Claude to use PedroCLI tools:

```
"Can you search for all TODO comments in the codebase?"
"Read the main.go file and explain how it works"
"Create a new feature that adds rate limiting to the API"
"Run the tests and fix any failures"
```

Claude will automatically use the appropriate PedroCLI tools to complete these tasks.

## Integration with Cline (VS Code Extension)

Cline is a VS Code extension that supports MCP servers.

### 1. Install Cline

Install the Cline extension from the VS Code marketplace.

### 2. Configure Cline for PedroCLI

Open Cline settings in VS Code and add the MCP server configuration:

```json
{
  "cline.mcpServers": {
    "pedrocli": {
      "command": "/path/to/pedrocli-server",
      "args": [],
      "env": {
        "PEDROCLI_CONFIG": "${workspaceFolder}/.pedroceli.json"
      }
    }
  }
}
```

### 3. Use PedroCLI in Cline

Cline will now have access to all PedroCLI tools. You can ask Cline to:

- Search codebases
- Edit files precisely
- Run tests
- Create git branches
- Execute controlled bash commands

## Integration with Custom MCP Clients

If you're building your own MCP client, you can integrate PedroCLI using the MCP protocol.

### Protocol Details

- **Transport**: stdio (standard input/output)
- **Protocol**: JSON-RPC 2.0
- **Message Format**: Newline-delimited JSON

### Example: Calling a Tool

Request:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "file_read",
    "arguments": {
      "path": "/path/to/file.go"
    }
  }
}
```

Response:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "package main\n\nfunc main() {\n  ...\n}"
      }
    ],
    "isError": false
  }
}
```

### Available Tools

Query available tools:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list"
}
```

This returns all available tools with their schemas and descriptions.

## Available Tools Reference

### file_read
Read a file from the filesystem.

**Arguments:**
- `path` (string): File path to read

### file_write
Write content to a file.

**Arguments:**
- `path` (string): File path to write
- `content` (string): Content to write

### code_edit
Edit specific lines in a file.

**Arguments:**
- `file_path` (string): Path to file
- `start_line` (int): Starting line number
- `end_line` (int): Ending line number
- `new_content` (string): Replacement content

### search_grep
Search files using grep patterns.

**Arguments:**
- `pattern` (string): Search pattern
- `path` (string): Optional directory path

### search_files
Find files by name pattern.

**Arguments:**
- `pattern` (string): Filename pattern
- `path` (string): Optional directory path

### navigate_outline
Get code structure outline.

**Arguments:**
- `file_path` (string): File to analyze

### git_branch
Create or switch git branches.

**Arguments:**
- `branch_name` (string): Branch name
- `create` (bool): Whether to create new branch

### git_commit
Commit changes with a message.

**Arguments:**
- `message` (string): Commit message

### bash_run
Execute allowed bash commands.

**Arguments:**
- `command` (string): Command to run
- `args` (array): Command arguments

### test_run
Run tests in a directory.

**Arguments:**
- `path` (string): Test directory

## Troubleshooting

### "MCP server not found"

Ensure the `pedrocli-server` binary is in your PATH or provide the full path in the configuration.

```bash
# Find the binary
which pedrocli-server

# Or use full path in config
/usr/local/bin/pedrocli-server
```

### "Permission denied"

Make sure the binary is executable:

```bash
chmod +x /path/to/pedrocli-server
```

### "Configuration not found"

The MCP server looks for `.pedroceli.json` in:
1. Current directory
2. Home directory (`~/.pedroceli.json`)
3. Environment variable: `PEDROCLI_CONFIG`

Set the environment variable in your MCP client config:

```json
{
  "env": {
    "PEDROCLI_CONFIG": "/path/to/.pedroceli.json"
  }
}
```

### "Tools not working"

Check the logs. The MCP server writes errors to stderr, which should appear in your MCP client's logs.

For Claude Desktop:
- macOS: `~/Library/Logs/Claude/mcp*.log`
- Windows: `%APPDATA%\Claude\logs\mcp*.log`
- Linux: `~/.config/Claude/logs/mcp*.log`

## Security Considerations

### Bash Command Restrictions

The MCP server only allows whitelisted bash commands. Configure this in `.pedroceli.json`:

```json
{
  "tools": {
    "allowed_bash_commands": [
      "go", "git", "cat", "ls", "head", "tail"
    ],
    "forbidden_commands": [
      "rm", "mv", "dd", "sudo", "chmod"
    ]
  }
}
```

### File Access

The MCP server has access to files within the configured `workdir` and can read/write files. Be cautious when:

- Running on sensitive directories
- Using with untrusted LLMs
- Exposing over network (stdio only, but container escapes possible)

### Recommendations

1. **Use project-specific configs**: Create `.pedroceli.json` per project
2. **Limit bash commands**: Only allow necessary commands
3. **Review changes**: Always review git diffs before committing
4. **Backup important files**: Use version control

## Advanced Usage

### Custom Tool Filtering

You can limit which tools are exposed to the MCP client by modifying the server code or using environment variables:

```bash
export PEDROCLI_TOOLS="file_read,file_write,search_grep"
./pedrocli-server
```

### Running in Docker

```bash
# Build image
docker build -t pedrocli .

# Run MCP server in container
docker run -i \
  -v /path/to/project:/workspace \
  -v ~/.pedroceli.json:/root/.pedroceli.json \
  pedrocli pedrocli-server
```

### Multiple Projects

Run separate MCP server instances for different projects:

```json
{
  "mcpServers": {
    "pedrocli-project-a": {
      "command": "/usr/local/bin/pedrocli-server",
      "env": {
        "PEDROCLI_CONFIG": "/projects/project-a/.pedroceli.json"
      }
    },
    "pedrocli-project-b": {
      "command": "/usr/local/bin/pedrocli-server",
      "env": {
        "PEDROCLI_CONFIG": "/projects/project-b/.pedroceli.json"
      }
    }
  }
}
```

## Further Reading

- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
- [Claude Desktop MCP Documentation](https://claude.ai/docs/mcp)
- [PedroCLI Main Documentation](../README.md)
- [Building Custom MCP Clients](https://github.com/modelcontextprotocol/specification)
