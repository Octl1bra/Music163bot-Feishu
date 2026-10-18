#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! $1 =~ ^bonjour\.fund/octl1bra/music163bot-feishu@sha256:[a-f0-9]{64}$ ]]; then
  echo "Usage: $0 bonjour.fund/octl1bra/music163bot-feishu@sha256:<64 lowercase hex characters>" >&2
  exit 2
fi

image=$1
repo_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
manifest=$(mktemp)
trap 'rm -f "$manifest"' EXIT

# Default to the company cluster; never depend on a shared current-context.
kube_context=${KUBE_CONTEXT:-bonjour}
sed "s|__IMAGE__|${image}|g" "$repo_dir/kubernetes/deployment.yaml" > "$manifest"
kubectl --context "$kube_context" --namespace internal-tools apply -f "$manifest"
kubectl --context "$kube_context" --namespace internal-tools rollout status deployment/music163bot --timeout=300s

actual_image=$(kubectl --context "$kube_context" --namespace internal-tools get deployment music163bot \
  -o 'jsonpath={.spec.template.spec.containers[?(@.name=="music163bot")].image}')
if [[ $actual_image != "$image" ]]; then
  echo "Deployment image mismatch: expected $image, got $actual_image" >&2
  exit 1
fi

pod_images=$(kubectl --context "$kube_context" --namespace internal-tools get pods -l app=music163bot \
  -o 'jsonpath={range .items[*]}{.spec.containers[?(@.name=="music163bot")].image}{"\n"}{end}')
if [[ -z $pod_images ]]; then
  echo 'No music163bot pods found after rollout' >&2
  exit 1
fi
while IFS= read -r pod_image; do
  if [[ $pod_image != "$image" ]]; then
    echo "Pod image mismatch: expected $image, got $pod_image" >&2
    exit 1
  fi
done <<< "$pod_images"

printf 'Deployed music163bot: %s\n' "$image"
kubectl --context "$kube_context" --namespace internal-tools get pods -l app=music163bot \
  -o 'jsonpath={range .items[*]}{.metadata.name}{"\t"}{.status.containerStatuses[?(@.name=="music163bot")].imageID}{"\n"}{end}'
