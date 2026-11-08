// Input-driven only: no ticker, animation recursion, canvas, blur, or global CSS mutation.
export function attachMaterialProximity(root, { radius = 150 } = {}) {
  const win = root.ownerDocument.defaultView;
  const fine = win.matchMedia('(hover: hover) and (pointer: fine)');
  const reduced = win.matchMedia('(prefers-reduced-motion: reduce)');
  const surfaces = new Map();
  let frame = 0, point = null, dirty = true, disposed = false;
  const stats = { events: 0, frames: 0, writes: 0, maxAffected: 0, flushMs: [], pending: false };
  const reset = () => {
    if (frame) win.cancelAnimationFrame(frame);
    frame = 0; point = null; stats.pending = false;
    for (const [node, record] of surfaces) {
      if (record.strength !== 0) { node.style.setProperty('--light-strength', '0'); stats.writes++; }
      record.strength = 0;
    }
  };
  const refresh = () => {
    for (const node of surfaces.keys()) if (!node.isConnected) { observer.unobserve(node); surfaces.delete(node); }
    for (const node of root.querySelectorAll('[data-light-surface]')) if (!surfaces.has(node)) surfaces.set(node, { rect: null, strength: 0 });
    dirty = true;
  };
  const flush = () => {
    frame = 0; stats.pending = false;
    if (disposed || !point || !fine.matches || reduced.matches) return;
    const started = win.performance.now();
    // Read every geometry first, then write the nearby surfaces as one input batch.
    if (dirty) { for (const [node, record] of surfaces) record.rect = node.getBoundingClientRect(); dirty = false; }
    let affected = 0;
    for (const [node, record] of surfaces) {
      const r = record.rect;
      const dx = Math.max(r.left - point.x, 0, point.x - r.right), dy = Math.max(r.top - point.y, 0, point.y - r.bottom);
      const distance = Math.hypot(dx, dy), strength = distance >= radius ? 0 : (1 - distance / radius) ** 2;
      if (!strength && !record.strength) continue;
      if (strength) {
        node.style.setProperty('--light-x', `${(point.x - r.left).toFixed(1)}px`);
        node.style.setProperty('--light-y', `${(point.y - r.top).toFixed(1)}px`);
        stats.writes += 2; affected++;
      }
      if (record.strength !== strength) { node.style.setProperty('--light-strength', strength.toFixed(4)); stats.writes++; }
      record.strength = strength;
    }
    stats.frames++; stats.maxAffected = Math.max(stats.maxAffected, affected);
    if (stats.flushMs.length < 600) stats.flushMs.push(win.performance.now() - started);
  };
  const move = event => {
    if (event.pointerType === 'touch' || !fine.matches || reduced.matches) return;
    stats.events++; point = { x: event.clientX, y: event.clientY };
    if (!frame) { stats.pending = true; frame = win.requestAnimationFrame(flush); }
  };
  const geometryChanged = () => { dirty = true; };
  const visibility = () => { if (root.ownerDocument.hidden) reset(); };
  const observer = new win.ResizeObserver(geometryChanged);
  refresh(); for (const node of surfaces.keys()) observer.observe(node);
  root.ownerDocument.addEventListener('pointermove', move, { passive: true });
  root.ownerDocument.addEventListener('pointerleave', reset);
  root.ownerDocument.addEventListener('visibilitychange', visibility);
  win.addEventListener('blur', reset);
  win.addEventListener('resize', geometryChanged, { passive: true });
  win.addEventListener('scroll', geometryChanged, { passive: true, capture: true });
  fine.addEventListener('change', reset); reduced.addEventListener('change', reset);
  return {
    refresh() { refresh(); for (const node of surfaces.keys()) observer.observe(node); },
    inspect: () => ({ ...stats, flushMs: [...stats.flushMs], surfaces: surfaces.size, enabled: !disposed && fine.matches && !reduced.matches }),
    dispose() {
      disposed = true; reset(); observer.disconnect();
      root.ownerDocument.removeEventListener('pointermove', move);
      root.ownerDocument.removeEventListener('pointerleave', reset);
      root.ownerDocument.removeEventListener('visibilitychange', visibility);
      win.removeEventListener('blur', reset); win.removeEventListener('resize', geometryChanged); win.removeEventListener('scroll', geometryChanged, true);
      fine.removeEventListener('change', reset); reduced.removeEventListener('change', reset); surfaces.clear();
    },
  };
}
