#!/bin/bash
# Validate Containerfile base images use pinned tags from trusted registries.

set -euo pipefail

CONTAINERFILE="${1:-Containerfile}"

if [[ ! -f "${CONTAINERFILE}" ]]; then
  echo "ERROR: ${CONTAINERFILE} not found"
  exit 1
fi

TRUSTED_REGISTRIES=(
  "registry.access.redhat.com"
  "registry.redhat.io"
  "registry.fedoraproject.org"
  "quay.io"
  "ghcr.io"
)

errors=0

while IFS= read -r line; do
  # Remove "FROM " prefix
  args="${line##FROM }"
  # Skip any flags (e.g., --platform=linux/amd64)
  while [[ "${args}" == --* ]]; do
    args="${args#* }"
  done
  # First remaining token is the image reference
  image="${args%% *}"
  # Strip build stage alias (e.g., "as builder")
  image="${image%% as *}"
  image="${image%% AS *}"

  # Extract the image name after the last slash for tag/digest checks
  image_name="${image##*/}"

  # Digest-pinned images (@sha256:...) are acceptable
  if [[ "${image}" == *"@sha256:"* ]]; then
    : # pinned by digest, acceptable
  elif [[ "${image_name}" == *":latest" ]]; then
    echo "ERROR: :latest tag used: ${image}"
    errors=$((errors + 1))
  elif [[ "${image_name}" != *":"* ]]; then
    echo "ERROR: no tag specified (implicit :latest): ${image}"
    errors=$((errors + 1))
  fi

  # Check for trusted registry
  trusted=false
  for registry in "${TRUSTED_REGISTRIES[@]}"; do
    if [[ "${image}" == "${registry}/"* ]]; then
      trusted=true
      break
    fi
  done

  if [[ "${trusted}" == "false" ]]; then
    echo "ERROR: untrusted registry: ${image}"
    errors=$((errors + 1))
  fi

done < <(grep -E '^FROM ' "${CONTAINERFILE}")

if [[ ${errors} -gt 0 ]]; then
  echo "FAIL: ${errors} Containerfile validation error(s) found"
  exit 1
fi

echo "OK: Containerfile validation passed"
