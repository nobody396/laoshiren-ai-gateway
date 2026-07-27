import unittest

from classify_changes import classify


class ClassifyChangesTests(unittest.TestCase):
    def test_frontend_only_uses_frontend_and_docker(self) -> None:
        self.assertEqual(
            classify(["frontend/src/main.ts", "frontend/package.json"]),
            {
                "backend": False,
                "integration": False,
                "frontend": True,
                "docker": True,
                "full": False,
            },
        )

    def test_backend_uses_backend_integration_and_docker(self) -> None:
        self.assertEqual(
            classify(["backend/internal/web/embed_on.go"]),
            {
                "backend": True,
                "integration": True,
                "frontend": False,
                "docker": True,
                "full": False,
            },
        )

    def test_docs_only_uses_no_build_gates(self) -> None:
        self.assertEqual(
            classify(["docs/ops/ENVIRONMENTS.md", "README.md"]),
            {
                "backend": False,
                "integration": False,
                "frontend": False,
                "docker": False,
                "full": False,
            },
        )

    def test_workflow_change_fails_open_to_full_matrix(self) -> None:
        self.assertTrue(all(classify([".github/workflows/ci.yml"]).values()))

    def test_unknown_path_fails_open_to_full_matrix(self) -> None:
        self.assertTrue(all(classify(["mystery/file.txt"]).values()))

    def test_empty_change_set_fails_open_to_full_matrix(self) -> None:
        self.assertTrue(all(classify([]).values()))


if __name__ == "__main__":
    unittest.main()
