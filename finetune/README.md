# Blog Writing Fine-Tuning Pipeline

This directory contains scripts for fine-tuning Qwen 3 base model on your blog writing style using LoRA/QLoRA.

## Overview

The pipeline consists of three stages:

1. **collect_data.py** - Gather training data from various sources
2. **prepare_dataset.py** - Format and split data for training
3. **train_lora.py** - Fine-tune the model using LoRA/QLoRA

## Prerequisites

```bash
pip install transformers peft datasets bitsandbytes accelerate torch psycopg2-binary
```

## Hardware Requirements

- **Minimum**: RTX 3090 (24GB VRAM) for QLoRA
- **Recommended**: RTX 5090 or DGX Spark for faster training
- QLoRA (4-bit) can run on 24GB VRAM
- Full LoRA requires 40GB+ VRAM

## Usage

### Step 1: Collect Training Data

```bash
python collect_data.py \
    --output training_data.jsonl \
    --min-quality 0.7 \
    --db-host localhost \
    --db-port 5432 \
    --db-user pedrocli \
    --db-password YOUR_PASSWORD \
    --db-name pedrocli_blog
```

This collects data from:
- Published blog posts (raw dictation → final post)
- Training pairs stored in the database
- Twitch VOD transcripts (TODO: configure path)

### Step 2: Prepare Dataset

```bash
python prepare_dataset.py \
    --input training_data.jsonl \
    --output-dir ./datasets \
    --train-ratio 0.9 \
    --show-example
```

This:
- Formats data as instruction-following examples
- Splits into train (90%) and validation (10%) sets
- Saves to `datasets/train.jsonl` and `datasets/val.jsonl`

### Step 3: Fine-Tune with LoRA/QLoRA

#### QLoRA (4-bit, recommended for most GPUs):

```bash
python train_lora.py \
    --train-data datasets/train.jsonl \
    --val-data datasets/val.jsonl \
    --base-model Qwen/Qwen-3-7B \
    --output-dir ./checkpoints \
    --epochs 3 \
    --batch-size 4 \
    --learning-rate 2e-4
```

#### Full LoRA (requires more VRAM):

```bash
python train_lora.py \
    --train-data datasets/train.jsonl \
    --val-data datasets/val.jsonl \
    --base-model Qwen/Qwen-3-7B \
    --output-dir ./checkpoints \
    --use-lora \
    --epochs 3 \
    --batch-size 2 \
    --learning-rate 2e-4
```

## Hyperparameters

Key parameters to tune:

- **lora_r** (16): LoRA rank - higher = more parameters, better quality, slower
- **lora_alpha** (32): LoRA scaling - usually 2x the rank
- **learning_rate** (2e-4): Adjust if loss plateaus or diverges
- **epochs** (3): More epochs may overfit with small datasets
- **batch_size** (4): Increase if you have more VRAM

## Monitoring Training

Training progress is logged to `./logs`. View with TensorBoard:

```bash
tensorboard --logdir ./logs
```

Or integrate with Weights & Biases by changing `report_to="wandb"` in `train_lora.py`.

## Using the Fine-Tuned Model

After training, load the adapter in your PedroCLI config:

```json
{
  "blog": {
    "enabled": true,
    "default_model": "qwen3:7b",
    "lora_adapter_path": "./finetune/checkpoints/final"
  }
}
```

TODO: Implement adapter loading in the LLM backend.

## Data Quality Tips

1. **Minimum Examples**: Aim for at least 50-100 high-quality examples
2. **Quality > Quantity**: Better to have 50 great examples than 500 mediocre ones
3. **Diversity**: Include various topics and post styles
4. **Review Before Training**: Use `--show-example` to review formatted data

## Troubleshooting

**OOM (Out of Memory)**:
- Reduce batch_size
- Reduce model_max_length
- Enable gradient checkpointing
- Use QLoRA instead of LoRA

**Training Loss Not Decreasing**:
- Check learning rate (try 1e-4 or 5e-5)
- Verify data quality
- Increase lora_r

**Model Outputs Nonsense**:
- May need more training examples
- Try lower learning rate
- Train for more epochs

## TODO

- [ ] Add support for loading Twitch VOD transcripts
- [ ] Implement automatic quality scoring
- [ ] Add evaluation metrics (BLEU, ROUGE)
- [ ] Support for multi-GPU training
- [ ] Adapter merging and quantization for deployment
