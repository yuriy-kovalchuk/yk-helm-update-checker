#!/bin/bash
# Deploy yk-update-checker Helm chart for testing
#
# Usage:
#   ./deploy-test.sh                    # uses default tag (Chart.appVersion)
#   ./deploy-test.sh v0.1.0             # uses specific tag
#   ./deploy-test.sh latest             # uses latest tag

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART_DIR="${SCRIPT_DIR}/../charts/yk-update-checker"
VALUES_FILE="${SCRIPT_DIR}/test-values.yaml"
NAMESPACE="${NAMESPACE:-yk-update-checker}"
RELEASE_NAME="${RELEASE_NAME:-yk-update-checker}"

TAG="${1:-}"

# Create namespace if it doesn't exist
kubectl get namespace "${NAMESPACE}" &>/dev/null || kubectl create namespace "${NAMESPACE}"

# Build helm args
HELM_ARGS=(
  upgrade --install "${RELEASE_NAME}"
  "${CHART_DIR}"
  --namespace "${NAMESPACE}"
  --values "${VALUES_FILE}"
)

if [[ -n "${TAG}" ]]; then
  HELM_ARGS+=(--set "image.tag=${TAG}")
  echo "Deploying with image tag: ${TAG}"
else
  echo "Deploying with default tag (Chart.appVersion)"
fi

# Deploy
helm "${HELM_ARGS[@]}"

echo ""
echo "Deployed! Check status with:"
echo "  kubectl get pods -n ${NAMESPACE}"
echo "  kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=yk-update-checker -f"
