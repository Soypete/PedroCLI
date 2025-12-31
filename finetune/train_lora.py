#!/usr/bin/env python3
"""
train_lora.py - Train LoRA/QLoRA adapter for blog writing

Fine-tunes Qwen 3 base model using LoRA or QLoRA on blog writing data.
Compatible with RTX 5090 or DGX Spark.
"""

import argparse
import json
import torch
from pathlib import Path
from dataclasses import dataclass, field
from typing import Optional

try:
    from transformers import (
        AutoModelForCausalLM,
        AutoTokenizer,
        TrainingArguments,
        Trainer,
        DataCollatorForLanguageModeling,
    )
    from peft import LoraConfig, get_peft_model, TaskType, prepare_model_for_kbit_training
    from datasets import load_dataset
    HAS_DEPS = True
except ImportError:
    HAS_DEPS = False


@dataclass
class FineTuneConfig:
    """Configuration for fine-tuning"""

    # Model
    base_model: str = "Qwen/Qwen-3-7B"  # TODO: Update to actual Qwen 3 model name
    model_max_length: int = 4096

    # LoRA
    lora_r: int = 16
    lora_alpha: int = 32
    lora_dropout: float = 0.05
    lora_target_modules: list = field(default_factory=lambda: ["q_proj", "v_proj"])

    # Training
    num_train_epochs: int = 3
    per_device_train_batch_size: int = 4
    per_device_eval_batch_size: int = 4
    gradient_accumulation_steps: int = 4
    learning_rate: float = 2e-4
    warmup_steps: int = 100
    logging_steps: int = 10
    eval_steps: int = 100
    save_steps: int = 100
    save_total_limit: int = 3

    # QLoRA (4-bit quantization)
    use_qlora: bool = True
    load_in_4bit: bool = True
    bnb_4bit_compute_dtype: str = "bfloat16"
    bnb_4bit_use_double_quant: bool = True
    bnb_4bit_quant_type: str = "nf4"

    # Paths
    output_dir: str = "./checkpoints"
    logging_dir: str = "./logs"


def format_prompt(example):
    """Format example into prompt for training"""
    instruction = example['instruction']
    input_text = example['input']
    output_text = example['output']

    # Format as Qwen-style prompt
    prompt = f"""<|im_start|>system
{instruction}<|im_end|>
<|im_start|>user
{input_text}<|im_end|>
<|im_start|>assistant
{output_text}<|im_end|>"""

    return {"text": prompt}


def load_and_prepare_data(train_path: str, val_path: str):
    """Load and prepare datasets"""
    print(f"\nLoading datasets...")
    print(f"  Train: {train_path}")
    print(f"  Val:   {val_path}")

    dataset = load_dataset('json', data_files={
        'train': train_path,
        'validation': val_path
    })

    # Format prompts
    dataset = dataset.map(format_prompt, remove_columns=dataset['train'].column_names)

    return dataset


def setup_model_and_tokenizer(config: FineTuneConfig):
    """Setup model and tokenizer with LoRA/QLoRA"""
    print(f"\nLoading base model: {config.base_model}")

    # Tokenizer
    tokenizer = AutoTokenizer.from_pretrained(
        config.base_model,
        trust_remote_code=True,
        padding_side="right"
    )
    tokenizer.pad_token = tokenizer.eos_token

    # Model loading config
    model_kwargs = {
        "trust_remote_code": True,
        "torch_dtype": torch.bfloat16,
    }

    if config.use_qlora:
        print("Using QLoRA (4-bit quantization)")
        from transformers import BitsAndBytesConfig

        model_kwargs["quantization_config"] = BitsAndBytesConfig(
            load_in_4bit=config.load_in_4bit,
            bnb_4bit_compute_dtype=getattr(torch, config.bnb_4bit_compute_dtype),
            bnb_4bit_use_double_quant=config.bnb_4bit_use_double_quant,
            bnb_4bit_quant_type=config.bnb_4bit_quant_type,
        )

    model = AutoModelForCausalLM.from_pretrained(
        config.base_model,
        **model_kwargs
    )

    if config.use_qlora:
        model = prepare_model_for_kbit_training(model)

    # Setup LoRA
    print(f"\nSetting up LoRA (r={config.lora_r}, alpha={config.lora_alpha})")
    peft_config = LoraConfig(
        task_type=TaskType.CAUSAL_LM,
        inference_mode=False,
        r=config.lora_r,
        lora_alpha=config.lora_alpha,
        lora_dropout=config.lora_dropout,
        target_modules=config.lora_target_modules,
    )

    model = get_peft_model(model, peft_config)
    model.print_trainable_parameters()

    return model, tokenizer


