/* AID-246 · Dev-only read-only performance report.
   The server decides whether this functionality is available. */
(() => {
  'use strict';
  const route = '/api/monitor-performance';
  const root = document.getElementById('mobileSidebarPanel');
  if (!root) return;
  const ko = () => (document.documentElement.lang || localStorage.getItem('task-mecca-language') || 'ko').toLowerCase().startsWith('ko');
  const T = (a,b) => ko() ? a : b;
  let overlay = null;
  let latest = null;
  let refreshTimer = null;
  const button = document.createElement('button');
  button.type = 'button';
  button.id = 'perfDiagnosticsNav';
  button.className = 'nav-item';
  button.hidden = true;
  button.title = 'Dev performance diagnostics';
  button.innerHTML = '<span class="nav-main"><span class="nav-icon" aria-hidden="true">◴</span><span class="nav-text">성능 진단 (Dev)</span></span>';
  const sidebarSection = document.getElementById('terminalNavBtn')?.closest('.sidebar-section');
  if (!sidebarSection) return;
  sidebarSection.appendChild(button);

  const style = document.createElement('style');
  style.textContent = `
    .perf-overlay[hidden]{display:none!important}
    .perf-overlay{position:fixed;inset:0;z-index:5000;background:rgba(5,9,20,.72);display:flex;align-items:center;justify-content:center;padding:20px}
    .perf-dialog{width:min(820px,100%);max-height:90vh;overflow:auto;color:var(--text);background:var(--panel,var(--surface,#141b2a));border:1px solid var(--border);border-radius:16px;box-shadow:0 24px 70px rgba(0,0,0,.4);padding:24px;font:inherit}
    .perf-head{display:flex;align-items:flex-start;gap:12px;justify-content:space-between}
    .perf-head h2{font-size:20px;margin:0 0 5px}
    .perf-note{font-size:12px;line-height:1.55;color:var(--muted)}
    .perf-actions{display:flex;gap:8px;flex-wrap:wrap;margin:18px 0}
    .perf-actions button,.perf-close{border:1px solid var(--border);color:var(--text);border-radius:8px;background:transparent;padding:9px 12px;cursor:pointer;font:inherit}
    .perf-actions button:focus-visible,.perf-close:focus-visible{outline:2px solid var(--accent,#7faeff)}
    .perf-metrics{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px;margin-bottom:14px}
    .perf-stat{border:1px solid var(--border);border-radius:10px;padding:13px;min-width:0}
    .perf-stat strong{font-size:22px;display:block;margin-top:5px}
    .perf-scroll{overflow-x:auto}
    .perf-table{width:100%;border-collapse:collapse;font-size:12px}
    .perf-table th,.perf-table td{padding:10px 9px;border-bottom:1px solid var(--border);text-align:right;white-space:nowrap}
    .perf-table th:first-child,.perf-table td:first-child{text-align:left}
    .perf-status{font-size:13px;min-height:22px;color:var(--muted)}
    .perf-disclaimer{margin-top:15px;border-top:1px solid var(--border);padding-top:12px}
    @media(max-width:600px){.perf-overlay{padding:7px}.perf-dialog{padding:14px;max-height:96vh}.perf-metrics{grid-template-columns:repeat(2,minmax(0,1fr))}}
  `;
  document.head.appendChild(style);

  function elem(tag, className, text) {
    const e = document.createElement(tag);
    if (className) e.className = className;
    if (text != null) e.textContent = String(text);
    return e;
  }
  function aggregate(snapshot) {
    const totals = {};
    const windows = [...(snapshot.history || []), snapshot.current].filter(Boolean);
    for (const w of windows) {
      for (const [kind, m] of Object.entries(w.metrics || {})) {
        const x = totals[kind] || (totals[kind] = {calls:0,skipped:0,changed:0,items:0,events:0,total_ms:0,max_ms:0});
        for (const key of ['calls','skipped','changed','items','events','total_ms']) x[key] += Number(m[key] || 0);
        x.max_ms = Math.max(x.max_ms,Number(m.max_ms || 0));
      }
    }
    return {windows,totals};
  }
  const number = n => Number(n || 0).toLocaleString(undefined,{maximumFractionDigits:1});
  function report(snapshot) {
    const {windows,totals} = aggregate(snapshot);
    const lines = [
      'Task Mecca · Dev performance report',
      'Captured: '+snapshot.captured_at,
      'Version: '+snapshot.version,
      'Collection: '+(snapshot.enabled?'enabled':'disabled'),
      'Window count: '+windows.length,
      'Active attention feeds: '+snapshot.active_attention_feeds,
      'Go goroutines: '+snapshot.goroutines,
      'Heap allocated bytes: '+snapshot.heap_alloc_bytes,
      'Measurement: wall-clock elapsed time; NOT CPU time or battery drain',
      'Counts: observed attention rows/operation stages, NOT full backlog or runtime inventory',
      '',
      'Category | Calls | Skipped | Revision changes | Observed items | Events/incidents | Total ms | Avg ms | Max ms'
    ];
    for (const [kind,m] of Object.entries(totals).sort((a,b)=>a[0].localeCompare(b[0]))) {
      lines.push([kind,m.calls,m.skipped,m.changed,m.items,m.events,number(m.total_ms),number(m.calls?m.total_ms/m.calls:0),number(m.max_ms)].join(' | '));
    }
    if (!Object.keys(totals).length) lines.push('No measurements yet.');
    lines.push('','Recent windows:');
    for (const w of windows) {
      lines.push(w.from+' — '+w.to);
      for (const [kind,m] of Object.entries(w.metrics||{})) lines.push('  '+kind+': calls='+m.calls+' skipped='+m.skipped+' changed='+m.changed+' total_ms='+number(m.total_ms)+' max_ms='+number(m.max_ms));
    }
    lines.push('','User observation (idle / browser open / multi-project): ');
    lines.push('CPU sample captured separately: yes / no');
    return lines.join('\n');
  }
  function draw() {
    if (!overlay || !latest) return;
    const box = overlay.querySelector('[data-perf-body]');
    box.replaceChildren();
    const {totals,windows} = aggregate(latest);
    const entries = Object.entries(totals).sort((a,b)=>b[1].total_ms-a[1].total_ms);
    const calls=entries.reduce((n,[,m])=>n+m.calls,0);
    const skipped=entries.reduce((n,[,m])=>n+m.skipped,0);
    const total=entries.reduce((n,[,m])=>n+m.total_ms,0);
    const grid=elem('div','perf-metrics');
    const cards=[
      [T('측정된 감시 호출','Measured monitor calls'),number(calls)],
      [T('건너뛴 감시','Skipped checks'),number(skipped)],
      [T('누적 경과 시간','Total elapsed (wall)'),number(total)+' ms'],
      [T('알림 감시 피드','Attention feeds'),number(latest.active_attention_feeds)],
      [T('Go 고루틴','Go goroutines'),number(latest.goroutines)],
      [T('힙 사용량','Heap allocation'),number((Number(latest.heap_alloc_bytes)||0)/1048576)+' MiB'],
    ];
    for (const [label,value] of cards) {
      const card=elem('div','perf-stat');card.append(elem('span','perf-note',label),elem('strong','',value));grid.append(card);
    }
    box.append(grid);
    box.append(elem('p','perf-note',T('측정 구간: ','Observation windows: ')+windows.length+' · '+T('수집 시각: ','Captured: ')+latest.captured_at));
    if (!entries.length) {box.append(elem('p','perf-note',T('아직 측정 데이터가 없습니다. 약 1분 후 새로고침하세요.','No observations yet. Refresh after about a minute.')));return;}
    const scroller=elem('div','perf-scroll'),table=elem('table','perf-table'),head=elem('thead'),hr=elem('tr');
    for(const label of [T('작업 종류','Operation'),T('횟수','Calls'),T('건너뜀','Skipped'),T('변경','Changed'),T('항목 합계','Items'),T('총 ms','Total ms'),T('평균 ms','Avg ms'),T('최대 ms','Max ms')])hr.append(elem('th','',label));
    head.append(hr);table.append(head);
    const body=elem('tbody');
    for(const [kind,m] of entries) {
      const row=elem('tr');
      for(const value of [kind,number(m.calls),number(m.skipped),number(m.changed),number(m.items),number(m.total_ms),number(m.calls?m.total_ms/m.calls:0),number(m.max_ms)])row.append(elem('td','',value));
      body.append(row);
    }
    table.append(body);scroller.append(table);box.append(scroller);
    box.append(elem('p','perf-note perf-disclaimer',T(
      '주의: 경과 시간은 CPU 점유율이 아닙니다. 항목 합계는 주의 대상/운영 단계 수이며 전체 백로그 수가 아닙니다. 변경 수는 현재 attention revision에서만 계측됩니다.',
      'Elapsed time is not CPU utilization. Items are attention rows/stages, not full backlog counts. Revision changes are counted for attention only.'
    )));
  }
  async function reload() {
    const status=overlay?.querySelector('[data-perf-status]');
    if(status) status.textContent=T('측정 자료 확인 중…','Loading measurements…');
    try {
      const response=await fetch(route,{cache:'no-store'});
      if(!response.ok) throw new Error('HTTP '+response.status);
      latest=await response.json();
      draw();
      if(status) status.textContent=T('최신 진단 자료를 불러왔습니다.','Measurements updated.');
    } catch(e) {
      if(status) status.textContent=T('진단 자료 조회 실패: ','Failed to load diagnostics: ')+e.message;
    }
  }
  function close() {
    if(refreshTimer){clearInterval(refreshTimer);refreshTimer=null;}
    if(overlay){overlay.remove();overlay=null;}
    button.focus();
  }
  function show() {
    if(overlay) return;
    overlay=elem('div','perf-overlay');
    overlay.setAttribute('role','presentation');
    const dialog=elem('section','perf-dialog');
    dialog.setAttribute('role','dialog');
    dialog.setAttribute('aria-modal','true');
    dialog.setAttribute('aria-label',T('개발 모드 성능 진단','Dev performance diagnostics'));
    const h=elem('div','perf-head'),heading=elem('div');
    heading.append(elem('h2','',T('성능 진단 · Dev','Performance Diagnostics · Dev')),elem('p','perf-note',T(
      '업데이트 후 자동 계측됩니다. 최근 측정값을 복사하거나 JSON으로 내려받아 보고할 수 있습니다.',
      'Automatically collected after update. Copy a report or export bounded JSON.'
    )));
    const x=elem('button','perf-close','×');x.type='button';x.setAttribute('aria-label',T('닫기','Close'));x.addEventListener('click',close);
    h.append(heading,x);dialog.append(h);
    const actions=elem('div','perf-actions');
    function action(label,fn){const b=elem('button','',label);b.type='button';b.addEventListener('click',fn);actions.append(b);}
    action(T('새로고침','Refresh'),reload);
    action(T('보고서 복사','Copy report'),async ()=>{
      if(!latest)return;
      try{await navigator.clipboard.writeText(report(latest));overlay.querySelector('[data-perf-status]').textContent=T('보고서를 복사했습니다.','Report copied.');}
      catch(e){overlay.querySelector('[data-perf-status]').textContent=T('복사 실패. JSON 내보내기를 이용하세요.','Copy failed. Use JSON export.');}
    });
    action(T('JSON 내보내기','Export JSON'),()=>{
      if(!latest)return;
      const blob=new Blob([JSON.stringify(latest,null,2)],{type:'application/json'});
      const url=URL.createObjectURL(blob);
      const a=document.createElement('a');a.href=url;a.download='task-mecca-perf-'+new Date().toISOString().slice(0,10)+'.json';
      document.body.append(a);a.click();a.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);
    });
    dialog.append(actions);
    const body=elem('div');body.dataset.perfBody='';dialog.append(body);
    const status=elem('p','perf-status');status.dataset.perfStatus='';status.setAttribute('role','status');dialog.append(status);
    overlay.append(dialog);document.body.append(overlay);
    overlay.addEventListener('click',e=>{if(e.target===overlay)close();});
    overlay.addEventListener('keydown',e=>{if(e.key==='Escape'){e.stopPropagation();close();}});
    x.focus();reload();refreshTimer=setInterval(()=>{if(overlay)reload();},30000);
  }
  button.addEventListener('click',show);
  fetch(route,{cache:'no-store'}).then(response=>{
    if(response.ok) {button.hidden=false;button.querySelector('.nav-text').textContent=T('성능 진단 (Dev)','Performance (Dev)');}
  }).catch(()=>{});
})();
