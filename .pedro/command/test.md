---
description: Run the test suite and analyze results
agent: build
---

Run the full test suite with coverage:

!`go test -v -cover ./...`

Analyze the test results above:
1. Identify any failing tests
2. Suggest fixes for failures
3. Highlight any tests with low coverage
4. Note any slow tests (>1s)
