const fs = require("node:fs"),
  path = require("node:path"),
  cp = require("node:child_process");
const root = path.resolve("goassets/template/_task_mecca/framework/web/ui");
function check(folder) {
  for (const entry of fs.readdirSync(folder, { withFileTypes: true })) {
    const file = path.join(folder, entry.name);
    if (entry.isDirectory()) check(file);
    else if (entry.name.endsWith(".js")) {
      const result = cp.spawnSync(process.execPath, ["--check", file], {
        stdio: "inherit",
      });
      if (result.status) process.exit(result.status);
    }
  }
}
check(root);
