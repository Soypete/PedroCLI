# PedroCLI Blog Writing Tools Guide

This guide covers the blog writing tools extension for PedroCLI.

## Overview

The blog writing tools add autonomous blog post creation capabilities to PedroCLI, including:

- **Writer Agent**: Transforms raw dictation into polished blog posts
- **Editor Agent**: Reviews and refines drafted posts
- **Newsletter System**: Auto-generates newsletter sections
- **Notion Integration**: Syncs with Notion for workflow management
- **Fine-Tuning Pipeline**: Train custom models on your writing style

## Setup

### 1. Database Setup

Create a PostgreSQL database for blog data:

```bash
# Create database and user
createdb pedrocli_blog
createuser -P pedrocli  # Enter password when prompted

# Grant permissions
psql -c "GRANT ALL PRIVILEGES ON DATABASE pedrocli_blog TO pedrocli;"
```

### 2. Configuration

Copy the example config and customize:

```bash
cp .pedrocli.json.blog-example .pedrocli.json
```

Edit `.pedrocli.json` and set:
- Database credentials
- Notion API key and database IDs (optional)
- Blog preferences (model, editor mode, etc.)

### 3. Run Migrations

Migrations run automatically on first startup when blog tools are enabled.

### 4. Install Dependencies (for fine-tuning)

```bash
cd finetune
pip install -r requirements.txt  # TODO: Create requirements.txt
```

## Workflow

### Step 1: Create Draft from Dictation

**Via HTTP API**:
```bash
curl -X POST http://localhost:8080/api/blog/dictation \
  -H "Content-Type: application/json" \
  -d '{
    "title": "My Post Title",
    "transcription": "Raw dictation text here..."
  }'
```

This creates a new blog post in `dictated` status.

### Step 2: Process with Writer Agent

```bash
curl -X POST http://localhost:8080/api/blog/process-writer \
  -H "Content-Type: application/json" \
  -d '{
    "post_id": "uuid-from-step-1"
  }'
```

The writer agent transforms your rambling dictation into a structured blog post.

### Step 3: Review with Editor Agent

```bash
curl -X POST http://localhost:8080/api/blog/process-editor \
  -H "Content-Type: application/json" \
  -d '{
    "post_id": "uuid-from-step-1",
    "auto_revise": false
  }'
```

Set `auto_revise: true` for automatic revisions, or `false` for review feedback.

### Step 4: Add Newsletter Section

```bash
curl -X POST http://localhost:8080/api/blog/add-newsletter \
  -H "Content-Type: application/json" \
  -d '{
    "post_id": "uuid-from-step-1"
  }'
```

Automatically pulls unused assets and generates newsletter addendum.

### Step 5: Publish

```bash
curl -X POST http://localhost:8080/api/blog/publish \
  -H "Content-Type: application/json" \
  -d '{
    "post_id": "uuid-from-step-1",
    "substack_url": "https://soypete.substack.com/p/my-post"
  }'
```

Publishes to Notion and sets paywall expiration.

## Fine-Tuning Your Writing Model

After you have several published posts, fine-tune a model on your style:

```bash
cd finetune

# Step 1: Collect training data
python collect_data.py --output training_data.jsonl

# Step 2: Prepare dataset
python prepare_dataset.py \
  --input training_data.jsonl \
  --output-dir ./datasets

# Step 3: Train LoRA adapter
python train_lora.py \
  --train-data datasets/train.jsonl \
  --val-data datasets/val.jsonl \
  --base-model Qwen/Qwen-3-7B \
  --output-dir ./checkpoints
```

See `finetune/README.md` for detailed fine-tuning instructions.

## Managing Newsletter Assets

Add assets to include in newsletter sections:

```sql
-- Add a video
INSERT INTO newsletter_assets (asset_type, title, url, embed_code)
VALUES ('video', 'My Latest Video', 'https://youtube.com/...', '<iframe>...</iframe>');

-- Add an upcoming event
INSERT INTO newsletter_assets (asset_type, title, event_date, url)
VALUES ('event', 'Forge Utah Meetup', '2025-02-15 18:00:00', 'https://...');

-- Add reading recommendation
INSERT INTO newsletter_assets (asset_type, title, url, description)
VALUES ('reading', 'Interesting Article', 'https://...', 'Great insights on...');
```

Assets marked as unused will be automatically pulled into the next newsletter generation.

## Web UI (TODO)

The web UI will include a "Blog Tools" mode with:

- Long dictation capture interface (mobile-optimized)
- Post pipeline status view
- Quick actions for each workflow stage
- Newsletter template editor
- Training data statistics

**Status**: Core infrastructure complete, UI integration pending.

## TODO Items

### Short Term
- [ ] Implement Whisper.cpp integration for long dictation
- [ ] Add web UI for blog tools mode
- [ ] Create HTTP API endpoints for blog operations
- [ ] Implement chunked upload for long audio files
- [ ] Add progress indicators for transcription

### Medium Term
- [ ] Substack API integration (if available)
- [ ] Automatic paywall removal scheduler
- [ ] Twitch VOD transcript collection
- [ ] Quality scoring for training data
- [ ] LoRA adapter loading in LLM backend

### Long Term
- [ ] Multi-model ensemble (different models for different sections)
- [ ] Automatic topic clustering and series detection
- [ ] SEO optimization suggestions
- [ ] Social media preview generation

## Architecture

### Database Schema

- **blog_posts**: Main post pipeline tracking
- **training_pairs**: Training data for fine-tuning
- **newsletter_assets**: Videos, events, links for newsletters

### Agents

- **WriterAgent** (`pkg/agents/writer.go`): Transforms dictation to draft
- **EditorAgent** (`pkg/agents/editor.go`): Reviews and refines drafts

### Storage

- **PostStore** (`pkg/storage/blog/posts.go`): Blog post CRUD
- **TrainingStore** (`pkg/storage/blog/training.go`): Training data management
- **NewsletterStore** (`pkg/storage/blog/newsletter.go`): Asset management

### Tools

- **DictationTool**: Create post from transcription
- **ProcessWriterTool**: Run writer agent
- **ProcessEditorTool**: Run editor agent
- **PublishTool**: Publish to Notion/Substack

## Troubleshooting

**Database connection fails**:
- Verify PostgreSQL is running: `pg_isready`
- Check credentials in `.pedrocli.json`
- Ensure database exists: `psql -l | grep pedrocli_blog`

**Writer/Editor agents produce poor output**:
- Verify correct model in config (`qwen3:7b`, not `qwen2.5-coder`)
- Check system prompts in `pkg/agents/prompts/`
- Consider fine-tuning on your content

**Migrations fail**:
- Check database permissions
- Manually run migrations from `pkg/database/migrations/`
- Verify PostgreSQL version >= 13

## Support

For issues, questions, or contributions:
- Open an issue on GitHub
- Review existing documentation in this repo
- Check `finetune/README.md` for fine-tuning help
