/* A-65 opt-in material pilot. The dev app owns all navigation and controls. */
(() => {
  try {
    const query = new URLSearchParams(location.search);
    if (query.has('floating')) {
      sessionStorage.setItem('task-mecca-dev-floating', query.get('floating') === '1' ? '1' : '0');
    }
    if (query.has('background')) {
      sessionStorage.setItem('task-mecca-dev-floating-background', query.get('background') === 'solid' ? 'solid' : 'image');
    }
    if (sessionStorage.getItem('task-mecca-dev-floating') !== '1') return;
    document.documentElement.dataset.floatingPilot = 'true';
    document.documentElement.dataset.floatingBackground = sessionStorage.getItem('task-mecca-dev-floating-background') || 'image';
  } catch (_) {
    // Storage unavailable: the unchanged dev UI remains usable.
  }
})();

// Native focus scrolling happens before dev Auto's focusin expansion changes
// the menu height. Reconcile only that keyboard event, after dev's own handler.
// No pointer tracking, frame loop, observers, or visual effect initialization.
document.addEventListener('focusin', event => {
  const root = document.documentElement;
  const item = event.target;
  if (!root.hasAttribute('data-floating-pilot') ||
      !window.matchMedia('(min-width: 681px)').matches ||
      !item.closest?.('#mobileSidebarPanel') ||
      document.querySelector('.app-shell')?.dataset.sidebarMode !== 'auto') return;
  queueMicrotask(() => {
    if (document.activeElement === item && item.isConnected)
      item.scrollIntoView({block: 'nearest', inline: 'nearest', behavior: 'instant'});
  });
});
