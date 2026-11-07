import { state } from "./state.js";
// One stream per visible screen. Existing server producers own source scans;
// this consumer never starts one stream per project or polls a fresh screen.
function newerRevision(next, current) {
  if (!next || next === current) return false;
  const a = String(next).match(/^(.*):(\d+)$/),
    b = String(current).match(/^(.*):(\d+)$/);
  return !(a && b && a[1] === b[1] && BigInt(a[2]) <= BigInt(b[2]));
}
let source = null,
  key = "",
  callback = null,
  retryTimer,
  attempts = 0;
export function closeLiveSource() {
  source?.close();
  source = null;
  key = "";
  clearTimeout(retryTimer);
}
export function connectLiveSource(
  onChange,
  onFailure = () => {},
  baseline = "",
) {
  if (document.hidden || typeof EventSource === "undefined") {
    closeLiveSource();
    return;
  }
  const global =
    ["home", "projects", "activity"].includes(state.view) && !state.detail;
  const next = global
    ? "/api/overview/events"
    : state.view === "backlog" && state.project && state.backlog
      ? "/api/events?" +
        new URLSearchParams({ project: state.project, backlog: state.backlog })
      : "";
  if (!next) {
    closeLiveSource();
    return;
  }
  callback = onChange;
  if (next === key && source) return;
  closeLiveSource();
  key = next;
  attempts = 0;
  let revision = global ? baseline : "";
  const open = () => {
    if (document.hidden || key !== next) return;
    const current = new EventSource(next);
    source = current;
    current.addEventListener("attention", (event) => {
      if (current !== source) return;
      let payload;
      try {
        payload = JSON.parse(event.data);
      } catch {
        return;
      }
      const nextRevision = JSON.stringify({
        attention: payload.attention,
        events: payload.notification_events,
        revision: payload.revision,
        signal_revision: payload.signal_revision,
        states: payload.states,
      });
      if (nextRevision === revision) return;
      const initial = !revision;
      revision = nextRevision;
      if (!initial) callback?.(payload);
    });
    current.addEventListener("overview", (event) => {
      if (current !== source) return;
      try {
        const payload = JSON.parse(event.data);
        if (payload.revision && !newerRevision(payload.revision, revision))
          return;
        revision = payload.revision || revision;
        callback?.(payload);
      } catch {}
    });
    current.onerror = () => {
      if (current !== source) return;
      current.close();
      source = null;
      if (attempts++ < 3)
        retryTimer = setTimeout(open, Math.min(8000, 1000 * 2 ** attempts));
      else onFailure();
    };
  };
  open();
}
