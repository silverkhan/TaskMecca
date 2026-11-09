// Accepted photo/map/7px blur. Chromium uses the native backdrop compositor;
// nested faces and unsupported engines retain viewport-fixed photo sampling.
// No canvas, pixel engine, scroll geometry writes, or animation loop.
const root = document.documentElement;
const backdrop = document.querySelector('#material-review-backdrop img');
const host = document.querySelector('#material-review');
const ns = 'http://www.w3.org/2000/svg';
// Neutral center: only the outer ~8% displaces sampled background coordinates.
// Red controls X and green controls Y. This is a spatial lens map, not a glint.
const map = `<svg xmlns="${ns}" width="256" height="256"><defs><linearGradient id="x"><stop stop-color="#008080"/><stop offset=".08" stop-color="#808080"/><stop offset=".92" stop-color="#808080"/><stop offset="1" stop-color="#ff8080"/></linearGradient><linearGradient id="y" x2="0" y2="1"><stop stop-color="#800080"/><stop offset="1" stop-color="#808080"/></linearGradient><linearGradient id="b" x2="0" y2="1"><stop stop-color="#808080"/><stop offset="1" stop-color="#80ff80"/></linearGradient></defs><path fill="url(#x)" d="M0 0h256v256H0z"/><path fill="url(#y)" d="M0 0h256v20H0z"/><path fill="url(#b)" d="M0 236h256v20H0z"/></svg>`;
const defs = document.createElementNS(ns, 'svg');
defs.classList.add('material-optical-defs');
defs.setAttribute('aria-hidden', 'true');
defs.innerHTML = `<defs><filter id="material-refraction" x="-.1" y="-.1" width="1.2" height="1.2" color-interpolation-filters="sRGB"><feGaussianBlur in="SourceGraphic" stdDeviation="7" result="soft"/><feImage href="data:image/svg+xml,${encodeURIComponent(map)}" x="0" y="0" width="100%" height="100%" preserveAspectRatio="none" result="lens"/><feDisplacementMap in="soft" in2="lens" scale="22" xChannelSelector="R" yChannelSelector="G"/></filter></defs>`;
document.body.prepend(defs);
// SVG CSS filter support differs from CSS.supports syntax support. Chromium's
// implementation is the validated enhancement; others retain blur/transmission/rim.
root.dataset.materialOptics = /Chrome|Chromium|Edg\//.test(navigator.userAgent) ? 'svg' : 'blur';
const layers = new Map();
const recordsByLayer = new WeakMap();
const nativeBackdrop = root.dataset.materialOptics === 'svg' &&
  CSS.supports('backdrop-filter', 'blur(7px) url("#material-refraction")');
let filterSequence = 0;
// Percent lengths in the zero-sized defs viewport produce an empty feImage.
// Give each map its face's numeric dimensions, without reading scroll geometry.
const sizes = new ResizeObserver(entries => {
  for (const entry of entries) {
    const record = recordsByLayer.get(entry.target);
    if (!record) continue;
    const { width, height } = entry.contentRect;
    if (width > 0 && height > 0) {
      const w = String(width), h = String(height);
      if (record.lens.getAttribute('width') !== w) record.lens.setAttribute('width', w);
      if (record.lens.getAttribute('height') !== h) record.lens.setAttribute('height', h);
    }
  }
});
let disposed = false;
function sync() {
  if (disposed) return;
  if (![...layers.values()].some(record => !record.native)) return;
  const rect = backdrop.getBoundingClientRect();
  const scale = Math.max(rect.width / 1920, rect.height / 1080);
  const width = 1920 * scale, height = 1080 * scale;
  const x = rect.left + (rect.width - width) / 2;
  const y = rect.top + (rect.height - height) / 2;
  for (const { layer, native } of layers.values()) {
    if (native) continue;
    // Fixed attachment samples viewport coordinates directly. Scrolling moves
    // the face over that photo without JS geometry reads/background rewrites.
    const size=`auto, ${width}px ${height}px`,position=`0 0, ${x}px ${y}px`;
    if(layer.style.backgroundSize!==size)layer.style.backgroundSize=size;
    if(layer.style.backgroundPosition!==position)layer.style.backgroundPosition=position;
  }
}
const observer = new ResizeObserver(sync);
observer.observe(backdrop);
function refresh() {
  if (disposed) return;
  for (const [surface, record] of layers) {
    if (surface.isConnected) continue;
    sizes.unobserve(record.layer);
    record.filter.remove();
    layers.delete(surface);
  }
  document.querySelectorAll('.material-surface').forEach(surface => {
    let record = layers.get(surface);
    if (!record) {
      const layer = document.createElement('div');
      layer.className = 'material-transmission';
      layer.setAttribute('aria-hidden', 'true');
      const filter = defs.querySelector('#material-refraction').cloneNode(true);
      filter.id = `material-face-refraction-${++filterSequence}`;
      const lens = filter.querySelector('feImage');
      lens.setAttribute('width', '1'); lens.setAttribute('height', '1');
      defs.querySelector('defs').append(filter);
      record = { layer, filter, lens, native: false };
      layers.set(surface, record);
      recordsByLayer.set(layer, record);
      surface.prepend(layer);
      sizes.observe(layer);
    }
    // A nested face keeps the accepted direct photo sample: it must not sample
    // and blur the already tinted parent or the parent's foreground content.
    record.native = nativeBackdrop && !surface.parentElement.closest('.material-surface');
    surface.classList.toggle('material-native-surface', record.native);
    record.layer.classList.toggle('material-native-transmission', record.native);
    const blur = record.filter.querySelector('feGaussianBlur');
    if (record.native) {
      blur?.remove();
    } else if (!blur) {
      record.filter.prepend(defs.querySelector('#material-refraction feGaussianBlur').cloneNode(true));
    }
    const displacement = record.filter.querySelector('feDisplacementMap');
    const input = record.native ? 'SourceGraphic' : 'soft';
    if (displacement.getAttribute('in') !== input) displacement.setAttribute('in', input);
    const url = `url("#${record.filter.id}")`;
    const filter = record.native ? 'none' : root.dataset.materialOptics === 'svg' ? url : '';
    const backdropFilter = record.native ? `blur(7px) ${url}` : '';
    if (record.layer.style.filter !== filter) record.layer.style.filter = filter;
    if (record.layer.style.backdropFilter !== backdropFilter) record.layer.style.backdropFilter = backdropFilter;
  });
  sync();
}
const mutation = new MutationObserver(refresh);
mutation.observe(host, { childList: true, subtree: true });

addEventListener('resize', sync, { passive: true });
visualViewport?.addEventListener('resize', sync, { passive: true });
backdrop.addEventListener('load', sync);
refresh();
addEventListener('pagehide', () => {
  disposed = true; observer.disconnect(); sizes.disconnect(); mutation.disconnect();
  removeEventListener('resize', sync);
  visualViewport?.removeEventListener('resize', sync);
  backdrop.removeEventListener('load', sync);
}, { once: true });

export function refreshOptics(){refresh();}
