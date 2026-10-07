import pathlib
import unittest


class HubArchiveContractTests(unittest.TestCase):
    def test_persistent_accessible_archive_and_choice_dialog(self):
        root = pathlib.Path(__file__).resolve().parents[1]
        for prefix in ("goassets/template", "src/task_mecca/template"):
            source = (root / prefix / "_task_mecca/framework/web/app.js").read_text(encoding="utf-8")
            required = ('id="hubHistoryToggle"', 'aria-controls="hubHistoryItems"',
                        'task-mecca-hub-history-expanded-v1', 'aria-expanded="${state.hubHistoryExpanded}"',
                        'hubHistoryFocus', 'migrationChoiceDialog', 'choice_required',
                        'data-migration-choice="cancel"', "event.key==='Escape'", "trigger?.focus()",
                        'data-project-action="restore"', 'data-project-action="forget"')
            missing = [value for value in required if value not in source]
            self.assertEqual(missing, [], "archive/choice contract missing")
            for forbidden in ('hubRemovalObservation', 'hubTrashLocation', 'present_after_trash',
                              'staging_path', 'folder_outcome', 'data-project-action="trash"',
                              'data-project-action="check"', 'data-project-action="delete-history"'):
                self.assertFalse(forbidden in source, f"obsolete project cleanup: {forbidden}")


if __name__ == "__main__":
    unittest.main()
