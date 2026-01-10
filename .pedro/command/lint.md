---
description: Run linters and fix any issues found
agent: build
---

Run the project linters:

!`make lint 2>&1 || golangci-lint run 2>&1`

Fix any issues found in the linter output above. For each issue:
1. Identify the file and line
2. Understand the problem
3. Apply the fix
4. Verify the fix is correct
