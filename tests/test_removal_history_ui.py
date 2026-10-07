import json
from pathlib import Path
import shutil
import subprocess
import unittest


class ArchiveUIContract(unittest.TestCase):
    def test_archive_renderer_copy_full_path_and_escaping(self):
        if not shutil.which("node"):
            self.skipTest("Node required")
        root = Path(__file__).resolve().parents[1]
        go_app = root / "goassets/template/_task_mecca/framework/web/app.js"
        py_app = root / "src/task_mecca/template/_task_mecca/framework/web/app.js"
        self.assertEqual(go_app.read_bytes(), py_app.read_bytes())
        script = r"""
const fs=require('fs'),vm=require('vm'),app=fs.readFileSync(process.argv[1],'utf8');
const helpers=app.slice(app.indexOf('function hubText('),app.indexOf('function hubFeedback('));
const view=app.slice(app.indexOf('function hubView('),app.indexOf('function normalizedVersion('));
const output=[];
for(const language of ['ko','en']){
 const context={state:{language,hub:{projects:[],cli:{}},hubManagement:{projects:[],archives:[{id:'archive-fixture',name:'<script>long Korean 프로젝트',path:'/full/path/<script>/project',archived_at:'2026-10-07'}]},hubHistoryExpanded:true},COPY_ICON:'copy',t:key=>key,esc:value=>String(value??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;')};
 vm.createContext(context);vm.runInContext(helpers+view,context);output.push(vm.runInContext('hubView()',context));
}
console.log(JSON.stringify(output));
"""
        result = subprocess.run(["node", "-e", script, str(go_app)], capture_output=True, text=True, check=True)
        ko, en = json.loads(result.stdout)
        self.assertTrue("보관한 프로젝트" in ko)
        self.assertTrue("다시 편입" in ko and "목록에서 제거" in ko)
        self.assertTrue("Archived projects" in en)
        for rendered in (ko, en):
            self.assertTrue("/full/path/&lt;script&gt;/project" in rendered)
            self.assertFalse("<script>" in rendered)
            self.assertFalse("Trash" in rendered)


if __name__ == "__main__":
    unittest.main()
