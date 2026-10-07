from pathlib import Path
import unittest


class WebAssetSyncTests(unittest.TestCase):
    def test_python_template_matches_go_embedded_web_assets(self):
        root = Path(__file__).resolve().parents[1]
        for filename in ("index.html", "app.js", "style.css"):
            go_asset = root / "goassets" / "template" / "_task_mecca" / "framework" / "web" / filename
            python_asset = root / "src" / "task_mecca" / "template" / "_task_mecca" / "framework" / "web" / filename
            self.assertEqual(
                go_asset.read_bytes(),
                python_asset.read_bytes(),
                f"Python template {filename} must stay in sync with the Go embedded asset",
            )
