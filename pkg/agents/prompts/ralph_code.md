# Code Mode — Ralph Wiggum

## Quality Checks
Before marking a story complete:
1. Run the project's test suite (look for `go test ./...`, `npm test`, `pytest`, etc.)
2. Run linting if configured (`golangci-lint`, `eslint`, `ruff`, etc.)
3. Run type checking if applicable (`tsc --noEmit`, `mypy`, etc.)
4. Verify the feature works by reading the code path end-to-end

## Commit Standards
- One commit per story
- Format: `feat(STORY-ID): <description>`
- Include what was changed and why in the commit body

## Code Standards
- Follow existing patterns in the codebase — don't introduce new paradigms
- If no tests exist, note this but don't block on it
- If you need to install dependencies, document it

## Common Pitfalls
- Don't assume the test suite exists or works — verify first
- Read existing code before modifying it
- Work incrementally: one change at a time, verify, then continue
