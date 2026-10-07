from pathlib import Path
import shutil
import subprocess
import unittest


class BacklogListControlsTests(unittest.TestCase):
    @unittest.skipUnless(shutil.which("node"), "Node runtime required for frontend contract")
    def test_list_control_interactions(self):
        root = Path(__file__).resolve().parents[1]
        subprocess.run(["node", str(root / "tests/backlog_list_controls.test.cjs")], check=True, cwd=root)
