#!/usr/bin/env python3
"""
collect_data.py - Collect training data from various sources for fine-tuning

This script gathers training data from:
1. Raw dictation transcripts (input)
2. Final published blog posts (output)
3. Twitch VOD transcripts (style examples)
4. Writer agent input/output pairs
"""

import sys
import json
import psycopg2
from pathlib import Path
from typing import List, Dict, Any
from dataclasses import dataclass, asdict
import argparse


@dataclass
class TrainingExample:
    """Represents a single training example"""
    input_text: str
    output_text: str
    source_type: str
    quality_score: float = 1.0
    metadata: Dict[str, Any] = None

    def to_dict(self):
        return asdict(self)


class DataCollector:
    """Collects training data from various sources"""

    def __init__(self, db_config: Dict[str, str]):
        self.db_config = db_config
        self.conn = None
        self.examples: List[TrainingExample] = []

    def connect(self):
        """Connect to PostgreSQL database"""
        try:
            self.conn = psycopg2.connect(
                host=self.db_config.get('host', 'localhost'),
                port=self.db_config.get('port', 5432),
                user=self.db_config.get('user', 'pedrocli'),
                password=self.db_config.get('password', 'pedrocli'),
                database=self.db_config.get('database', 'pedrocli_blog')
            )
            print(f"✓ Connected to database: {self.db_config['database']}")
        except Exception as e:
            print(f"✗ Failed to connect to database: {e}", file=sys.stderr)
            sys.exit(1)

    def close(self):
        """Close database connection"""
        if self.conn:
            self.conn.close()

    def collect_from_blog_posts(self):
        """Collect training pairs from published blog posts"""
        print("\nCollecting data from blog posts...")

        cursor = self.conn.cursor()
        query = """
            SELECT raw_transcription, final_content, title, id
            FROM blog_posts
            WHERE raw_transcription IS NOT NULL
              AND final_content IS NOT NULL
              AND status IN ('published', 'public')
        """

        cursor.execute(query)
        rows = cursor.fetchall()

        for raw, final, title, post_id in rows:
            if raw and final:
                example = TrainingExample(
                    input_text=raw,
                    output_text=final,
                    source_type='blog',
                    metadata={
                        'post_id': str(post_id),
                        'title': title
                    }
                )
                self.examples.append(example)

        cursor.close()
        print(f"  → Collected {len(rows)} examples from blog posts")

    def collect_from_training_pairs(self):
        """Collect existing training pairs from database"""
        print("\nCollecting data from training_pairs table...")

        cursor = self.conn.cursor()
        query = """
            SELECT input_text, output_text, source_type, quality_score, metadata
            FROM training_pairs
            WHERE output_text IS NOT NULL
              AND output_text != ''
        """

        cursor.execute(query)
        rows = cursor.fetchall()

        for input_text, output_text, source_type, quality_score, metadata in rows:
            example = TrainingExample(
                input_text=input_text,
                output_text=output_text,
                source_type=source_type,
                quality_score=quality_score or 1.0,
                metadata=metadata or {}
            )
            self.examples.append(example)

        cursor.close()
        print(f"  → Collected {len(rows)} examples from training_pairs")

    def collect_from_twitch(self, twitch_dir: Path):
        """Collect data from Twitch VOD transcripts

        TODO: Implement when Twitch VOD transcript location is known
        """
        print("\nCollecting data from Twitch VODs...")
        print("  → TODO: Implement Twitch VOD transcript collection")
        # Placeholder for Twitch integration

    def filter_by_quality(self, min_score: float = 0.5):
        """Filter examples by quality score"""
        before = len(self.examples)
        self.examples = [ex for ex in self.examples if ex.quality_score >= min_score]
        after = len(self.examples)
        print(f"\nFiltered {before - after} low-quality examples (min_score={min_score})")

    def save_to_jsonl(self, output_path: Path):
        """Save collected examples to JSONL file"""
        with open(output_path, 'w') as f:
            for example in self.examples:
                f.write(json.dumps(example.to_dict()) + '\n')

        print(f"\n✓ Saved {len(self.examples)} examples to {output_path}")

    def print_stats(self):
        """Print collection statistics"""
        print("\n" + "="*60)
        print("COLLECTION STATISTICS")
        print("="*60)
        print(f"Total examples: {len(self.examples)}")

        # Count by source type
        source_counts = {}
        for ex in self.examples:
            source_counts[ex.source_type] = source_counts.get(ex.source_type, 0) + 1

        print("\nBy source type:")
        for source, count in sorted(source_counts.items()):
            print(f"  {source:15s}: {count:6d} examples")

        # Quality distribution
        if self.examples:
            scores = [ex.quality_score for ex in self.examples]
            avg_score = sum(scores) / len(scores)
            print(f"\nAverage quality score: {avg_score:.2f}")


def main():
    parser = argparse.ArgumentParser(description='Collect training data for blog writing fine-tuning')
    parser.add_argument('--output', type=str, default='training_data.jsonl',
                        help='Output JSONL file path')
    parser.add_argument('--min-quality', type=float, default=0.5,
                        help='Minimum quality score for examples')
    parser.add_argument('--db-host', type=str, default='localhost',
                        help='Database host')
    parser.add_argument('--db-port', type=int, default=5432,
                        help='Database port')
    parser.add_argument('--db-user', type=str, default='pedrocli',
                        help='Database user')
    parser.add_argument('--db-password', type=str, default='pedrocli',
                        help='Database password')
    parser.add_argument('--db-name', type=str, default='pedrocli_blog',
                        help='Database name')
    parser.add_argument('--twitch-dir', type=str, default=None,
                        help='Directory containing Twitch VOD transcripts')

    args = parser.parse_args()

    db_config = {
        'host': args.db_host,
        'port': args.db_port,
        'user': args.db_user,
        'password': args.db_password,
        'database': args.db_name
    }

    collector = DataCollector(db_config)

    try:
        collector.connect()

        # Collect from all sources
        collector.collect_from_blog_posts()
        collector.collect_from_training_pairs()

        if args.twitch_dir:
            collector.collect_from_twitch(Path(args.twitch_dir))

        # Filter and save
        collector.filter_by_quality(args.min_quality)
        collector.print_stats()
        collector.save_to_jsonl(Path(args.output))

    finally:
        collector.close()


if __name__ == '__main__':
    main()
