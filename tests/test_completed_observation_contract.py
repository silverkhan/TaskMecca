from pathlib import Path
import unittest


class CompletedObservationContractTests(unittest.TestCase):
    def test_templates_preserve_completion_observation_distinction(self):
        root = Path(__file__).resolve().parents[1]
        go = (root / "goassets/template/_task_mecca/framework/web/app.js").read_text()
        py = (root / "src/task_mecca/template/_task_mecca/framework/web/app.js").read_text()
        self.assertEqual(go, py)
        for text in (
            "작업 완료 확인, 당시 실행 관측은 미확인",
            "Task completion verified; runtime observation at the time remains unverified",
            "resolved_observations",
            "telegram_transport_disabled",
            "proof.event_id",
            "proof.assignment_id",
            "proof.attempt_id",
        ):
            self.assertIn(text, go)


if __name__ == "__main__":
    unittest.main()
