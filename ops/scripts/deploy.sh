#!/usr/bin/env bash
# Deploy PedroCLI to k3s cluster via Helm
# Usage: ./ops/scripts/deploy.sh [--dry-run] [extra helm args...]
#
# Secrets are pulled automatically from OpenBao at secret/apps/pedrocli.
# Override by setting env vars before running:
#   DATABASE_URL, NOTION_TOKEN, CAL_API_KEY
#
# Example:
#   ./ops/scripts/deploy.sh
#   ./ops/scripts/deploy.sh --dry-run

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CHART_DIR="${REPO_ROOT}/ops/helm/pedrocli"

RELEASE_NAME="pedrocli"
NAMESPACE="pedrocli"
OPENBAO_ADDR="${OPENBAO_ADDR:-http://100.81.89.62:8200}"
OPENBAO_SECRET_PATH="secret/apps/pedrocli"

# Pull secrets from OpenBao if not already set in environment
_bao_get() {
    VAULT_ADDR="${OPENBAO_ADDR}" vault kv get -field="$1" "${OPENBAO_SECRET_PATH}" 2>/dev/null || true
}

if [[ -z "${DATABASE_URL:-}" ]]; then
    echo "==> Fetching DATABASE_URL from OpenBao..."
    DATABASE_URL="$(_bao_get database_url)"
fi

if [[ -z "${NOTION_TOKEN:-}" ]]; then
    echo "==> Fetching NOTION_TOKEN from OpenBao..."
    NOTION_TOKEN="$(_bao_get notion_token)"
fi

if [[ -z "${CAL_API_KEY:-}" ]]; then
    echo "==> Fetching CAL_API_KEY from OpenBao..."
    CAL_API_KEY="$(_bao_get cal_api_key)"
fi

if [[ -z "${DATABASE_URL:-}" ]]; then
    echo "ERROR: DATABASE_URL not found in OpenBao (${OPENBAO_ADDR} ${OPENBAO_SECRET_PATH}) and not set in environment." >&2
    exit 1
fi

# Collect secret overrides
EXTRA_ARGS=(
    --set-string "pedrocli.env.DATABASE_URL=${DATABASE_URL}"
)

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
