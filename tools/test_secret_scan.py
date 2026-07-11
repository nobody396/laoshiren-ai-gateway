import subprocess
import tempfile
import unittest
from pathlib import Path


class SecretScanTests(unittest.TestCase):
    def test_seeded_secret_is_redacted_and_nonzero(self) -> None:
        scanner = Path(__file__).with_name("secret_scan.py")
        with tempfile.TemporaryDirectory() as directory:
            fixture = Path(directory) / "fixture.txt"
            fixture.write_text("sk-" + "proj-" + "A" * 24, encoding="utf-8")
            result = subprocess.run(
                ["python3", str(scanner), str(fixture)],
                text=True,
                capture_output=True,
                check=False,
            )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("[REDACTED]", result.stderr)
        self.assertNotIn("A" * 24, result.stderr)


if __name__ == "__main__":
    unittest.main()
