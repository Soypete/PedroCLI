#!/usr/bin/env bash
# Deploy PedroCLI to k3s cluster via Helm
# Usage: ./ops/scripts/deploy.sh [--dry-run] [extra helm args...]
#
# Required env vars (or pass via --set-string):
#   NOTION_TOKEN   - Notion integration secret
#   CAL_API_KEY    - Cal.com API key
#
# Example:
#   op run --env-file=.env -- ./ops/scripts/deploy.sh
#   ./ops/scripts/deploy.sh --dry-run

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CHART_DIR="${REPO_ROOT}/ops/helm/pedrocli"

RELEASE_NAME="pedrocli"
NAMESPACE="pedrocli"

# Collect optional secret overrides
EXTRA_ARGS=()

if [[ -n "${NOTION_TOKEN:-}" ]]; then
    EXTRA_ARGS+=(--set-string "pedrocli.env.NOTION_TOKEN=${NOTION_TOKEN}")
fi

if [[ -n "${CAL_API_KEY:-}" ]]; then
    EXTRA_ARGS+=(--set-string "pedrocli.env.CAL_API_KEY=${CAL_API_KEY}")
fi

# Pass any additional args (e.g. --dry-run, --set foo=bar)
EXTRA_ARGS+=("$@")

echo "==> Deploying ${RELEASE_NAME} to namespace ${NAMESPACE}..."
echo "    Chart: ${CHART_DIR}"
echo "    Args: ${EXTRA_ARGS[*]:-none}"
echo ""

helm upgrade --install "${RELEASE_NAME}" "${CHART_DIR}" \
    --namespace "${NAMESPACE}" \
    --create-namespace \
    "${EXTRA_ARGS[@]}"

echo ""
echo "✓ Deployment complete"
echo ""
echo "Verify pods:"
echo "  kubectl get pods -n ${NAMESPACE}"
echo ""
echo "Check Tailscale MagicDNS (device appears after ~30s):"
echo "  tailscale status | grep pedrocli"
echo ""
echo "Watch logs:"
echo "  kubectl logs -n ${NAMESPACE} -l app=pedrocli-http -f"
