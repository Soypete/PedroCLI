# Phase: Analyze

The goal of this phase is to **understand the codebase** and gather context for the task.

## What to do

1. Explore the relevant parts of the codebase
2. Understand the existing patterns and conventions
3. Identify files that will need to be modified
4. Gather necessary context (APIs, data structures, dependencies)

## Tools available

You have access to: search, navigate, file, git, context

## Output

Return a JSON object with:
- `analysis`: Description of what you found
- `relevant_files`: List of files that are relevant to the task
- `context_needed`: Any additional context that would help
- `risks`: Potential issues or concerns identified