# whisper-server/Dockerfile
# Builds whisper.cpp from source and runs whisper-server in HTTP mode.
# Build: docker build -t whisper-server .
# Run:   docker run -p 8080:8080 -v /path/to/models:/models whisper-server

FROM debian:bookworm-slim AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    cmake \
    git \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Clone and build whisper.cpp
RUN git clone --depth 1 https://github.com/ggerganov/whisper.cpp.git . \
    && cmake -B build -DWHISPER_BUILD_EXAMPLES=ON -DWHISPER_BUILD_SERVER=ON \
    && cmake --build build --config Release -j$(nproc) --target whisper-server

# Download default model at build time (base.en, ~142MB)
RUN bash ./models/download-ggml-model.sh base.en

# --- Runtime stage ---
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd -g 1000 whisper && \
    useradd -u 1000 -g whisper -m whisper

WORKDIR /app

# Copy built binary and default model
COPY --from=builder /build/build/bin/whisper-server /app/whisper-server
COPY --from=builder /build/models/ggml-base.en.bin /app/models/ggml-base.en.bin

# Create directories for transcripts and models
RUN mkdir -p /data/transcripts /models \
    && cp /app/models/ggml-base.en.bin /models/ggml-base.en.bin \
    && chown -R whisper:whisper /app /data /models

# Environment variables for runtime configuration
ENV WHISPER_MODEL_PATH=/models/ggml-base.en.bin
ENV WHISPER_THREADS=4
ENV WHISPER_PORT=8080

EXPOSE ${WHISPER_PORT}

USER whisper

ENTRYPOINT ["/bin/sh", "-c"]
CMD ["exec /app/whisper-server \
    --model ${WHISPER_MODEL_PATH} \
    --host 0.0.0.0 \
    --port ${WHISPER_PORT} \
    --threads ${WHISPER_THREADS} \
    --convert"]

# Notes
# - The --convert flag uses ffmpeg to convert incoming audio (WebM, mp3, etc.) to WAV
# - Default model (base.en) is baked into the image for zero-config startup
# - Mount a PVC at /models to use a larger model (e.g., ggml-large-v3.bin)
# - Mount a PVC at /data/transcripts for persistent transcript storage
# - Health check: GET /health on the configured port
