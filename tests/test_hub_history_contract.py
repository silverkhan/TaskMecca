import pathlib
import unittest


class HubHistoryContractTests(unittest.TestCase):
    def test_persistent_accessible_disclosure_and_recovery_copy(self):
        root = pathlib.Path(__file__).resolve().parents[1]
        for prefix in ("goassets/template", "src/task_mecca/template"):
            source = (root / prefix / "_task_mecca/framework/web/app.js").read_text(encoding="utf-8")
            for required in (
                'id="hubHistoryToggle"',
                'aria-expanded="${state.hubHistoryExpanded}"',
                'aria-controls="hubHistoryItems"',
                'id="hubHistoryItems"',
                "task-mecca-hub-history-expanded-v1",
                "hubHistoryFocus",
                "cleanup_unknown",
                "staging_path",
                "Never overwrite an existing folder",
            ):
                self.assertIn(required, source)


if __name__ == "__main__":
    unittest.main()
