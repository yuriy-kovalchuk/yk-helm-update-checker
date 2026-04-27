#!/bin/bash
# Deploy yk-update-checker Helm chart for local testing
#
# Usage:
#   ./deploy-test.sh                    # uses default tag (Chart.appVersion)
#   ./deploy-test.sh v0.2.0             # pins all three images to a specific tag
#   ./deploy-test.sh latest             # pins all three images to latest

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART_DIR="${SCRIPT_DIR}/../charts/yk-update-checker"
VALUES_FILE="${SCRIPT_DIR}/test-values.yaml"
NAMESPACE="${NAMESPACE:-yk-update-checker}"
RELEASE_NAME="${RELEASE_NAME:-yk-update-checker}"

TAG="${1:-}"

kubectl get namespace "${NAMESPACE}" &>/dev/null || kubectl create namespace "${NAMESPACE}"

HELM_ARGS=(
  upgrade --install "${RELEASE_NAME}"
  "${CHART_DIR}"
  --namespace "${NAMESPACE}"
  --values "${VALUES_FILE}"
)

if [[ -n "${TAG}" ]]; then
  HELM_ARGS+=(
    --set "api.image.tag=${TAG}"
    --set "scanner.image.tag=${TAG}"
    --set "dashboard.image.tag=${TAG}"
  )
  echo "Deploying with image tag: ${TAG}"
else
  echo "Deploying with default tag (Chart.appVersion)"
fi

helm "${HELM_ARGS[@]}"

echo ""
echo "Deployed! Useful commands:"
echo "  kubectl get pods -n ${NAMESPACE}"
echo "  kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=api -f"
echo "  kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/component=dashboard -f"
echo "  kubectl port-forward -n ${NAMESPACE} svc/${RELEASE_NAME}-dashboard 8080:80"
