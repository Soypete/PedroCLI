# PedroCLI Scripts

Utility scripts for PedroCLI setup and management.

## setup-podcast-tokens.go

Helper script to store OAuth tokens and API keys in the PedroCLI database for podcast tools.

### Usage

#### Store Notion API Key

```bash
go run scripts/setup-podcast-tokens.go \
  -provider notion \
  -service database \
  -api-key "secret_YOUR_NOTION_API_KEY"
```

#### Store Google Calendar OAuth Token

```bash
go run scripts/setup-podcast-tokens.go \
  -provider google \
  -service calendar \
  -access-token "ya29...." \
  -refresh-token "1//..." \
  -expires-in 3600 \
  -scope "https://www.googleapis.com/auth/calendar"
```

### Parameters

| Flag | Required | Description |
|------|----------|-------------|
| `-provider` | Yes | Token provider: `notion` or `google` |
| `-service` | Yes | Service name: `database`, `calendar` |
| `-api-key` | Notion only | Notion API key (format: `secret_...`) |
| `-access-token` | Google only | OAuth access token |
| `-refresh-token` | Google only | OAuth refresh token (optional but recommended) |
| `-expires-in` | Google only | Token expiration in seconds (e.g., 3600 for 1 hour) |
| `-scope` | Google only | OAuth scope (e.g., `https://www.googleapis.com/auth/calendar`) |

### Prerequisites

1. **Database configured**: `.pedrocli.json` must have `repo_storage.database_path` set
2. **Notion API key**: Get from https://www.notion.so/my-integrations
3. **Google OAuth credentials**: Get from Google Cloud Console

### Getting Notion API Key

1. Go to https://www.notion.so/my-integrations
2. Create a new integration
3. Copy the "Internal Integration Token" (starts with `secret_`)
4. Share your Notion database with the integration

### Getting Google OAuth Token

For Google Calendar, you need to:
1. Create OAuth 2.0 credentials in Google Cloud Console
2. Configure OAuth consent screen
3. Use OAuth 2.0 playground or build authentication flow
4. Extract access token and refresh token

**Note**: If you only provide an access token without a refresh token, you'll need to re-authenticate when it expires. Always provide both tokens for automatic refresh.

### Verifying Token Storage

After storing a token, verify it's in the database:

```bash
sqlite3 ~/.pedrocli/pedrocli.db "SELECT provider, service, expires_at FROM oauth_tokens;"
```

Example output:
```
notion|database|
google|calendar|2024-12-27T15:30:00Z
```

### Security Notes

- **Never commit tokens** to version control
- Tokens are stored in SQLite database (default: `~/.pedrocli/pedrocli.db`)
- Access tokens are automatically refreshed if a refresh token is provided
- Notion API keys don't expire, so no refresh token is needed

### Troubleshooting

**Error: "database path not configured"**
- Add to `.pedrocli.json`:
  ```json
  {
    "repo_storage": {
      "database_path": "~/.pedrocli/pedrocli.db"
    }
  }
  ```

**Error: "failed to save token"**
- Ensure database exists and is writable
- Check that the database schema is up to date (run `pedrocli` once to auto-migrate)

**Token not working in podcast commands**
- Verify token is stored: `sqlite3 ~/.pedrocli/pedrocli.db "SELECT * FROM oauth_tokens;"`
- Check that provider and service names match exactly
- For Google tokens, ensure the token hasn't expired
