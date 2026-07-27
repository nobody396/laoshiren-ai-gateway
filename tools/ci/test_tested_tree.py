import tempfile
import unittest
from pathlib import Path
import subprocess
import urllib.request

from tested_tree import (
    CrossOriginRedirectHandler,
    fallback,
    record_attestation,
    successful_pr_runs,
)


class TestedTreeTests(unittest.TestCase):
    def test_cross_origin_redirect_drops_authorization(self) -> None:
        request = urllib.request.Request(
            "https://api.github.com/repos/owner/repo/actions/artifacts/1/zip",
            headers={"Authorization": "Bearer test-token"},
        )
        redirected = CrossOriginRedirectHandler().redirect_request(
            request,
            None,
            302,
            "Found",
            {},
            "https://artifact.example.test/signed-download",
        )

        self.assertIsNotNone(redirected)
        self.assertIsNone(redirected.get_header("Authorization"))

    def test_fallback_is_fail_closed(self) -> None:
        self.assertEqual(
            fallback("missing"),
            {"fast_path": False, "reason": "missing"},
        )

    def test_record_attestation_uses_exact_commit_and_tree(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            repo = Path(temp)
            subprocess.run(["git", "init", "-q", str(repo)], check=True)
            subprocess.run(["git", "-C", str(repo), "config", "user.name", "Test"], check=True)
            subprocess.run(
                ["git", "-C", str(repo), "config", "user.email", "test@example.com"],
                check=True,
            )
            (repo / "file.txt").write_text("tested\n", encoding="utf-8")
            subprocess.run(["git", "-C", str(repo), "add", "file.txt"], check=True)
            subprocess.run(["git", "-C", str(repo), "commit", "-qm", "test"], check=True)

            result = record_attestation(
                repo=repo,
                repository="owner/repo",
                pr_number=51,
                head_sha="a" * 40,
                base_sha="b" * 40,
                run_id=123,
            )

            self.assertEqual(result["version"], 1)
            self.assertEqual(result["repository"], "owner/repo")
            self.assertEqual(result["pr_number"], 51)
            self.assertEqual(result["workflow_run_id"], 123)
            self.assertEqual(len(result["tested_commit"]), 40)
            self.assertEqual(len(result["tested_tree"]), 40)

    def test_successful_pr_runs_does_not_depend_on_pull_requests_array(self) -> None:
        head_sha = "a" * 40
        runs = [
            {
                "id": 12,
                "event": "pull_request",
                "status": "completed",
                "conclusion": "success",
                "head_sha": head_sha,
                "pull_requests": [],
            },
            {
                "id": 11,
                "event": "pull_request",
                "status": "completed",
                "conclusion": "success",
                "head_sha": head_sha,
                "pull_requests": [{"number": 52}],
            },
            {
                "id": 13,
                "event": "push",
                "status": "completed",
                "conclusion": "success",
                "head_sha": head_sha,
                "pull_requests": [],
            },
        ]

        self.assertEqual(
            [run["id"] for run in successful_pr_runs(runs, head_sha)],
            [12, 11],
        )


if __name__ == "__main__":
    unittest.main()
