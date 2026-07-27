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
MAINTENANCE_REPOSITORY = f"{IMAGE_REPOSITORY}-maintenance"
MAINTENANCE_REF = f"{MAINTENANCE_REPOSITORY}@{OTHER_DIGEST}"
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


def metadata_payload(**overrides: str) -> str:
    values = {
        "commit_sha": COMMIT,
        "image_ref": IMAGE_REF,
        "build_digest": DIGEST,
        "maintenance_image_ref": MAINTENANCE_REF,
        "maintenance_build_digest": OTHER_DIGEST,
    }
    values.update(overrides)
    return "\n".join(f"{key}={value}" for key, value in values.items()) + "\n"


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

    def test_release_checkout_gate_is_wired_before_deploy_ref_output(self) -> None:
        source = (RELEASE_DIR / "release_contract.py").read_text(encoding="utf-8")
        deploy_branch = source.split('if args.command == "validate-deploy-ref":', 1)[1]
        self.assertLess(
            deploy_branch.index("validate_release_checkout()"),
            deploy_branch.index("print(validate_digest_ref"),
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

    def test_metadata_requires_exact_commit_repositories_and_digests(self) -> None:
        result = release_contract.validate_metadata(
            metadata_payload(),
            expected_commit=COMMIT,
            image_repository=IMAGE_REPOSITORY,
            maintenance_image_repository=MAINTENANCE_REPOSITORY,
        )
        self.assertEqual(result["image_ref"], IMAGE_REF)
        self.assertEqual(result["maintenance_image_ref"], MAINTENANCE_REF)

        with self.assertRaisesRegex(release_contract.ContractError, "release commit"):
            release_contract.validate_metadata(
                metadata_payload(),
                expected_commit=OTHER_COMMIT,
                image_repository=IMAGE_REPOSITORY,
                maintenance_image_repository=MAINTENANCE_REPOSITORY,
            )
        with self.assertRaisesRegex(release_contract.ContractError, "build digest"):
            release_contract.validate_metadata(
                metadata_payload(build_digest=OTHER_DIGEST),
                expected_commit=COMMIT,
                image_repository=IMAGE_REPOSITORY,
                maintenance_image_repository=MAINTENANCE_REPOSITORY,
            )
        with self.assertRaisesRegex(release_contract.ContractError, "approved repository"):
            release_contract.validate_metadata(
                metadata_payload(),
                expected_commit=COMMIT,
                image_repository="ghcr.io/unapproved/image",
                maintenance_image_repository=MAINTENANCE_REPOSITORY,
            )

    def test_metadata_rejects_unknown_duplicate_and_missing_keys(self) -> None:
        common = {
            "expected_commit": COMMIT,
            "image_repository": IMAGE_REPOSITORY,
            "maintenance_image_repository": MAINTENANCE_REPOSITORY,
        }
        with self.assertRaisesRegex(release_contract.ContractError, "unknown key"):
            release_contract.validate_metadata(metadata_payload() + "extra=value\n", **common)
        with self.assertRaisesRegex(release_contract.ContractError, "duplicate key"):
            release_contract.validate_metadata(metadata_payload() + f"commit_sha={COMMIT}\n", **common)
        with self.assertRaisesRegex(release_contract.ContractError, "missing keys"):
            release_contract.validate_metadata(
                "\n".join(metadata_payload().splitlines()[:-1]) + "\n", **common
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

    def test_shell_resolver_retries_transient_registry_pull_failures(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            docker_log = tmp_path / "docker.log"
            attempt_file = tmp_path / "attempts"
            inspect_path = tmp_path / "inspect.json"
            inspect_path.write_text(docker_inspect_payload(), encoding="utf-8")

            fake_docker = tmp_path / "docker"
            fake_docker.write_text(
                textwrap.dedent(
                    """\
                    #!/usr/bin/env bash
                    set -euo pipefail
                    printf '%s\\n' "$*" >> "$FAKE_DOCKER_LOG"
                    if [[ "$1" == "pull" ]]; then
                      attempt=0
                      if [[ -f "$FAKE_ATTEMPT_FILE" ]]; then
                        attempt="$(cat "$FAKE_ATTEMPT_FILE")"
                      fi
                      attempt=$((attempt + 1))
                      printf '%s\\n' "$attempt" > "$FAKE_ATTEMPT_FILE"
                      if ((attempt < 3)); then
                        exit 1
                      fi
                    elif [[ "$1 $2" == "image inspect" ]]; then
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
            env["FAKE_ATTEMPT_FILE"] = str(attempt_file)
            env["FAKE_INSPECT_JSON"] = str(inspect_path)
            env["PULL_RETRY_SECONDS"] = "0"

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
            self.assertEqual(attempt_file.read_text(encoding="utf-8").strip(), "3")
            self.assertEqual(
                docker_log.read_text(encoding="utf-8").splitlines(),
                [
                    f"pull --quiet --platform linux/amd64 {IMAGE_TAG}",
                    f"pull --quiet --platform linux/amd64 {IMAGE_TAG}",
                    f"pull --quiet --platform linux/amd64 {IMAGE_TAG}",
                    f"image inspect {IMAGE_TAG}",
                ],
            )

    def test_image_publication_requires_main_fast_path_or_fallback_gates(self) -> None:
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
        self.assertIn("- frontend-tests", ci_workflow)
        self.assertIn("tested-tree-", ci_workflow)
        self.assertIn("needs.release-proof.outputs.fast_path == 'true'", ci_workflow)

        pr_start = ci_workflow.index("  docker-pr:")
        main_start = ci_workflow.index("  docker-main:")
        required_start = ci_workflow.index("  required:")
        self.assertIn("publish: false", ci_workflow[pr_start:main_start])
        self.assertNotIn("publish: true", ci_workflow[pr_start:main_start])
        self.assertIn("publish: true", ci_workflow[main_start:required_start])
        self.assertIn(
            "github.event_name == 'push' || github.event_name == 'workflow_dispatch'",
            ci_workflow[main_start:required_start],
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
