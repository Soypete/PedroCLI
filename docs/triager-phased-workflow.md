# Triager Phased Workflow - Future Implementation

This document describes the planned phased workflow for the TriagerAgent, to be implemented in a future PR.

## Overview

The TriagerAgent diagnoses issues without implementing fixes. It's designed for initial issue triage to understand scope, severity, and recommended approaches before assigning to a developer or automated agent.

## Proposed Phases

### Phase 1: COLLECT
**Goal:** Gather all available evidence about the issue.

**Tools:** `search`, `file`, `git`, `github`, `bash`

**Actions:**
- Parse provided error logs and stack traces
- Fetch GitHub issue details if issue number provided
- Extract relevant information from reproduction steps
- Check git history for recent changes to affected areas

**Output:**
```json
{
  "evidence": {
    "error_messages": ["..."],
    "affected_files": ["..."],
    "recent_changes": ["commit hashes"],
    "reproduction_info": "..."
  }
}
```

### Phase 2: MAP
**Goal:** Understand the scope and affected components.

**Tools:** `lsp`, `search`, `navigate`, `file`, `context`

**Actions:**
- Use LSP to find definitions and references
- Map the call graph around the error location
- Identify upstream and downstream dependencies
- Check for related issues or comments in code

**Output:**
```json
{
  "scope": {
    "primary_components": ["pkg/auth", "pkg/session"],
    "dependencies": ["pkg/database", "pkg/cache"],
    "related_code": ["similar patterns found"],
    "test_coverage": "partial"
  }
}
```

### Phase 3: CLASSIFY
**Goal:** Determine severity and category of the issue.

**Tools:** `context`

**Actions:**
- Assess impact based on scope and affected functionality
- Categorize the issue type
- Determine urgency

**Severity Levels:**
- `critical`: System down, data loss, security vulnerability
- `high`: Major feature broken, significant impact
- `medium`: Feature partially broken, workaround exists
- `low`: Minor issue, cosmetic, edge case
- `info`: Not an issue, or needs more information

**Categories:**
- `bug`: Incorrect behavior
- `performance`: Slow or resource-intensive
- `security`: Security vulnerability
- `dependency`: External dependency issue
- `infrastructure`: Build, deploy, config issue
- `test`: Test failure or flakiness
- `documentation`: Docs are incorrect or missing

**Output:**
```json
{
  "classification": {
    "severity": "high",
    "category": "bug",
    "impact": "User authentication fails under specific conditions",
    "urgency": "Should be fixed this sprint"
  }
}
```

### Phase 4: ANALYZE
**Goal:** Determine the root cause (without fixing).

**Tools:** `file`, `lsp`, `search`, `context`

**Actions:**
- Trace the code path that leads to the error
- Identify the root cause vs symptoms
- Document the exact location and mechanism of failure

**Output:**
```json
{
  "root_cause": {
    "location": "pkg/auth/validator.go:142",
    "mechanism": "Nil pointer dereference when session is expired",
    "trigger": "User attempts action after session timeout",
    "confidence": "high"
  }
}
```

### Phase 5: RECOMMEND
**Goal:** Propose fix approaches with trade-offs.

**Tools:** `context`, `search`

**Actions:**
- Generate 2-3 possible fix approaches
- Estimate complexity for each approach
- Identify risks and trade-offs
- Recommend which approach to take

**Output:**
```json
{
  "recommendations": [
    {
      "approach": "Add nil check before session access",
      "complexity": "low",
      "estimated_files": 1,
      "risks": ["Might mask deeper issues"],
      "recommended": false
    },
    {
      "approach": "Refactor session handling to use optional pattern",
      "complexity": "medium",
      "estimated_files": 3,
      "risks": ["More changes, needs careful testing"],
      "recommended": true,
      "reason": "Addresses root cause and improves overall robustness"
    }
  ]
}
```

### Phase 6: REPORT
**Goal:** Compile everything into a structured triage report.

**Tools:** `context`

**Actions:**
- Compile all findings from previous phases
- Generate structured JSON report
- Generate human-readable markdown summary

**Output:**
```json
{
  "triage_report": {
    "summary": "Authentication fails when session expires during user action",
    "severity": "high",
    "category": "bug",
    "root_cause": {...},
    "affected_components": [...],
    "recommendations": [...],
    "estimated_effort": "2-4 hours",
    "suggested_assignee": "auth-team"
  }
}
```

## Implementation Notes

### File Structure
```
pkg/agents/
├── triager_phased.go          # Main agent implementation
└── prompts/
    ├── triager_phased_collect.md
    ├── triager_phased_map.md
    ├── triager_phased_classify.md
    ├── triager_phased_analyze.md
    ├── triager_phased_recommend.md
    └── triager_phased_report.md
```

### Key Differences from Other Agents

1. **No Code Modification:** Triager never edits files
2. **No Git Commits:** Only reads, never writes
3. **Focus on Analysis:** Output is a report, not working code
4. **Handoff Ready:** Output should be actionable for developers or other agents

### Integration Points

The triage report can be used to:
- Automatically assign issues to teams based on affected components
- Feed into BuilderAgent or DebuggerAgent with context
- Generate GitHub issue labels and priority
- Estimate sprint planning effort

### Tool Restrictions

The triager should have restricted tools:
```go
Tools: []string{"search", "file", "lsp", "navigate", "git", "github", "context"}
// Note: NO code_edit, NO bash (except read-only), NO test (except listing)
```

## Priority

This is a **medium priority** enhancement. The existing TriagerAgent works well for simpler cases. The phased version would provide:
- More structured output
- Better handling of complex issues
- Integration with sprint planning tools
- Foundation for automated issue routing

## Related

- `pkg/agents/triager.go` - Current implementation
- `docs/adr/ADR-XXX-phased-workflows.md` - Architecture decision record
- `pkg/agents/phased_executor.go` - Base phased executor
