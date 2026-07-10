#!/usr/bin/env python3
import json
import os
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path

import release_contract


COMMIT = "a" * 40
OTHER_COMMIT = "b" * 40
DIGEST = "sha256:" + "1" * 64
OTHER_DIGEST = "sha256:" + "2" * 64
IMAGE_REPOSITORY = "ghcr.io/nobody396/laoshiren-ai-gateway"
IMAGE_TAG = f"{IMAGE_REPOSITORY}:{COMMIT}"
IMAGE_REF = f"{IMAGE_REPOSITORY}@{DIGEST}"
RELEASE_DIR = Path(__file__).resolve().parent
REPO_ROOT = RELEASE_DIR.parents[1]


def docker_inspect_payload(*, revision: str = COMMIT, digest: str = DIGEST) -> str:
    return json.dumps(
        [
            {
                "RepoDigests": [f"{IMAGE_REPOSITORY}@{digest}"],
                "Config": {
                    "Labels": {
                        "org.opencontainers.image.revision": revision,
                    }
                },
            }
        ]
    )


class ReleaseContractTests(unittest.TestCase):
    def test_publish_tag_must_equal_the_exact_commit(self) -> None:
        self.assertEqual(
            release_contract.validate_publish_tag(IMAGE_TAG, COMMIT),
            IMAGE_REPOSITORY,
        )

        with self.assertRaisesRegex(release_contract.ContractError, "exact commit"):
            release_contract.validate_publish_tag(f"{IMAGE_REPOSITORY}:main", COMMIT)

        with self.assertRaisesRegex(release_contract.ContractError, "exact commit"):
            release_contract.validate_publish_tag(
                f"{IMAGE_REPOSITORY}:{OTHER_COMMIT}", COMMIT
            )

    def test_deploy_input_accepts_digest_only(self) -> None:
        self.assertEqual(release_contract.validate_digest_ref(IMAGE_REF), IMAGE_REF)

        with self.assertRaisesRegex(release_contract.ContractError, "digest reference"):
            release_contract.validate_digest_ref(IMAGE_TAG)

        with self.assertRaisesRegex(release_contract.ContractError, "must not include a tag"):
            release_contract.validate_digest_ref(
                f"{IMAGE_REPOSITORY}:{COMMIT}@{DIGEST}"
            )

    def test_deployed_service_reference_is_canonicalized_to_digest_only(self) -> None:
        self.assertEqual(
            release_contract.canonicalize_deployed_ref(
                f"{IMAGE_REPOSITORY}:main@{DIGEST}"
            ),
            IMAGE_REF,
        )
        self.assertEqual(
            release_contract.canonicalize_deployed_ref(IMAGE_REF), IMAGE_REF
        )

        with self.assertRaisesRegex(release_contract.ContractError, "registry digest"):
            release_contract.canonicalize_deployed_ref(
                f"{IMAGE_REPOSITORY}:main"
            )

    def test_inspect_resolution_requires_matching_oci_revision(self) -> None:
        with self.assertRaisesRegex(release_contract.ContractError, "OCI revision"):
            release_contract.resolve_inspect_payload(
                docker_inspect_payload(revision=OTHER_COMMIT),
                IMAGE_TAG,
                COMMIT,
                DIGEST,
            )

    def test_inspect_resolution_requires_the_build_digest(self) -> None:
        with self.assertRaisesRegex(release_contract.ContractError, "build digest"):
            release_contract.resolve_inspect_payload(
                docker_inspect_payload(digest=OTHER_DIGEST),
                IMAGE_TAG,
                COMMIT,
                DIGEST,
            )

    def test_inspect_resolution_returns_immutable_reference(self) -> None:
        self.assertEqual(
            release_contract.resolve_inspect_payload(
                docker_inspect_payload(), IMAGE_TAG, COMMIT, DIGEST
            ),
            IMAGE_REF,
        )

    def test_shell_resolver_pulls_then_emits_only_the_digest_reference(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            docker_log = tmp_path / "docker.log"
            inspect_path = tmp_path / "inspect.json"
            inspect_path.write_text(docker_inspect_payload(), encoding="utf-8")

            fake_docker = tmp_path / "docker"
            fake_docker.write_text(
                textwrap.dedent(
                    """\
                    #!/usr/bin/env bash
                    set -euo pipefail
                    printf '%s\\n' "$*" >> "$FAKE_DOCKER_LOG"
                    if [[ "$1 $2" == "image inspect" ]]; then
                      cat "$FAKE_INSPECT_JSON"
                    fi
                    """
                ),
                encoding="utf-8",
            )
            fake_docker.chmod(0o755)

            env = os.environ.copy()
            env["PATH"] = f"{tmp_path}:{env['PATH']}"
            env["FAKE_DOCKER_LOG"] = str(docker_log)
            env["FAKE_INSPECT_JSON"] = str(inspect_path)

            result = subprocess.run(
                [
                    "bash",
                    str(RELEASE_DIR / "resolve_image_digest.sh"),
                    IMAGE_TAG,
                    COMMIT,
                    DIGEST,
                ],
                check=False,
                capture_output=True,
                text=True,
                env=env,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.strip(), IMAGE_REF)
            self.assertEqual(
                docker_log.read_text(encoding="utf-8").splitlines(),
                [
                    f"pull --quiet --platform linux/amd64 {IMAGE_TAG}",
                    f"image inspect {IMAGE_TAG}",
                ],
            )

    def test_shell_resolver_rejects_main_before_docker_is_called(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            docker_log = tmp_path / "docker.log"
            fake_docker = tmp_path / "docker"
            fake_docker.write_text(
                "#!/usr/bin/env bash\nprintf 'called' > \"$FAKE_DOCKER_LOG\"\n",
                encoding="utf-8",
            )
            fake_docker.chmod(0o755)
            env = os.environ.copy()
            env["PATH"] = f"{tmp_path}:{env['PATH']}"
            env["FAKE_DOCKER_LOG"] = str(docker_log)

            result = subprocess.run(
                [
                    "bash",
                    str(RELEASE_DIR / "resolve_image_digest.sh"),
                    f"{IMAGE_REPOSITORY}:main",
                    COMMIT,
                    DIGEST,
                ],
                check=False,
                capture_output=True,
                text=True,
                env=env,
            )

            self.assertNotEqual(result.returncode, 0)
            self.assertIn("exact commit", result.stderr)
            self.assertFalse(docker_log.exists(), "Docker must not run for a moving tag")

    def test_image_publication_is_only_called_from_the_required_ci_gate(self) -> None:
        ci_workflow = (REPO_ROOT / ".github/workflows/ci.yml").read_text(
            encoding="utf-8"
        )
        image_workflow = (
            REPO_ROOT / ".github/workflows/docker-image.yml"
        ).read_text(encoding="utf-8")

        self.assertIn("workflow_call:", image_workflow)
        self.assertNotIn("workflow_dispatch:", image_workflow)
        self.assertNotIn("branches:\n      - main", image_workflow)
        self.assertNotIn(":main", image_workflow)
        self.assertIn("uses: ./.github/workflows/docker-image.yml", ci_workflow)
        self.assertNotIn("docker/build-push-action", ci_workflow)
        self.assertIn("- backend-integration", ci_workflow)
        self.assertIn("- frontend", ci_workflow)
        self.assertIn(
            "publish: ${{ github.event_name == 'push' && github.ref == 'refs/heads/main' }}",
            ci_workflow,
        )
        self.assertIn(
            "tags: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ inputs.commit_sha }}",
            image_workflow,
        )
        self.assertIn(
            "org.opencontainers.image.revision=${{ inputs.commit_sha }}",
            image_workflow,
        )
        self.assertIn("context: ./deploy/maintenance", image_workflow)
        self.assertIn("MAINTENANCE_IMAGE_NAME", image_workflow)
        self.assertIn("maintenance_image_ref", image_workflow)
        self.assertNotIn("-maintenance:main", image_workflow)


if __name__ == "__main__":
    unittest.main()
