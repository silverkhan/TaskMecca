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
