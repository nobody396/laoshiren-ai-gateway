#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: $0 IMAGE_TAG EXPECTED_COMMIT EXPECTED_BUILD_DIGEST" >&2
  exit 64
fi

image_tag="$1"
expected_commit="$2"
expected_build_digest="$3"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
contract="$script_dir/release_contract.py"

# Reject moving tags before any registry request.
python3 "$contract" validate-publish \
  --image-tag "$image_tag" \
  --expected-commit "$expected_commit"

# The production image is linux/amd64. Pulling the exact commit tag provides
# both its registry RepoDigest and its OCI config labels for verification.
docker pull --quiet --platform linux/amd64 "$image_tag" >&2
docker image inspect "$image_tag" | python3 "$contract" resolve-inspect \
  --image-tag "$image_tag" \
  --expected-commit "$expected_commit" \
  --expected-digest "$expected_build_digest"
