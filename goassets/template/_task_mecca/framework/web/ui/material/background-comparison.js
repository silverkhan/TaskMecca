// Review-only backdrop choice; optical sampling lives in transmissive.js.
import './transmissive.js';
const root = document.documentElement;
const storageKey = 'task-mecca-material-review-background';
const choices = ['image', 'solid'];
const query = new URLSearchParams(location.search).get('background');
let saved;
try { saved = localStorage.getItem(storageKey); } catch { /* optional preference */ }
let mode = choices.includes(query) ? query : choices.includes(saved) ? saved : 'image';
const section = document.createElement('section');
section.className = 'material-background-comparison';
section.setAttribute('aria-labelledby', 'material-background-title');
section.innerHTML = `<h2 id="material-background-title">배경 비교</h2><div class="material-theme-control" role="group" aria-label="검토 시안 배경"><button type="button" data-material-background="image" aria-pressed="false">이미지</button><button type="button" data-material-background="solid" aria-pressed="false">단색</button></div><p>AI 생성 실내 이미지입니다. 사이드바·툴바·카드는 같은 위치의 배경을 흐리게 투과합니다. 지원 브라우저에서는 가장자리의 배경 위치도 왜곡합니다. 근접광은 반사 표현입니다.</p><p id="material-background-feedback" role="status" aria-live="polite"></p>`;
document.querySelector('.material-review-notes').prepend(section);
function apply() {
  root.dataset.materialBackground = mode;
  section.querySelectorAll('[data-material-background]').forEach(button => button.setAttribute('aria-pressed', String(button.dataset.materialBackground === mode)));
}
section.addEventListener('click', event => {
  const button = event.target.closest('button[data-material-background]');
  if (!button) return;
  mode = button.dataset.materialBackground;
  try { localStorage.setItem(storageKey, mode); } catch { /* works without storage */ }
  apply();
});
const image = document.querySelector('#material-review-backdrop img');
image.addEventListener('error', () => {
  mode = 'solid'; apply();
  section.querySelector('#material-background-feedback').textContent = '배경 이미지를 읽지 못해 단색으로 표시합니다.';
});
apply();