def train(config: FineTuneConfig, train_path: str, val_path: str):
    """Run fine-tuning"""

    # Load data
    dataset = load_and_prepare_data(train_path, val_path)

    # Setup model
    model, tokenizer = setup_model_and_tokenizer(config)

    # Tokenize
    def tokenize_function(examples):
        return tokenizer(
            examples["text"],
            truncation=True,
            max_length=config.model_max_length,
            padding="max_length",
        )

    print("\nTokenizing datasets...")
    tokenized_dataset = dataset.map(
        tokenize_function,
        batched=True,
        remove_columns=dataset["train"].column_names,
    )

    # Data collator
    data_collator = DataCollatorForLanguageModeling(
        tokenizer=tokenizer,
        mlm=False,
    )

    # Training arguments
    training_args = TrainingArguments(
        output_dir=config.output_dir,
        num_train_epochs=config.num_train_epochs,
        per_device_train_batch_size=config.per_device_train_batch_size,
        per_device_eval_batch_size=config.per_device_eval_batch_size,
        gradient_accumulation_steps=config.gradient_accumulation_steps,
        learning_rate=config.learning_rate,
        warmup_steps=config.warmup_steps,
        logging_dir=config.logging_dir,
        logging_steps=config.logging_steps,
        eval_steps=config.eval_steps,
        save_steps=config.save_steps,
        save_total_limit=config.save_total_limit,
        evaluation_strategy="steps",
        save_strategy="steps",
        load_best_model_at_end=True,
        bf16=True,
        report_to="none",  # Change to "wandb" if using Weights & Biases
    )

    # Trainer
    trainer = Trainer(
        model=model,
        args=training_args,
        train_dataset=tokenized_dataset["train"],
        eval_dataset=tokenized_dataset["validation"],
        data_collator=data_collator,
    )

    # Train
    print("\n" + "="*80)
    print("Starting training...")
    print("="*80 + "\n")

    trainer.train()

    # Save final model
    print("\nSaving final model...")
    trainer.save_model(config.output_dir + "/final")

    print("\n✓ Training complete!")
    print(f"Model saved to: {config.output_dir}")


def main():
    if not HAS_DEPS:
        print("Error: Required dependencies not installed", file=sys.stderr)
        print("\nPlease install: pip install transformers peft datasets bitsandbytes accelerate", file=sys.stderr)
        return

    parser = argparse.ArgumentParser(description='Fine-tune Qwen 3 for blog writing with LoRA/QLoRA')
    parser.add_argument('--train-data', type=str, required=True,
                        help='Path to training JSONL file')
    parser.add_argument('--val-data', type=str, required=True,
                        help='Path to validation JSONL file')
    parser.add_argument('--base-model', type=str, default='Qwen/Qwen-3-7B',
                        help='Base model to fine-tune')
    parser.add_argument('--output-dir', type=str, default='./checkpoints',
                        help='Output directory for checkpoints')
    parser.add_argument('--use-lora', action='store_true',
                        help='Use LoRA instead of QLoRA (full precision)')
    parser.add_argument('--lora-r', type=int, default=16,
                        help='LoRA rank')
    parser.add_argument('--lora-alpha', type=int, default=32,
                        help='LoRA alpha')
    parser.add_argument('--epochs', type=int, default=3,
                        help='Number of training epochs')
    parser.add_argument('--batch-size', type=int, default=4,
                        help='Training batch size per device')
    parser.add_argument('--learning-rate', type=float, default=2e-4,
                        help='Learning rate')

    args = parser.parse_args()

    # Create config
    config = FineTuneConfig(
        base_model=args.base_model,
        lora_r=args.lora_r,
        lora_alpha=args.lora_alpha,
        num_train_epochs=args.epochs,
        per_device_train_batch_size=args.batch_size,
        per_device_eval_batch_size=args.batch_size,
        learning_rate=args.learning_rate,
        output_dir=args.output_dir,
        use_qlora=not args.use_lora,
    )

    # Run training
    train(config, args.train_data, args.val_data)


if __name__ == '__main__':
    main()
