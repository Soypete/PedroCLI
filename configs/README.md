# PedroCLI Configuration Files

This directory contains organized configuration files for different LLM backends and models.

## Directory Structure

```
configs/
├── llamacpp/              # llama.cpp (llama-server) configurations
│   ├── qwen2.5-coder-7b/      # Qwen 2.5 Coder 7B (32K context)
│   ├── qwen2.5-coder-32b/     # Qwen 2.5 Coder 32B (16K context) - PRODUCTION
│   ├── qwen2.5-coder-32b-32k/ # Qwen 2.5 Coder 32B (32K context, native tools)
│   └── qwen2.5-coder-32b-test/# Qwen 2.5 Coder 32B (32K context, minimal config)
│
├── ollama/                # Ollama configurations
│   └── qwen2.5-coder-32b/     # Qwen 2.5 Coder 32B (16K context)
│
└── examples/              # Example configurations
    ├── llamacpp/              # llamacpp example config
    └── blog/                  # Blog-specific config example
```

## Usage

Use the `--config` flag to specify which configuration to use:

```bash
# Using llamacpp with 16K context (production)
./pedrocli --config configs/llamacpp/qwen2.5-coder-32b/.pedrocli.json build -description "Add feature"

# Using Ollama
./pedrocli --config configs/ollama/qwen2.5-coder-32b/.pedrocli.json build -description "Add feature"

# Using llamacpp with 32K context
./pedrocli --config configs/llamacpp/qwen2.5-coder-32b-32k/.pedrocli.json build -description "Add feature"

# Using 7B model (faster, less VRAM)
./pedrocli --config configs/llamacpp/qwen2.5-coder-7b/.pedrocli.json build -description "Add feature"
```

## Configuration Differences

### LLM Backend Comparison

| Config | Backend | Model | Context | Features | Use Case |
|--------|---------|-------|---------|----------|----------|
| `llamacpp/qwen2.5-coder-32b` | llama-server | 32B Q4_K_M | 16K | Production stable | **Default** - tested compaction |
| `llamacpp/qwen2.5-coder-32b-32k` | llama-server | 32B Q4_K_M | 32K | Native tools | Large context needs (VRAM warning) |
| `llamacpp/qwen2.5-coder-7b` | llama-server | 7B | 32K | Native tools | Fast testing, lower VRAM |
| `ollama/qwen2.5-coder-32b` | Ollama | 32B | 16K | Auto-context detection | Easy setup, auto-managed |

### Key Configuration Fields

#### llamacpp configs:
```json
{
  "model": {
    "type": "llamacpp",
    "server_url": "http://localhost:8082",
    "model_name": "qwen2.5-coder:32b",
    "context_size": 16384,
    "usable_context": 12288,  // 75% of total
    "temperature": 0.2
  }
}
```

#### Ollama configs:
```json
{
  "model": {
    "type": "ollama",
    "server_url": "http://localhost:11434",
    "model_name": "qwen2.5-coder:32b",
    "context_size": 16384,
    "usable_context": 12288,
    "temperature": 0.2
  }
}
```

## Context Window Guidelines

- **16K context (12K usable)**: Production stable on M1 Max, tested with compaction
- **32K context**: Requires ~8.2GB VRAM for KV cache - may crash on M1 Max
- **Usable context**: Set to 75% of total (leaves room for LLM response)
- **Compaction threshold**: Triggers at 75% of usable (9,216 tokens for 16K config)

## Compaction Testing Results

The **16K context configuration** (`llamacpp/qwen2.5-coder-32b/.pedrocli.json`) has been tested and validated with context window compaction:

- ✅ Completed 25/25 rounds (vs 8-round failure with 32K mismatch)
- ✅ Tokens managed: 64% → 105% → 67% of threshold
- ✅ No crashes from context overflow
- ✅ Dynamic summarization: 4 → 8 → 10 summary sections

See `learnings/context-compaction-testing-2026-01-05.md` for full test results.

## Creating New Configurations

1. Copy an existing config from the appropriate backend directory
2. Create a new directory: `configs/{backend}/{model-name}/`
3. Modify the `.pedrocli.json` file with your settings
4. Test with: `./pedrocli --config configs/{backend}/{model-name}/.pedrocli.json list`

## Default Behavior

If `--config` is not specified, pedrocli looks for `.pedrocli.json` in:
1. Current working directory
2. Home directory (`~/.pedrocli.json`)

## Migration from Root Config

Old root-level config files have been organized:
- `.pedrocli.json` → `configs/llamacpp/qwen2.5-coder-32b/.pedrocli.json`
- `.pedrocli-llamacpp-32b.json` → `configs/llamacpp/qwen2.5-coder-32b-32k/.pedrocli.json`
- `.pedrocli-llamacpp-7b-test.json` → `configs/llamacpp/qwen2.5-coder-7b/.pedrocli.json`

The original `.pedrocli.json` remains in the root for backward compatibility.
