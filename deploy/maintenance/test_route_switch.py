#!/usr/bin/env python3
from __future__ import annotations

import contextlib
import io
import os
from pathlib import Path
import tempfile
import unittest

import route_switch


APP = "laoshirenai-app-tazu5m"
MAINTENANCE = "laoshirenai-maintenance"
SECRET_PLACEHOLDER = "origin-secret-placeholder-never-print"


def route_config(service: str = APP, *, occurrences: int = 3) -> str:
    services = "\n".join(
        f"    service-{index}:\n      loadBalancer:\n        servers:\n          - url: http://{service}:8080"
        for index in range(occurrences)
    )
    return (
        "http:\n"
        "  routers:\n"
        "    secure:\n"
        f"      rule: Host(`example.test`) && Header(`X-Origin`, `{SECRET_PLACEHOLDER}`)\n"
        "  services:\n"
        f"{services}\n"
    )


class RouteSwitchTests(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.config = self.root / "route.yml"
        self.backup = self.root / "route.yml.pre-maintenance"
        self.original = route_config()
        self.config.write_text(self.original, encoding="utf-8")

    def tearDown(self) -> None:
        self.tmp.cleanup()

    def test_switch_and_restore_preserve_unrelated_secret_text(self) -> None:
        result = route_switch.switch_route(
            self.config,
            self.backup,
            APP,
            MAINTENANCE,
            port=8080,
            expected_count=3,
        )

        self.assertEqual(result["status"], "switched")
        switched = self.config.read_text(encoding="utf-8")
        self.assertEqual(switched.count(f"http://{MAINTENANCE}:8080"), 3)
        self.assertIn(SECRET_PLACEHOLDER, switched)
        self.assertEqual(self.backup.read_text(encoding="utf-8"), self.original)
        self.assertEqual(os.stat(self.config).st_mode & 0o777, 0o600)
        self.assertEqual(os.stat(self.backup).st_mode & 0o777, 0o600)

        restored = route_switch.restore_route(
            self.config,
            self.backup,
            MAINTENANCE,
            APP,
            port=8080,
            expected_count=3,
        )
        self.assertEqual(restored["status"], "restored")
        self.assertEqual(self.config.read_text(encoding="utf-8"), self.original)

    def test_second_switch_is_idempotent_when_secure_backup_exists(self) -> None:
        route_switch.switch_route(
            self.config, self.backup, APP, MAINTENANCE, port=8080, expected_count=3
        )
        result = route_switch.switch_route(
            self.config, self.backup, APP, MAINTENANCE, port=8080, expected_count=3
        )
        self.assertEqual(result["status"], "already_switched")

    def test_switch_can_be_reactivated_after_a_verified_restore(self) -> None:
        route_switch.switch_route(
            self.config, self.backup, APP, MAINTENANCE, port=8080, expected_count=3
        )
        route_switch.restore_route(
            self.config, self.backup, MAINTENANCE, APP, port=8080, expected_count=3
        )
        result = route_switch.switch_route(
            self.config, self.backup, APP, MAINTENANCE, port=8080, expected_count=3
        )
        self.assertEqual(result["status"], "switched_reusing_backup")
        self.assertEqual(
            self.config.read_text(encoding="utf-8").count(
                f"http://{MAINTENANCE}:8080"
            ),
            3,
        )

    def test_wrong_topology_count_refuses_without_mutation(self) -> None:
        self.config.write_text(route_config(occurrences=2), encoding="utf-8")
        with self.assertRaisesRegex(route_switch.RouteSwitchError, "occurrence count"):
            route_switch.switch_route(
                self.config,
                self.backup,
                APP,
                MAINTENANCE,
                port=8080,
                expected_count=3,
            )
        self.assertEqual(self.config.read_text(encoding="utf-8"), route_config(occurrences=2))
        self.assertFalse(self.backup.exists())

    def test_restore_refuses_when_current_backend_is_unexpected(self) -> None:
        self.backup.write_text(self.original, encoding="utf-8")
        self.config.write_text(route_config("unexpected-service"), encoding="utf-8")
        with self.assertRaisesRegex(route_switch.RouteSwitchError, "not fully pointed"):
            route_switch.restore_route(
                self.config,
                self.backup,
                MAINTENANCE,
                APP,
                port=8080,
                expected_count=3,
            )

    def test_restore_refuses_a_tampered_backup_topology(self) -> None:
        route_switch.switch_route(
            self.config, self.backup, APP, MAINTENANCE, port=8080, expected_count=3
        )
        self.backup.write_text(route_config("unexpected-service"), encoding="utf-8")
        with self.assertRaisesRegex(route_switch.RouteSwitchError, "restore topology"):
            route_switch.restore_route(
                self.config,
                self.backup,
                MAINTENANCE,
                APP,
                port=8080,
                expected_count=3,
            )

    def test_second_restore_is_idempotent(self) -> None:
        route_switch.switch_route(
            self.config, self.backup, APP, MAINTENANCE, port=8080, expected_count=3
        )
        route_switch.restore_route(
            self.config, self.backup, MAINTENANCE, APP, port=8080, expected_count=3
        )
        result = route_switch.restore_route(
            self.config, self.backup, MAINTENANCE, APP, port=8080, expected_count=3
        )
        self.assertEqual(result["status"], "already_restored")

    def test_symlink_config_is_refused(self) -> None:
        actual = self.root / "actual.yml"
        actual.write_text(self.original, encoding="utf-8")
        self.config.unlink()
        self.config.symlink_to(actual)
        with self.assertRaisesRegex(route_switch.RouteSwitchError, "non-symlink"):
            route_switch.switch_route(
                self.config,
                self.backup,
                APP,
                MAINTENANCE,
                port=8080,
                expected_count=3,
            )

    def test_cli_output_never_contains_route_secret(self) -> None:
        stdout = io.StringIO()
        stderr = io.StringIO()
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            status = route_switch.main(
                [
                    "switch",
                    "--config",
                    str(self.config),
                    "--backup",
                    str(self.backup),
                    "--from-service",
                    APP,
                    "--to-service",
                    MAINTENANCE,
                ]
            )
        self.assertEqual(status, 0)
        self.assertNotIn(SECRET_PLACEHOLDER, stdout.getvalue())
        self.assertNotIn(SECRET_PLACEHOLDER, stderr.getvalue())


if __name__ == "__main__":
    unittest.main()
