# web-ui/Dockerfile
# Multi-stage build: Node build → nginx serve
# Build: docker build -t whisper-web-ui .
# Run:   docker run -p 80:80 whisper-web-ui

# --- Build stage ---
FROM node:20-alpine AS build

WORKDIR /app

# Install dependencies first (layer caching)
COPY package*.json ./
RUN npm ci --production=false

# Copy source and build
COPY . .
RUN npm run build

# --- Serve stage ---
FROM nginx:alpine

# Remove default nginx config
RUN rm /etc/nginx/conf.d/default.conf

# Copy custom nginx config
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy built SPA from build stage
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
    CMD wget -q --spider http://localhost/healthz || exit 1

# Notes
# - The nginx.conf proxies /api/transcribe to whisper-svc in-cluster.
# - SPA routing uses try_files to serve index.html for all frontend routes.
# - The whisper backend URL can be overridden via the Helm chart's configmap.
# - Health check hits /healthz which returns 200 directly from nginx.
