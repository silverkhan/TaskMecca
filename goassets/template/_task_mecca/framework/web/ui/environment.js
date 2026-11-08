import { text, esc } from "./format.js";

const sha40 = (value) => typeof value === "string" && /^[a-f0-9]{40}$/i.test(value);
const sha64 = (value) => typeof value === "string" && /^[a-f0-9]{64}$/i.test(value);
const present = (value) => typeof value === "string" && value.trim().length > 0;
const targetKey = (target) => JSON.stringify([
  target?.kind, target?.channel, target?.pr_number, target?.head_sha,
  target?.original_dev_version, target?.original_dev_sha256, target?.original_dev_backup,
]);
const prIdentity = (target) => target?.kind === "pr" && Number.isInteger(target.pr_number) && target.pr_number > 0 && sha40(target.head_sha);
const baselineRecorded = (target) => present(target?.original_dev_version) && sha64(target?.original_dev_sha256) && present(target?.original_dev_backup);

export function environmentModel(snapshot) {
  const env = snapshot.environment, health = snapshot.health;
  const target = env?.target?.kind === "pr" ? env.target : health?.execution_target?.kind === "pr" ? health.execution_target : env?.target || health?.execution_target;
  const model = { target, health, displayOrigin: target === env?.target ? "record" : "response", hidden: false, canReturn: false, reason: "", state: snapshot.phase, busy: snapshot.busy, identityVerified: false };
  if (snapshot.loading) { model.reason = text("실행 환경 확인 중", "Checking execution environment"); return model; }
  if (snapshot.error) { model.reason = snapshot.error; return model; }
  if (!snapshot.environmentOK || !snapshot.healthOK || !target || !["pr", "dev", "stable"].includes(target.kind)) {
    model.reason = text("실행 환경 미확인. 상태를 다시 확인하세요.", "Execution environment unverified. Check its status again."); return model;
  }
  if (health.preview_mode === true) {
    model.state = "preview";
    model.reason = text("격리 Preview입니다. 이 서버에서는 설치된 환경을 Dev로 복귀할 수 없습니다.", "This is an isolated Preview. It cannot restore the installed environment to Dev."); return model;
  }
  if (health.ok !== true || health.preview_mode !== false || !present(health.boot_id) || !present(health.instance_id) || !present(health.version)) {
    model.reason = text("관리 실행 서버의 identity를 확인하지 못했습니다. 복귀를 실행하지 않습니다.", "The managed execution server identity is unverified. Return is unavailable."); return model;
  }
  if (targetKey(env.target) !== targetKey(health.execution_target)) {
    model.reason = text("저장된 대상과 현재 실행 응답이 다릅니다. 상태를 다시 확인하세요.", "The recorded target and current execution response differ. Check the status again."); return model;
  }
  model.identityVerified = true;
  if (!Object.hasOwn(env, "transition")) {
    model.reason = text("전환 상태 미확인. 복귀를 실행하지 않습니다.", "Transition status is unverified. Return is unavailable."); return model;
  }
  if (env.transition) {
    model.reason = text("진행 중이거나 복구가 필요한 전환", "An active or recoverable transition") + ": " + String(env.transition.phase || "unknown"); return model;
  }
  if (target.kind !== "pr") { model.hidden = !snapshot.notice; return model; }
  if (!prIdentity(target) || target.channel !== "dev" || !baselineRecorded(target)) {
    model.reason = text("PR 식별자 또는 보관된 Dev 기록이 불완전합니다. 복귀를 실행하지 않습니다.", "The PR identity or retained Dev record is incomplete. Return is unavailable."); return model;
  }
  if (env.switch_supported !== true) {
    model.reason = text("이 운영체제에서는 Dev 복귀가 지원되지 않습니다.", "Return to Dev is unsupported on this operating system."); return model;
  }
  model.reason = text("실행 응답과 PR 기록 일치 · 복귀 권한과 보관 파일은 서버가 요청 시 검증합니다.", "Execution response matches the PR record. The server verifies return permission and the retained file on request.");
  model.canReturn = !snapshot.busy && !snapshot.blocked;
  return model;
}

