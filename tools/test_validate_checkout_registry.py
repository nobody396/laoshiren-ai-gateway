import json
import tempfile
import unittest
import sys
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from validate_checkout_registry import validate


class CheckoutRegistryTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.canonical = self.root / "canonical"
        self.release_only = self.root / "release-only"
        self.report = self.root / "report"
        self.unknown = self.root / "unknown"
        for path in (self.canonical, self.release_only, self.report, self.unknown):
            path.mkdir()
        self.registry = self.root / "registry.json"
        self.registry.write_text(json.dumps({"checkouts": [
            {"path": str(self.canonical), "kind": "canonical", "allowed_actions": ["release"]},
            {
                "path": str(self.release_only),
                "kind": "release-only",
                "allowed_actions": ["release"],
            },
            {"path": str(self.report), "kind": "report-only", "allowed_actions": ["report"]}
        ]}), encoding="utf-8")

    def tearDown(self) -> None:
        self.temp.cleanup()

    def test_canonical_release_passes(self) -> None:
        self.assertEqual(validate(self.registry, self.canonical, "release")["kind"], "canonical")

    def test_release_only_release_passes(self) -> None:
        self.assertEqual(
            validate(self.registry, self.release_only, "release")["kind"],
            "release-only",
        )

    def test_report_checkout_cannot_release(self) -> None:
        with self.assertRaisesRegex(ValueError, "forbids action"):
            validate(self.registry, self.report, "release")

    def test_unregistered_checkout_fails_closed(self) -> None:
        with self.assertRaisesRegex(ValueError, "unregistered"):
            validate(self.registry, self.unknown, "test", datetime.now(timezone.utc))


if __name__ == "__main__":
    unittest.main()
