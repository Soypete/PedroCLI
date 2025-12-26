#!/usr/bin/env python3
"""
prepare_dataset.py - Prepare training dataset for LoRA/QLoRA fine-tuning

Converts collected training data into format suitable for fine-tuning:
- Formats as instruction-following examples
- Splits into train/validation sets
- Tokenizes and prepares for training
"""

import json
import argparse
from pathlib import Path
from typing import List, Dict, Any
import random


def format_as_instruction(example: Dict[str, Any]) -> Dict[str, str]:
    """
    Format training example as instruction-following format

    Uses format compatible with most fine-tuning libraries:
    {
        "instruction": "Transform this dictation into a blog post",
        "input": "<raw dictation>",
        "output": "<polished blog post>"
    }
    """
    instruction = """Transform the following raw dictation into a polished, narrative-driven blog post.

Guidelines:
- Identify and clearly state the central thesis
- Organize content with strong narrative flow
- Maintain authentic voice and tone
- Include engaging opening and strong conclusion
- End with clear call to action
"""

    return {
        "instruction": instruction,
        "input": example['input_text'],
        "output": example['output_text'],
        "source": example.get('source_type', 'unknown')
    }


def split_dataset(examples: List[Dict], train_ratio: float = 0.9) -> tuple:
    """Split dataset into train and validation sets"""
    random.shuffle(examples)
    split_idx = int(len(examples) * train_ratio)
    return examples[:split_idx], examples[split_idx:]


def save_jsonl(examples: List[Dict], path: Path):
    """Save examples to JSONL file"""
    with open(path, 'w') as f:
        for example in examples:
            f.write(json.dumps(example) + '\n')


def print_example(example: Dict):
    """Pretty print an example for review"""
    print("\n" + "="*80)
    print("INSTRUCTION:")
    print(example['instruction'][:200] + "...")
    print("\nINPUT:")
    print(example['input'][:300] + "..." if len(example['input']) > 300 else example['input'])
    print("\nOUTPUT:")
    print(example['output'][:300] + "..." if len(example['output']) > 300 else example['output'])
    print("="*80)


def main():
    parser = argparse.ArgumentParser(description='Prepare training dataset for fine-tuning')
    parser.add_argument('--input', type=str, required=True,
                        help='Input JSONL file from collect_data.py')
    parser.add_argument('--output-dir', type=str, default='./datasets',
                        help='Output directory for prepared datasets')
    parser.add_argument('--train-ratio', type=float, default=0.9,
                        help='Ratio of training examples (rest is validation)')
    parser.add_argument('--seed', type=int, default=42,
                        help='Random seed for reproducibility')
    parser.add_argument('--show-example', action='store_true',
                        help='Show a sample example')

    args = parser.parse_args()

    random.seed(args.seed)
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    print(f"Loading examples from {args.input}...")
    examples = []
    with open(args.input, 'r') as f:
        for line in f:
            examples.append(json.loads(line))

    print(f"Loaded {len(examples)} examples")

    # Format as instruction-following
    print("\nFormatting as instruction-following examples...")
    formatted = [format_as_instruction(ex) for ex in examples]

    # Show example if requested
    if args.show_example and formatted:
        print("\nSample formatted example:")
        print_example(formatted[0])

    # Split dataset
    print(f"\nSplitting dataset (train_ratio={args.train_ratio})...")
    train, val = split_dataset(formatted, args.train_ratio)

    print(f"  Train: {len(train)} examples")
    print(f"  Val:   {len(val)} examples")

    # Save datasets
    train_path = output_dir / 'train.jsonl'
    val_path = output_dir / 'val.jsonl'

    save_jsonl(train, train_path)
    save_jsonl(val, val_path)

    print(f"\n✓ Saved training dataset to {train_path}")
    print(f"✓ Saved validation dataset to {val_path}")

    # Print statistics
    print("\n" + "="*80)
    print("DATASET STATISTICS")
    print("="*80)

    def calc_stats(examples, name):
        input_lengths = [len(ex['input']) for ex in examples]
        output_lengths = [len(ex['output']) for ex in examples]
        print(f"\n{name} set:")
        print(f"  Examples: {len(examples)}")
        print(f"  Avg input length:  {sum(input_lengths) / len(input_lengths):.0f} chars")
        print(f"  Avg output length: {sum(output_lengths) / len(output_lengths):.0f} chars")
        print(f"  Max input length:  {max(input_lengths)} chars")
        print(f"  Max output length: {max(output_lengths)} chars")

    calc_stats(train, "Training")
    calc_stats(val, "Validation")

    print("\n" + "="*80)
    print("\nNext steps:")
    print(f"1. Review the prepared datasets in {output_dir}")
    print("2. Run train_lora.py to start fine-tuning")


if __name__ == '__main__':
    main()
