#!/usr/bin/env node
// Starts only an isolated foreground binary with public synthetic projects.
// No operating registry, user Preview switch, service installation or ledger is used.
const fs = require("node:fs"),
  os = require("node:os"),
  path = require("node:path"),
  cp = require("node:child_process"),
  net = require("node:net");
(async () => {
  const binary = path.resolve(process.argv[2] || ""),
    root =
      process.env.A65_PREVIEW_ROOT ||
      fs.mkdtempSync(path.join(os.tmpdir(), "task-mecca-a65-preview-")),
    management = path.join(root, "management");
  if (!fs.existsSync(binary))
    throw Error("Pass a built Task Mecca binary path");
  fs.mkdirSync(management, { recursive: true });
  const env = {
    ...process.env,
    TASK_MECCA_HOME: management,
    TASK_MECCA_PREVIEW_SANDBOX: "1",
    TASK_MECCA_TELEGRAM_TRANSPORT: "disabled",
  };
  const projects = [];
  for (const [key, name, n] of [
    ["task-mecca", "Task Mecca", 36],
    ["research", "Research Board", 14],
    ["release", "Release Desk", 8],
  ]) {
    const project = path.join(root, key),
      backlog = path.join(project, "_task_mecca/data/backlog");
    fs.mkdirSync(backlog, { recursive: true });
    for (let i = 1; i <= n; i++) {
      const id = "SYN-" + i,
        state = i === 1 ? "hold" : i < 5 ? "doing" : i < 10 ? "todo" : "done",
        title =
          i === 1
            ? "핵심 화면 디자인 검토를 기다립니다"
            : i === 2
              ? "프로젝트 관제와 작업 흐름 연결"
              : `${name} 합성 작업 ${i}`;
      fs.writeFileSync(
        path.join(
          backlog,
          `${String(i).padStart(6, "0")}.${id}.synthetic.${state}.md`,
        ),
        `# ${id} ${title}\n\n- Agent: /root/synthetic-worker\n- Tags: area:frontend, phase:prototype\n${i === 1 ? "- Type: user-wait\n- Hold: 사용자 확인이 필요합니다\n- 재개조건: 디자인 검토 후 재개\n" : ""}\n## 목적\n격리된 공개 합성 데이터로 실제 API와 화면 연결을 확인합니다.\n\n## 요구사항\nPC와 모바일에서 동일한 작업 근거를 확인합니다.\n\n## 완료 기준\n화면과 실제 조회 동작을 검토합니다.\n\n## 검증\n이 자료는 사용자 작업이나 실제 수행 결과가 아닙니다.\n`,
      );
    }
    const added = cp.spawnSync(binary, ["projects", "add", "--path", project], {
      env,
      encoding: "utf8",
    });
    if (added.status !== 0) throw Error(added.stderr || added.stdout);
    projects.push({ name, project, backlog, tasks: n });
  }
  const port =
    Number(process.env.A65_PREVIEW_PORT) ||
    (await new Promise((resolve) => {
      const s = net.createServer();
      s.listen(0, "127.0.0.1", () => {
        const p = s.address().port;
        s.close(() => resolve(p));
      });
    }));
  const log = fs.openSync(path.join(root, "server.log"), "a"),
    child = cp.spawn(
      binary,
      [
        "web",
        "--foreground",
        "--project",
        projects[0].project,
        "--host",
        "127.0.0.1",
        "--port",
        String(port),
        "--no-open",
      ],
      { env, stdio: ["ignore", log, log] },
    );
  const source = cp
      .spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" })
      .stdout.trim(),
    provenance = {
      synthetic: true,
      binary,
      source_head: source,
      root,
      management,
      projects,
      host: "127.0.0.1",
      port,
      url: "http://127.0.0.1:" + port,
      pid: child.pid,
      service_installed: false,
      user_preview_switched: false,
    };
  fs.writeFileSync(
    path.join(root, "provenance.json"),
    JSON.stringify(provenance, null, 2),
  );
  console.log(JSON.stringify(provenance));
  const stop = () => child.kill("SIGTERM");
  process.on("SIGINT", stop);
  process.on("SIGTERM", stop);
  child.on("exit", (code, signal) => {
    console.log(JSON.stringify({ stopped: true, code, signal, root }));
    process.exitCode = code || 0;
  });
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
