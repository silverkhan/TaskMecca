(() => {
  const params = new URLSearchParams(location.search);
  const language = params.get('lang') || localStorage.getItem('task-mecca-language') || (navigator.language?.toLowerCase().startsWith('ko') ? 'ko' : 'en');
  const I18N = {
    ko: {
      back:'대시보드',
      checking:'Terminal 접근 확인 중…',
      local:'로컬',
      tailscale:'Tailscale 원격',
      blocked:'차단된 연결',
      connected:'연결됨',
      disconnected:'연결 끊김',
      ready:'준비',
      remoteOn:'원격 Terminal 켜짐',
      remoteOff:'원격 Terminal 꺼짐',
      enableRemote:'원격 Terminal 켜기',
      disableRemote:'원격 Terminal 끄기',
      remoteEnableTitle:'원격 Terminal을 켜시겠습니까?',
      remoteEnableBody:'원격 Terminal은 Tailscale HTTPS로 접속한 기기에서만 사용할 수 있습니다. Tailnet 접근 권한이 있는 기기만 사용하세요.',
      remoteDisableBody:'원격 Terminal을 끄면 현재 Terminal 세션도 종료됩니다.',
      localAccessTitle:'로컬 Terminal 사용 가능',
      localAccessBody:'이 PC의 localhost에서 접속 중입니다. Terminal은 현재 프로젝트 루트에서 실행됩니다.',
      remoteAccessTitle:'Tailscale 원격 Terminal 사용 가능',
      remoteAccessBody:'Tailscale HTTPS를 통해 연결되었습니다. 별도의 localhost 사전 활성화 없이 바로 사용할 수 있습니다.',
      remoteLockedTitle:'원격 Terminal이 꺼져 있습니다',
      remoteLockedBody:'이 PC의 원격 Terminal 설정이 꺼져 있습니다. Tailscale HTTPS로 접속한 현재 화면에서도 다시 켤 수 있습니다.',
      blockedTitle:'원격 Terminal은 Tailscale에서만 사용할 수 있습니다',
      blockedBody:'현재 연결은 Tailscale HTTPS로 확인되지 않았습니다. 원격에서는 Task Mecca의 Tailscale HTTPS 주소로 접속하세요. 일반 LAN/공인망 Terminal 접속은 차단됩니다.',
      machineSetting:'원격 Terminal은 Tailscale HTTPS에서만 허용됩니다. localhost는 항상 사용할 수 있으며 일반 LAN/공인망에서는 차단됩니다.',
      start:'Terminal 시작',
      restart:'재시작',
      close:'닫기',
      paste:'붙여넣기',
      pastePrompt:'클립보드 읽기 권한을 사용할 수 없습니다. 아래 입력창을 길게 눌러 붙여넣으세요.',
      paste:'Paste',
      pastePrompt:'Clipboard access is unavailable. Long-press the field below and paste your command.',
      interrupt:'Ctrl+C',
      shell:'Shell',
      idleTitle:'Terminal 준비됨',
      idleText:'현재 프로젝트 디렉터리에서 셸을 시작합니다.',
      runtimeLoading:'Terminal renderer 불러오는 중…',
      runtimeXterm:'xterm.js · ANSI/키 입력 활성',
      runtimeFallback:'기본 renderer · 명령 입력 fallback',
      lifecycle:'브라우저 연결이 끊긴 세션은 5분 후 자동 종료됩니다.',
      ended:'Terminal 세션이 종료되었습니다.',
      startFailed:'Terminal을 시작하지 못했습니다.',
      streamFailed:'Terminal 스트림 연결에 실패했습니다.',
      inputFailed:'Terminal 입력 전송에 실패했습니다.',
      settingsFailed:'Terminal 설정을 읽지 못했습니다.',
      fallbackNotice:'xterm.js를 불러오지 못해 기본 명령 입력 모드로 전환했습니다.',
      send:'전송',
      shellStarting:'셸 시작 중…',
      localOpen:'이 PC에서 열기',
      confirm:'확인',
      cancel:'취소'
    },
    en: {
      back:'Dashboard',
      checking:'Checking Terminal access…',
      local:'Local',
      tailscale:'Tailscale remote',
      blocked:'Blocked connection',
      connected:'Connected',
      disconnected:'Disconnected',
      ready:'Ready',
      remoteOn:'Remote Terminal on',
      remoteOff:'Remote Terminal off',
      enableRemote:'Enable Remote Terminal',
      disableRemote:'Disable Remote Terminal',
      remoteEnableTitle:'Enable Remote Terminal?',
      remoteEnableBody:'Remote Terminal is available only through Tailscale HTTPS. Use it only from devices authorized on your tailnet.',
      remoteDisableBody:'Turning Remote Terminal off also closes current Terminal sessions.',
      localAccessTitle:'Local Terminal available',
      localAccessBody:'You are connected through localhost. The shell starts at the current project root.',
      remoteAccessTitle:'Tailscale Remote Terminal available',
      remoteAccessBody:'You are connected through Tailscale HTTPS. No prior localhost activation is required.',
      remoteLockedTitle:'Remote Terminal is off',
      remoteLockedBody:'Remote Terminal is disabled on this machine. You can enable it again directly from this Tailscale HTTPS page.',
      blockedTitle:'Remote Terminal is available only through Tailscale',
      blockedBody:'This connection was not verified as Tailscale HTTPS. For remote access, open Task Mecca through its Tailscale HTTPS address. Ordinary LAN/public-network Terminal access is blocked.',
      machineSetting:'Remote Terminal is allowed only through Tailscale HTTPS. Localhost is always available; ordinary LAN/public-network access is blocked.',
      start:'Start terminal',
      restart:'Restart',
      close:'Close',
      interrupt:'Ctrl+C',
      shell:'Shell',
      idleTitle:'Terminal ready',
      idleText:'Start a shell in the current project directory.',
      runtimeLoading:'Loading terminal renderer…',
      runtimeXterm:'xterm.js · ANSI and interactive keys enabled',
      runtimeFallback:'Basic renderer · command-input fallback',
      lifecycle:'Detached browser sessions are closed automatically after 5 minutes.',
      ended:'Terminal session ended.',
      startFailed:'Could not start Terminal.',
      streamFailed:'Terminal stream connection failed.',
      inputFailed:'Failed to send Terminal input.',
      settingsFailed:'Could not load Terminal settings.',
      fallbackNotice:'xterm.js could not be loaded, so Task Mecca switched to the basic command-input fallback.',
      send:'Send',
      shellStarting:'Starting shell…',
      localOpen:'Open on this PC',
      confirm:'Confirm',
      cancel:'Cancel'
    }
  };
  const t = key => (I18N[language] || I18N.en)[key] || I18N.en[key] || key;
  const $ = selector => document.querySelector(selector);

  const state = {
    project: params.get('project') || '',
    settings: null,
    session: null,
    terminal: null,
    terminalDataDisposable: null,
    streamAbort: null,
    resizeObserver: null,
    resizeTimer: 0,
    inputQueue: '',
    inputTimer: 0,
    inputChain: Promise.resolve(),
    fallbackDecoder: new TextDecoder(),
    fallbackText: '',
    fallbackHistory: [],
    fallbackHistoryIndex: 0,
    runtimeMode: '',
    busy: false,
  };

  function applyTheme() {
    const saved = localStorage.getItem('task-mecca-theme') || 'system';
    document.documentElement.dataset.theme = saved;
  }

  function projectQuery() {
    const query = new URLSearchParams();
    if (state.project) query.set('project', state.project);
    const value = query.toString();
    return value ? '?' + value : '';
  }

  function dashboardURL() {
    const query = new URLSearchParams();
    query.set('view', 'backlog');
    if (state.project) query.set('project', state.project);
    return '/?' + query.toString();
  }

  function sessionStorageKey() {
    return 'task-mecca-terminal-session:' + (state.project || 'default');
  }

  function setConnection(kind, label) {
    const el = $('#terminalConnection');
    if (!el) return;
    el.classList.remove('ok', 'warn', 'error');
    if (kind) el.classList.add(kind);
    const span = el.querySelector('span:last-child');
    if (span) span.textContent = label;
  }

  function setBusy(busy) {
    state.busy = busy;
    ['terminalStartBtn','terminalRestartBtn','terminalCloseBtn','terminalPasteBtn','terminalInterruptBtn'].forEach(id => {
      const el = $('#' + id);
      if (el) el.disabled = busy;
    });
  }

  function localTerminalURL() {
    const base = state.settings?.local_terminal_url || 'http://127.0.0.1:18765/terminal';
    const query = new URLSearchParams();
    if (state.project) query.set('project', state.project);
    query.set('lang', language);
    return base + (base.includes('?') ? '&' : '?') + query.toString();
  }

  function renderSecurity() {
    const box = $('#terminalSecurity');
    const workspace = $('#terminalWorkspace');
    if (!state.settings) {
      box.innerHTML = '<div class="terminal-loading">' + t('checking') + '</div>';
      workspace.hidden = true;
      return;
    }
    const s = state.settings;
    const project = s.project || state.project || '';
    if (project && !state.project) state.project = project;
    $('#terminalProject').textContent = state.project || project || '-';
    $('#terminalBack').textContent = '← ' + t('back');
    $('#terminalBack').href = dashboardURL();

    let title = '', body = '', pill = '', actions = '';
    if (s.connection === 'local') {
      title = t('localAccessTitle');
      body = t('localAccessBody');
      pill = '<span class="terminal-pill ' + (s.remote_enabled ? 'ok' : '') + '">' + (s.remote_enabled ? t('remoteOn') : t('remoteOff')) + '</span>';
      actions = '<button type="button" class="terminal-btn secondary" id="remoteToggleBtn">' + (s.remote_enabled ? t('disableRemote') : t('enableRemote')) + '</button>';
      setConnection('ok', t('local'));
    } else if (s.connection === 'tailscale' && s.remote_enabled) {
      title = t('remoteAccessTitle');
      body = t('remoteAccessBody');
      pill = '<span class="terminal-pill ok">' + t('remoteOn') + '</span>';
      actions = s.can_disable_remote ? '<button type="button" class="terminal-btn secondary" id="remoteToggleBtn">' + t('disableRemote') + '</button>' : '';
      setConnection('ok', t('tailscale'));
    } else if (s.connection === 'tailscale') {
      title = t('remoteLockedTitle');
      body = t('remoteLockedBody');
      pill = '<span class="terminal-pill warn">' + t('remoteOff') + '</span>';
      actions = s.can_enable_remote ? '<button type="button" class="terminal-btn secondary" id="remoteToggleBtn">' + t('enableRemote') + '</button>' : '';
      setConnection('warn', t('tailscale'));
    } else {
      title = t('blockedTitle');
      body = t('blockedBody');
      pill = '<span class="terminal-pill danger">' + t('blocked') + '</span>';
      setConnection('error', t('blocked'));
    }

    box.innerHTML =
      '<div class="terminal-security-row">' +
        '<div class="terminal-security-copy"><strong>' + title + '</strong><p>' + body + '</p><p class="terminal-note">' + t('machineSetting') + '</p></div>' +
        '<div class="terminal-security-actions">' + pill + actions + '</div>' +
      '</div>';

    workspace.hidden = !s.access_allowed;
    $('#terminalIdleTitle').textContent = t('idleTitle');
    $('#terminalIdleText').textContent = t('idleText');
    $('#terminalStartBtn').textContent = t('start');
    $('#terminalRestartBtn').textContent = t('restart');
    $('#terminalCloseBtn').textContent = t('close');
    $('#terminalPasteBtn').textContent = t('paste');
    $('#terminalInterruptBtn').textContent = t('interrupt');
    $('#terminalLifecycle').textContent = t('lifecycle');

    $('#remoteToggleBtn')?.addEventListener('click', toggleRemoteAccess);
    updateSessionControls();
  }

  function escapeHTML(value) {
    return String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  }

  async function jsonResponse(response) {
    let body = {};
    try { body = await response.json(); } catch (_) {}
    if (!response.ok) throw new Error(body.error || ('HTTP ' + response.status));
    return body;
  }

  async function loadSettings() {
    try {
      const response = await fetch('/api/terminal/settings' + projectQuery(), {cache:'no-store'});
      state.settings = await jsonResponse(response);
      if (!state.project && state.settings.project) {
        state.project = state.settings.project;
        const next = new URL(location.href);
        next.searchParams.set('project', state.project);
        next.searchParams.set('lang', language);
        history.replaceState({}, '', next.pathname + '?' + next.searchParams.toString());
      }
      renderSecurity();
      if (state.settings.access_allowed) {
        await restoreSession();
      } else {
        forgetSession();
      }
    } catch (error) {
      state.settings = null;
      $('#terminalSecurity').innerHTML = '<div class="terminal-error">' + t('settingsFailed') + '<br>' + escapeHTML(error.message || error) + '</div>';
      $('#terminalWorkspace').hidden = true;
      setConnection('error', t('disconnected'));
    }
  }

  async function toggleRemoteAccess() {
    if (!state.settings) return;
    const enable = !state.settings.remote_enabled;
    const message = enable ? (t('remoteEnableTitle') + '\n\n' + t('remoteEnableBody')) : t('remoteDisableBody');
    if (!window.confirm(message)) return;
    const button = $('#remoteToggleBtn');
    if (button) button.disabled = true;
    try {
      const response = await fetch('/api/terminal/settings' + projectQuery(), {
        method:'POST',
        headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},
        body:JSON.stringify({remote_enabled:enable})
      });
      state.settings = await jsonResponse(response);
      if (!enable) forgetSession();
      renderSecurity();
    } catch (error) {
      alert(String(error.message || error));
      if (button) button.disabled = false;
    }
  }

  function estimatedSize() {
    const surface = $('#terminalSurface');
    const width = Math.max(360, surface?.clientWidth || window.innerWidth || 1000);
    const height = Math.max(240, surface?.clientHeight || 520);
    return {
      cols: Math.max(40, Math.min(300, Math.floor(width / 8.4))),
      rows: Math.max(10, Math.min(120, Math.floor(height / 18)))
    };
  }

  async function startSession() {
    if (!state.settings?.access_allowed || state.busy) return;
    setBusy(true);
    setConnection('warn', t('shellStarting'));
    try {
      const size = estimatedSize();
      const response = await fetch('/api/terminal/sessions' + projectQuery(), {
        method:'POST',
        headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},
        body:JSON.stringify(size)
      });
      state.session = await jsonResponse(response);
      sessionStorage.setItem(sessionStorageKey(), JSON.stringify({id:state.session.id, token:state.session.token}));
      updateSessionControls();
      await mountSession();
    } catch (error) {
      setConnection('error', t('startFailed'));
      alert(t('startFailed') + '\n' + String(error.message || error));
    } finally {
      setBusy(false);
    }
  }

  async function restoreSession() {
    if (state.session || !state.settings?.access_allowed) return;
    let saved = null;
    try { saved = JSON.parse(sessionStorage.getItem(sessionStorageKey()) || 'null'); } catch (_) {}
    if (!saved?.id || !saved?.token) return;
    try {
      const response = await fetch('/api/terminal/sessions/' + encodeURIComponent(saved.id), {
        cache:'no-store',
        headers:{'X-Task-Mecca-Terminal':saved.token}
      });
      const current = await jsonResponse(response);
      if (current.closed) {
        sessionStorage.removeItem(sessionStorageKey());
        return;
      }
      state.session = {...current, token:saved.token};
      updateSessionControls();
      await mountSession();
    } catch (_) {
      sessionStorage.removeItem(sessionStorageKey());
    }
  }

  function forgetSession() {
    cleanupRenderer();
    state.session = null;
    sessionStorage.removeItem(sessionStorageKey());
    updateSessionControls();
  }

  async function closeSession(silent=false) {
    const session = state.session;
    if (!session) return;
    setBusy(true);
    stopStream();
    try {
      await fetch('/api/terminal/sessions/' + encodeURIComponent(session.id), {
        method:'DELETE',
        headers:{'X-Task-Mecca-Terminal':session.token,'X-Task-Mecca-Action':'1'}
      });
    } catch (_) {}
    forgetSession();
    if (!silent) setConnection(state.settings?.access_allowed ? 'ok' : 'warn', state.settings?.connection === 'local' ? t('local') : t('tailscale'));
    setBusy(false);
  }

  async function restartSession() {
    await closeSession(true);
    await startSession();
  }

  function sessionActionURL(action) {
    if (!state.session) return '';
    return '/api/terminal/sessions/' + encodeURIComponent(state.session.id) + '/' + action;
  }

  async function pasteIntoTerminal() {
    if (!state.session) return;
    let value = '';
    try {
      if (navigator.clipboard?.readText) value = await navigator.clipboard.readText();
    } catch (_) {}
    if (!value) {
      const manual = window.prompt(t('pastePrompt'), '');
      if (manual == null || manual === '') return;
      value = manual;
    }
    sendInput(value);
    try { state.terminal?.focus(); } catch (_) {}
  }

  function bindNativePaste(target) {
    if (!target || target.dataset.taskMeccaPasteBound === '1') return;
    target.dataset.taskMeccaPasteBound = '1';
    target.addEventListener('paste', event => {
      const value = event.clipboardData?.getData('text');
      if (!value) return;
      event.preventDefault();
      sendInput(value);
      try { state.terminal?.focus(); } catch (_) {}
    });
  }

  function sendInput(data) {
    if (!state.session || !data) return;
    state.inputQueue += data;
    if (state.inputTimer) return;
    state.inputTimer = window.setTimeout(() => {
      state.inputTimer = 0;
      const outgoing = state.inputQueue;
      state.inputQueue = '';
      if (!outgoing || !state.session) return;
      const session = state.session;
      state.inputChain = state.inputChain.then(async () => {
        const response = await fetch('/api/terminal/sessions/' + encodeURIComponent(session.id) + '/input', {
          method:'POST',
          headers:{
            'Content-Type':'application/octet-stream',
            'X-Task-Mecca-Terminal':session.token,
            'X-Task-Mecca-Action':'1'
          },
          body:new TextEncoder().encode(outgoing)
        });
        if (!response.ok) {
          let detail = '';
          try { detail = (await response.json()).error || ''; } catch (_) {}
          throw new Error(detail || ('HTTP ' + response.status));
        }
      }).catch(error => {
        setConnection('error', t('inputFailed'));
        appendFallback('\n[Task Mecca] ' + t('inputFailed') + ': ' + String(error.message || error) + '\n');
      });
    }, 12);
  }

  function scheduleResize() {
    clearTimeout(state.resizeTimer);
    state.resizeTimer = window.setTimeout(async () => {
      if (!state.session) return;
      const size = estimatedSize();
      if (state.terminal) {
        try { state.terminal.resize(size.cols, size.rows); } catch (_) {}
      }
      try {
        await fetch(sessionActionURL('resize'), {
          method:'POST',
          headers:{
            'Content-Type':'application/json',
            'X-Task-Mecca-Terminal':state.session.token,
            'X-Task-Mecca-Action':'1'
          },
          body:JSON.stringify(size)
        });
      } catch (_) {}
    }, 100);
  }

  function loadStylesheet(url, integrity='') {
    return new Promise((resolve, reject) => {
      const existing = [...document.querySelectorAll('link[data-xterm-css]')].find(link => link.href === new URL(url, location.href).href);
      if (existing) { resolve(); return; }
      const link = document.createElement('link');
      link.rel = 'stylesheet';
      link.href = url;
      link.dataset.xtermCss = '1';
      if (integrity) {
        link.integrity = integrity;
        link.crossOrigin = 'anonymous';
      }
      link.onload = () => resolve();
      link.onerror = () => { link.remove(); reject(new Error('CSS load failed')); };
      document.head.appendChild(link);
    });
  }

  function loadScript(url, integrity='') {
    return new Promise((resolve, reject) => {
      if (window.Terminal) { resolve(); return; }
      const script = document.createElement('script');
      script.src = url;
      script.async = true;
      script.dataset.xtermScript = '1';
      if (integrity) {
        script.integrity = integrity;
        script.crossOrigin = 'anonymous';
      }
      script.onload = () => window.Terminal ? resolve() : reject(new Error('Terminal global missing'));
      script.onerror = () => { script.remove(); reject(new Error('script load failed')); };
      document.head.appendChild(script);
    });
  }

  async function loadXtermRuntime() {
    const sources = [
      {js:'/vendor/xterm.js', css:'/vendor/xterm.css'},
      {
        js:'https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/lib/xterm.min.js',
        jsIntegrity:'sha384-J4qzUjBl1FxyLsl/kQPQIOeINsmp17OHYXDOMpMxlKX53ZfYsL+aWHpgArvOuof9',
        css:'https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/css/xterm.min.css',
        cssIntegrity:'sha384-tStR1zLfWgsiXCF3IgfB3lBa8KmBe/lG287CL9WCeKgQYcp1bjb4/+mwN6oti4Co'
      }
    ];
    for (const source of sources) {
      try {
        await loadStylesheet(source.css, source.cssIntegrity || '');
        if (!window.Terminal) await loadScript(source.js, source.jsIntegrity || '');
        if (window.Terminal) return true;
      } catch (_) {}
    }
    return false;
  }

  function cleanupRenderer() {
    stopStream();
    if (state.resizeObserver) {
      state.resizeObserver.disconnect();
      state.resizeObserver = null;
    }
    if (state.terminalDataDisposable) {
      try { state.terminalDataDisposable.dispose(); } catch (_) {}
      state.terminalDataDisposable = null;
    }
    if (state.terminal) {
      try { state.terminal.dispose(); } catch (_) {}
      state.terminal = null;
    }
    state.runtimeMode = '';
    state.fallbackText = '';
    state.fallbackDecoder = new TextDecoder();
    const host = $('#xtermHost');
    if (host) { host.innerHTML = ''; host.hidden = true; }
    const fallback = $('#terminalFallbackOutput');
    if (fallback) { fallback.textContent = ''; fallback.hidden = true; }
    const composer = $('#terminalFallbackComposer');
    if (composer) composer.hidden = true;
  }

  async function mountSession() {
    if (!state.session) return;
    cleanupRenderer();
    updateSessionControls();
    $('#terminalIdle').hidden = true;
    $('#terminalRuntime').textContent = t('runtimeLoading');
    setConnection('warn', t('shellStarting'));

    const xtermAvailable = await loadXtermRuntime();
    if (!state.session) return;
    if (xtermAvailable) {
      const host = $('#xtermHost');
      host.hidden = false;
      const prefersDark = document.documentElement.dataset.theme === 'dark' ||
        (document.documentElement.dataset.theme === 'system' && window.matchMedia?.('(prefers-color-scheme: dark)').matches);
      const term = new window.Terminal({
        cursorBlink:true,
        convertEol:false,
        scrollback:5000,
        fontFamily:'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
        fontSize:14,
        lineHeight:1.15,
        theme: prefersDark
          ? {background:'#080c12',foreground:'#e8edf5',cursor:'#dce5f2',selectionBackground:'#33415a'}
          : {background:'#080c12',foreground:'#e8edf5',cursor:'#dce5f2',selectionBackground:'#33415a'}
      });
      term.open(host);
      bindNativePaste(host);
      state.terminal = term;
      state.terminalDataDisposable = term.onData(data => sendInput(data));
      state.runtimeMode = 'xterm';
      $('#terminalRuntime').textContent = t('runtimeXterm');
      const size = estimatedSize();
      term.resize(size.cols, size.rows);
      state.resizeObserver = new ResizeObserver(scheduleResize);
      state.resizeObserver.observe($('#terminalSurface'));
      scheduleResize();
      term.focus();
    } else {
      state.runtimeMode = 'fallback';
      $('#terminalFallbackOutput').hidden = false;
      $('#terminalFallbackComposer').hidden = false;
      $('#terminalRuntime').textContent = t('runtimeFallback');
      appendFallback('[Task Mecca] ' + t('fallbackNotice') + '\n');
      bindFallbackComposer();
      scheduleResize();
    }
    void startStream();
  }

  function stripAnsi(value) {
    return String(value)
      .replace(/\x1B\][^\x07]*(?:\x07|\x1B\\)/g, '')
      .replace(/\x1B\[[0-?]*[ -\/]*[@-~]/g, '')
      .replace(/\x1B[@-_]/g, '')
      .replace(/\r/g, '');
  }

  function appendFallback(text) {
    const output = $('#terminalFallbackOutput');
    if (!output) return;
    state.fallbackText += stripAnsi(text);
    if (state.fallbackText.length > 300000) state.fallbackText = state.fallbackText.slice(-300000);
    output.textContent = state.fallbackText;
    output.scrollTop = output.scrollHeight;
  }

  function bindFallbackComposer() {
    const input = $('#terminalFallbackInput');
    const send = $('#terminalFallbackSend');
    if (!input || !send) return;
    send.textContent = t('send');
    const submit = () => {
      const value = input.value;
      if (!value) return;
      state.fallbackHistory.push(value);
      state.fallbackHistoryIndex = state.fallbackHistory.length;
      sendInput(value + '\r');
      input.value = '';
    };
    send.onclick = submit;
    input.onkeydown = event => {
      if (event.key === 'Enter') {
        event.preventDefault();
        submit();
      } else if (event.key === 'ArrowUp') {
        event.preventDefault();
        state.fallbackHistoryIndex = Math.max(0, state.fallbackHistoryIndex - 1);
        input.value = state.fallbackHistory[state.fallbackHistoryIndex] || '';
      } else if (event.key === 'ArrowDown') {
        event.preventDefault();
        state.fallbackHistoryIndex = Math.min(state.fallbackHistory.length, state.fallbackHistoryIndex + 1);
        input.value = state.fallbackHistory[state.fallbackHistoryIndex] || '';
      } else if (event.key.toLowerCase() === 'c' && event.ctrlKey) {
        event.preventDefault();
        sendInput('\x03');
      }
    };
    input.focus();
  }

  async function startStream() {
    stopStream();
    if (!state.session) return;
    const session = state.session;
    const controller = new AbortController();
    state.streamAbort = controller;
    try {
      const response = await fetch('/api/terminal/sessions/' + encodeURIComponent(session.id) + '/stream', {
        cache:'no-store',
        headers:{'X-Task-Mecca-Terminal':session.token},
        signal:controller.signal
      });
      if (!response.ok) {
        let detail = '';
        try { detail = (await response.json()).error || ''; } catch (_) {}
        throw new Error(detail || ('HTTP ' + response.status));
      }
      setConnection('ok', t('connected'));
      const reader = response.body.getReader();
      while (state.session?.id === session.id) {
        const {value, done} = await reader.read();
        if (done) break;
        if (!value?.length) continue;
        if (state.terminal) {
          state.terminal.write(value);
        } else {
          appendFallback(state.fallbackDecoder.decode(value, {stream:true}));
        }
      }
      if (state.session?.id === session.id) {
        setConnection('warn', t('ended'));
        await refreshSessionStatus();
      }
    } catch (error) {
      if (controller.signal.aborted) return;
      if (state.session?.id === session.id) {
        setConnection('error', t('streamFailed'));
        appendFallback('\n[Task Mecca] ' + t('streamFailed') + ': ' + String(error.message || error) + '\n');
      }
    }
  }

  function stopStream() {
    if (state.streamAbort) {
      state.streamAbort.abort();
      state.streamAbort = null;
    }
  }

  async function refreshSessionStatus() {
    if (!state.session) return;
    try {
      const response = await fetch('/api/terminal/sessions/' + encodeURIComponent(state.session.id), {
        cache:'no-store',
        headers:{'X-Task-Mecca-Terminal':state.session.token}
      });
      const current = await jsonResponse(response);
      if (current.closed) {
        sessionStorage.removeItem(sessionStorageKey());
        state.session = null;
        updateSessionControls();
      }
    } catch (_) {}
  }

  function updateSessionControls() {
    const active = Boolean(state.session);
    $('#terminalStartBtn').hidden = active;
    $('#terminalRestartBtn').hidden = !active;
    $('#terminalCloseBtn').hidden = !active;
    $('#terminalPasteBtn').hidden = !active;
    $('#terminalInterruptBtn').hidden = !active;
    $('#terminalIdle').hidden = active;
    $('#terminalShellLabel').textContent = active ? (state.session.shell || t('shell')) : t('shell');
    $('#terminalCwd').textContent = active ? (state.session.cwd || state.project || '') : (state.project || '');
    if (!active) {
      cleanupRenderer();
      $('#terminalRuntime').textContent = 'PTY: ' + t('ready');
    }
  }

  $('#terminalStartBtn').addEventListener('click', startSession);
  $('#terminalRestartBtn').addEventListener('click', restartSession);
  $('#terminalCloseBtn').addEventListener('click', () => closeSession(false));
  $('#terminalPasteBtn').addEventListener('click', pasteIntoTerminal);
  $('#terminalInterruptBtn').addEventListener('click', () => sendInput('\x03'));
  window.addEventListener('beforeunload', stopStream);
  window.addEventListener('resize', () => { if (state.session) scheduleResize(); });

  document.documentElement.lang = language;
  applyTheme();
  $('#terminalBack').textContent = '← ' + t('back');
  $('#terminalIdleTitle').textContent = t('idleTitle');
  $('#terminalIdleText').textContent = t('idleText');
  $('#terminalLifecycle').textContent = t('lifecycle');
  loadSettings();
})();
