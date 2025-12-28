# Google Calendar MCP Server

A Model Context Protocol (MCP) server for Google Calendar integration with PedroCLI.

## Setup

### 1. Create Google Cloud Project

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the Google Calendar API:
   - Go to "APIs & Services" > "Library"
   - Search for "Google Calendar API"
   - Click "Enable"

### 2. Create OAuth 2.0 Credentials

1. Go to "APIs & Services" > "Credentials"
2. Click "Create Credentials" > "OAuth client ID"
3. Choose "Desktop app" as the application type
4. Download the credentials JSON file
5. Save it to a secure location (e.g., `~/.config/pedrocli/google-calendar-credentials.json`)

### 3. Configure PedroCLI

Add to your `.pedrocli.json`:

```json
{
  "podcast": {
    "enabled": true,
    "calendar": {
      "enabled": true,
      "credentials_path": "/Users/yourusername/.config/pedrocli/google-calendar-credentials.json",
      "calendar_id": "primary"
    }
  }
}
```

Or set environment variables:

```bash
export GOOGLE_CALENDAR_CREDENTIALS=/path/to/credentials.json
export GOOGLE_CALENDAR_ID=primary  # or specific calendar ID
```

### 4. First Run - OAuth Authorization

The first time you run the server, it will:
1. Print an authorization URL to stderr
2. Ask you to visit the URL in your browser
3. Request you to authorize the application
4. Ask you to paste the authorization code

This will create a `token.json` file in the current directory for future use.

## Usage

The calendar MCP server is automatically started by PedroCLI when you use calendar tools in podcast mode.

### Available Tools

**list_events** - List upcoming calendar events
```json
{
  "tool": "calendar",
  "args": {
    "action": "list_events",
    "time_min": "2024-01-01T00:00:00Z",
    "time_max": "2024-01-31T23:59:59Z",
    "max_results": 10
  }
}
```

**create_event** - Create a new calendar event
```json
{
  "tool": "calendar",
  "args": {
    "action": "create_event",
    "summary": "[Recording] Episode 42",
    "start_time": "2024-01-15T14:00:00-08:00",
    "end_time": "2024-01-15T15:30:00-08:00",
    "description": "Recording session for episode 42",
    "location": "Riverside Studio"
  }
}
```

**update_event** - Update an existing event
```json
{
  "tool": "calendar",
  "args": {
    "action": "update_event",
    "event_id": "abc123xyz",
    "summary": "[Recording] Episode 42 - UPDATED",
    "description": "Updated description"
  }
}
```

**delete_event** - Delete a calendar event
```json
{
  "tool": "calendar",
  "args": {
    "action": "delete_event",
    "event_id": "abc123xyz"
  }
}
```

## Manual Testing

You can test the MCP server directly:

```bash
# Build the server
make build-calendar

# Run with credentials path
./pedrocli-calendar-mcp ~/.config/pedrocli/google-calendar-credentials.json
```

Then send JSON-RPC requests via stdin:

```bash
# List tools
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./pedrocli-calendar-mcp creds.json

# List events
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_events","arguments":{}}}' | ./pedrocli-calendar-mcp creds.json
```

## Security Notes

- **Never commit credentials**: Add `google-calendar-credentials.json` and `token.json` to `.gitignore`
- **Token storage**: The OAuth token is stored in `token.json` in the current directory
- **Scope**: This server requests full calendar access (`calendar.CalendarScope`)

## Troubleshooting

**"Unable to read credentials file"**
- Check that the credentials path is correct
- Ensure the JSON file is valid OAuth credentials from Google Cloud Console

**"Unable to retrieve token from web"**
- Make sure you're using the correct authorization code
- Check that the OAuth client is configured for "Desktop app"
- Verify the redirect URI is correct

**"Access denied"**
- Re-authorize by deleting `token.json` and running again
- Check that the Google Calendar API is enabled in your project