function pause(ms, signal) {
  return new Promise((resolve, reject) => {
    const abort = () => { clearTimeout(timer); reject(new Error("Request cancelled")); };
    const timer = setTimeout(() => { signal?.removeEventListener("abort", abort); resolve(); }, ms);
    if (signal?.aborted) abort(); else signal?.addEventListener("abort", abort, { once: true });
  });
}

export function createEnvironmentController({
  request = (...args) => fetch(...args), confirm = (message) => window.confirm(message),
  reload = () => location.reload(), changed = () => {}, now = () => Date.now(), delay = pause,
  restartTimeout = 45000, pollInterval = 600,
} = {}) {
  const data = { environment: null, health: null, environmentOK: false, healthOK: false, loading: true, busy: false, blocked: false, phase: "checking", error: "", notice: "", actionError: "", failureKind: "" };
  let reading = null, readAbort = null, operationAbort = null, disposed = false;
  const emit = () => { if (!disposed) changed({ ...data }); };
  const snapshot = () => ({ ...data });
  async function json(url, options = {}) {
    const abort = new AbortController();
    const forward = () => abort.abort();
    options.signal?.addEventListener("abort", forward, { once: true });
    if (options.signal?.aborted) abort.abort();
    const timeout = setTimeout(() => abort.abort(), options.method === "POST" ? 120000 : 8000);
    try {
      const response = await request(url, { cache: "no-store", ...options, signal: abort.signal });
      let body;
      try { body = await response.json(); } catch { throw new Error(text("서버 응답을 읽지 못했습니다.", "The server response could not be read.")); }
      if (!response.ok) { const error = new Error(body?.error || "HTTP " + response.status); error.status = response.status; throw error; }
      return body;
    } finally { clearTimeout(timeout); options.signal?.removeEventListener("abort", forward); }
  }
  async function pair(signal) {
    const results = await Promise.allSettled([json("/api/environment", { signal }), json("/api/health", { signal })]);
    return { environment: results[0].status === "fulfilled" ? results[0].value : null, health: results[1].status === "fulfilled" ? results[1].value : null,
      environmentOK: results[0].status === "fulfilled", healthOK: results[1].status === "fulfilled",
      error: results.filter((result) => result.status === "rejected").map((result) => result.reason?.message || String(result.reason)).join(" · ") };
  }
  function refresh() {
    if (disposed || data.busy) return Promise.resolve(snapshot());
    if (reading) return reading;
    data.loading = true; data.phase = "checking"; emit();
    readAbort = new AbortController();
    const timeout = setTimeout(() => readAbort?.abort(), 8000);
    reading = (async () => {
      const result = await pair(readAbort.signal);
      if (disposed) return snapshot();
      if (result.environment) data.environment = result.environment;
      if (result.health) data.health = result.health;
      Object.assign(data, { environmentOK: result.environmentOK, healthOK: result.healthOK, loading: false,
        phase: result.error ? "error" : "ready", error: result.error, notice: "" });
      if (data.blocked) {
        const observed = environmentModel({ ...data, error: "", blocked: false });
        if (observed.identityVerified && data.environment?.target?.kind === "dev" && data.environment.transition === null) {
          data.blocked = false; data.error = ""; data.phase = "observed-dev";
          data.notice = text("현재 Dev 실행 응답을 확인했습니다. 이전 요청의 처리 성공을 판정한 것은 아닙니다.", "The current Dev execution response is verified. This does not establish success of the earlier request.");
        } else if (data.failureKind === "not-sent" && observed.canReturn) {
          data.blocked = false; data.error = ""; data.phase = "ready";
          data.notice = text("현재 실행 환경을 다시 확인했습니다. 이전 복귀 요청은 전송되지 않았습니다.", "The current environment was checked again. The previous return request was not sent.");
        } else {
          data.error = [data.actionError, result.error].filter(Boolean).join(" · ");
          data.phase = data.failureKind === "denied" ? "denied" : "unconfirmed";
        }
      }
      emit(); return snapshot();
    })().finally(() => { clearTimeout(timeout); readAbort = null; reading = null; });
    return reading;
  }
  async function waitForDev(previous, target, signal) {
    const started = now();
    while (!signal.aborted && now() - started < restartTimeout) {
      try {
        const health = await json("/api/health?env_wait=" + now(), { signal });
        if (health.ok === true && present(health.boot_id) && health.boot_id !== previous.boot_id && present(health.instance_id) &&
          health.preview_mode === false && health.execution_target?.kind === "dev" && health.execution_target?.channel === "dev" && health.version === target.original_dev_version) {
          const env = await json("/api/environment", { signal });
          if (env.target?.kind === "dev" && env.target.channel === "dev" && env.transition === null) {
            Object.assign(data, { environment: env, health, environmentOK: true, healthOK: true, phase: "verified", error: "", notice: text("새 Dev 실행 identity를 확인했습니다.", "The new Dev execution identity is verified.") });
            emit(); reload(); return;
          }
        }
      } catch { /* Restart temporarily disconnects the same-origin endpoint. */ }
      if (!signal.aborted) await delay(pollInterval, signal);
    }
    throw new Error(text("Web 재시작과 Dev identity를 확인하지 못했습니다. 복귀 성공으로 처리하지 않았습니다. 상태를 다시 확인하거나 task-mecca web status/logs를 확인하세요.", "Web restart and Dev identity were not verified. Return is not confirmed. Check status again or task-mecca web status/logs."));
  }
  async function returnToDev() {
    const model = environmentModel(data);
    if (disposed || data.busy || !model.canReturn) return false;
    const expected = data.environment.target;
    let submitted = false;
    data.busy = true; data.error = ""; data.notice = ""; data.actionError = ""; data.failureKind = ""; data.phase = "confirming"; emit();
    try {
      const approved = confirm(text(
        `PR #${expected.pr_number} (${expected.head_sha})에서 진입 직전 Dev ${expected.original_dev_version}로 복귀하시겠습니까?\n\nPR 실행 중 변경된 프로젝트 데이터는 자동 복원되지 않습니다.`,
        `Restore the retained Dev ${expected.original_dev_version} from PR #${expected.pr_number} (${expected.head_sha})?\n\nProject data changed while the PR ran is not reverted.`));
      if (!approved) { data.notice = text("Dev 복귀를 취소했습니다.", "Return to Dev cancelled."); data.phase = "ready"; return false; }
      operationAbort = new AbortController();
      data.phase = "checking"; emit();
      const fresh = await pair(operationAbort.signal);
      Object.assign(data, fresh);
      const gate = environmentModel({ ...data, busy: false, loading: false });
      if (targetKey(fresh.environment?.target) !== targetKey(expected)) {
        throw new Error(text("실행 대상이 변경됐습니다. 상태를 다시 확인한 뒤 선택하세요.", "The execution target changed. Check its status before returning."));
      }
      if (!gate.canReturn) throw new Error(gate.reason);
      data.phase = "returning"; emit();
      submitted = true;
      const response = await json("/api/environment/return-dev", { method: "POST", signal: operationAbort.signal,
        headers: { "Content-Type": "application/json", "X-Task-Mecca-Action": "1" }, body: JSON.stringify({ approve: true }) });
      if (response?.ok !== true || response.restarting !== true) throw new Error(text("복귀 요청 수락을 확인하지 못했습니다.", "The return request was not confirmed as accepted."));
      data.phase = "verifying"; emit();
      const restartDeadline = setTimeout(() => operationAbort?.abort(), restartTimeout);
      try { await waitForDev(fresh.health, expected, operationAbort.signal); }
      finally { clearTimeout(restartDeadline); }
      return true;
    } catch (error) {
      if (!disposed) {
        data.failureKind = error?.status === 403 ? "denied" : submitted ? "unconfirmed" : "not-sent";
        data.error = data.actionError = error?.message || String(error);
        data.phase = data.failureKind === "denied" ? "denied" : "unconfirmed";
        data.blocked = true; emit();
      }
      return false;
    } finally {
      data.busy = false; operationAbort = null; emit();
    }
  }
  return { snapshot, refresh, returnToDev, dispose() { disposed = true; readAbort?.abort(); operationAbort?.abort(); }, resume() { disposed = false; return refresh(); } };
}

