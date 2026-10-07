import json
from pathlib import Path
import shutil
import subprocess
import unittest


class RemovalHistoryUIContract(unittest.TestCase):
    def test_history_projection_copy_and_escaping(self):
        if not shutil.which("node"):
            self.skipTest("Node is required for renderer contract")
        root = Path(__file__).resolve().parents[1]
        go_app = root / "goassets/template/_task_mecca/framework/web/app.js"
        py_app = root / "src/task_mecca/template/_task_mecca/framework/web/app.js"
        self.assertEqual(go_app.read_bytes(), py_app.read_bytes())
        script = r"""
const fs=require('fs'),vm=require('vm');
const app=fs.readFileSync(process.argv[1],'utf8');
const helper=app.slice(app.indexOf('function hubRemovalObservation('),app.indexOf('function hubFeedback('));
const results=[];
for(const language of ['ko','en']) {
 const context={state:{language},esc:s=>String(s??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;'),hubText:s=>s};
 vm.createContext(context);vm.runInContext(helper,context);
 context.item={folder_outcome:'moved_to_trash',presence:'present',source_state:'present_after_trash',parent_holder_path:'/holder/<script>',staging_path:'/holder/.task-mecca-recycle-x/project',staging_holder_path:'/holder/.task-mecca-recycle-x',staging_holder_state:'retained_for_restore'};
 results.push(vm.runInContext('hubRemovalObservation(item)',context));
}
console.log(JSON.stringify(results));
"""
        result = subprocess.run(["node", "-e", script, str(go_app)], capture_output=True, text=True, check=True)
        ko, en = json.loads(result.stdout)
        self.assertIn("과거 처리 결과", ko)
        self.assertIn("현재 원래 경로", ko)
        self.assertIn("자동으로 다시 삭제하지 않습니다", ko)
        self.assertIn("Historical outcome", en)
        self.assertIn("Current original path", en)
        self.assertIn("not a deletion target", en)
        self.assertIn("Retained for OS Restore", en)
        self.assertIn("will not be deleted again automatically", en)
        for rendered in (ko, en):
            self.assertIn("&lt;script&gt;", rendered)
            self.assertNotIn("<script>", rendered)


if __name__ == "__main__":
    unittest.main()
