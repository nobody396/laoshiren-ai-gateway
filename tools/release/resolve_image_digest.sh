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
pull_attempts="${PULL_ATTEMPTS:-3}"
pull_retry_seconds="${PULL_RETRY_SECONDS:-3}"

# Reject moving tags before any registry request.
python3 "$contract" validate-publish \
  --image-tag "$image_tag" \
  --expected-commit "$expected_commit"

# The production image is linux/amd64. Pulling the exact commit tag provides
# both its registry RepoDigest and its OCI config labels for verification.
# Registry TLS/proxy handshakes can fail transiently on self-hosted runners, so
# retry the same immutable tag without weakening digest/label verification.
pull_succeeded=0
for ((attempt = 1; attempt <= pull_attempts; attempt++)); do
  if docker pull --quiet --platform linux/amd64 "$image_tag" >&2; then
    pull_succeeded=1
    break
  fi
  if ((attempt < pull_attempts)); then
    echo "Registry pull attempt $attempt/$pull_attempts failed; retrying." >&2
    sleep "$pull_retry_seconds"
  fi
done
[[ "$pull_succeeded" == "1" ]] || {
  echo "Unable to pull the immutable image after $pull_attempts attempts." >&2
  exit 1
}

docker image inspect "$image_tag" | python3 "$contract" resolve-inspect \
  --image-tag "$image_tag" \
  --expected-commit "$expected_commit" \
  --expected-digest "$expected_build_digest"