let host = null, lastHTML = "", started = false;
const execution = createEnvironmentController({ changed: () => paint() });
export function environmentSlot() { return `<section id="executionEnvironment" class="execution-banner" aria-label="${text("실행 환경", "Execution environment")}" hidden></section>`; }
function bannerHTML(data, model) {
  const target = model.target, pr = target?.kind === "pr", current = model.health?.execution_target;
  const title = pr ? "PR #" + (Number.isInteger(target.pr_number) ? target.pr_number : text("미확인", "Unverified")) : model.state === "preview" ? text("격리 Preview", "Isolated Preview") : model.identityVerified && target?.kind === "dev" ? text("Dev 실행 환경", "Dev execution environment") : text("실행 환경 미확인", "Execution environment unverified");
  const identity = pr ? `<code class="execution-sha" aria-label="${text("전체 PR commit SHA", "Complete PR commit SHA")}">${esc(target.head_sha || text("SHA 미확인", "SHA unverified"))}</code>` : "";
  const recorded = data.environment?.target;
  const mismatch = current && recorded && targetKey(current) !== targetKey(recorded) ? `<p>${text("저장된 대상", "Recorded target")}: ${esc(recorded.kind)} ${recorded.kind === "pr" ? `#${esc(recorded.pr_number)} <code>${esc(recorded.head_sha || "")}</code>` : ""} · ${text("현재 실행 응답", "Current execution response")}: ${esc(current.kind)} ${current.kind === "pr" ? `#${esc(current.pr_number)} <code>${esc(current.head_sha || "")}</code>` : ""}</p>` : "";
  const baseline = pr ? `<details class="execution-baseline"><summary>${text("보관된 Dev 기록", "Retained Dev record")}: ${esc(target.original_dev_version || text("미확인", "Unverified"))}</summary><p>${text("실제 파일 존재와 checksum 유효성은 복귀 요청 시 서버가 검증합니다.", "The server checks file presence and checksum validity when return is requested.")}</p><code>${esc(target.original_dev_sha256 || text("미확인", "Unverified"))}</code><p class="execution-path">${esc(target.original_dev_backup || "")}</p></details>` : "";
  const busy = data.busy ? text(data.phase === "verifying" ? "재시작 확인 중…" : "복귀 확인 중…", data.phase === "verifying" ? "Verifying restart…" : "Checking return…") : text("Dev 복귀", "Return to Dev");
  const progress = { confirming: text("Dev 복귀 확인 대기", "Awaiting return confirmation"), checking: text("복귀 전 실행 상태를 다시 확인하고 있습니다.", "Checking execution status before return."), returning: text("서버가 Dev 복귀 요청을 처리하고 있습니다.", "The server is processing the Dev return request."), verifying: text("새 Dev Web의 재시작과 identity를 확인하고 있습니다.", "Verifying the restarted Dev Web identity.") };
  const prefix = data.failureKind === "denied" ? text("서버가 요청을 거부했습니다: ", "The server rejected the request: ") : data.failureKind === "unconfirmed" && data.error ? text("Dev 복귀 결과 미확인: ", "Dev return outcome unconfirmed: ") : data.failureKind === "not-sent" && data.error ? text("복귀 요청을 보내지 않았습니다: ", "The return request was not sent: ") : "";
  const statusMessage = data.busy ? progress[data.phase] || model.reason : prefix + (model.reason || data.notice || text("전환 상태 확인 중", "Checking transition status"));
  return `<div class="execution-identity"><strong>${esc(title)}</strong>${identity}<span>${model.displayOrigin === "record" ? text("저장된 설치 대상", "Recorded installation target") : text("현재 실행 응답 · 기록과 비교 필요", "Current execution response · Check against record")}</span></div><div class="execution-actions">${pr ? `<button id="prReturnDev" type="button" ${!model.canReturn ? "disabled" : ""}>${busy}</button>` : ""}<button id="executionRefresh" type="button" class="quiet" ${data.busy || data.loading ? "disabled" : ""}>${text("상태 재확인", "Check status")}</button></div><p class="execution-status" role="status" aria-live="polite">${esc(statusMessage)}</p>${data.notice && model.reason ? `<p>${esc(data.notice)}</p>` : ""}${mismatch}${baseline}`;
}
function paint() {
  const node = document.getElementById("executionEnvironment");
  if (!node) return;
  const data = execution.snapshot(), model = environmentModel(data);
  const compact = model.target?.kind !== "pr" && model.state !== "preview";
  let summary = document.getElementById("executionEnvironmentSummary");
  const context = document.querySelector(".topbar > span");
  if (compact) {
    node.hidden = true;
    if (node.innerHTML) node.innerHTML = "";
    host = node; lastHTML = "";
    if (model.hidden) { summary?.remove(); context?.classList.remove("execution-context"); return; }
    if (!summary && context) {
      summary = document.createElement("span"); summary.id = "executionEnvironmentSummary";
      context.classList.add("execution-context"); context.append(summary);
    }
    if (summary) {
      const label = data.loading ? text("실행 환경 확인 중", "Checking environment") : model.identityVerified ? text("Dev 응답 확인", "Dev response observed") : text("실행 환경 미확인", "Environment unverified");
      const explanation = model.reason || data.notice || label;
      const html = `<button id="executionRefresh" type="button" class="quiet execution-summary" title="${esc(explanation)}" aria-label="${esc(explanation + " · " + text("상태 재확인", "Check status"))}" ${data.busy || data.loading ? "disabled" : ""}>${esc(label)}</button>`;
      if (summary.innerHTML !== html) {
        summary.innerHTML = html;
        summary.querySelector("button")?.addEventListener("click", () => { void execution.refresh(); });
      }
    }
    return;
  }
  summary?.remove(); context?.classList.remove("execution-context");
  const html = model.hidden ? "" : bannerHTML(data, model);
  node.hidden = model.hidden; node.dataset.kind = model.target?.kind || model.state;
  if (node !== host || html !== lastHTML) {
    node.innerHTML = html; host = node; lastHTML = html;
    node.querySelector("#prReturnDev")?.addEventListener("click", () => {
      void execution.returnToDev().finally(() => {
        const button = document.querySelector("#prReturnDev:not(:disabled), #executionRefresh:not(:disabled)");
        button?.focus({ preventScroll: true });
      });
    });
    node.querySelector("#executionRefresh")?.addEventListener("click", () => { void execution.refresh(); });
  }
}
export function mountExecutionEnvironment() {
  if (!document.getElementById("executionEnvironmentStyle")) {
    const link = document.createElement("link"); link.id = "executionEnvironmentStyle"; link.rel = "stylesheet";
    link.href = "/ui/environment.css"; document.head.append(link);
  }
  paint();
  if (!started) { started = true; void execution.refresh(); }
}
export const refreshExecutionEnvironment = () => execution.refresh();
export const stopExecutionEnvironment = () => execution.dispose();
export const resumeExecutionEnvironment = () => execution.resume();
