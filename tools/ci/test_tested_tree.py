import tempfile
import unittest
from pathlib import Path
import subprocess

from tested_tree import fallback, record_attestation


class TestedTreeTests(unittest.TestCase):
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


if __name__ == "__main__":
    unittest.main()
