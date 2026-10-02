const state = {
  snapshot: null,
  listData: null,
  detailTask: null,
  eventSource: null,
  eventStreamKey: '',
  view: 'hub',
  hub: null,
  loadError: '',
  project: new URLSearchParams(location.search).get('project') || '',
  lastProject: localStorage.getItem('task-mecca-last-project') || new URLSearchParams(location.search).get('project') || '',
  statusFilters: ['all'],
  tagFilters: [],
  tagExplorerOpen: false,
  query: '',
  detail: null,
  raw: false,
  lastFetch: 0,
  lastHubFetch: 0,
  theme: localStorage.getItem('task-mecca-theme') || 'system',
  manual: null,
  manualTab: 'quick',
  backlog: localStorage.getItem('task-mecca-backlog-folder') || '',
  listPage: 1,
  listPageMode: localStorage.getItem('task-mecca-list-page-mode-v1') || 'auto',
  listPageSize: Number(localStorage.getItem('task-mecca-list-page-size') || localStorage.getItem('task-mecca-done-page-size') || 20),
  autoListPageSize: 8,
  listSort: localStorage.getItem('task-mecca-list-sort-v2') || 'id_desc',
  selectedIndex: 0,
  sidebarCollapsed: localStorage.getItem('task-mecca-sidebar-collapsed') === '1',
  sidebarPeek: false,
  projectMenuOpen: false,
  openProjects: (()=>{ try { const raw=JSON.parse(localStorage.getItem('task-mecca-open-projects')||'[]'); return Array.isArray(raw)?raw:[]; } catch(_) { return []; } })(),
  language: localStorage.getItem('task-mecca-language') || (navigator.language?.toLowerCase().startsWith('ko') ? 'ko' : 'en'),
  manualByLanguage: {},
  notificationSettings: (()=>{ try { return {...{intervention:true,completed:true,stalled:true},...JSON.parse(localStorage.getItem('task-mecca-notifications')||'{}')}; } catch(_) { return {intervention:true,completed:true,stalled:true}; } })(),
  previousTasksByProject: (()=>{ try { const raw=JSON.parse(localStorage.getItem('task-mecca-previous-tasks')||'{}'); return raw&&typeof raw==='object'?raw:{}; } catch(_) { return {}; } })(),
  versionInfo: null,
  contentRevision: '',
  contentStateSnapshot: [],
  pendingContentUpdate: false,
  pendingContentReason: '',
  pendingContentChanges: [],
  eventStreamInitialized: false,
  attentionRevision: '',
  attentionRevisionKey: '',
  releaseNotes: [],
  releaseNotesTotal: 0,
  releaseNotesHasMore: false,
  releaseNotesLoaded: false,
  releaseNotesLoading: false,
  releaseNoteDetails: {},
  releaseNoteExpanded: '',
  releaseNotePopup: null,
  releaseNotePopupMode: 'installed',
  releaseNotePopupCheckedVersion: '',
  runtimeHistoryOpen: false,
  runtimeHistoryLoading: false,
  runtimeHistoryError: '',
  runtimeHistory: {items:[],page:1,page_size:20,total:0,total_pages:0,retention:{}},
  runtimeAttemptDisclosure: {},
  runtimeTransitionDisclosure: {},
  runtimeStorageOpen: false,
  runtimeStorageLoading: false,
  runtimeStorageError: '',
  runtimeStorage: null,
};


function currentProjectData() {
  return state.view==='backlog' && state.listData ? state.listData : state.snapshot;
}

const LANGUAGES = {
  ko: { label: '한국어', locale: 'ko-KR' },
  en: { label: 'English', locale: 'en-US' },
};
const I18N = {
  ko: {
    backlog:'백로그', operations:'운영', help:'도움말', workload:'서브에이전트 워크로드', attention:'확인 필요', issues:'이슈', manual:'사용자 매뉴얼', searchPlaceholder:'ID, 제목, 요구사항, 수용 기준 검색…', backlogFolder:'백로그', language:'언어', refresh:'새로고침', themeSystem:'시스템', themeLight:'라이트', themeDark:'다크', accessUnchecked:'권한 미확인', loading:'Task Mecca 불러오는 중…', disconnected:'연결 끊김', move:'이동', open:'열기', back:'뒤로', search:'검색', page:'페이지', status:'상태', all:'전체', ready:'대기', working:'작업 중', hold:'보류', blocked:'차단', done:'완료', todo:'할 일', active:'활성', quiet:'조용함', stale:'정체', workerMissing:'워커 없음', runtimeUnknown:'런타임 미확인', awaitingFinalize:'완료 처리 필요', needsUser:'사용자 개입 필요', stalled:'정체 확인 필요', notificationSettings:'알림 설정', allowBrowserNotifications:'브라우저 알림 허용', notifyIntervention:'사용자 개입 필요', notifyCompleted:'태스크 완료', notifyStalled:'작업 정체', notificationsBlocked:'브라우저에서 알림이 차단되어 있습니다.', notificationsOn:'알림 켜짐', notificationsOff:'알림 꺼짐', notificationsPermissionNeeded:'브라우저 알림을 켜야 실제 알림을 받을 수 있습니다.', notificationsDeniedGuide:'브라우저 주소창의 사이트 설정에서 알림을 허용한 뒤 페이지를 새로고침하세요.', notificationTypesDisabled:'알림 유형이 모두 꺼져 있습니다.', sort:'정렬', perPage:'페이지당', updatedNewest:'최근 업데이트순', updatedOldest:'오래된 업데이트순', autoRows:'자동 ({n})', pageSummary:'{page} / {pages} 페이지 · {total}개', autoRowsSummary:' · 자동 {n}행', allStatuses:'전체 상태', items:'개', needsAttention:'확인 필요', updated:'업데이트', agent:'에이전트', activeTime:'활성 시간', task:'작업', id:'ID', repository:'저장소', noMatches:'선택한 필터에 맞는 백로그가 없습니다.', manualIntro:'Task Mecca를 처음 사용하는 순서와 운영 규칙을 UI 안에서 확인합니다.', dashboardLaunch:'대시보드 실행', keyboard:'키보드', quickStart:'빠른 시작', detailedGuide:'상세 운영 가이드', manualLoading:'매뉴얼을 불러오는 중…', manualUnavailable:'매뉴얼을 불러올 수 없습니다.', operationsEyebrow:'운영', diagnostics:'진단', noAttention:'현재 확인이 필요한 작업이 없습니다.', advisory:'사용자 개입, 완료 처리 필요, 런타임 정체 등 대응이 필요한 작업을 표시합니다.', workers:'워커', doing:'진행', downstreamBlocked:'하위 차단', readyContinuity:'연속 작업 후보', noWorkload:'할당된 서브에이전트 워크로드가 없습니다.', noCurrentDoing:'현재 진행 중인 작업이 없습니다.', unassignedDoing:'미할당 진행 작업', releasedHold:'해제된 보류 소유권', continuity:'연속', blocks:'차단', scope:'변경범위', workloadIntro:'백로그/Git 기반의 할당 현황입니다. 관측 가능한 경우 runtime health를 함께 표시합니다.', issuesIntro:'현재 스냅샷의 doctor 및 hold-review 진단 결과입니다.', noIssues:'진단 이슈가 없습니다.', taskDefinition:'작업 정의', simpleNote:'Simple Task · 별도 요건정의 확인이 필요하지 않습니다.', goal:'목표', acceptance:'수용 기준', requirements:'요건 정의서', definedNote:'Defined Task · 등록 전 요건 정의를 확인한 작업입니다.', background:'배경 및 문제', scopeIn:'포함', scopeOut:'제외', constraints:'제약 및 보존 조건', legacyTask:'Legacy 작업', legacyNote:'Legacy 백로그 · 없는 요건 데이터를 만들지 않고 기존 필드를 표시합니다.', description:'설명', tocTitle:'이 작업의 목차', tocOpen:'목차 열기', tocClose:'목차 닫기', lifecycle:'Lifecycle', noLifecycle:'아직 사용할 수 있는 lifecycle 전환 기록이 없습니다.', stayed:'유지', provisional:'임시', overview:'개요', registrant:'등록자', changeScope:'변경범위', dependsOn:'선행', related:'연관', location:'위치', completed:'완료', activity:'활동', lastSignal:'마지막 신호', signalSource:'신호 근거', execution:'실행 정보', runtimeProvider:'RuntimeProvider', dispatchStatus:'Dispatch 상태', executionEvidence:'실행 근거', fallbackEvidence:'Fallback 근거', workNotes:'작업 노트', result:'결과', verification:'검증', rawMarkdown:'Raw Markdown', rendered:'렌더링', waitTime:'대기 시간', queueTime:'큐 시간', leadTime:'리드 시간', provisionalTiming:'임시 lifecycle 시간', incompleteHistory:'불완전한 lifecycle 이력', lastObservable:'마지막 관측 활동 {ago} ({source}).', workerMissingDetail:'작업은 진행 중이지만 할당된 워커가 runtime registry에 없습니다.', quietAdvisory:'참고용 경고입니다. 장시간 작업은 정상적으로 조용할 수 있습니다.', backToBacklog:'← 백로그', taskNotFound:'작업을 찾을 수 없습니다.', restrictedNow:'현재 제한됨', lastFullAccess:'마지막 Full Access', fullAccess:'Full Access', lastRestricted:'마지막 검증 제한', accessLastChecked:'마지막 권한 확인', accessNotChecked:'권한 미확인', networkOff:'네트워크 꺼짐', notChecked:'확인 안 됨', freshDispatch:'dispatch 직전에 항상 새 active preflight를 실행합니다.', dispatchDisabled:'서브에이전트 dispatch 중지', enableFullAccess:'현재 런타임 제한이 감지되었습니다. dispatch 전에 Full Access를 활성화하세요.', auto:'자동', manualMode:'수동', noBacklog:'선택된 백로그 없음', copyCode:'코드 복사', copied:'복사됨', copyFailed:'복사 실패', mermaid:'Mermaid', showSource:'소스 보기', hideSource:'소스 숨기기', renderingDiagram:'다이어그램 렌더링 중…', mermaidUnavailable:'Mermaid renderer를 사용할 수 없습니다.', mermaidUnavailableDetail:'로컬 및 CDN Mermaid 런타임을 불러오지 못했습니다. Source에서 원문을 확인하거나 복사할 수 있습니다.', mermaidFailed:'Mermaid 렌더링 실패', invalidMermaid:'유효하지 않은 Mermaid 문법', secondsAgo:'{n}초 전', minutesAgo:'{n}분 전', hoursAgo:'{n}시간 전', daysAgo:'{n}일 전', justNow:'방금', updatedColumn:'마지막 업데이트', activeColumn:'활성', agentColumn:'에이전트', statusColumn:'상태', taskColumn:'작업', expandSidebar:'사이드바 펼치기', collapseSidebar:'사이드바 접기', eventRegistered:'등록', eventStarted:'착수', eventHold:'보류', eventCompleted:'완료', observatory:'백로그 관제', theme:'테마', followSystemTheme:'시스템 테마 따르기', useLightTheme:'라이트 테마 사용', useDarkTheme:'다크 테마 사용', selectBacklog:'백로그 폴더 선택', provisionalTimingDetail:'현재 상태전환이 아직 Git에 기록되지 않아 runtime 관측값으로 시간을 계산합니다.', incompleteHistoryDetail:'과거 lifecycle의 착수 근거가 없어 일부 시간은 정확히 복원할 수 없습니다.'
  },
  en: {
    backlog:'Backlog', operations:'Operations', help:'Help', workload:'Subagent Workload', attention:'Needs Attention', issues:'Issues', manual:'User Manual', searchPlaceholder:'Search ID, title, requirements, acceptance criteria…', backlogFolder:'Backlog', language:'Language', refresh:'Refresh', themeSystem:'System', themeLight:'Light', themeDark:'Dark', accessUnchecked:'Access unchecked', loading:'Loading Task Mecca…', disconnected:'Disconnected', move:'move', open:'open', back:'back', search:'search', page:'page', status:'Status', all:'All', ready:'Ready', working:'Working', hold:'Hold', blocked:'Blocked', done:'Done', todo:'Todo', active:'Active', quiet:'Quiet', stale:'Stale', workerMissing:'Worker missing', runtimeUnknown:'Runtime unknown', awaitingFinalize:'Needs finalization', needsUser:'User action required', stalled:'Stalled', notificationSettings:'Notification settings', allowBrowserNotifications:'Allow browser notifications', notifyIntervention:'User action required', notifyCompleted:'Task completed', notifyStalled:'Task stalled', notificationsBlocked:'Notifications are blocked by the browser.', notificationsOn:'Notifications on', notificationsOff:'Notifications off', notificationsPermissionNeeded:'Enable browser notifications to receive actual alerts.', notificationsDeniedGuide:'Allow notifications in the browser site settings, then refresh this page.', notificationTypesDisabled:'All notification types are turned off.', sort:'Sort', perPage:'Per page', updatedNewest:'Updated newest', updatedOldest:'Updated oldest', autoRows:'Auto ({n})', pageSummary:'Page {page} / {pages} · {total} items', autoRowsSummary:' · Auto {n} rows', allStatuses:'All statuses', items:'items', needsAttention:'Needs attention', updated:'Updated', agent:'Agent', activeTime:'Active time', task:'Task', id:'ID', repository:'Repository', noMatches:'No backlog items match the selected filters.', manualIntro:'Review the first-use flow and Task Mecca operating rules inside the UI.', dashboardLaunch:'Dashboard launch', keyboard:'Keyboard', quickStart:'Quick Start', detailedGuide:'Detailed Operations Guide', manualLoading:'Manual is loading…', manualUnavailable:'Manual unavailable.', operationsEyebrow:'Operations', diagnostics:'Diagnostics', noAttention:'No tasks currently need attention.', advisory:'Shows tasks that need user action, finalization, or runtime investigation.', workers:'Workers', doing:'Doing', downstreamBlocked:'Downstream blocked', readyContinuity:'Ready continuity', noWorkload:'No assigned subagent workload.', noCurrentDoing:'No current doing task.', unassignedDoing:'Unassigned doing', releasedHold:'Released hold ownership', continuity:'Continuity', blocks:'Blocks', scope:'Change scope', workloadIntro:'Durable allocation view from backlog/Git. Runtime health is shown when observable.', issuesIntro:'Doctor and hold-review findings from the current snapshot.', noIssues:'No diagnostic issues.', taskDefinition:'Task Definition', simpleNote:'Simple Task · no separate requirement-definition confirmation required.', goal:'Goal', acceptance:'Acceptance criteria', requirements:'Requirements', definedNote:'Defined Task · requirement definition confirmed before registration.', background:'Background & problem', scopeIn:'IN', scopeOut:'OUT', constraints:'Constraints & preservation', legacyTask:'Legacy task', legacyNote:'Legacy backlog · rendered from available fields without inventing missing requirement data.', description:'Description', tocTitle:'On this task', tocOpen:'Open table of contents', tocClose:'Close table of contents', lifecycle:'Lifecycle', noLifecycle:'No lifecycle transitions are available yet.', stayed:'Stayed', provisional:'provisional', overview:'Overview', registrant:'Registrant', changeScope:'Change scope', dependsOn:'Depends on', related:'Related', location:'Location', completed:'Completed', activity:'Activity', lastSignal:'Last signal', signalSource:'Signal source', execution:'Execution', runtimeProvider:'RuntimeProvider', dispatchStatus:'Dispatch status', executionEvidence:'Execution evidence', fallbackEvidence:'Fallback evidence', workNotes:'Work notes', result:'Result', verification:'Verification', rawMarkdown:'Raw Markdown', rendered:'Rendered', waitTime:'Wait time', queueTime:'Queue time', leadTime:'Lead time', provisionalTiming:'Provisional lifecycle timing', incompleteHistory:'Incomplete lifecycle history', lastObservable:'Last observable activity {ago} ({source}).', workerMissingDetail:'Task is doing but the assigned worker is absent from the available runtime registry.', quietAdvisory:'This is advisory; long-running work can be legitimately quiet.', backToBacklog:'← Backlog', taskNotFound:'Task not found.', restrictedNow:'Restricted now', lastFullAccess:'Last Full Access', fullAccess:'Full Access', lastRestricted:'Last check restricted', accessLastChecked:'Access last checked', accessNotChecked:'Access not checked', networkOff:'Net off', notChecked:'Not checked', freshDispatch:'dispatch always runs a fresh active preflight.', dispatchDisabled:'Subagent dispatch disabled', enableFullAccess:'Current runtime restriction detected. Enable Full Access before dispatch.', auto:'Auto', manualMode:'Manual', noBacklog:'No backlog selected', copyCode:'Copy code', copied:'Copied', copyFailed:'Copy failed', mermaid:'Mermaid', showSource:'Show source', hideSource:'Hide source', renderingDiagram:'Rendering diagram…', mermaidUnavailable:'Mermaid renderer unavailable.', mermaidUnavailableDetail:'Task Mecca could not load the local or CDN Mermaid runtime. Use Source to view or copy the diagram text.', mermaidFailed:'Mermaid render failed.', invalidMermaid:'Invalid Mermaid syntax', secondsAgo:'{n}s ago', minutesAgo:'{n}m ago', hoursAgo:'{n}h ago', daysAgo:'{n}d ago', justNow:'just now', updatedColumn:'Updated', activeColumn:'Active', agentColumn:'Agent', statusColumn:'Status', taskColumn:'Task', expandSidebar:'Expand sidebar', collapseSidebar:'Collapse sidebar', eventRegistered:'Registered', eventStarted:'Started', eventHold:'Hold', eventCompleted:'Completed', observatory:'Backlog Observatory', theme:'Theme', followSystemTheme:'Follow system theme', useLightTheme:'Use light theme', useDarkTheme:'Use dark theme', selectBacklog:'Select backlog folder', provisionalTimingDetail:'The current state transition is not yet recorded in Git, so timing uses the runtime observation.', incompleteHistoryDetail:'Some timing cannot be reconstructed exactly because the historical lifecycle has no observed start evidence.'
  }
};
Object.assign(I18N.ko,{
  notificationsInsecureTitle:'시스템 알림을 사용할 수 없는 접속입니다.',
  notificationsInsecureGuide:'현재 페이지가 HTTPS 보안 연결이 아닙니다. 원격 브라우저의 시스템 알림 권한은 HTTPS에서만 사용할 수 있습니다. Task Mecca의 Tailscale HTTPS 주소로 접속한 뒤 다시 시도하세요.',
  notificationsIOSHomeTitle:'iPhone/iPad 알림 설정이 필요합니다.',
  notificationsIOSHomeGuide:'iOS에서는 사이트를 홈 화면에 추가한 뒤 홈 화면의 Task Mecca 웹 앱으로 열어야 알림 권한을 요청할 수 있습니다.',
  notificationsUnsupportedTitle:'이 브라우저는 시스템 알림을 지원하지 않습니다.',
  notificationsUnsupportedGuide:'현재 브라우저에서는 시스템 알림 API를 사용할 수 없습니다. 지원되는 브라우저 또는 HTTPS 환경을 사용하세요.',
  notificationPermissionError:'알림 권한 요청에 실패했습니다.',
});
Object.assign(I18N.en,{
  notificationsInsecureTitle:'System notifications are unavailable on this connection.',
  notificationsInsecureGuide:'This page is not using a secure HTTPS connection. Remote browsers require HTTPS before notification permission can be requested. Reopen Task Mecca through its Tailscale HTTPS URL and try again.',
  notificationsIOSHomeTitle:'iPhone/iPad notification setup is required.',
  notificationsIOSHomeGuide:'On iOS, add this site to the Home Screen and open the Task Mecca web app from the Home Screen before requesting notification permission.',
  notificationsUnsupportedTitle:'System notifications are not supported by this browser.',
  notificationsUnsupportedGuide:'The current browser cannot use the system Notification API. Use a supported browser or HTTPS environment.',
  notificationPermissionError:'Notification permission request failed.',
});

Object.assign(I18N.ko,{
  humanSummary:'핵심 요약', summaryPurpose:'목적', summaryChange:'핵심 변경', summaryStatusResult:'상태·결과', summaryFollowUp:'확인·후속',
  summaryFallback:'기존 기록에서 구성한 요약', detailLinks:'상세 바로가기', backgroundProblem:'배경 및 문제', workScope:'작업 범위',
  completionCriteria:'완료 기준', progressResult:'진행 상황·작업 결과', verificationDetail:'검증 상세', relatedWork:'관련 작업',
  operationsEvidence:'운영·실행 근거', summaryTodoFallback:'등록됨. 완료 기준 충족 전입니다.',
  summaryDoingFallback:'현재 구현 또는 검증이 진행 중입니다.', summaryHoldFallback:'현재 재개 조건을 기다리고 있습니다.',
  summaryDoneFallback:'완료 처리되었습니다.', noFollowUp:'현재 별도 확인·후속 사항이 없습니다.',
  detailExpand:'상세 펼치기', detailCollapse:'상세 접기', requirementsAndConstraints:'요구사항·제약', legacyDetails:'기존 작업 상세',
  source:'출처', openSource:'외부 원본 열기'
});
Object.assign(I18N.ko,{
  runtimeObservability:'Runtime 실행 관측',
  runtimeObservabilityIntro:'Codex·Claude의 실제 Subagent 실행 lifecycle을 관측합니다. 이 정보는 백로그 상태를 직접 변경하지 않습니다.',
  runtimeAttempts:'실행',
  runtimeRunning:'실행 중',
  runtimeTerminal:'종료',
  runtimeUnbound:'미연결',
  runtimeAmbiguous:'연결 모호',
  runtimeStale:'관측 정체',
  runtimeHookStatus:'Hook 상태',
  runtimeHookNote:'Hook은 별도 프로그램이 아니라 Codex/Claude 설정에 Task Mecca 관측 명령을 연결하는 규칙입니다. 설정됨과 실제 실행 가능 상태는 다를 수 있습니다.',
  runtimeHookUnconfigured:'미설정',
  runtimeHookVerificationRequired:'확인 필요',
  runtimeHookObserved:'관측 확인됨',
  runtimeHookConfigure:'설정',
  runtimeHookDisable:'관측 끄기',
  runtimeHookConfiguring:'설정 중…',
  runtimeHookDisabling:'끄는 중…',
  runtimeHookEnableConfirm:'{provider} 프로젝트 설정에 Task Mecca lifecycle Hook 규칙을 추가합니다. 기존 Hook은 유지됩니다. 계속할까요?',
  runtimeHookDisableConfirm:'{provider} 설정에서 Task Mecca가 추가한 lifecycle Hook만 제거합니다. 기존 관측 기록은 유지되고 다른 Hook은 건드리지 않습니다. 계속할까요?',
  runtimeHookActivity:'Activity',
  runtimeHookStart:'Start',
  runtimeHookStop:'Stop',
  runtimeHookReceived:'수신됨',
  runtimeHookNotObserved:'미관측',
  runtimeHookLastObserved:'마지막 수신',
  runtimeCodexUnconfigured:'설정 후 Codex에서 Hook 신뢰 검토가 필요합니다.',
  runtimeCodexVerify:'Codex에서 /hooks를 열어 Task Mecca Hook을 검토·승인하세요. 승인 전에 이미 생성된 Subagent의 Start/Stop 이벤트는 소급되지 않을 수 있습니다. 승인 후 새 Subagent를 실행해 확인하세요.',
  runtimeCodexActivityOnly:'Activity는 수신되었습니다. Start가 아직 미관측입니다. Hook 승인 전에 시작된 Agent일 수 있으므로 승인 후 새 Subagent를 생성해 Start/Stop을 확인하세요.',
  runtimeCodexObserved:'실제 Codex Hook 이벤트 수신이 확인되었습니다. Hook 정의가 변경되면 Codex에서 다시 신뢰 검토가 필요할 수 있습니다.',
  runtimeClaudeUnconfigured:'설정 후 interactive Claude Code에서는 이 프로젝트가 workspace trust된 상태여야 Hook이 실행됩니다.',
  runtimeClaudeVerify:'Interactive Claude Code에서 프로젝트 workspace trust 상태를 확인한 뒤 새 Subagent를 실행하세요. claude -p/SDK는 별도 trust dialog 없이 설정 Hook이 실행될 수 있습니다.',
  runtimeClaudeActivityOnly:'Activity는 수신되었습니다. Start가 아직 미관측입니다. 기존 Agent가 설정 이전에 시작됐을 수 있으므로 새 Subagent로 Start/Stop을 확인하세요.',
  runtimeClaudeObserved:'실제 Claude Code Hook 이벤트 수신이 확인되었습니다.',
  runtimeHookHistoryPreserved:'관측을 꺼도 기존 Execution Ledger 기록은 삭제되지 않습니다.',
  runtimeNoAttempts:'아직 관측된 Agent 실행이 없습니다. Provider 설정과 신뢰 상태를 확인한 뒤 새 Subagent를 실행하면 여기에 표시됩니다.',
  runtimeCurrentExecutions:'현재 실행',
  runtimeNoCurrentExecutions:'현재 실행 중인 Agent가 없습니다.',
  runtimeRecentCompleted:'최근 종료',
  runtimeHistoryTitle:'실행 이력',
  runtimeHistoryOpen:'실행 이력 보기',
  runtimeHistoryClose:'실행 이력 닫기',
  runtimeHistoryLoading:'실행 이력을 불러오는 중…',
  runtimeHistoryEmpty:'보관된 종료 실행 이력이 없습니다.',
  runtimeHistoryRetention:'Raw 이벤트 {raw}일 · 실행 요약 {days}일 · 최대 {max}건 보존',
  runtimeHistoryPage:'{page} / {pages} 페이지 · {total}건',
  runtimeAttempt:'Attempt',
  runtimeId:'Runtime ID',
  runtimeBinding:'Binding',
  runtimeStarted:'시작',
  runtimeLastActivity:'마지막 활동',
  runtimeEnded:'종료',
  runtimeElapsed:'경과',
  runtimeObservedActive:'관측 Active',
  runtimeWaiting:'명시적 대기',
  runtimeUnavailable:'관측 불가',
  runtimeRecentTransitions:'최근 상태 이력',
  runtimeEvidence:'근거',
  runtimeBound:'연결됨',
  runtimeStateStarting:'시작 중',
  runtimeStateRunning:'실행 중',
  runtimeStateWaitingUser:'사용자 입력 대기',
  runtimeStateWaitingApproval:'승인 대기',
  runtimeStateInterrupted:'중단',
  runtimeStateCompleted:'완료',
  runtimeStateErrored:'오류',
  runtimeStateShutdown:'종료됨',
  runtimeStateUnknown:'런타임 미확인',
  runtimeAssignedWorkload:'백로그 기반 워커 할당',
  runtimeAssignedWorkloadIntro:'아래 영역은 기존 백로그/Git 기반 할당 정보입니다. 위 runtime 관측 정보와 독립적으로 유지됩니다.'
});
Object.assign(I18N.en,{
  runtimeObservability:'Runtime execution observability',
  runtimeObservabilityIntro:'Observes actual Codex/Claude subagent execution lifecycle. This does not directly mutate backlog state.',
  runtimeAttempts:'Attempts',
  runtimeRunning:'Running',
  runtimeTerminal:'Terminal',
  runtimeUnbound:'Unbound',
  runtimeAmbiguous:'Ambiguous binding',
  runtimeStale:'Stale evidence',
  runtimeHookStatus:'Hook status',
  runtimeHookNote:'Hooks are configuration rules that connect Codex/Claude events to Task Mecca. Being configured is not the same as being trusted or actually observed.',
  runtimeHookUnconfigured:'Not configured',
  runtimeHookVerificationRequired:'Verification needed',
  runtimeHookObserved:'Observation confirmed',
  runtimeHookConfigure:'Configure',
  runtimeHookDisable:'Turn off observation',
  runtimeHookConfiguring:'Configuring…',
  runtimeHookDisabling:'Turning off…',
  runtimeHookEnableConfirm:'Add Task Mecca lifecycle Hook rules to this project’s {provider} settings while preserving existing Hooks?',
  runtimeHookDisableConfirm:'Remove only Task Mecca lifecycle Hooks from {provider}. Existing observation history and unrelated Hooks will be preserved.',
  runtimeHookActivity:'Activity',
  runtimeHookStart:'Start',
  runtimeHookStop:'Stop',
  runtimeHookReceived:'Received',
  runtimeHookNotObserved:'Not observed',
  runtimeHookLastObserved:'Last received',
  runtimeCodexUnconfigured:'After configuration, Codex requires Hook trust review.',
  runtimeCodexVerify:'Open /hooks in Codex and review/trust the Task Mecca Hooks. Start/Stop events from subagents created before trust may not be recoverable. Create a new subagent after approval to verify.',
  runtimeCodexActivityOnly:'Activity is arriving, but Start has not been observed. The agent may have started before Hook approval. Create a new subagent after approval to verify Start/Stop.',
  runtimeCodexObserved:'Actual Codex Hook events have been received. A changed Hook definition may require trust review again.',
  runtimeClaudeUnconfigured:'After configuration, interactive Claude Code requires this project to be workspace-trusted for settings Hooks to run.',
  runtimeClaudeVerify:'Confirm workspace trust in interactive Claude Code, then create a new subagent. claude -p/SDK may run settings Hooks without a separate trust dialog.',
  runtimeClaudeActivityOnly:'Activity is arriving, but Start has not been observed. Create a new subagent to verify Start/Stop after configuration/trust.',
  runtimeClaudeObserved:'Actual Claude Code Hook events have been received.',
  runtimeHookHistoryPreserved:'Turning observation off does not delete the existing Execution Ledger.',
  runtimeNoAttempts:'No Agent execution has been observed yet. Verify provider configuration/trust and create a new subagent.',
  runtimeCurrentExecutions:'Current executions',
  runtimeNoCurrentExecutions:'No Agent is currently running.',
  runtimeRecentCompleted:'Recent terminal executions',
  runtimeHistoryTitle:'Execution history',
  runtimeHistoryOpen:'View execution history',
  runtimeHistoryClose:'Close execution history',
  runtimeHistoryLoading:'Loading execution history…',
  runtimeHistoryEmpty:'No terminal execution history is retained.',
  runtimeHistoryRetention:'Raw events {raw} days · summaries {days} days · up to {max} attempts',
  runtimeHistoryPage:'Page {page} / {pages} · {total} items',
  runtimeAttempt:'Attempt',
  runtimeId:'Runtime ID',
  runtimeBinding:'Binding',
  runtimeStarted:'Started',
  runtimeLastActivity:'Last activity',
  runtimeEnded:'Ended',
  runtimeElapsed:'Elapsed',
  runtimeObservedActive:'Observed active',
  runtimeWaiting:'Explicit wait',
  runtimeUnavailable:'Unavailable',
  runtimeRecentTransitions:'Recent transitions',
  runtimeEvidence:'Evidence',
  runtimeBound:'Bound',
  runtimeStateStarting:'Starting',
  runtimeStateRunning:'Running',
  runtimeStateWaitingUser:'Waiting for user',
  runtimeStateWaitingApproval:'Waiting for approval',
  runtimeStateInterrupted:'Interrupted',
  runtimeStateCompleted:'Completed',
  runtimeStateErrored:'Errored',
  runtimeStateShutdown:'Shutdown',
  runtimeStateUnknown:'Runtime unknown',
  runtimeAssignedWorkload:'Backlog-based worker allocation',
  runtimeAssignedWorkloadIntro:'This area is the existing backlog/Git allocation view and remains independent from runtime observations above.'
});

Object.assign(I18N.en,{
  humanSummary:'Summary', summaryPurpose:'Purpose', summaryChange:'Key change', summaryStatusResult:'Status / result', summaryFollowUp:'Checks / follow-up',
  summaryFallback:'Summary derived from legacy record', detailLinks:'Open details', backgroundProblem:'Background & problem', workScope:'Work scope',
  completionCriteria:'Completion criteria', progressResult:'Progress / result', verificationDetail:'Verification details', relatedWork:'Related work',
  operationsEvidence:'Operations / evidence', summaryTodoFallback:'Registered; completion criteria are not yet satisfied.',
  summaryDoingFallback:'Implementation or verification is currently in progress.', summaryHoldFallback:'Waiting for the recorded resume condition.',
  summaryDoneFallback:'Task has been completed.', noFollowUp:'No separate check or follow-up is currently recorded.',
  detailExpand:'Expand details', detailCollapse:'Collapse details', requirementsAndConstraints:'Requirements / constraints', legacyDetails:'Legacy task details',
  source:'Source', openSource:'Open external source'
});
Object.assign(I18N.ko,{tags:'태그',tagExplore:'태그 탐색',tagFilter:'태그 필터',clearTags:'태그 필터 해제',noTags:'태그 없음',tagTotal:'전체',tagActive:'활성',tagHold:'보류',tagDone:'완료',unregisteredTag:'미등록 태그',tagDescription:'설명'});
Object.assign(I18N.en,{tags:'Tags',tagExplore:'Explore tags',tagFilter:'Tag filter',clearTags:'Clear tag filters',noTags:'No tags',tagTotal:'Total',tagActive:'Active',tagHold:'Hold',tagDone:'Done',unregisteredTag:'Unregistered tag',tagDescription:'Description'});
Object.assign(I18N.ko,{
  rootPromptTitle:'Root 세션 프롬프트', rootPromptIntro:'Root로 사용할 사용자-facing 세션에 아래 프롬프트를 그대로 붙여넣으세요.',
  rootPromptPath:'원본 파일', rootPromptUnavailable:'Root 세션 프롬프트를 불러올 수 없습니다.'
});
Object.assign(I18N.en,{
  rootPromptTitle:'Root session prompt', rootPromptIntro:'Paste the prompt below into the single user-facing session that will act as Root.',
  rootPromptPath:'Source file', rootPromptUnavailable:'The Root session prompt could not be loaded.'
});
Object.assign(I18N.ko,{
  updateAvailable:'업데이트 가능', currentVersion:'현재 버전', projectMigration:'프로젝트 마이그레이션 필요',
  newContentAvailable:'새 내용이 업데이트되었습니다.', refreshToSee:'현재 읽고 있는 내용은 유지됩니다. 새 내용을 보려면 새로고침하세요.',
  refreshNow:'새로고침', runtimeChanged:'작업 상태가 변경되었습니다.', contentChanged:'백로그 내용이 변경되었습니다.',
  statusChanges:'상태 변화', taskRegistered:'등록됨', taskRemoved:'목록에서 제거됨', moreStatusChanges:'외 {n}건',
  upgrading:'업그레이드 중…', migrating:'마이그레이션 중…'
});
Object.assign(I18N.en,{
  updateAvailable:'Update available', currentVersion:'Current version', projectMigration:'Project migration required',
  newContentAvailable:'New content is available.', refreshToSee:'Your current reading position is preserved. Refresh when you want to see the update.',
  refreshNow:'Refresh', runtimeChanged:'Task status changed.', contentChanged:'Backlog content changed.',
  statusChanges:'Status changes', taskRegistered:'Registered', taskRemoved:'Removed from backlog', moreStatusChanges:'{n} more',
  upgrading:'Upgrading…', migrating:'Migrating…'
});
Object.assign(I18N.ko,{
  releaseNotes:'업데이트 기록', releaseNotesIntro:'Task Mecca의 사용자용 변경사항을 버전별로 확인합니다.',
  releaseNotesLoading:'업데이트 기록을 불러오는 중…', releaseNotesUnavailable:'업데이트 기록을 불러올 수 없습니다.',
  releaseNotesMore:'이전 업데이트 더 보기', releaseNotesLatest:'최신', releaseNotesNew:'NEW',
  whatsNew:'이번 업데이트', updatedToVersion:'Task Mecca {version}으로 업데이트되었습니다.',
  releaseClose:'닫기', releaseDetails:'자세히 보기', releaseConfirm:'확인', releaseMarkRead:'읽음 처리',
  releaseUnreadNotice:'이 버전의 업데이트 내용을 아직 확인하지 않았습니다.',
  releaseUnreadBanner:'{version} 업데이트 내용을 아직 확인하지 않았습니다.',
  releaseViewAgain:'보기', updateChanges:'변경사항 보기', updateNow:'업데이트',
  updateChangesTitle:'Task Mecca {version} 업데이트 내용',
  updateChangesUnavailable:'이 버전의 사용자용 변경사항은 제공되지 않습니다.',
  releaseNewFeature:'새 기능', releaseImproved:'개선', releaseFixed:'수정', releaseImportant:'중요 변경',
  releaseMigrationRequired:'프로젝트 업데이트 필요', releaseInstructionRefresh:'Root 운영 지침 재확인 필요',
  releaseNoEntries:'표시할 업데이트 기록이 없습니다.'
});
Object.assign(I18N.en,{
  releaseNotes:'Update history', releaseNotesIntro:'Review user-facing Task Mecca changes by version.',
  releaseNotesLoading:'Loading update history…', releaseNotesUnavailable:'Update history is unavailable.',
  releaseNotesMore:'Load older updates', releaseNotesLatest:'Latest', releaseNotesNew:'NEW',
  whatsNew:"What's new", updatedToVersion:'Task Mecca was updated to {version}.',
  releaseClose:'Close', releaseDetails:'View details', releaseConfirm:'Got it', releaseMarkRead:'Mark as read',
  releaseUnreadNotice:'You have not marked this update as read yet.',
  releaseUnreadBanner:'You have not reviewed the {version} update yet.',
  releaseViewAgain:'View', updateChanges:'View changes', updateNow:'Update',
  updateChangesTitle:'What changes in Task Mecca {version}',
  updateChangesUnavailable:'User-facing changes are not available for this version.',
  releaseNewFeature:'New', releaseImproved:'Improved', releaseFixed:'Fixed', releaseImportant:'Important',
  releaseMigrationRequired:'Project update required', releaseInstructionRefresh:'Root operating instructions need review',
  releaseNoEntries:'No update history is available.'
});
Object.assign(I18N.ko,{
  channelSwitchTitle:'릴리스 채널을 전환할까요?', channelCurrent:'현재', channelTarget:'전환 대상',
  channelStable:'Stable', channelDev:'Dev', channelSwitchCancel:'취소', channelSwitchAction:'{channel}로 전환',
  channelDevWarning:'Dev 채널에는 검증 중인 기능이 포함될 수 있습니다.',
  channelStableNotice:'정식 Stable 채널의 최신 버전으로 돌아갑니다.',
  channelSwitching:'채널 전환 중…', channelSwitchRestart:'Task Mecca Web을 재시작하고 있습니다.',
  channelSwitchUnavailable:'대상 채널 정보를 확인할 수 없습니다.',
  frameworkSyncPreview:'채널 전환 후 {n}개 프로젝트의 framework를 {channel} 버전에 맞춰야 합니다.',
  frameworkSyncTitle:'프로젝트 framework를 {channel}에 맞출까요?',
  frameworkSyncIntro:'실행 파일과 프로젝트 framework 버전을 맞추면 채널 전환을 안전하게 완료할 수 있습니다.',
  frameworkSyncStableIntro:'Dev framework가 남아 있습니다. Stable 실행 파일과 호환되는 framework로 되돌리는 것을 권장합니다.',
  frameworkSyncLater:'나중에', frameworkSyncAction:'{channel} framework로 동기화',
  frameworkSyncing:'framework 동기화 중…', frameworkSyncDone:'프로젝트 framework 동기화를 완료했습니다.',
  frameworkSyncFailed:'framework 동기화에 실패했습니다.'
});
Object.assign(I18N.en,{
  channelSwitchTitle:'Switch release channel?', channelCurrent:'Current', channelTarget:'Target',
  channelStable:'Stable', channelDev:'Dev', channelSwitchCancel:'Cancel', channelSwitchAction:'Switch to {channel}',
  channelDevWarning:'The Dev channel may include features still under validation.',
  channelStableNotice:'Return to the latest Stable release.',
  channelSwitching:'Switching channel…', channelSwitchRestart:'Restarting Task Mecca Web.',
  channelSwitchUnavailable:'The target channel is currently unavailable.',
  frameworkSyncPreview:'After switching, {n} project framework(s) should be aligned with {channel}.',
  frameworkSyncTitle:'Align project framework with {channel}?',
  frameworkSyncIntro:'Aligning the executable and project framework versions safely completes the channel switch.',
  frameworkSyncStableIntro:'Dev framework remains in the project. Align it with the Stable executable for compatibility.',
  frameworkSyncLater:'Later', frameworkSyncAction:'Sync to {channel} framework',
  frameworkSyncing:'Syncing framework…', frameworkSyncDone:'Project framework sync completed.',
  frameworkSyncFailed:'Framework sync failed.'
});

function t(key, vars = {}) {
  const dict = I18N[state.language] || I18N.en;
  let value = dict[key] ?? I18N.en[key] ?? key;
  Object.entries(vars).forEach(([k,v]) => { value = value.replaceAll(`{${k}}`, String(v)); });
  return value;
}
function localeCode() { return LANGUAGES[state.language]?.locale || 'en-US'; }

const $ = s => document.querySelector(s);
const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
function releaseLocalized(value) {
  if(value==null)return '';
  if(typeof value==='string')return value;
  return String(value[state.language]||value.en||value.ko||'');
}
function releaseSectionLabel(type) {
  return t({new:'releaseNewFeature',improved:'releaseImproved',fixed:'releaseFixed',important:'releaseImportant'}[type]||type);
}
function releaseSectionMarkup(sections=[]) {
  return (Array.isArray(sections)?sections:[]).map(section=>{
    const items=(section.items||[]).map(item=>'<li>'+esc(releaseLocalized(item))+'</li>').join('');
    if(!items)return '';
    return '<section class="release-section release-'+esc(section.type||'improved')+'"><h3>'+esc(releaseSectionLabel(section.type))+'</h3><ul>'+items+'</ul></section>';
  }).join('');
}
function currentReleaseVersion() {
  return normalizedVersion(state.versionInfo?.cli?.current||state.hub?.cli?.current||'');
}
function releaseNoteKnown(version) {
  return Boolean(state.releaseNoteDetails[version] || state.releaseNotes.some(item=>item.version===version));
}
function releaseNoteUnread(version=currentReleaseVersion()) {
  const current=currentReleaseVersion();
  if(!version||version!==current||!releaseNoteKnown(version))return false;
  return localStorage.getItem('task-mecca-release-notes-seen-version')!==version;
}
function renderReleaseNotesBadge() {
  const badge=$('#releaseNotesBadge');
  if(!badge)return;
  const current=currentReleaseVersion();
  if(releaseNoteUnread(current)){
    badge.textContent=t('releaseNotesNew');
    badge.classList.add('new');
    badge.removeAttribute('aria-hidden');
  }else{
    badge.textContent='';
    badge.classList.remove('new');
    badge.setAttribute('aria-hidden','true');
  }
}
function markReleaseNoteSeen(version) {
  if(!version)return;
  localStorage.setItem('task-mecca-release-notes-seen-version',version);
  localStorage.setItem('task-mecca-release-notes-dismissed-version',version);
  if(state.releaseNotePopup?.version===version)state.releaseNotePopup=null;
  state.releaseNotePopupMode='installed';
  renderReleaseNoteModal();
  renderReleaseNotesBadge();
  renderReleaseUnreadPrompt();
  if(state.view==='release-notes')render();
}
function dismissReleaseNotePopup() {
  const version=state.releaseNotePopup?.version;
  if(version&&state.releaseNotePopupMode!=='available'){
    localStorage.setItem('task-mecca-release-notes-dismissed-version',version);
  }
  state.releaseNotePopup=null;
  state.releaseNotePopupMode='installed';
  renderReleaseNoteModal();
  renderReleaseNotesBadge();
  renderReleaseUnreadPrompt();
}
function renderReleaseUnreadPrompt() {
  const el=$('#releaseUnreadPrompt');
  if(!el)return;
  const version=currentReleaseVersion();
  const dismissed=localStorage.getItem('task-mecca-release-notes-dismissed-version')===version;
  const visible=Boolean(version)&&!version.includes('-dev.')&&dismissed&&releaseNoteUnread(version)&&!state.releaseNotePopup&&state.view!=='release-notes';
  if(!visible){
    el.innerHTML='';
    return;
  }
  el.innerHTML='<div class="release-unread-copy"><span class="global-update-dot"></span><strong>'+esc(t('releaseUnreadBanner',{version}))+'</strong></div>'+
    '<button type="button" class="release-unread-action" id="releaseUnreadViewBtn">'+esc(t('releaseViewAgain'))+'</button>';
  $('#releaseUnreadViewBtn')?.addEventListener('click',async()=>{
    const detail=await loadReleaseNoteDetail(version);
    if(!detail)return;
    state.releaseNotePopup=detail;
    state.releaseNotePopupMode='installed';
    renderReleaseUnreadPrompt();
    renderReleaseNoteModal();
  });
}
async function loadReleaseNoteDetail(version) {
  version=String(version||'').trim();
  if(!version)return null;
  if(state.releaseNoteDetails[version])return state.releaseNoteDetails[version];
  try{
    const r=await fetch('/api/release-notes/'+encodeURIComponent(version),{cache:'no-store'});
    if(!r.ok)return null;
    const detail=await r.json();
    state.releaseNoteDetails[version]=detail;
    renderReleaseNotesBadge();
    return detail;
  }catch(_){ return null; }
}
async function loadReleaseNotes(reset=false) {
  if(state.releaseNotesLoading)return;
  if(reset){
    state.releaseNotes=[];
    state.releaseNotesTotal=0;
    state.releaseNotesHasMore=false;
    state.releaseNotesLoaded=false;
  }else if(state.releaseNotesLoaded&&!state.releaseNotesHasMore){
    return;
  }
  state.releaseNotesLoading=true;
  if(state.view==='release-notes')render();
  try{
    const offset=reset?0:state.releaseNotes.length;
    const r=await fetch('/api/release-notes?offset='+offset+'&limit=20',{cache:'no-store'});
    if(!r.ok)throw new Error('HTTP '+r.status);
    const body=await r.json();
    const items=Array.isArray(body.items)?body.items:[];
    state.releaseNotes=reset?items:[...state.releaseNotes,...items.filter(item=>!state.releaseNotes.some(existing=>existing.version===item.version))];
    state.releaseNotesTotal=Number(body.total)||state.releaseNotes.length;
    state.releaseNotesHasMore=Boolean(body.has_more);
    state.releaseNotesLoaded=true;
  }catch(_){
    state.releaseNotesLoaded=true;
  }finally{
    state.releaseNotesLoading=false;
    renderReleaseNotesBadge();
    if(state.view==='release-notes')render();
  }
}
function releaseNoteDetailMarkup(detail,options={}) {
  if(!detail)return '';
  const compact=Boolean(options.compact);
  const migration=detail.migration||{};
  const flags=[
    migration.required?'<span class="release-flag important">'+esc(t('releaseMigrationRequired'))+'</span>':'',
    migration.instruction_refresh?'<span class="release-flag">'+esc(t('releaseInstructionRefresh'))+'</span>':''
  ].filter(Boolean).join('');
  return '<div class="release-detail '+(compact?'compact':'')+'">'+
    '<div class="release-detail-head"><div><span class="release-version">v'+esc(detail.version||'')+'</span><span class="release-date">'+esc(detail.date||'')+'</span></div>'+(flags?'<div class="release-flags">'+flags+'</div>':'')+'</div>'+
    '<p class="release-summary">'+esc(releaseLocalized(detail.summary))+'</p>'+
    '<div class="release-sections">'+releaseSectionMarkup(detail.sections||[])+'</div></div>';
}
function renderReleaseNoteModal() {
  const modal=$('#releaseNoteModal');
  if(!modal)return;
  const detail=state.releaseNotePopup;
  if(!detail){
    modal.hidden=true;
    modal.innerHTML='';
    document.body.classList.remove('release-modal-open');
    renderReleaseUnreadPrompt();
    return;
  }
  const available=state.releaseNotePopupMode==='available';
  modal.hidden=false;
  document.body.classList.add('release-modal-open');
  const title=available?t('updateChangesTitle',{version:detail.version||''}):t('updatedToVersion',{version:detail.version||''});
  const actions=available
    ? '<button type="button" class="action-btn secondary" id="releaseModalClose">'+esc(t('releaseClose'))+'</button><button type="button" class="action-btn" id="releaseModalUpgrade">'+esc(t('updateNow'))+'</button>'
    : '<button type="button" class="action-btn secondary" id="releaseModalDetails">'+esc(t('releaseDetails'))+'</button><button type="button" class="action-btn" id="releaseModalConfirm">'+esc(t('releaseConfirm'))+'</button>';
  modal.innerHTML='<div class="release-modal-backdrop" data-release-dismiss></div>'+
    '<section class="release-modal-card" role="dialog" aria-modal="true" aria-labelledby="releaseModalTitle">'+
      '<button class="release-modal-close" type="button" data-release-dismiss aria-label="'+esc(t('releaseClose'))+'" title="'+esc(t('releaseClose'))+'">×</button>'+
      '<div class="eyebrow">'+esc(available?t('updateChanges'):t('whatsNew'))+'</div>'+
      '<h2 id="releaseModalTitle">'+esc(title)+'</h2>'+
      releaseNoteDetailMarkup(detail,{compact:true})+
      '<div class="release-modal-actions">'+actions+'</div>'+
    '</section>';
  modal.querySelectorAll('[data-release-dismiss]').forEach(el=>el.addEventListener('click',dismissReleaseNotePopup));
  if(available){
    $('#releaseModalClose')?.addEventListener('click',dismissReleaseNotePopup);
    $('#releaseModalUpgrade')?.addEventListener('click',()=>{
      state.releaseNotePopup=null;
      state.releaseNotePopupMode='installed';
      renderReleaseNoteModal();
      performUpgrade();
    });
  }else{
    $('#releaseModalConfirm')?.addEventListener('click',()=>markReleaseNoteSeen(detail.version));
    $('#releaseModalDetails')?.addEventListener('click',()=>{
      localStorage.setItem('task-mecca-release-notes-dismissed-version',detail.version);
      state.releaseNotePopup=null;
      state.releaseNotePopupMode='installed';
      renderReleaseNoteModal();
      navigateView('release-notes');
    });
  }
}
async function showAvailableUpdateNotes() {
  const cli=state.versionInfo?.cli||state.hub?.cli||{};
  const version=normalizedVersion(cli.latest||'');
  if(!version||cli.channel==='dev')return;
  const detail=await loadReleaseNoteDetail(version);
  if(!detail){
    alert(t('updateChangesUnavailable'));
    return;
  }
  state.releaseNotePopup=detail;
  state.releaseNotePopupMode='available';
  renderReleaseUnreadPrompt();
  renderReleaseNoteModal();
}
async function maybeShowCurrentReleaseNote() {
  const version=currentReleaseVersion();
  if(!version||version.includes('-dev.')){ renderReleaseNotesBadge(); return; }
  if(state.releaseNotePopupCheckedVersion===version){ renderReleaseNotesBadge(); return; }
  state.releaseNotePopupCheckedVersion=version;
  const detail=await loadReleaseNoteDetail(version);
  if(!detail){ renderReleaseNotesBadge(); return; }
  renderReleaseNotesBadge();
  const seen=localStorage.getItem('task-mecca-release-notes-seen-version');
  const dismissed=localStorage.getItem('task-mecca-release-notes-dismissed-version');
  if(seen===version||dismissed===version){
    renderReleaseUnreadPrompt();
    return;
  }
  state.releaseNotePopup=detail;
  state.releaseNotePopupMode='installed';
  renderReleaseUnreadPrompt();
  renderReleaseNoteModal();
}


function renderGlobalUpdateIndicator() {
  const el=$('#globalUpdateIndicator');
  if(!el)return;
  const payload=state.versionInfo||{};
  const cli=payload.cli||state.hub?.cli||{};
  const project=payload.project||{};
  const projectMatches=state.project && project.path===state.project;
  if(cli.update_available){
    const canShowChanges=cli.channel!=='dev';
    el.innerHTML='<div class="global-update-group">'+
      '<span class="global-update-pill available"><span class="global-update-dot"></span><span>'+esc(t('updateAvailable'))+'</span><strong>'+esc(cli.latest||'')+'</strong></span>'+
      (canShowChanges?'<button type="button" class="global-update-link" id="globalUpdateChangesBtn">'+esc(t('updateChanges'))+'</button>':'')+
      '<button type="button" class="global-update-link primary" id="globalUpgradeBtn">'+esc(t('updateNow'))+'</button></div>';
    $('#globalUpdateChangesBtn')?.addEventListener('click',showAvailableUpdateNotes);
    $('#globalUpgradeBtn')?.addEventListener('click',e=>performUpgrade(e.currentTarget));
    return;
  }
  if(projectMatches && project.migration_available){
    el.innerHTML=`<button type="button" class="global-update-pill migration" id="globalMigrateBtn" title="${esc(t('projectMigration'))}"><span class="global-update-dot"></span><span>${esc(t('projectMigration'))}</span><strong>${esc(project.framework_version||'?')} → ${esc(cli.current||'')}</strong></button>`;
    $('#globalMigrateBtn')?.addEventListener('click',e=>performProjectMigration(state.project,e.currentTarget));
    return;
  }
  el.innerHTML='';
}

function revisionStateMap(rows=[]) {
  const out=new Map();
  (Array.isArray(rows)?rows:[]).forEach(row=>{
    const id=String(row?.id||'').toUpperCase();
    if(id)out.set(id,{id,title:String(row?.title||id),file_state:String(row?.file_state||'')});
  });
  return out;
}

function diffRevisionStates(beforeRows=[],afterRows=[]) {
  const before=revisionStateMap(beforeRows), after=revisionStateMap(afterRows), changes=[];
  after.forEach((next,id)=>{
    const previous=before.get(id);
    if(!previous){
      changes.push({id,title:next.title,from:'',to:next.file_state,kind:'registered'});
      return;
    }
    if(previous.file_state!==next.file_state){
      changes.push({id,title:next.title||previous.title,from:previous.file_state,to:next.file_state,kind:'state'});
    }
  });
  before.forEach((previous,id)=>{
    if(!after.has(id))changes.push({id,title:previous.title,from:previous.file_state,to:'',kind:'removed'});
  });
  changes.sort((a,b)=>a.id.localeCompare(b.id));
  return changes;
}

function statusChangeMarkup(changes=[]) {
  if(!changes.length)return '';
  const visible=changes.slice(0,3);
  const rows=visible.map(change=>{
    let transition='';
    if(change.kind==='registered')transition=`${t('taskRegistered')} · ${stateLabel(change.to)}`;
    else if(change.kind==='removed')transition=t('taskRemoved');
    else transition=`${stateLabel(change.from)} → ${stateLabel(change.to)}`;
    return `<li class="${change.to==='done'?'completed':''}"><strong>${esc(change.id)}</strong><span>${esc(change.title)}</span><b>${esc(transition)}</b></li>`;
  }).join('');
  const more=changes.length>visible.length?`<div class="content-update-more">${esc(t('moreStatusChanges',{n:changes.length-visible.length}))}</div>`:'';
  return `<div class="content-update-changes"><span class="content-update-changes-title">${esc(t('statusChanges'))}</span><ul>${rows}</ul>${more}</div>`;
}

function renderContentUpdatePrompt() {
  const el=$('#contentUpdatePrompt');
  if(!el)return;
  if(!state.pendingContentUpdate || !state.project || state.view!=='backlog'){
    el.innerHTML='';
    return;
  }
  const reason=state.pendingContentReason==='runtime'?t('runtimeChanged'):t('contentChanged');
  el.innerHTML=`<div class="content-update-copy"><strong>${esc(t('newContentAvailable'))}</strong><span class="content-update-reason">${esc(reason)} ${esc(t('refreshToSee'))}</span>${statusChangeMarkup(state.pendingContentChanges)}</div><button type="button" class="content-update-action" id="contentUpdateRefreshBtn">${esc(t('refreshNow'))}</button>`;
  $('#contentUpdateRefreshBtn')?.addEventListener('click',refreshVisibleContent);
}

function markContentUpdate(reason='content',changes=[]) {
  if(!state.project)return;
  if(state.view!=='backlog'){
    queueMicrotask(()=>refresh());
    return;
  }
  const wasPending=state.pendingContentUpdate;
  const previousReason=state.pendingContentReason;
  state.pendingContentUpdate=true;
  if(reason==='runtime' || !state.pendingContentReason)state.pendingContentReason=reason;
  if(Array.isArray(changes) && changes.length)state.pendingContentChanges=changes;
  if(!wasPending || previousReason!==state.pendingContentReason || changes.length)renderContentUpdatePrompt();
}

function acceptContentRevision(revision='',states=null) {
  if(revision)state.contentRevision=revision;
  if(Array.isArray(states))state.contentStateSnapshot=states;
  state.pendingContentUpdate=false;
  state.pendingContentReason='';
  state.pendingContentChanges=[];
  renderContentUpdatePrompt();
}

async function refreshVersionInfo(force=false) {
  const params=new URLSearchParams();
  if(state.project)params.set('project',state.project);
  if(force)params.set('refresh','1');
  try{
    const r=await fetch('/api/version?'+params.toString(),{cache:'no-store'});
    if(!r.ok)throw new Error(`HTTP ${r.status}`);
    state.versionInfo=await r.json();
    renderGlobalUpdateIndicator();
    renderReleaseNotesBadge();
    queueMicrotask(()=>maybeShowCurrentReleaseNote());
    queueMicrotask(()=>maybeShowPendingFrameworkSync());
  }catch(_){}
}

let revisionCheckInFlight=null;
async function checkContentRevision(establishOnly=false) {
  if(!state.project)return;
  if(revisionCheckInFlight)return revisionCheckInFlight;
  const project=state.project, backlog=state.backlog;
  revisionCheckInFlight=(async()=>{
    try{
      const params=new URLSearchParams();
      params.set('project',project);
      if(backlog)params.set('backlog',backlog);
      const r=await fetch('/api/revision?'+params.toString(),{cache:'no-store'});
      if(!r.ok)return;
      const body=await r.json();
      if(project!==state.project || backlog!==state.backlog)return;
      const revision=body.revision||'', states=Array.isArray(body.states)?body.states:[];
      if(!revision)return;
      if(!state.contentRevision || establishOnly){
        acceptContentRevision(revision,states);
        return;
      }
      if(revision!==state.contentRevision){
        const changes=diffRevisionStates(state.contentStateSnapshot,states);
        if(state.view==='backlog')markContentUpdate('content',changes);
        else {
          acceptContentRevision(revision,states);
          queueMicrotask(()=>refresh());
        }
      } else if(!state.pendingContentUpdate && states.length) {
        state.contentStateSnapshot=states;
      }
    }catch(_){}
    finally{revisionCheckInFlight=null;}
  })();
  return revisionCheckInFlight;
}

async function refreshVisibleContent() {
  const scrollY=window.scrollY;
  state.pendingContentUpdate=false;
  state.pendingContentReason='';
  renderContentUpdatePrompt();
  if(state.view==='backlog'){
    await refreshList();
    if(state.detail)await loadTaskDetail(state.detail);
    await checkContentRevision(true);
  } else {
    await refresh();
    await checkContentRevision(true);
  }
  requestAnimationFrame(()=>window.scrollTo({top:scrollY,left:0,behavior:'auto'}));
}

async function performUpgrade(button) {
  if(button){button.disabled=true;button.textContent=t('upgrading');}
  const content=$('#content');
  try{
    const r=await fetch('/api/upgrade',{method:'POST',headers:{'X-Task-Mecca-Action':'1'}});
    const body=await r.json();
    if(!r.ok)throw new Error(body.error||'Upgrade failed');
    if(body.restart_required && body.to && body.to!==body.from){
      if(content)content.innerHTML=`<div class="upgrade-restart"><div class="upgrade-spinner"></div><h2>Task Mecca ${esc(body.to)}로 업그레이드했습니다</h2><p>Web 서버를 재시작하고 있습니다. 완료되면 이 페이지가 자동으로 새로고침됩니다.</p></div>`;
      await waitForRestartedWeb(body.to);
      return;
    }
    await refreshVersionInfo(true);
  }catch(e){
    alert(String(e?.message||e));
    if(button){button.disabled=false;renderGlobalUpdateIndicator();}
  }
}

async function loadRuntimeHistory(page=1) {
  state.runtimeHistoryLoading=true;
  state.runtimeHistoryError='';
  if(state.view==='workload')render();
  try{
    const params=new URLSearchParams();
    if(state.project)params.set('project',state.project);
    params.set('page',String(Math.max(1,Number(page)||1)));
    params.set('page_size','20');
    const r=await fetch('/api/runtime/history?'+params.toString(),{cache:'no-store'});
    const body=await r.json();
    if(!r.ok)throw new Error(body.error||`HTTP ${r.status}`);
    state.runtimeHistory=body;
    state.runtimeHistoryLoading=false;
    state.runtimeHistoryError='';
  }catch(e){
    state.runtimeHistoryLoading=false;
    state.runtimeHistoryError=String(e?.message||e||'Runtime history failed');
  }
  if(state.view==='workload')render();
}
async function toggleRuntimeHistory() {
  state.runtimeHistoryOpen=!state.runtimeHistoryOpen;
  if(state.runtimeHistoryOpen && !(state.runtimeHistory?.items||[]).length && !state.runtimeHistoryLoading) {
    await loadRuntimeHistory(1);
    return;
  }
  render();
}

async function loadRuntimeStorage(force=false) {
  if(state.runtimeStorageLoading)return;
  if(state.runtimeStorage&&!force)return;
  state.runtimeStorageLoading=true;
  state.runtimeStorageError='';
  if(state.view==='workload')render();
  try{
    const params=new URLSearchParams();
    if(state.project)params.set('project',state.project);
    const qs=params.toString()?`?${params}`:'';
    const r=await fetch('/api/runtime/storage'+qs,{cache:'no-store'});
    const body=await r.json();
    if(!r.ok)throw new Error(body.error||`HTTP ${r.status}`);
    state.runtimeStorage=body;
  }catch(e){
    state.runtimeStorageError=String(e?.message||e||'Runtime storage failed');
  }finally{
    state.runtimeStorageLoading=false;
  }
  if(state.view==='workload')render();
}
async function toggleRuntimeStorage() {
  state.runtimeStorageOpen=!state.runtimeStorageOpen;
  if(state.runtimeStorageOpen) {
    await loadRuntimeStorage(true);
    return;
  }
  render();
}
async function performRuntimeCleanup(button) {
  const reclaim=Number(state.runtimeStorage?.cleanup?.reclaimable_bytes||0);
  if(reclaim<=0)return;
  if(!window.confirm(t('runtimeCleanupConfirm',{bytes:fmtBytes(reclaim)})))return;
  const original=button?.textContent||'';
  if(button){button.disabled=true;button.textContent=t('runtimeCleanupRunning');}
  try{
    const params=new URLSearchParams();
    if(state.project)params.set('project',state.project);
    const qs=params.toString()?`?${params}`:'';
    const r=await fetch('/api/runtime/storage'+qs,{
      method:'POST',
      headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},
      body:JSON.stringify({action:'cleanup'})
    });
    const body=await r.json();
    if(!r.ok)throw new Error(body.error||'Runtime cleanup failed');
    state.runtimeStorage=body.storage||null;
    alert(t('runtimeCleanupDone',{bytes:fmtBytes(body.result?.reclaimed_bytes||0)}));
    state.runtimeHistory={items:[],page:1,page_size:20,total:0,total_pages:0,retention:{}};
    if(state.runtimeHistoryOpen)await loadRuntimeHistory(1);
    await refresh();
  }catch(e){
    alert(String(e?.message||e));
    if(button){button.disabled=false;button.textContent=original;}
  }
}

async function performRuntimeHookAction(button) {
  const provider=button?.dataset?.provider||'all';
  const action=button?.dataset?.runtimeHookAction||'enable';
  const providerLabel=String(provider).toUpperCase();
  const confirmKey=action==='disable'?'runtimeHookDisableConfirm':'runtimeHookEnableConfirm';
  if(!window.confirm(t(confirmKey,{provider:providerLabel})))return;
  const original=button?.textContent||'';
  if(button){
    button.disabled=true;
    button.textContent=action==='disable'?t('runtimeHookDisabling'):t('runtimeHookConfiguring');
  }
  try{
    const params=new URLSearchParams();
    if(state.project)params.set('project',state.project);
    const qs=params.toString()?`?${params}`:'';
    const r=await fetch('/api/runtime/hooks'+qs,{
      method:'POST',
      headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},
      body:JSON.stringify({provider,action})
    });
    const body=await r.json();
    if(!r.ok)throw new Error(body.error||'Runtime hook action failed');
    await refresh();
  }catch(e){
    alert(String(e?.message||e));
    if(button){button.disabled=false;button.textContent=original;}
  }
}


async function performProjectMigration(project,button) {
  if(!project)return;
  if(button){button.disabled=true;button.textContent=t('migrating');}
  try{
    const r=await fetch('/api/migrate',{method:'POST',headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},body:JSON.stringify({project})});
    const body=await r.json();
    if(!r.ok)throw new Error(body.error||'Migration failed');
    await refreshVersionInfo(false);
    await refreshVisibleContent();
    if(body.instruction_refresh_required||body.legacy_bootstrap)showMigrationResyncModal(body);
  }catch(e){
    alert(String(e?.message||e));
    if(button){button.disabled=false;renderGlobalUpdateIndicator();}
  }
}
const fmtSec = n => {
  if (n == null || Number.isNaN(+n)) return '-';
  n = Math.max(0, Math.floor(+n));
  const d = Math.floor(n / 86400); n %= 86400;
  const h = Math.floor(n / 3600); n %= 3600;
  const m = Math.floor(n / 60), s = n % 60;
  return d ? `${d}d ${String(h).padStart(2,'0')}h ${String(m).padStart(2,'0')}m`
    : h ? `${String(h).padStart(2,'0')}:${String(m).padStart(2,'0')}:${String(s).padStart(2,'0')}`
    : `${String(m).padStart(2,'0')}:${String(s).padStart(2,'0')}`;
};
const ago = iso => {
  if (!iso) return '-';
  const sec = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (sec < 5) return t('justNow');
  if (sec < 60) return t('secondsAgo',{n:Math.floor(sec)});
  if (sec < 3600) return t('minutesAgo',{n:Math.floor(sec/60)});
  if (sec < 86400) return t('hoursAgo',{n:Math.floor(sec/3600)});
  return t('daysAgo',{n:Math.floor(sec/86400)});
};
const dateLabel = iso => {
  if (!iso) return '-';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '-';
  return d.toLocaleDateString(localeCode(), {year:'numeric', month:'2-digit', day:'2-digit'});
};
const dateTimeLabel = (iso, compact=false) => {
  if (!iso) return '-';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '-';
  const opts = compact
    ? {month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit'}
    : {year:'numeric', month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit', second:'2-digit'};
  return d.toLocaleString(localeCode(), opts);
};
function inline(s) {
  return esc(s)
    .replace(/`([^`]+)`/g,'<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g,'<strong>$1</strong>')
    .replace(/\[([^\]]+)\]\((https?:\/\/[^)]+)\)/g,'<a href="$2" target="_blank" rel="noreferrer">$1</a>');
}
function markdown(md, opts = {}) {
  const copyCode = Boolean(opts.copyCode);
  md = String(md || '').trim();
  if (!md) return '<p class="summary">-</p>';
  const lines = md.split(/\r?\n/);
  let out = '', list = null, code = false, codeLang = '', codeLines = [], i = 0;
  const close = () => { if (list) { out += `</${list}>`; list = null; } };
  const flushCode = () => {
    const raw = codeLines.join('\n');
    const body = esc(raw);
    out += codeLang === 'mermaid' ? mermaidBlock(body) : (copyCode ? copyableCodeBlock(body) : `<pre><code>${body}</code></pre>`);
    code = false; codeLang = ''; codeLines = [];
  };
  while (i < lines.length) {
    const l = lines[i];
    if (l.startsWith('```')) {
      close();
      if (code) flushCode();
      else { code = true; codeLang = l.slice(3).trim().toLowerCase().split(/\s+/)[0] || ''; }
      i++; continue;
    }
    if (code) { codeLines.push(l); i++; continue; }
    if (/^\|.*\|$/.test(l) && i+1 < lines.length && /^\|\s*:?-+/.test(lines[i+1])) {
      close();
      const head = l.slice(1,-1).split('|').map(x=>x.trim()); i += 2;
      const rows = [];
      while (i < lines.length && /^\|.*\|$/.test(lines[i])) { rows.push(lines[i].slice(1,-1).split('|').map(x=>x.trim())); i++; }
      out += '<table><thead><tr>' + head.map(x=>`<th>${inline(x)}</th>`).join('') + '</tr></thead><tbody>' +
        rows.map(r=>'<tr>'+head.map((_,j)=>`<td>${inline(r[j]||'')}</td>`).join('')+'</tr>').join('') + '</tbody></table>';
      continue;
    }
    const h = l.match(/^(#{1,4})\s+(.+)/);
    if (h) { close(); const n = h[1].length; out += `<h${n}>${inline(h[2])}</h${n}>`; i++; continue; }
    const chk = l.match(/^-\s+\[([ xX])\]\s+(.+)/);
    if (chk) { close(); out += `<div class="check ${chk[1].toLowerCase()==='x'?'done':''}"><span class="check-box">${chk[1].toLowerCase()==='x'?'✓':''}</span><span>${inline(chk[2])}</span></div>`; i++; continue; }
    const ul = l.match(/^\s*-\s+(.+)/), ol = l.match(/^\s*\d+\.\s+(.+)/);
    if (ul || ol) {
      const want = ul ? 'ul' : 'ol';
      if (list !== want) { close(); list = want; out += `<${want}>`; }
      out += `<li>${inline((ul || ol)[1])}</li>`; i++; continue;
    }
    if (!l.trim()) { close(); i++; continue; }
    close(); out += `<p>${inline(l)}</p>`; i++;
  }
  close();
  if (code) flushCode();
  return out;
}

const MERMAID_LOCAL_URL = '/vendor/mermaid.min.js';
const MERMAID_CDN_URLS = [
  'https://cdnjs.cloudflare.com/ajax/libs/mermaid/11.10.0/mermaid.min.js',
  'https://cdn.jsdelivr.net/npm/mermaid@11.10.0/dist/mermaid.min.js',
];
let mermaidModulePromise = null;
let mermaidRenderSeq = 0;
const SOURCE_ICON = `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 18-6-6 6-6"/><path d="m15 6 6 6-6 6"/></svg>`;
function mermaidBlock(body) {
  return `<div class="mermaid-wrap"><div class="mermaid-toolbar"><span class="mermaid-label">${esc(t('mermaid'))}</span><div class="mermaid-actions"><button class="mermaid-source-toggle" type="button" aria-label="${esc(t('showSource'))}" title="${esc(t('showSource'))}">${SOURCE_ICON}</button><button class="copy-code-btn mermaid-copy-btn" type="button" aria-label="${esc(t('copyCode'))}" title="${esc(t('copyCode'))}">${COPY_ICON}</button></div></div><div class="mermaid-canvas" aria-busy="true"><span class="mermaid-loading">${esc(t('renderingDiagram'))}</span></div><div class="mermaid-source" hidden><pre><code>${body}</code></pre></div></div>`;
}
function effectiveTheme() {
  if (state.theme === 'dark' || state.theme === 'light') return state.theme;
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}
function loadMermaidScript(url, label) {
  if (window.mermaid) return Promise.resolve(window.mermaid);
  return new Promise((resolve, reject) => {
    const existing = [...document.scripts].find(s => s.src === new URL(url, window.location.href).href);
    const finish = () => window.mermaid
      ? resolve(window.mermaid)
      : reject(new Error(`${label} loaded but did not expose window.mermaid`));
    if (existing) {
      if (existing.dataset.taskMeccaLoaded === '1') { finish(); return; }
      existing.addEventListener('load', finish, {once:true});
      existing.addEventListener('error', () => reject(new Error(`${label} failed to load`)), {once:true});
      return;
    }
    const script = document.createElement('script');
    script.src = url;
    script.async = true;
    script.dataset.taskMeccaMermaid = '1';
    script.onload = () => { script.dataset.taskMeccaLoaded = '1'; finish(); };
    script.onerror = () => reject(new Error(`${label} failed to load`));
    document.head.appendChild(script);
  });
}
function loadLocalMermaid() {
  return loadMermaidScript(MERMAID_LOCAL_URL, 'Local Mermaid runtime');
}
async function loadCdnMermaid() {
  let lastError = null;
  for (const [idx, url] of MERMAID_CDN_URLS.entries()) {
    try { return await loadMermaidScript(url, `Mermaid CDN runtime ${idx + 1}`); }
    catch (err) { lastError = err; }
  }
  throw lastError || new Error('All Mermaid CDN runtimes failed to load');
}
async function getMermaid() {
  if (!mermaidModulePromise) {
    mermaidModulePromise = loadLocalMermaid().catch(() => loadCdnMermaid());
  }
  return mermaidModulePromise;
}
async function renderMermaidDiagrams(force = false) {
  const blocks = [...document.querySelectorAll('.mermaid-wrap')];
  if (!blocks.length) return;
  let mermaid;
  try { mermaid = await getMermaid(); }
  catch (err) {
    blocks.forEach(block => {
      const canvas = block.querySelector('.mermaid-canvas');
      if (!canvas) return;
      canvas.setAttribute('aria-busy','false');
      canvas.innerHTML = `<div class="mermaid-error"><strong>${esc(t('mermaidUnavailable'))}</strong><span>${esc(t('mermaidUnavailableDetail'))}</span></div>`;
    });
    return;
  }
  const theme = effectiveTheme();
  mermaid.initialize({startOnLoad:false, securityLevel:'strict', theme, suppressErrorRendering:true});
  for (const block of blocks) {
    if (!force && block.dataset.renderedTheme === theme) continue;
    const code = block.querySelector('.mermaid-source code')?.textContent?.trim() || '';
    const canvas = block.querySelector('.mermaid-canvas');
    if (!canvas || !code) continue;
    canvas.setAttribute('aria-busy','true');
    try {
      const id = `task-mecca-mermaid-${++mermaidRenderSeq}`;
      const {svg, bindFunctions} = await mermaid.render(id, code);
      canvas.innerHTML = svg;
      bindFunctions?.(canvas);
      block.dataset.renderedTheme = theme;
      canvas.setAttribute('aria-busy','false');
    } catch (err) {
      canvas.setAttribute('aria-busy','false');
      canvas.innerHTML = `<div class="mermaid-error"><strong>${esc(t('mermaidFailed'))}</strong><span>${esc(String(err?.message || err || t('invalidMermaid')))}</span></div>`;
    }
  }
}
function bindMermaidControls() {
  document.querySelectorAll('.mermaid-source-toggle').forEach(btn => {
    if (btn.dataset.bound === '1') return; btn.dataset.bound = '1';
    btn.addEventListener('click', () => {
      const wrap = btn.closest('.mermaid-wrap'); const src = wrap?.querySelector('.mermaid-source'); if (!src) return;
      src.hidden = !src.hidden; btn.classList.toggle('active', !src.hidden); btn.title = src.hidden ? t('showSource') : t('hideSource');
    });
  });
}

const COPY_ICON = `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M9 8h9a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2v-9a2 2 0 0 1 2-2Z"/><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h2"/></svg>`;
const CHECK_ICON = `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m5 12 4 4L19 6"/></svg>`;
function copyableCodeBlock(body) {
  return `<div class="copy-code-wrap"><button class="copy-code-btn" type="button" aria-label="${esc(t('copyCode'))}" title="${esc(t('copyCode'))}">${COPY_ICON}</button><pre><code>${body}</code></pre></div>`;
}
async function copyText(text) {
  if (navigator.clipboard && window.isSecureContext) { await navigator.clipboard.writeText(text); return; }
  const ta=document.createElement('textarea'); ta.value=text; ta.setAttribute('readonly',''); ta.style.position='fixed'; ta.style.opacity='0'; document.body.appendChild(ta); ta.select();
  const ok=document.execCommand('copy'); ta.remove(); if(!ok) throw new Error('copy failed');
}
function bindCopyButtons() {
  document.querySelectorAll('.copy-code-btn').forEach(btn=>{
    if(btn.dataset.bound==='1') return; btn.dataset.bound='1';
    btn.addEventListener('click',async()=>{
      const code=btn.closest('.copy-code-wrap')?.querySelector('code') || btn.closest('.mermaid-wrap')?.querySelector('.mermaid-source code'); if(!code) return;
      try {
        await copyText(code.textContent || '');
        btn.classList.add('copied'); btn.innerHTML=CHECK_ICON; btn.setAttribute('aria-label',t('copied')); btn.title=t('copied');
        setTimeout(()=>{btn.classList.remove('copied');btn.innerHTML=COPY_ICON;btn.setAttribute('aria-label',t('copyCode'));btn.title=t('copyCode');},1500);
      } catch(e) { btn.classList.add('copy-error'); btn.title=t('copyFailed'); setTimeout(()=>btn.classList.remove('copy-error'),1500); }
    });
  });
}

function titleOf(t) { return String(t.title || '').replace(new RegExp('^'+t.id+'\\s+','i'),'') || t.id; }
function summaryText(value) {
  const raw=String(value||'').trim();
  if(!raw)return '';
  const line=raw.split(/\r?\n/).map(x=>x.trim()).find(Boolean)||'';
  return line
    .replace(/^#{1,4}\s+/,'')
    .replace(/^[-*]\s+/,'')
    .replace(/^\d+\.\s+/,'')
    .replace(/^\[[ xX]\]\s+/,'')
    .replace(/\*\*([^*]+)\*\*/g,'$1')
    .replace(/\[([^\]]+)\]\([^)]+\)/g,'$1')
    .replace(/`([^`]+)`/g,'$1')
    .trim();
}
function humanSummary(task) {
  const doc=task.document||{}, raw=doc.summary||{}, req=doc.requirements||{}, reason=task.attention_reason||null;
  const result=summaryText(doc.result||task.fields?.결과);
  const notes=summaryText(doc.notes||task.fields?.메모);
  const purpose=summaryText(raw.purpose)||summaryText(req.goal)||summaryText(task.fields?.설명)||titleOf(task);
  const change=summaryText(raw.change)||(task.file_state==='done'?result:'');
  let statusDetail='';
  if(reason)statusDetail=summaryText(reason.message||reason.title);
  else statusDetail=summaryText(raw.status_result);
  if(!statusDetail){
    if(task.file_state==='done')statusDetail=result||t('summaryDoneFallback');
    else if(task.file_state==='hold')statusDetail=summaryText(task.fields?.대기)||t('summaryHoldFallback');
    else if(task.file_state==='doing')statusDetail=notes||t('summaryDoingFallback');
    else statusDetail=t('summaryTodoFallback');
  }
  let follow='';
  if(reason?.resume_condition)follow=summaryText(reason.resume_condition);
  if(!follow)follow=summaryText(raw.follow_up);
  if(!follow&&task.file_state==='hold')follow=summaryText(task.fields?.재개조건);
  const statusResult=`${stateLabel(task.state)} · ${statusDetail}`;
  return {purpose,change,status_result:statusResult,follow_up:follow,canonical:Boolean(doc.summary_present)};
}
function summaryPreview(value,max=150) {
  const text=summaryText(value);
  if(text.length<=max)return text;
  return text.slice(0,Math.max(0,max-1)).trimEnd()+'…';
}
function detailDisclosure(id,label,preview,body,open=false) {
  return `<details class="section detail-section detail-disclosure" id="${esc(id)}" data-toc-label="${esc(label)}" ${open?'open':''}><summary><div><h2>${esc(label)}</h2>${preview?`<p>${esc(preview)}</p>`:''}</div><span class="detail-disclosure-chevron">⌄</span></summary><div class="detail-disclosure-body">${body}</div></details>`;
}
function semanticSection(id,label,summary,body) {
  const value=summaryText(summary);
  if(value)return detailDisclosure(id,label,value,body);
  return `<section class="section detail-section detail-static" id="${esc(id)}" data-toc-label="${esc(label)}"><div class="detail-static-head"><h2>${esc(label)}</h2></div><div class="detail-static-body">${body}</div></section>`;
}
function relationBadges(ids,kind) {
  const rows=(ids||[]).filter(Boolean);
  if(!rows.length)return '<span class="summary">-</span>';
  return rows.map(id=>`<button type="button" class="relation-link badge" data-relation-id="${esc(id)}">${esc(id)} · ${esc(kind)}</button>`).join(' ');
}
function humanSummaryCard(task) {
  const s=humanSummary(task), reason=task.attention_reason||null;
  const rows=[[t('summaryPurpose'),s.purpose],[t('summaryChange'),s.change],[t('summaryStatusResult'),s.status_result],[t('summaryFollowUp'),s.follow_up]].filter(([,value])=>Boolean(value));
  const links=[];
  const kind=task.document?.contract_kind||'legacy';
  links.push(['related-work',t('relatedWork')],['operations',t('operationsEvidence')],['lifecycle',t('lifecycle')]);
  if(kind==='defined')links.push(['background',t('backgroundProblem')],['requirements',t('requirements')],['scope',t('workScope')],['acceptance',t('completionCriteria')]);
  else if(kind==='simple')links.push(['task-definition',t('taskDefinition')],['acceptance',t('completionCriteria')]);
  else links.push(['legacy-task',t('legacyDetails')]);
  links.push(['progress-result',t('progressResult')],['verification',t('verificationDetail')]);
  const urgent=reason?`<div class="summary-alert ${reason.severity==='danger'?'danger':''}"><strong>${esc(reason.title||t('needsAttention'))}</strong><span>${esc(reason.message||'')}</span>${reason.resume_condition?`<span>${esc(reason.resume_condition)}</span>`:''}</div>`:'';
  return `<section class="human-summary-card detail-section" id="human-summary" data-toc-label="${esc(t('humanSummary'))}"><div class="human-summary-head"><div><div class="eyebrow">${esc(t('humanSummary'))}</div>${!s.canonical?`<span class="summary-source">${esc(t('summaryFallback'))}</span>`:''}</div></div><div class="human-summary-grid">${rows.map(([label,value])=>`<div class="human-summary-row"><span>${esc(label)}</span><strong>${esc(value)}</strong></div>`).join('')}</div>${urgent}<div class="detail-jump-list"><span>${esc(t('detailLinks'))}</span>${links.map(([id,label])=>`<a href="#${esc(id)}" data-detail-target="${esc(id)}">${esc(label)}</a>`).join('')}</div></section>`;
}
function healthLabel(h) { return ({healthy:t('active'),quiet:t('quiet'),stale:t('stale'),worker_missing:t('workerMissing'),runtime_unknown:t('runtimeUnknown'),awaiting_finalize:t('awaitingFinalize'),needs_user:t('needsUser'),'n/a':'-'})[h] || h; }
function stateLabel(s) { return ({doing:t('working'),ready:t('ready'),blocked:t('blocked'),hold:t('hold'),done:t('done'),todo:t('todo'),needs_user:t('needsUser'),awaiting_finalize:t('awaitingFinalize'),stalled:t('stalled')})[s] || s; }
function lifecycleEventLabel(label) { return ({Registered:t('eventRegistered'),Started:t('eventStarted'),Hold:t('eventHold'),Completed:t('eventCompleted')})[label] || label; }
function completionAt(t) { return t.completion_sort_at || t.completed_at || t.mtime || ''; }
function updatedAt(t) { return t.updated_at || t.mtime || t.completed_at || ''; }

function runningSeconds(t, key) {
  const raw = t[key+'_seconds'];
  let base = raw == null ? null : +raw;
  const lc = t.lifecycle || {};
  const currentState = lc.current_state || t.file_state;
  const enteredAt = lc.current_state_at;
  const snapSegment = +(lc.current_segment_seconds || 0);
  const segmentKey = currentState === 'doing' ? 'active' : currentState === 'hold' ? 'wait' : null;
  if (enteredAt && segmentKey === key) {
    if (base == null) base = 0;
    const elapsedNow = Math.max(0, (Date.now() - new Date(enteredAt).getTime()) / 1000);
    base += Math.max(0, elapsedNow - snapSegment);
  }
  if (key === 'queue' && !lc.started_at && !lc.completed_at && lc.created_at) {
    base = Math.max(0, (Date.now() - new Date(lc.created_at).getTime()) / 1000);
  }
  if (key === 'lead' && !lc.completed_at && lc.created_at) {
    base = Math.max(0, (Date.now() - new Date(lc.created_at).getTime()) / 1000);
  }
  return base;
}

function renderAccess() {
  const a = currentProjectData()?.access || {}, el = $('#accessPill');
  if (!el) return;
  const st = a.status || 'unknown';
  const currentRestriction = Boolean(a.restriction_current);
  const historical = Boolean(a.cache_stale) && !currentRestriction;
  el.className = `access-pill ${currentRestriction ? 'restricted' : historical ? 'historical' : st}`;
  let label;
  if (currentRestriction) label = t('restrictedNow');
  else if (st === 'full') label = a.checked_at ? `${historical ? t('lastFullAccess') : t('fullAccess')} · ${ago(a.checked_at)}` : t('fullAccess');
  else if (st === 'restricted') label = a.checked_at ? `${t('lastRestricted')} · ${ago(a.checked_at)}` : t('lastRestricted');
  else label = a.checked_at ? `${t('accessLastChecked')} · ${ago(a.checked_at)}` : t('accessNotChecked');
  if (a.network === 'disabled') label += ` · ${t('networkOff')}`;
  el.textContent = label;
  el.title = `${currentRestriction ? t('enableFullAccess') : label}
${a.checked_at ? `${t('accessLastChecked')} ${ago(a.checked_at)}` : t('notChecked')} · ${t('freshDispatch')}`;
}
function accessBanner() {
  const a = currentProjectData()?.access || {};
  if (!a.restriction_current) return '';
  return `<div class="global-access danger"><div><strong>${esc(t('dispatchDisabled'))}</strong><span>${esc(t('enableFullAccess'))}</span></div><code>uv run _task_mecca/collab_tools.py preflight --require-full-access --json</code></div>`;
}
function diagnosticBanner() {
  const rows=currentProjectData()?.diagnostics||[];
  if(!rows.length)return '';
  return `<div class="global-access"><div><strong>Partial diagnostics</strong><span>${esc(rows.map(x=>`${x.component}: ${x.error}`).join(' · '))}</span></div></div>`;
}

function isIOSDevice() {
  return /iPad|iPhone|iPod/.test(navigator.userAgent) || (navigator.platform==='MacIntel' && navigator.maxTouchPoints>1);
}
function isStandaloneWebApp() {
  return window.matchMedia?.('(display-mode: standalone)').matches || navigator.standalone===true;
}
function notificationCapability() {
  if(!window.isSecureContext) return {mode:'insecure',canRequest:false};
  if(isIOSDevice()&&!isStandaloneWebApp()) return {mode:'ios-home',canRequest:false};
  if(typeof Notification==='undefined') return {mode:'unsupported',canRequest:false};
  return {mode:'supported',canRequest:true,permission:Notification.permission};
}
async function notificationWorker() {
  if(!('serviceWorker' in navigator)||!window.isSecureContext)return null;
  try{
    await navigator.serviceWorker.register('/sw.js',{scope:'/'});
    return await navigator.serviceWorker.ready;
  }catch(_){ return null; }
}

function saveNotificationSettings() {
  localStorage.setItem('task-mecca-notifications',JSON.stringify(state.notificationSettings));
}
function notificationSeenSet() {
  try { const raw=JSON.parse(localStorage.getItem('task-mecca-notification-seen')||'[]'); return new Set(Array.isArray(raw)?raw:[]); } catch(_) { return new Set(); }
}
function rememberNotification(key) {
  const seen=[...notificationSeenSet()];
  if(!seen.includes(key))seen.push(key);
  localStorage.setItem('task-mecca-notification-seen',JSON.stringify(seen.slice(-250)));
}
async function sendBrowserNotification(kind,task,reason,key) {
  if(notificationSeenSet().has(key))return;
  if(!state.notificationSettings[kind]){
    rememberNotification(key);
    return;
  }
  const capability=notificationCapability();
  if(capability.mode!=='supported'||Notification.permission!=='granted')return;
  const projectName=(state.project||'').split(/[\\/]/).pop()||'Task Mecca';
  const title=kind==='completed'
    ? `${projectName} · ${task.id} 완료`
    : `${projectName} · ${task.id} · ${reason?.title||t(kind==='stalled'?'notifyStalled':'notifyIntervention')}`;
  const body=kind==='completed'
    ? (titleOf(task)||task.id)
    : [titleOf(task),reason?.message,reason?.resume_condition].filter(Boolean).join(' · ');
  const tag=`task-mecca:${state.project}:${task.id}:${kind}`;
  const target=`/tasks/${encodeURIComponent(task.id)}?project=${encodeURIComponent(state.project||'')}`;
  try {
    const registration=await notificationWorker();
    if(registration){
      await registration.showNotification(title,{body,tag,data:{url:target}});
      rememberNotification(key);
      return;
    }
    const n=new Notification(title,{body,tag});
    n.onclick=()=>{ window.focus(); openTask(task.id); n.close(); };
    rememberNotification(key);
  } catch(_) {}
}
function processTaskNotifications(snapshot) {
  if(!state.project||!snapshot)return;
  const current=snapshot.all_items||{};
  const previous=state.previousTasksByProject[state.project]||null;
  const serverEvents=Array.isArray(snapshot.notification_events)?snapshot.notification_events:[];
  const completedByServer=new Set();

  serverEvents.forEach(event=>{
    if(event?.kind!=='completed'||!event.task_id||!event.id)return;
    completedByServer.add(event.task_id);
    const task=current[event.task_id]||(snapshot.done_items||[]).find(x=>x.id===event.task_id);
    if(!task)return;
    const key=`server:${state.project}:${event.id}`;
    const eventAt=Date.parse(event.at||'');
    if(Number.isFinite(eventAt) && Date.now()-eventAt>24*60*60*1000){
      rememberNotification(key);
      return;
    }
    sendBrowserNotification('completed',task,null,key);
  });

  Object.values(current).forEach(task=>{
    const reason=task.attention_reason||null;
    if(reason){
      const kind=reason.type==='runtime_stalled'?'stalled':'intervention';
      const key=`${state.project}:${task.id}:${kind}:${reason.type||''}:${task.updated_at||task.mtime||''}`;
      sendBrowserNotification(kind,task,reason,key);
    }
    if(previous&&!completedByServer.has(task.id)){
      const before=previous[task.id];
      if(task.file_state==='done' && before && before.file_state!=='done'){
        const key=`${state.project}:${task.id}:completed:${task.completed_at||task.mtime||task.updated_at||''}`;
        sendBrowserNotification('completed',task,null,key);
      }
    }
  });
  const compact={...(previous||{})};
  Object.values(current).forEach(task=>compact[task.id]={file_state:task.file_state,state:task.state,updated_at:task.updated_at});
  state.previousTasksByProject[state.project]=compact;
  localStorage.setItem('task-mecca-previous-tasks',JSON.stringify(state.previousTasksByProject));
}
function updateNotificationIndicator() {
  const btn=$('#notificationBtn'),badge=$('#notificationBadge');
  if(!btn||!badge)return;
  const capability=notificationCapability();
  const permission=capability.mode==='supported'?Notification.permission:capability.mode;
  const enabled=Object.values(state.notificationSettings).some(Boolean);
  btn.classList.remove('notification-ok','notification-warning','notification-off');
  badge.textContent='';
  if(permission==='granted'&&enabled){
    btn.classList.add('notification-ok');
    btn.title=t('notificationsOn');
    btn.setAttribute('aria-label',t('notificationsOn'));
  } else if(permission==='default'||permission==='denied'||permission==='unsupported'||permission==='insecure'||permission==='ios-home'){
    btn.classList.add('notification-warning');
    badge.textContent='!';
    btn.title=permission==='denied'?t('notificationsBlocked')
      :permission==='insecure'?t('notificationsInsecureTitle')
      :permission==='ios-home'?t('notificationsIOSHomeTitle')
      :permission==='unsupported'?t('notificationsUnsupportedTitle')
      :t('notificationsPermissionNeeded');
    btn.setAttribute('aria-label',btn.title);
  } else {
    btn.classList.add('notification-off');
    badge.textContent='×';
    btn.title=t('notificationTypesDisabled');
    btn.setAttribute('aria-label',btn.title);
  }
}
function renderNotificationPanel() {
  const panel=$('#notificationPanel');
  if(!panel)return;
  const capability=notificationCapability();
  const permission=capability.mode==='supported'?Notification.permission:capability.mode;
  const enabled=Object.values(state.notificationSettings).some(Boolean);
  let guide='';
  if(permission==='granted'){
    guide=enabled
      ? `<div class="notification-state ok"><strong>${esc(t('notificationsOn'))}</strong><span>${esc(t('notificationSettings'))}</span></div>`
      : `<div class="notification-state off"><strong>${esc(t('notificationsOff'))}</strong><span>${esc(t('notificationTypesDisabled'))}</span></div>`;
  } else if(permission==='denied'){
    guide=`<div class="notification-state warning"><strong>${esc(t('notificationsBlocked'))}</strong><span>${esc(t('notificationsDeniedGuide'))}</span></div>`;
  } else if(permission==='insecure'){
    guide=`<div class="notification-state warning"><strong>${esc(t('notificationsInsecureTitle'))}</strong><span>${esc(t('notificationsInsecureGuide'))}</span></div>`;
  } else if(permission==='ios-home'){
    guide=`<div class="notification-state warning"><strong>${esc(t('notificationsIOSHomeTitle'))}</strong><span>${esc(t('notificationsIOSHomeGuide'))}</span></div>`;
  } else if(permission==='unsupported'){
    guide=`<div class="notification-state warning"><strong>${esc(t('notificationsUnsupportedTitle'))}</strong><span>${esc(t('notificationsUnsupportedGuide'))}</span></div>`;
  } else {
    guide=`<div class="notification-state warning"><strong>${esc(t('notificationsPermissionNeeded'))}</strong><span>${esc(t('allowBrowserNotifications'))}</span></div>`;
  }
  panel.innerHTML=`<div class="notification-panel-head"><strong>${esc(t('notificationSettings'))}</strong><button type="button" id="notificationClose">×</button></div>
    ${guide}
    <label><input type="checkbox" data-notification-setting="intervention" ${state.notificationSettings.intervention?'checked':''}> <span>${esc(t('notifyIntervention'))}</span></label>
    <label><input type="checkbox" data-notification-setting="completed" ${state.notificationSettings.completed?'checked':''}> <span>${esc(t('notifyCompleted'))}</span></label>
    <label><input type="checkbox" data-notification-setting="stalled" ${state.notificationSettings.stalled?'checked':''}> <span>${esc(t('notifyStalled'))}</span></label>
    ${permission==='default'&&capability.canRequest?`<button type="button" class="action-btn notification-permission" id="notificationPermission">${esc(t('allowBrowserNotifications'))}</button>`:''}`;
  panel.querySelectorAll('[data-notification-setting]').forEach(input=>input.addEventListener('change',()=>{
    state.notificationSettings[input.dataset.notificationSetting]=input.checked;
    saveNotificationSettings();
    updateNotificationIndicator();
    renderNotificationPanel();
  }));
  $('#notificationClose')?.addEventListener('click',()=>panel.classList.remove('open'));
  $('#notificationPermission')?.addEventListener('click',async()=>{
    try {
      await Notification.requestPermission();
      if(Notification.permission==='granted')await notificationWorker();
    } catch(_) {
      alert(t('notificationPermissionError'));
    }
    updateNotificationIndicator();
    renderNotificationPanel();
  });
  updateNotificationIndicator();
}
function saveOpenProjects() {
  localStorage.setItem('task-mecca-open-projects',JSON.stringify(state.openProjects));
}
function ensureOpenProject(path) {
  if(!path)return;
  if(!state.openProjects.includes(path)){ state.openProjects.push(path); saveOpenProjects(); }
}
function switchProject(path) {
  if(!path)return;
  ensureOpenProject(path);
  state.project=path;
  state.lastProject=path;
  localStorage.setItem('task-mecca-last-project',path);
  state.backlog='';
  state.snapshot=null;
  state.listData=null;
  state.detailTask=null;
  state.contentRevision='';
  state.pendingContentUpdate=false;
  state.pendingContentReason='';
  state.loadError='';
  state.view='backlog';
  state.detail=null;
  state.listPage=1;
  state.selectedIndex=0;
  closeAttentionStream();
  localStorage.removeItem('task-mecca-backlog-folder');
  history.pushState({},'',`/?project=${encodeURIComponent(path)}&view=backlog`);
  render();
  refreshList();
  ensureAttentionStream();
  refreshVersionInfo(false);
}
function closeProjectSession(path) {
  state.openProjects=state.openProjects.filter(p=>p!==path);
  saveOpenProjects();
  if(state.project===path){
    const next=state.openProjects[0]||'';
    if(next){ switchProject(next); return; }
    state.project=''; state.snapshot=null; state.listData=null; state.detailTask=null; closeAttentionStream(); state.view='hub'; state.detail=null;
    history.pushState({},'','/?view=hub'); render(); refresh();
  } else render();
}
function renderBacklogPicker() {
  const picker = $('#backlogPicker');
  const data=currentProjectData();
  if (!picker || !data) return;
  const selection = data.backlog_selection || {};
  const candidates = selection.candidates || [];
  const selected = selection.selected || '';
  const selectedCandidate = candidates.find(c => c.path === selected);
  const autoLabel = selectedCandidate ? `${t('auto')} · ${selectedCandidate.name}` : t('auto');
  picker.innerHTML = `<option value="" ${state.backlog?'':'selected'}>${esc(autoLabel)}</option>` + candidates.map(c => {
    const stamp = c.latest_modified ? dateLabel(c.latest_modified) : '-';
    return `<option value="${esc(c.path)}" ${state.backlog===c.path?'selected':''}>${esc(c.name)} · ${c.record_count||0} · ${esc(stamp)}</option>`;
  }).join('');
  picker.disabled = candidates.length <= 1;
  picker.title = state.backlog ? `${t('manualMode')}: ${state.backlog}` : `${t('auto')}: ${selected || t('noBacklog')}`;
}

function backlogUrl() {
  const p=new URLSearchParams();
  if(!state.statusFilters.includes('all'))p.set('filter',state.statusFilters.join(','));
  if(state.tagFilters.length)p.set('tags',state.tagFilters.join(','));
  if(state.project)p.set('project',state.project);
  const qs=p.toString();
  return '/'+(qs?'?'+qs:'');
}
function setTagFilter(tag) {
  if(!tag)return;
  const next=state.tagFilters.includes(tag)?state.tagFilters.filter(x=>x!==tag):[...state.tagFilters,tag];
  state.tagFilters=next;
  state.view='backlog'; state.detail=null; state.selectedIndex=0; state.listPage=1;
  history.pushState({},'',backlogUrl());
  refreshList();
}
function clearTagFilters() {
  state.tagFilters=[]; state.listPage=1; state.selectedIndex=0;
  history.pushState({},'',backlogUrl()); refreshList();
}
function tagChip(tag,clickable=true) {
  return `<button type="button" class="tag-chip ${state.tagFilters.includes(tag)?'active':''}" ${clickable?`data-tag-filter="${esc(tag)}"`:''} title="${esc(tag)}">${esc(tag)}</button>`;
}
function setStatusFilter(key) {
  const allowed = ['all','ready','doing','hold','blocked','done'];
  if (!allowed.includes(key)) return;
  if (key === 'all') {
    state.statusFilters = ['all'];
  } else {
    let next = state.statusFilters.filter(x => x !== 'all');
    if (next.includes(key)) next = next.filter(x => x !== key);
    else next.push(key);
    state.statusFilters = next.length ? next : ['all'];
  }
  state.view = 'backlog';
  state.detail = null;
  state.selectedIndex = 0;
  state.listPage = 1;
  history.pushState({},'',backlogUrl());
  refreshList();
}
function resolveProjectContext() {
  if(state.project)return state.project;
  if(state.lastProject)return state.lastProject;
  if(state.openProjects.length)return state.openProjects[0];
  const projects=state.hub?.projects||[];
  return projects[0]?.path||'';
}
function navigateView(view) {
  if(view==='release-notes'){
    state.project='';
    state.snapshot=null;
    state.listData=null;
    state.detailTask=null;
    closeAttentionStream();
    state.loadError='';
    state.view='release-notes';
    state.detail=null;
    state.projectMenuOpen=false;
    history.pushState({},'','/?view=release-notes');
    render();
    loadReleaseNotes(!state.releaseNotesLoaded);
    return;
  }
  if(view==='hub'){
    state.project='';
    state.snapshot=null;
    state.listData=null;
    state.detailTask=null;
    state.contentRevision='';
    state.contentStateSnapshot=[];
    state.pendingContentUpdate=false;
    state.pendingContentReason='';
    state.pendingContentChanges=[];
    closeAttentionStream();
    state.loadError='';
    state.view='hub';
    state.detail=null;
    state.projectMenuOpen=false;
    history.pushState({},'','/?view=hub');
    render();
    refresh();
    return;
  }
  const needsProject=['backlog','workload','attention','issues','manual'].includes(view);
  if(needsProject && !state.project){
    const project=resolveProjectContext();
    if(project){
      state.project=project;
      state.lastProject=project;
      localStorage.setItem('task-mecca-last-project',project);
      ensureOpenProject(project);
    }
  }
  state.view=view;
  state.detail=null;
  state.selectedIndex=0;
  state.listPage=1;
  const params=new URLSearchParams();
  params.set('view',view);
  if(state.project)params.set('project',state.project);
  history.pushState({},'',`/?${params.toString()}`);
  state.detailTask=null;
  if(state.project)refreshVersionInfo(false);
  if(view==='backlog'){
    render();
    refreshList();
    ensureAttentionStream();
  } else if(needsProject){
    render();
    refresh();
  } else {
    render();
  }
}
function nav() {
  const c = currentProjectData()?.counts || {};
  const projects=state.hub?.projects||[];
  const byPath=new Map(projects.map(p=>[p.path,p]));
  if(state.project)ensureOpenProject(state.project);
  const sessions=state.openProjects.filter(path=>byPath.has(path)||path===state.project);
  state.openProjects=sessions;
  saveOpenProjects();
  const closed=projects.filter(p=>!sessions.includes(p.path));
  const sessionRows=sessions.map(path=>{
    const p=byPath.get(path)||{name:path.split(/[\\/]/).pop()||path,counts:{}};
    const pc=p.counts||{};
    const active=state.project===path && state.view!=='hub';
    const badge=(pc.working||0)+(pc.ready||0);
    return `<div class="session-row ${active?'active':''}" data-session-project="${esc(path)}"><button class="session-open" type="button" title="${esc(path)}"><span class="session-dot ${pc.working?'busy':''}"></span><span class="session-name">${esc(p.name)}</span><span class="session-count">${badge||''}</span></button><button class="session-close" type="button" data-close-project="${esc(path)}" aria-label="Close ${esc(p.name)}" title="Close">×</button></div>`;
  }).join('');
  const menu=state.projectMenuOpen?`<div class="project-open-menu">${closed.length?closed.map(p=>`<button type="button" data-add-project="${esc(p.path)}"><span>${esc(p.name)}</span><small>${esc(p.path)}</small></button>`).join(''):'<div class="project-open-empty">No closed projects</div>'}</div>`:'';
  $('#stateNav').innerHTML =
    `<div class="sidebar-label">SYSTEM</div><button class="nav-item ${state.view==='hub'?'active':''}" id="hubNavBtn" type="button"><span class="nav-main"><span class="nav-icon">⌂</span><span class="nav-text">Global Hub</span></span></button><div class="sidebar-label">BACKLOGS</div><div class="session-list">${sessionRows||'<div class="session-empty">No open backlogs</div>'}</div><button class="nav-item session-add" id="openProjectBtn" type="button"><span class="nav-main"><span class="nav-icon">＋</span><span class="nav-text">Open Project</span></span></button>${menu}`;
  $('#workloadCount').textContent = (state.snapshot?.workload?.agents || []).length || '';
  $('#attentionCount').textContent = c.attention || '';
  $('#issueCount').textContent = c.issues || '';
  document.querySelectorAll('[data-session-project]').forEach(row=>row.querySelector('.session-open')?.addEventListener('click',()=>switchProject(row.dataset.sessionProject)));
  document.querySelectorAll('[data-close-project]').forEach(btn=>btn.addEventListener('click',e=>{e.stopPropagation();closeProjectSession(btn.dataset.closeProject)}));
  document.querySelectorAll('[data-add-project]').forEach(btn=>btn.addEventListener('click',()=>{state.projectMenuOpen=false;switchProject(btn.dataset.addProject)}));
  $('#openProjectBtn')?.addEventListener('click',()=>{state.projectMenuOpen=!state.projectMenuOpen;render()});
  $('#hubNavBtn')?.addEventListener('click',()=>navigateView('hub'));
  document.querySelectorAll('[data-view]').forEach(b => {
    b.classList.toggle('active', state.view === b.dataset.view);
    b.onclick = () => navigateView(b.dataset.view);
  });
  renderReleaseNotesBadge();
}

function releaseNotesView() {
  const head='<div class="page-head"><div><div class="eyebrow">TASK MECCA</div><h1>'+esc(t('releaseNotes'))+'</h1><p class="summary">'+esc(t('releaseNotesIntro'))+'</p></div></div>';
  if(state.releaseNotesLoading&&!state.releaseNotes.length)return head+'<div class="loading">'+esc(t('releaseNotesLoading'))+'</div>';
  if(state.releaseNotesLoaded&&!state.releaseNotes.length)return head+'<div class="empty">'+esc(t('releaseNoEntries'))+'</div>';

  let previousYear='';
  const rows=state.releaseNotes.map((item,index)=>{
    const year=String(item.date||'').slice(0,4)||'—';
    const yearHead=year!==previousYear?'<h2 class="release-year">'+esc(year)+'</h2>':'';
    previousYear=year;
    const expanded=state.releaseNoteExpanded===item.version;
    const detail=expanded?state.releaseNoteDetails[item.version]:null;
    const latest=index===0?'<span class="badge">'+esc(t('releaseNotesLatest'))+'</span>':'';
    const unread=releaseNoteUnread(item.version)?'<span class="badge warn">'+esc(t('releaseNotesNew'))+'</span>':'';
    const migration=item.migration?.required?'<span class="release-flag important">'+esc(t('releaseMigrationRequired'))+'</span>':'';
    const detailHTML=expanded?'<div class="release-history-detail">'+(detail?releaseNoteDetailMarkup(detail):'<div class="loading">'+esc(t('releaseNotesLoading'))+'</div>')+(releaseNoteUnread(item.version)&&detail?'<div class="release-read-row"><span>'+esc(t('releaseUnreadNotice'))+'</span><button type="button" class="action-btn secondary" data-release-confirm="'+esc(item.version)+'">'+esc(t('releaseMarkRead'))+'</button></div>':'')+'</div>':'';
    return yearHead+'<article class="release-history-item '+(expanded?'expanded':'')+'">'+
      '<button type="button" class="release-history-toggle" data-release-version="'+esc(item.version)+'" aria-expanded="'+(expanded?'true':'false')+'">'+
        '<span class="release-history-version"><strong>v'+esc(item.version)+'</strong><small>'+esc(item.date||'')+'</small></span>'+
        '<span class="release-history-summary">'+esc(releaseLocalized(item.summary))+'</span>'+
        '<span class="release-history-badges">'+latest+unread+migration+'</span>'+
        '<span class="release-history-chevron">'+(expanded?'−':'+')+'</span>'+
      '</button>'+detailHTML+'</article>';
  }).join('');
  const more=state.releaseNotesHasMore?'<div class="release-more"><button type="button" class="action-btn secondary" id="releaseNotesMore" '+(state.releaseNotesLoading?'disabled':'')+'>'+esc(t('releaseNotesMore'))+'</button></div>':'';
  return head+'<div class="release-history">'+rows+'</div>'+more;
}
function bindReleaseNotesActions() {
  document.querySelectorAll('[data-release-version]').forEach(btn=>btn.addEventListener('click',async()=>{
    const version=btn.dataset.releaseVersion;
    if(state.releaseNoteExpanded===version){
      state.releaseNoteExpanded='';
      render();
      return;
    }
    state.releaseNoteExpanded=version;
    render();
    await loadReleaseNoteDetail(version);
    if(state.view==='release-notes'&&state.releaseNoteExpanded===version)render();
  }));
  document.querySelectorAll('[data-release-confirm]').forEach(btn=>btn.addEventListener('click',e=>{
    e.stopPropagation();
    markReleaseNoteSeen(btn.dataset.releaseConfirm);
  }));
  $('#releaseNotesMore')?.addEventListener('click',()=>loadReleaseNotes(false));
}

const channelGesture={phase:'first',taps:[],firstBatchAt:0,secondStartedAt:0};
let frameworkSyncDismissedFor='';
function resetChannelGesture(seedTime=0) {
  channelGesture.phase='first';
  channelGesture.taps=seedTime?[seedTime]:[];
  channelGesture.firstBatchAt=0;
  channelGesture.secondStartedAt=0;
}
function recordChannelGestureTap() {
  const now=performance.now();
  if(channelGesture.phase==='quiet'){
    const waited=now-channelGesture.firstBatchAt;
    if(waited<3000)return;
    if(waited>5000){resetChannelGesture(now);return}
    channelGesture.phase='second';
    channelGesture.taps=[now];
    channelGesture.secondStartedAt=now;
    return;
  }
  if(channelGesture.phase==='second'){
    if(now-channelGesture.secondStartedAt>3000){resetChannelGesture(now);return}
    channelGesture.taps.push(now);
    if(channelGesture.taps.length>=5){
      resetChannelGesture();
      showChannelSwitchModal();
    }
    return;
  }
  channelGesture.taps=channelGesture.taps.filter(at=>now-at<=3000);
  channelGesture.taps.push(now);
  if(channelGesture.taps.length>=5){
    channelGesture.phase='quiet';
    channelGesture.taps=[];
    channelGesture.firstBatchAt=now;
  }
}
function channelLabel(channel) {
  return channel==='dev'?t('channelDev'):t('channelStable');
}
function savePendingFrameworkSync(body) {
  const rows=Array.isArray(body?.framework_sync)?body.framework_sync:[];
  if(!rows.length){
    localStorage.removeItem('task-mecca-pending-framework-sync-v1');
    return;
  }
  localStorage.setItem('task-mecca-pending-framework-sync-v1',JSON.stringify({
    target_channel:body.channel||'',
    target_version:body.to||'',
    projects:rows
  }));
}
function readPendingFrameworkSync() {
  try{
    const value=JSON.parse(localStorage.getItem('task-mecca-pending-framework-sync-v1')||'null');
    return value&&Array.isArray(value.projects)?value:null;
  }catch(_){ return null; }
}
async function showChannelSwitchModal() {
  document.querySelector('.channel-switch-overlay')?.remove();
  let options=null;
  try{
    const r=await fetch('/api/channel-options',{cache:'no-store'});
    const body=await r.json();
    if(!r.ok)throw new Error(body.error||'Channel lookup failed');
    options=body;
  }catch(e){
    alert(String(e?.message||e));
    return;
  }
  const current=options.current_channel||'stable';
  const target=current==='dev'?'stable':'dev';
  const targetInfo=options[target]||{};
  const targetVersion=targetInfo.version||'';
  const syncRows=Array.isArray(targetInfo.framework_sync)?targetInfo.framework_sync:[];
  const unavailable=!targetVersion||targetInfo.error||options.environment_override;
  const overlay=document.createElement('div');
  overlay.className='channel-switch-overlay';
  const notice=target==='dev'?t('channelDevWarning'):t('channelStableNotice');
  const errorText=options.environment_override?'TASK_MECCA_CHANNEL environment override':(targetInfo.error||t('channelSwitchUnavailable'));
  const syncPreview=syncRows.length?'<div class="channel-sync-preview">'+esc(t('frameworkSyncPreview',{n:syncRows.length,channel:channelLabel(target)}))+
    '<div class="channel-sync-projects">'+syncRows.map(row=>'<span>'+esc(row.name||row.path||'-')+' · '+esc(row.from_version||'-')+' → '+esc(row.to_version||targetVersion)+'</span>').join('')+'</div></div>':'';
  overlay.innerHTML='<div class="channel-switch-modal" role="dialog" aria-modal="true" aria-labelledby="channelSwitchTitle">'+
    '<div class="eyebrow">TASK MECCA</div><h2 id="channelSwitchTitle">'+esc(t('channelSwitchTitle'))+'</h2>'+
    '<div class="channel-switch-grid"><div><span>'+esc(t('channelCurrent'))+'</span><strong>'+esc(channelLabel(current))+' · '+esc(options.current_version||'-')+'</strong></div>'+
    '<div class="channel-switch-arrow">→</div><div><span>'+esc(t('channelTarget'))+'</span><strong>'+esc(channelLabel(target))+' · '+esc(targetVersion||'-')+'</strong></div></div>'+
    '<p class="channel-switch-note">'+esc(unavailable?errorText:notice)+'</p>'+syncPreview+
    '<div class="channel-switch-actions"><button type="button" class="action-btn secondary" data-channel-cancel>'+esc(t('channelSwitchCancel'))+'</button>'+
    '<button type="button" class="action-btn" data-channel-confirm '+(unavailable?'disabled':'')+'>'+esc(t('channelSwitchAction',{channel:channelLabel(target)}))+'</button></div></div>';
  document.body.appendChild(overlay);
  const close=()=>overlay.remove();
  overlay.querySelector('[data-channel-cancel]')?.addEventListener('click',close);
  overlay.addEventListener('click',e=>{if(e.target===overlay)close();});
  overlay.querySelector('[data-channel-confirm]')?.addEventListener('click',async e=>{
    const button=e.currentTarget;
    button.disabled=true;
    button.textContent=t('channelSwitching');
    try{
      const r=await fetch('/api/channel-switch',{method:'POST',headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},body:JSON.stringify({channel:target})});
      const body=await r.json();
      if(!r.ok)throw new Error(body.error||'Channel switch failed');
      savePendingFrameworkSync(body);
      if(body.restart_required&&body.to){
        overlay.innerHTML='<div class="channel-switch-modal channel-switch-progress"><div class="upgrade-spinner"></div><h2>'+esc(channelLabel(target))+' · '+esc(body.to)+'</h2><p>'+esc(t('channelSwitchRestart'))+'</p></div>';
        await waitForRestartedWeb(body.to);
        return;
      }
      close();
      await refreshVersionInfo(true);
      await refreshHub(true);
      render();
      maybeShowPendingFrameworkSync();
    }catch(err){
      alert(String(err?.message||err));
      button.disabled=false;
      button.textContent=t('channelSwitchAction',{channel:channelLabel(target)});
    }
  });
}
function bindChannelGesture() {
  const mark=$('#hubVersionMark');
  if(mark)mark.addEventListener('click',recordChannelGestureTap);
}
async function maybeShowPendingFrameworkSync() {
  const pending=readPendingFrameworkSync();
  if(!pending||!pending.projects.length)return;
  if(document.querySelector('.framework-sync-overlay'))return;
  const cli=state.versionInfo?.cli||state.hub?.cli||{};
  if(normalizedVersion(cli.current)!==normalizedVersion(pending.target_version))return;
  if((cli.channel||'stable')!==(pending.target_channel||'stable'))return;
  if(frameworkSyncDismissedFor===pending.target_version)return;

  let projects=pending.projects;
  try{
    const hub=await refreshHub(true);
    const currentByPath=new Map((hub?.projects||[]).map(row=>[row.path,row.framework_version]));
    projects=projects.filter(row=>normalizedVersion(currentByPath.get(row.path))!==normalizedVersion(pending.target_version));
  }catch(_){}
  if(!projects.length){
    localStorage.removeItem('task-mecca-pending-framework-sync-v1');
    return;
  }

  const targetChannel=pending.target_channel||'stable';
  const overlay=document.createElement('div');
  overlay.className='framework-sync-overlay';
  const intro=targetChannel==='stable'?t('frameworkSyncStableIntro'):t('frameworkSyncIntro');
  overlay.innerHTML='<div class="framework-sync-modal" role="dialog" aria-modal="true" aria-labelledby="frameworkSyncTitle">'+
    '<div class="eyebrow">FRAMEWORK SYNC</div><h2 id="frameworkSyncTitle">'+esc(t('frameworkSyncTitle',{channel:channelLabel(targetChannel)}))+'</h2>'+
    '<p class="channel-switch-note">'+esc(intro)+'</p>'+
    '<div class="framework-sync-list">'+projects.map(row=>'<div><strong>'+esc(row.name||String(row.path||'').split(/[\\/]/).pop()||'-')+'</strong><span>'+esc(row.from_version||'-')+' → '+esc(pending.target_version||row.to_version||'-')+'</span></div>').join('')+'</div>'+
    '<div class="channel-switch-actions"><button type="button" class="action-btn secondary" data-framework-later>'+esc(t('frameworkSyncLater'))+'</button>'+
    '<button type="button" class="action-btn" data-framework-sync>'+esc(t('frameworkSyncAction',{channel:channelLabel(targetChannel)}))+'</button></div></div>';
  document.body.appendChild(overlay);
  overlay.querySelector('[data-framework-later]')?.addEventListener('click',()=>{
    frameworkSyncDismissedFor=pending.target_version;
    overlay.remove();
  });
  overlay.addEventListener('click',e=>{
    if(e.target===overlay){
      frameworkSyncDismissedFor=pending.target_version;
      overlay.remove();
    }
  });
  overlay.querySelector('[data-framework-sync]')?.addEventListener('click',async e=>{
    const button=e.currentTarget;
    button.disabled=true;
    button.textContent=t('frameworkSyncing');
    let instructionResult=null;
    for(const row of projects){
      try{
        const r=await fetch('/api/migrate',{method:'POST',headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},body:JSON.stringify({project:row.path})});
        const body=await r.json();
        if(!r.ok)throw new Error(body.error||'Migration failed');
        if(body.instruction_refresh_required||body.legacy_bootstrap)instructionResult=body;
      }catch(err){
        alert(t('frameworkSyncFailed')+'\n'+(row.name||row.path||'')+'\n'+String(err?.message||err));
        button.disabled=false;
        button.textContent=t('frameworkSyncAction',{channel:channelLabel(targetChannel)});
        return;
      }
    }
    localStorage.removeItem('task-mecca-pending-framework-sync-v1');
    frameworkSyncDismissedFor='';
    overlay.remove();
    await refreshHub(true);
    await refreshVersionInfo(false);
    if(state.view==='hub')render();
    if(instructionResult)showMigrationResyncModal(instructionResult);
  });
}

function hubView() {
  const h=state.hub||{};
  const cli=h.cli||{};
  const projects=Array.isArray(h.projects)?h.projects:[];
  const channelBadge=cli.channel==='dev' ? '<span class="badge warn">DEV</span>' : '';
  const currentVersionMark='<button type="button" class="badge hub-version-mark" id="hubVersionMark">'+esc(cli.current||'-')+'</button>';
  const cliStatus=cli.update_available
    ? currentVersionMark+'<span class="badge warn">→ '+esc(cli.latest||'-')+'</span>'
    : currentVersionMark;
  const cards=projects.map(p=>{
    const c=p?.counts||{};
    const name=p?.name||String(p?.path||'Project').split(/[\\/]/).pop()||'Project';
    const framework=p?.framework_version||'unknown';
    return `<article class="project-card">
      <div class="project-card-head"><div><div class="eyebrow">${esc(p?.path||'')}</div><h2>${esc(name)}</h2></div><span class="badge">${esc(framework)}</span></div>
      <div class="project-stats"><span><strong>${c.working||0}</strong> working</span><span><strong>${c.ready||0}</strong> ready</span><span><strong>${c.hold||0}</strong> hold</span></div>
      <div class="project-actions">${p?.migration_available?`<button class="action-btn secondary" data-migrate="${esc(p.path)}">Migrate</button>`:''}<button class="action-btn" data-open-project="${esc(p?.path||'')}">Open</button></div>
    </article>`;
  }).join('');
  const updateActions=cli.update_available
    ? (cli.channel!=='dev'?'<button class="action-btn secondary" id="hubUpdateChangesBtn">'+esc(t('updateChanges'))+'</button>':'')+'<button class="action-btn" id="upgradeBtn">'+esc(t('updateNow'))+'</button>'
    : '';
  return `<div class="page-head"><div><div class="eyebrow">TASK MECCA</div><h1>Global Hub</h1><p class="summary">CLI와 등록 프로젝트의 framework 상태를 관리합니다.</p></div><div class="hub-cli"><strong>CLI</strong> ${channelBadge} ${cliStatus} ${updateActions}</div></div>
    ${cli.update_available?'<div class="timing-note"><strong>Upgrade</strong><span>업그레이드가 완료되면 Task Mecca Web이 자동으로 재시작되며, 현재 브라우저 페이지도 자동으로 새로고침됩니다.</span></div>':''}
    ${cli.error?`<div class="timing-note"><strong>Version check</strong><span>${esc(cli.error)}</span></div>`:''}
    <div class="project-grid">${cards||'<div class="empty">등록된 Task Mecca 프로젝트가 없습니다.</div>'}</div>`;
}
function normalizedVersion(value) {
  return String(value??'').trim().replace(/(?:\\r|\\n)+$/g,'').trim();
}
async function waitForRestartedWeb(targetVersion) {
  targetVersion=normalizedVersion(targetVersion);
  const started=Date.now();
  while(Date.now()-started<30000){
    try {
      const r=await fetch(`/api/health?restart_wait=${Date.now()}`,{cache:'no-store'});
      if(r.ok){
        const body=await r.json();
        if(!targetVersion||normalizedVersion(body.version)===targetVersion){ location.reload(); return; }
      }
    } catch(_) {}
    await new Promise(resolve=>setTimeout(resolve,500));
  }
  const c=$('#content');
  if(c)c.innerHTML=`<div class="load-error"><h2>Task Mecca Web 재시작을 확인하지 못했습니다</h2><p>CLI 업데이트 자체는 완료됐을 수 있습니다. 터미널에서 <code>task-mecca web status</code>로 상태를 확인하고, 이전 버전이 계속 실행 중이면 <code>task-mecca web restart</code>를 실행한 뒤 이 페이지를 새로고침하세요.</p></div>`;
}
function migrationResyncPrompt(result) {
  const changed=(result.changed_instructions||[]).join(', ') || '-';
  if(state.language==='ko'){
    return 'Task Mecca 프레임워크가 '+(result.from_version||'?')+' → '+(result.to_version||'?')+'로 마이그레이션되었습니다.\n\n'+
      '현재 Root 세션의 Task Mecca 운영 지침을 최신 상태로 재동기화해주세요.\n\n'+
      '새 작업이나 subagent dispatch를 시작하기 전에 다음 파일을 다시 읽고 현재 세션에 적용하세요:\n'+
      '- _task_mecca/ROOT_PROMPT.md\n'+
      '- _task_mecca/framework/SESSION_GUIDE.md\n'+
      '- _task_mecca/framework/collab.md\n'+
      '- _task_mecca/framework/roles/root.md\n\n'+
      '이번 마이그레이션에서 변경된 운영 지침:\n'+changed+'\n\n'+
      '현재 세션은 그대로 유지하되, 이전에 읽은 지침과 새 지침이 충돌하면 최신 파일의 지침을 우선하세요.\n'+
      '재동기화가 완료되면 현재 작업 방식에 영향을 주는 변경사항만 간단히 요약하고, 그 다음부터 최신 Task Mecca 규칙으로 계속 진행해주세요.';
  }
  return 'Task Mecca framework was migrated from '+(result.from_version||'?')+' to '+(result.to_version||'?')+'.\n\n'+
    'Resynchronize the current Root session with the latest Task Mecca operating instructions.\n\n'+
    'Before starting any new work or subagent dispatch, reread and apply:\n'+
    '- _task_mecca/ROOT_PROMPT.md\n'+
    '- _task_mecca/framework/SESSION_GUIDE.en.md\n'+
    '- _task_mecca/framework/collab.md\n'+
    '- _task_mecca/framework/roles/root.md\n\n'+
    'Operational instruction files changed by this migration:\n'+changed+'\n\n'+
    'Keep the current session. If previously-read instructions conflict with the updated files, follow the latest files.\n'+
    'After resynchronization, briefly summarize only the changes that affect the current workflow, then continue using the latest Task Mecca rules.';
}
function showMigrationResyncModal(result) {
  document.querySelector('.migration-resync-overlay')?.remove();
  const ko=state.language==='ko';
  const prompt=migrationResyncPrompt(result);
  const changed=(result.changed_instructions||[]).map(x=>'<code>'+esc(x)+'</code>').join('');
  const overlay=document.createElement('div');
  overlay.className='migration-resync-overlay';
  overlay.innerHTML='<div class="migration-resync-modal" role="dialog" aria-modal="true" aria-labelledby="migrationResyncTitle">'+
    '<div class="migration-resync-head"><div><div class="eyebrow">FRAMEWORK MIGRATION</div><h2 id="migrationResyncTitle">'+
    (ko?'Root 세션 재동기화 필요':'Root session resynchronization required')+'</h2></div>'+
    '<button type="button" class="migration-resync-close" aria-label="'+(ko?'닫기':'Close')+'">×</button></div>'+
    '<p class="migration-resync-summary">'+(result.legacy_bootstrap?
      (ko?'레거시 Task Mecca 프로젝트를 최신 framework 구조로 안전하게 전환했습니다. 기존 project data/backlog는 보존되며, 교체 전 framework는 백업했습니다.':'Legacy Task Mecca was safely bootstrapped to the current framework layout. Project data/backlogs were preserved and the previous framework was backed up.'):
      (ko?'운영 지침이 변경되었습니다. 현재 Root 세션은 이전 지침을 기억하고 있을 수 있으므로 아래 프롬프트를 복사해 현재 세션에 붙여넣어 주세요.':'Operational instructions changed. The active Root session may still carry the previous rules. Copy the prompt below and paste it into the current Root session.'))+'</p>'+
    (result.backup_path?'<div class="migration-resync-files"><strong>'+(ko?'레거시 framework 백업':'Legacy framework backup')+'</strong><div><code>'+esc(result.backup_path)+'</code></div></div>':'')+
    '<div class="migration-resync-files"><strong>'+(ko?'변경된 지침':'Changed instructions')+'</strong><div>'+(changed||'<span>-</span>')+'</div></div>'+
    '<div class="migration-resync-prompt"><div class="migration-resync-label">'+(ko?'Root 세션에 붙여넣을 프롬프트':'Prompt to paste into the Root session')+'</div>'+
    copyableCodeBlock(esc(prompt))+'</div>'+
    '<div class="migration-resync-actions"><button type="button" class="action-btn" id="migrationResyncDone">'+(ko?'확인':'Done')+'</button></div>'+
    '</div>';
  document.body.appendChild(overlay);
  bindCopyButtons();
  const close=()=>overlay.remove();
  overlay.querySelector('.migration-resync-close')?.addEventListener('click',close);
  overlay.querySelector('#migrationResyncDone')?.addEventListener('click',close);
  overlay.addEventListener('click',e=>{if(e.target===overlay)close();});
}
function bindHubActions() {
  document.querySelectorAll('[data-open-project]').forEach(btn=>btn.addEventListener('click',()=>{
    const path=btn.dataset.openProject||'';
    if(path)switchProject(path);
  }));
  document.querySelectorAll('[data-migrate]').forEach(btn=>btn.addEventListener('click',e=>performProjectMigration(btn.dataset.migrate,e.currentTarget)));
  const changes=$('#hubUpdateChangesBtn');
  if(changes)changes.addEventListener('click',showAvailableUpdateNotes);
  const up=$('#upgradeBtn');
  if(up)up.addEventListener('click',e=>performUpgrade(e.currentTarget));
  bindChannelGesture();
}

function matchesStatusFilter(t, key) {
  if (key === 'ready') return t.state === 'ready';
  if (key === 'doing') return t.state === 'doing' || t.file_state === 'doing';
  if (key === 'hold') return t.state === 'hold' || t.file_state === 'hold';
  if (key === 'blocked') return t.state === 'blocked';
  if (key === 'done') return t.state === 'done' || t.file_state === 'done';
  return true;
}
function allRowsForView() {
  if(state.listData)return Array.isArray(state.listData.items)?state.listData.items:[];
  let items = Object.values(state.snapshot?.all_items || {});
  if (!state.statusFilters.includes('all')) {
    items = items.filter(t => state.statusFilters.some(key => matchesStatusFilter(t,key)));
  }
  if(state.tagFilters.length){
    items=items.filter(task=>state.tagFilters.every(tag=>(task.tags||[]).includes(tag)));
  }
  const q = state.query.trim().toLowerCase();
  if (q) items = items.filter(t => JSON.stringify([t.id,t.title,t.fields,t.document,t.archive_month]).toLowerCase().includes(q));
  const cmpUpdated = (a,b) => String(updatedAt(a)).localeCompare(String(updatedAt(b))) || String(a.sort_key).localeCompare(String(b.sort_key)) || String(a.id).localeCompare(String(b.id));
  if (state.listSort === 'updated_asc') items.sort(cmpUpdated);
  else if (state.listSort === 'updated_desc') items.sort((a,b)=>-cmpUpdated(a,b));
  else if (state.listSort === 'id_asc') items.sort((a,b)=>String(a.sort_key).localeCompare(String(b.sort_key)) || String(a.id).localeCompare(String(b.id)));
  else items.sort((a,b)=>String(b.sort_key).localeCompare(String(a.sort_key)) || String(b.id).localeCompare(String(a.id)));
  return items;
}
function effectiveListPageSize() {
  return state.listPageMode === 'auto' ? Math.max(1, Number(state.autoListPageSize) || 1) : Math.max(1, Number(state.listPageSize) || 20);
}
function pageInfo() {
  const items = allRowsForView();
  if(state.listData){
    const page=Math.max(1,Number(state.listData.page)||1);
    const pages=Math.max(1,Number(state.listData.pages)||1);
    const pageSize=Math.max(1,Number(state.listData.page_size)||effectiveListPageSize());
    state.listPage=page;
    return {items,pageItems:items,page,pages,total:Number(state.listData.total)||0,pageSize};
  }
  const pageSize = effectiveListPageSize();
  const pages = Math.max(1, Math.ceil(items.length / pageSize));
  state.listPage = Math.min(Math.max(1,state.listPage),pages);
  const start = (state.listPage-1)*pageSize;
  return {items, pageItems:items.slice(start,start+pageSize), page:state.listPage, pages, total:items.length, pageSize};
}
function statusFilterBar() {
  const data=currentProjectData()||{};
  const c = data.counts || {};
  const defs = [
    ['all',t('all'),c.all??Object.keys(state.snapshot?.all_items || {}).length],
    ['ready',t('ready'),c.ready||0],
    ['doing',t('working'),c.working||0],
    ['hold',t('hold'),c.hold||0],
    ['blocked',t('blocked'),c.blocked||0],
    ['done',t('done'),c.done||0],
  ];
  return `<div class="status-filter-wrap"><div class="filter-label">${esc(t('status'))}</div><div class="status-filter-bar" role="group" aria-label="${esc(t('status'))}">${defs.map(([k,label,n])=>`<button class="status-filter-btn ${state.statusFilters.includes(k)?'active':''}" data-status-filter="${k}" aria-pressed="${state.statusFilters.includes(k)?'true':'false'}">${esc(label)}<span>${n}</span></button>`).join('')}</div></div>`;
}
function tagFilterBar() {
  const catalog=currentProjectData()?.tag_catalog||{}, stats=Array.isArray(catalog.stats)?catalog.stats:[];
  const used=stats.filter(x=>x.total>0);
  const selected=state.tagFilters.map(tag=>tagChip(tag,true)).join('');
  const explorer=state.tagExplorerOpen?tagExplorerPanel(used):'';
  return `<div class="tag-filter-wrap"><div class="filter-label">${esc(t('tags'))}</div><div class="tag-filter-actions"><div class="tag-filter-selected">${selected||`<span class="tag-empty">${esc(t('noTags'))}</span>`}</div><button type="button" class="tag-explore-btn" id="tagExploreBtn">${esc(t('tagExplore'))} · ${used.length}</button>${state.tagFilters.length?`<button type="button" class="tag-clear-btn" id="tagClearBtn">${esc(t('clearTags'))}</button>`:''}</div></div>${explorer}`;
}
function tagExplorerPanel(stats) {
  const groups={};
  stats.forEach(row=>{(groups[row.namespace]||(groups[row.namespace]=[])).push(row)});
  const registry=new Map((currentProjectData()?.tag_catalog?.registry||[]).map(x=>[x.canonical,x]));
  return `<div class="tag-explorer">${Object.keys(groups).sort().map(ns=>`<section><h3>${esc(ns.toUpperCase())}</h3><div class="tag-stat-list">${groups[ns].sort((a,b)=>b.total-a.total||a.tag.localeCompare(b.tag)).map(row=>{const def=registry.get(row.tag)||{};return `<button type="button" class="tag-stat-row ${state.tagFilters.includes(row.tag)?'active':''}" data-tag-filter="${esc(row.tag)}"><span><strong>${esc(row.tag.split(':')[1]||row.tag)}</strong><small>${esc(def.description||row.description||'')}</small></span><span class="tag-stat-counts"><b>${row.total||0}</b><small>A ${row.active||0} · H ${row.hold||0} · D ${row.done||0}</small></span></button>`}).join('')}</div></section>`).join('')}</div>`;
}
function listControls(info) {
  const autoSelected = state.listPageMode === 'auto';
  const pageValue = autoSelected ? 'auto' : String(state.listPageSize);
  const pageText=t('pageSummary',{page:info.page,pages:info.pages,total:info.total})+(autoSelected?t('autoRowsSummary',{n:info.pageSize}):'');
  return `${statusFilterBar()}${tagFilterBar()}<div class="list-controls"><div class="control-group"><label>${esc(t('sort'))}<select id="listSort"><option value="id_desc" ${state.listSort==='id_desc'?'selected':''}>ID ↓</option><option value="id_asc" ${state.listSort==='id_asc'?'selected':''}>ID ↑</option><option value="updated_desc" ${state.listSort==='updated_desc'?'selected':''}>${esc(t('updatedNewest'))}</option><option value="updated_asc" ${state.listSort==='updated_asc'?'selected':''}>${esc(t('updatedOldest'))}</option></select></label><label>${esc(t('perPage'))}<select id="listPageSize"><option value="auto" ${pageValue==='auto'?'selected':''}>${esc(t('autoRows',{n:info.pageSize}))}</option><option value="10" ${pageValue==='10'?'selected':''}>10</option><option value="20" ${pageValue==='20'?'selected':''}>20</option><option value="50" ${pageValue==='50'?'selected':''}>50</option></select></label></div><div class="pager"><button id="prevPage" ${info.page<=1?'disabled':''}>←</button><span>${esc(pageText)}</span><button id="nextPage" ${info.page>=info.pages?'disabled':''}>→</button></div></div>`;
}

function listView() {
  const data=currentProjectData()||{};
  const info = pageInfo(), rows = info.pageItems, c = data.counts || {};
  state.selectedIndex = Math.min(Math.max(0,state.selectedIndex), Math.max(0,rows.length-1));
  const meta = data.backlog_selection || {};
  const filterLabel = state.statusFilters.includes('all') ? t('allStatuses') : state.statusFilters.map(stateLabel).join(' + ');
  const head=`<div class="task-list-head"><div>${esc(t('id'))}</div><div>${esc(t('taskColumn'))}</div><div>${esc(t('statusColumn'))}</div><div>${esc(t('agentColumn'))}</div><div>${esc(t('activeColumn'))}</div><div>${esc(t('updatedColumn'))}</div><div></div></div>`;
  return `<div class="page-head"><div><div class="eyebrow">${esc(data.repo||t('repository'))}</div><h1>${esc(t('backlog'))}</h1><p class="summary">${esc(meta.selected ? String(meta.selected).split(/[\/]/).pop() : '')} · ${info.total} ${esc(t('items'))} · ${esc(filterLabel)}</p></div></div><div class="metrics"><div class="metric"><strong>${c.working||0}</strong><span>${esc(t('working'))}</span></div><div class="metric"><strong>${c.ready||0}</strong><span>${esc(t('ready'))}</span></div><div class="metric"><strong>${c.hold||0}</strong><span>${esc(t('hold'))}</span></div><div class="metric"><strong>${c.attention||0}</strong><span>${esc(t('needsAttention'))}</span></div></div>${listControls(info)}${rows.length?`${head}<div class="task-list">${rows.map((task,i)=>{
    const act=task.activity||{}, h=act.health;
    const time=task.file_state==='doing'?fmtSec(runningSeconds(task,'active')):(task.file_state==='done'?fmtSec(task.active_seconds):'-');
    const hs=humanSummary(task);
    const sub=hs.purpose||task.scope||'';
    const summaryLine=[hs.change,hs.follow_up?`${t('summaryFollowUp')}: ${hs.follow_up}`:''].filter(Boolean).join(' · ');
    const previewLine=[sub,summaryLine].filter(Boolean).join(' · ');
    const statusSummary=summaryPreview(hs.status_result,90);
    const tags=task.tags||[];
    const visibleTags=tags.slice(0,2);
    const hiddenTags=tags.slice(2);
    const taskMeta=[task.source?.label?sourceLink(task.source,true):'',...visibleTags.map(tag=>tagChip(tag,true)),hiddenTags.length?`<span class="tag-overflow" title="${esc(hiddenTags.join(' · '))}">+${hiddenTags.length}</span>`:''].filter(Boolean).join('');
    const updated=updatedAt(task);
    const alias=(task.agent||'').split('/').pop()||'-';
    return `<div class="task-row ${i===state.selectedIndex?'keyboard-selected':''}" data-id="${esc(task.id)}" data-row-index="${i}" tabindex="-1"><div class="task-id">${esc(task.id)}</div><div class="task-main"><div class="task-mobile-id">${esc(task.id)}</div><div class="task-title">${esc(titleOf(task))}</div>${taskMeta?`<div class="task-meta-row">${taskMeta}</div>`:''}${previewLine?`<div class="task-preview">${esc(previewLine)}</div>`:''}</div><div class="state-col"><span class="status ${esc(task.state)}">${esc(stateLabel(task.state))}</span>${statusSummary?`<div class="task-state-summary">${esc(statusSummary)}</div>`:''}${['quiet','stale','worker_missing'].includes(h)?`<div class="task-sub">${esc(healthLabel(h))}</div>`:''}</div><div class="task-agent"><div>${esc(alias)}</div>${task.archive_month?`<div class="task-sub">archive/${esc(task.archive_month)}</div>`:''}</div><div class="task-active timer live-timer" data-id="${esc(task.id)}">${time}</div><div class="task-updated" title="${esc(dateTimeLabel(updated))}"><strong>${esc(ago(updated))}</strong><span>${esc(dateTimeLabel(updated,true))}</span></div><div class="chev">›</div></div>`;
  }).join('')}</div>`:`<div class="empty">${esc(t('noMatches'))}</div>`}`;
}

function attentionView() {
  const snapshot=state.snapshot||{}, arr=snapshot.attention||[], all=snapshot.all_items||{};
  return `<div class="page-head"><div><div class="eyebrow">${esc(t('operationsEyebrow'))}</div><h1>${esc(t('attention'))}</h1><p class="summary">${esc(t('advisory'))}</p></div></div>${arr.length?arr.map(a=>{const task=all[a.id]||{},label=a.title||healthLabel(a.health),message=a.message||'',resume=a.resume_condition||'';return `<div class="attention-card" data-id="${esc(a.id)}"><div class="task-id">${esc(a.id)}</div><div><strong>${esc(titleOf(task))}</strong><p><b>${esc(label)}</b>${message?` · ${esc(message)}`:''}${resume?` · ${esc(resume)}`:''}</p></div><span class="badge ${a.severity==='danger'||a.health==='worker_missing'||a.health==='stale'?'danger':'warn'}">${esc(label)}</span></div>`}).join(''):`<div class="empty">${esc(t('noAttention'))}</div>`}`;
}
function rootPromptForLanguage(raw,language=state.language) {
  raw=String(raw||'');
  if(!raw.trim())return '';
  const heading=language==='ko'?'한국어':'English';
  const marker='## '+heading;
  const start=raw.indexOf(marker);
  if(start<0)return raw.trim();
  const section=raw.slice(start+marker.length);
  const match=section.match(/\`\`\`(?:text)?\s*\n([\s\S]*?)\n\`\`\`/);
  return (match?.[1]||section.split(/^##\s+/m)[0]||'').trim();
}
function manualRootPromptCard(m) {
  const prompt=rootPromptForLanguage(m?.root_prompt||'',state.language);
  const path=m?.root_prompt_path||'_task_mecca/ROOT_PROMPT.md';
  if(!prompt)return `<section class="root-prompt-card unavailable"><div class="root-prompt-head"><div><strong>${esc(t('rootPromptTitle'))}</strong><span>${esc(t('rootPromptUnavailable'))}</span></div></div></section>`;
  return `<section class="root-prompt-card"><div class="root-prompt-head"><div><strong>${esc(t('rootPromptTitle'))}</strong><span>${esc(t('rootPromptIntro'))}</span></div><div class="root-prompt-path"><span>${esc(t('rootPromptPath'))}</span><code>${esc(path)}</code></div></div><div class="copy-code-wrap root-prompt-code"><button class="copy-code-btn" type="button" aria-label="${esc(t('copyCode'))}" title="${esc(t('copyCode'))}">${COPY_ICON}</button><pre><code>${esc(prompt)}</code></pre></div></section>`;
}
function manualView() {
  const m=state.manualByLanguage[state.language]||{}, tab=state.manualTab||'quick', body=tab==='operations'?(m.session_guide||''):(m.readme||'');
  const rootPrompt=tab==='quick'?manualRootPromptCard(m):'';
  return `<div class="manual-shell"><div class="manual-hero"><div class="eyebrow">${esc(t('help'))}</div><h1>${esc(t('manual'))}</h1><p class="summary">${esc(t('manualIntro'))}</p></div><div class="manual-callout"><strong>${esc(t('dashboardLaunch'))}</strong><div class="copy-code-wrap compact"><button class="copy-code-btn" type="button" aria-label="${esc(t('copyCode'))}" title="${esc(t('copyCode'))}">${COPY_ICON}</button><pre><code>task-mecca web</code></pre></div></div><div class="shortcut-manual"><strong>${esc(t('keyboard'))}</strong><span><kbd>↑/↓</kbd> ${esc(t('move'))}</span><span><kbd>Enter/→</kbd> ${esc(t('open'))}</span><span><kbd>←/Esc</kbd> ${esc(t('back'))}</span><span><kbd>/</kbd> ${esc(t('search'))}</span><span><kbd>PgUp/PgDn</kbd> ${esc(t('page'))}</span></div><div class="manual-tabs"><button class="manual-tab ${tab==='quick'?'active':''}" data-manual-tab="quick">${esc(t('quickStart'))}</button><button class="manual-tab ${tab==='operations'?'active':''}" data-manual-tab="operations">${esc(t('detailedGuide'))}</button></div>${rootPrompt}<div class="section markdown">${markdown(body||t('manualLoading'),{copyCode:true})}</div></div>`;
}
Object.assign(I18N.ko,{
  runtimeCurrentValid:'현재 유효',
  runtimeNeedsCheck:'상태 확인 필요',
  runtimeNoNeedsCheck:'현재 상태 확인이 필요한 Runtime 세션이 없습니다.',
  runtimeTerminalArchived:'종료·보관',
  runtimeStorage:'Runtime 기록 저장소',
  runtimeStorageOpen:'저장소 관리',
  runtimeStorageClose:'저장소 닫기',
  runtimeStorageLoading:'저장소 사용량을 계산하는 중…',
  runtimeStorageTotal:'전체 사용량',
  runtimeStorageRaw:'Raw 이벤트',
  runtimeStorageHistory:'종료 실행 이력',
  runtimeStorageLegacy:'Legacy 기록',
  runtimeStorageProtected:'현재/미확정 보호 데이터',
  runtimeStorageReclaimable:'안전 정리 가능',
  runtimeStorageOldest:'가장 오래된 정리 후보',
  runtimeStoragePolicy:'Raw {raw}일 · 종료 이력 {days}일 · 최대 {max}건',
  runtimeCleanup:'안전 정리',
  runtimeCleanupNone:'현재 정책 기준으로 안전하게 정리할 기록이 없습니다.',
  runtimeCleanupPreview:'정리 대상: 파일 {files}개 · 이력 {attempts}건 · 약 {bytes}',
  runtimeCleanupConfirm:'현재 실행 및 상태 미확정 세션은 보존합니다. 종료가 확정되고 보존 정책을 초과한 Runtime 기록 약 {bytes}를 정리할까요?',
  runtimeCleanupRunning:'정리 중…',
  runtimeCleanupDone:'Runtime 기록 {bytes}를 정리했습니다.',
  runtimeFiles:'파일 {n}개',
});
Object.assign(I18N.en,{
  runtimeCurrentValid:'Current valid',
  runtimeNeedsCheck:'Needs verification',
  runtimeNoNeedsCheck:'No runtime session currently needs verification.',
  runtimeTerminalArchived:'Terminal / archived',
  runtimeStorage:'Runtime record storage',
  runtimeStorageOpen:'Manage storage',
  runtimeStorageClose:'Close storage',
  runtimeStorageLoading:'Calculating runtime storage usage…',
  runtimeStorageTotal:'Total usage',
  runtimeStorageRaw:'Raw events',
  runtimeStorageHistory:'Terminal history',
  runtimeStorageLegacy:'Legacy records',
  runtimeStorageProtected:'Protected current/unknown data',
  runtimeStorageReclaimable:'Safely reclaimable',
  runtimeStorageOldest:'Oldest cleanup candidate',
  runtimeStoragePolicy:'Raw {raw}d · terminal history {days}d · max {max}',
  runtimeCleanup:'Safe cleanup',
  runtimeCleanupNone:'No runtime records are safely cleanable under the current policy.',
  runtimeCleanupPreview:'Cleanup: {files} files · {attempts} history records · about {bytes}',
  runtimeCleanupConfirm:'Current and uncertain sessions will be preserved. Clean about {bytes} of terminal runtime records beyond retention?',
  runtimeCleanupRunning:'Cleaning…',
  runtimeCleanupDone:'Cleaned {bytes} of runtime records.',
  runtimeFiles:'{n} files',
});

function fmtBytes(value) {
  let n=Math.max(0,Number(value)||0);
  const units=['B','KB','MB','GB'];
  let i=0;
  while(n>=1024&&i<units.length-1){n/=1024;i++;}
  const digits=i===0?0:n>=100?0:n>=10?1:2;
  return `${n.toFixed(digits)} ${units[i]}`;
}
function runtimeStoragePanel() {
  if(!state.runtimeStorageOpen)return '';
  if(state.runtimeStorageLoading)return `<section class="runtime-storage-panel"><div class="runtime-storage-loading">${esc(t('runtimeStorageLoading'))}</div></section>`;
  if(state.runtimeStorageError)return `<section class="runtime-storage-panel"><div class="runtime-history-error">${esc(state.runtimeStorageError)}</div></section>`;
  const r=state.runtimeStorage||{}, cleanup=r.cleanup||{}, policy=r.retention||{};
  const reclaim=Number(cleanup.reclaimable_bytes||0);
  return `<section class="runtime-storage-panel">
    <div class="runtime-storage-head"><div><h3>${esc(t('runtimeStorage'))}</h3><p>${esc(t('runtimeStoragePolicy',{raw:policy.raw_days||7,days:policy.history_days||90,max:policy.history_max_attempts||2000}))}</p></div><strong>${esc(fmtBytes(r.total_bytes||0))}</strong></div>
    <div class="runtime-storage-grid">
      <div><span>${esc(t('runtimeStorageRaw'))}</span><strong>${esc(fmtBytes(r.raw?.bytes||0))}</strong><small>${esc(t('runtimeFiles',{n:r.raw?.files||0}))}</small></div>
      <div><span>${esc(t('runtimeStorageHistory'))}</span><strong>${esc(fmtBytes(r.history?.bytes||0))}</strong><small>${Number(r.history_attempts||0)} attempts</small></div>
      <div><span>${esc(t('runtimeStorageLegacy'))}</span><strong>${esc(fmtBytes(r.legacy?.bytes||0))}</strong><small>${esc(t('runtimeFiles',{n:r.legacy?.files||0}))}</small></div>
      <div><span>${esc(t('runtimeStorageProtected'))}</span><strong>${esc(fmtBytes(r.protected_raw?.bytes||0))}</strong><small>${esc(t('runtimeFiles',{n:r.protected_raw?.files||0}))}</small></div>
    </div>
    <div class="runtime-cleanup-preview">
      <div><span>${esc(t('runtimeStorageReclaimable'))}</span><strong>${esc(fmtBytes(reclaim))}</strong></div>
      ${cleanup.oldest_candidate_at?`<div><span>${esc(t('runtimeStorageOldest'))}</span><strong>${esc(dateTimeLabel(cleanup.oldest_candidate_at,true))}</strong></div>`:''}
    </div>
    <p class="runtime-storage-note">${reclaim>0?esc(t('runtimeCleanupPreview',{files:cleanup.candidate_files||0,attempts:cleanup.candidate_attempts||0,bytes:fmtBytes(reclaim)})):esc(t('runtimeCleanupNone'))}</p>
    <button class="runtime-cleanup-btn" id="runtimeCleanupBtn" ${reclaim<=0?'disabled':''}>${esc(t('runtimeCleanup'))}</button>
  </section>`;
}

function runtimeStateLabel(value) {
  return {
    starting:t('runtimeStateStarting'),
    running:t('runtimeStateRunning'),
    waiting_user:t('runtimeStateWaitingUser'),
    waiting_approval:t('runtimeStateWaitingApproval'),
    interrupted:t('runtimeStateInterrupted'),
    completed:t('runtimeStateCompleted'),
    errored:t('runtimeStateErrored'),
    shutdown:t('runtimeStateShutdown'),
    runtime_unknown:t('runtimeStateUnknown'),
  }[value]||value||t('runtimeStateUnknown');
}
function runtimeBindingLabel(value) {
  return value==='bound'?t('runtimeBound'):value==='ambiguous'?t('runtimeAmbiguous'):t('runtimeUnbound');
}
function runtimeElapsedSeconds(attempt) {
  if(attempt?.terminal || !attempt?.started_at)return Math.max(0,Number(attempt?.elapsed_ms||0)/1000);
  const started=new Date(attempt.started_at).getTime();
  if(Number.isNaN(started))return Math.max(0,Number(attempt?.elapsed_ms||0)/1000);
  return Math.max(0,(Date.now()-started)/1000);
}
function runtimeTransitionLabel(row) {
  if(row?.kind==='binding')return `${t('runtimeBinding')} · ${runtimeBindingLabel(row.reason)}`;
  return runtimeStateLabel(row?.state);
}
function runtimeAttemptCard(attempt,findings) {
  const name=attempt.agent_path||attempt.runtime_agent_id||attempt.attempt_id;
  const task=attempt.task_id||'-';
  const last=attempt.last_activity_at||attempt.last_observed_at||'';
  const active=attempt.active_time_available&&attempt.observed_active_ms!=null?fmtSec(Number(attempt.observed_active_ms)/1000):t('runtimeUnavailable');
  const waiting=fmtSec(Number(attempt.waiting_ms||0)/1000);
  const transitions=(attempt.recent_transitions||[]).slice(-8);
  const bindingClass=attempt.binding_state==='ambiguous'?'danger':attempt.binding_state==='unbound'?'warn':'ok';
  const terminalClass=['errored','interrupted','shutdown'].includes(attempt.current_state)?'danger':attempt.current_state==='completed'?'ok':'';
  const findingHTML=(findings||[]).map(x=>`<span class="badge ${x.severity==='warning'?'warn':x.severity==='error'?'danger':''}">${esc(x.code)}</span>`).join('');
  const attentionState=['waiting_user','waiting_approval','errored','interrupted','shutdown'].includes(attempt.current_state);
  const attentionFinding=(findings||[]).some(x=>x.severity==='warning'||x.severity==='error');
  const openByDefault=attentionState||attentionFinding;
  const disclosureKey=String(attempt.attempt_id||attempt.runtime_agent_id||name||'runtime-attempt');
  const hasRememberedOpen=Object.prototype.hasOwnProperty.call(state.runtimeAttemptDisclosure,disclosureKey);
  const isOpen=hasRememberedOpen?Boolean(state.runtimeAttemptDisclosure[disclosureKey]):openByDefault;
  const historyOpen=Boolean(state.runtimeTransitionDisclosure[disclosureKey]);
  const elapsedAttrs=`class="runtime-elapsed" data-started-at="${esc(attempt.started_at||'')}" data-ended-at="${esc(attempt.ended_at||'')}" data-elapsed-ms="${Number(attempt.elapsed_ms||0)}"`;
  return `<details class="runtime-attempt-card runtime-attempt-disclosure" data-runtime-attempt-key="${esc(disclosureKey)}" ${isOpen?'open':''}>
    <summary class="runtime-attempt-summary">
      <div class="runtime-attempt-identity">
        <div class="runtime-attempt-name">${esc(name)}</div>
        <div class="runtime-attempt-sub">${esc(attempt.provider||'-')} · ${esc(task)}</div>
      </div>
      <div class="runtime-attempt-badges">
        <span class="status ${esc(attempt.current_state||'runtime_unknown')} ${terminalClass}">${esc(runtimeStateLabel(attempt.current_state))}</span>
        <span class="badge ${bindingClass}">${esc(runtimeBindingLabel(attempt.binding_state))}</span>
      </div>
      <div class="runtime-attempt-glance">
        <div><span>${esc(t('runtimeElapsed'))}</span><strong ${elapsedAttrs}>${esc(fmtSec(runtimeElapsedSeconds(attempt)))}</strong></div>
        <div><span>${esc(t('runtimeLastActivity'))}</span><strong title="${esc(dateTimeLabel(last))}">${esc(last?ago(last):'-')}</strong></div>
      </div>
      <span class="runtime-attempt-chevron" aria-hidden="true">›</span>
    </summary>
    <div class="runtime-attempt-detail">
      <div class="runtime-meta-grid">
        <div><span>${esc(t('runtimeAttempt'))}</span><strong title="${esc(attempt.attempt_id||'')}">${esc(attempt.attempt_id||'-')}</strong></div>
        <div><span>${esc(t('runtimeId'))}</span><strong title="${esc(attempt.runtime_agent_id||'')}">${esc(attempt.runtime_agent_id||'-')}</strong></div>
        <div><span>${esc(t('runtimeStarted'))}</span><strong>${esc(dateTimeLabel(attempt.started_at,true))}</strong></div>
        <div><span>${esc(t('runtimeEnded'))}</span><strong>${esc(dateTimeLabel(attempt.ended_at,true))}</strong></div>
        <div><span>${esc(t('runtimeObservedActive'))}</span><strong>${esc(active)}</strong></div>
        <div><span>${esc(t('runtimeWaiting'))}</span><strong>${esc(waiting)}</strong></div>
      </div>
      ${findingHTML?`<div class="runtime-findings">${findingHTML}</div>`:''}
      <details class="runtime-history" data-runtime-history-key="${esc(disclosureKey)}" ${historyOpen?'open':''}>
        <summary>${esc(t('runtimeRecentTransitions'))} · ${transitions.length}</summary>
        <div class="runtime-transition-list">
          ${transitions.length?transitions.map(row=>`<div class="runtime-transition"><time>${esc(dateTimeLabel(row.at,true))}</time><strong>${esc(runtimeTransitionLabel(row))}</strong><span>${esc((row.evidence_source||'-')+'/'+(row.observation_quality||'-'))}</span></div>`).join(''):`<div class="worker-empty">-</div>`}
        </div>
      </details>
    </div>
  </details>`;
}
function runtimeHookStateLabel(stateValue) {
  return stateValue==='observed'?t('runtimeHookObserved'):stateValue==='verification_required'?t('runtimeHookVerificationRequired'):t('runtimeHookUnconfigured');
}
function runtimeHookStateClass(stateValue) {
  return stateValue==='observed'?'ok':stateValue==='verification_required'?'warn':'';
}
function runtimeHookGuidance(hook) {
  const events=hook?.observed_events||{};
  const activityOnly=Boolean(events.activity)&&!events.start;
  if(hook?.provider==='codex') {
    if(!hook?.configured)return t('runtimeCodexUnconfigured');
    if(activityOnly)return t('runtimeCodexActivityOnly');
    if(hook?.state==='observed')return t('runtimeCodexObserved');
    return t('runtimeCodexVerify');
  }
  if(!hook?.configured)return t('runtimeClaudeUnconfigured');
  if(activityOnly)return t('runtimeClaudeActivityOnly');
  if(hook?.state==='observed')return t('runtimeClaudeObserved');
  return t('runtimeClaudeVerify');
}
function runtimeHookEventChip(label,received) {
  return `<span class="runtime-hook-event ${received?'received':''}"><b>${esc(label)}</b><span>${esc(received?t('runtimeHookReceived'):t('runtimeHookNotObserved'))}</span></span>`;
}
function runtimeHookCard(hook) {
  const events=hook?.observed_events||{}, provider=String(hook?.provider||'').toUpperCase();
  const action=hook?.configured?'disable':'enable';
  const actionLabel=hook?.configured?t('runtimeHookDisable'):t('runtimeHookConfigure');
  const last=hook?.last_observed_at||'';
  return `<article class="runtime-hook-card">
    <div class="runtime-hook-card-head">
      <div class="runtime-hook-identity"><strong>${esc(provider)}</strong><span title="${esc(hook?.path||'')}">${esc(hook?.path||'-')}</span></div>
      <span class="badge ${runtimeHookStateClass(hook?.state)}">${esc(runtimeHookStateLabel(hook?.state))}</span>
    </div>
    <p class="runtime-hook-guidance">${esc(runtimeHookGuidance(hook))}</p>
    <div class="runtime-hook-events">
      ${runtimeHookEventChip(t('runtimeHookActivity'),Boolean(events.activity))}
      ${runtimeHookEventChip(t('runtimeHookStart'),Boolean(events.start))}
      ${runtimeHookEventChip(t('runtimeHookStop'),Boolean(events.stop))}
    </div>
    ${last?`<div class="runtime-hook-last">${esc(t('runtimeHookLastObserved'))} · ${esc(ago(last))}</div>`:''}
    <div class="runtime-hook-actions">
      <button class="runtime-hook-action ${action==='disable'?'secondary':''}" data-runtime-hook-action="${action}" data-provider="${esc(hook?.provider||'')}">${esc(actionLabel)}</button>
    </div>
  </article>`;
}
function runtimeHistoryRow(attempt) {
  const label=attempt.agent_path||attempt.runtime_agent_id||attempt.attempt_id||'-';
  const ended=attempt.ended_at||attempt.last_observed_at||'';
  const elapsed=fmtSec(Number(attempt.elapsed_ms||0)/1000);
  return `<div class="runtime-history-row">
    <div class="runtime-history-main"><strong title="${esc(label)}">${esc(label)}</strong><span>${esc((attempt.provider||'-').toUpperCase())} · ${esc(attempt.attempt_id||'-')}</span></div>
    <span class="status ${esc(attempt.current_state||'runtime_unknown')}">${esc(runtimeStateLabel(attempt.current_state))}</span>
    <div class="runtime-history-meta"><span>${esc(dateTimeLabel(ended,true))}</span><span>${esc(elapsed)}</span><span>${esc(runtimeBindingLabel(attempt.binding_state))}</span></div>
  </div>`;
}
function runtimeHistoryPanel(meta={}) {
  const history=state.runtimeHistory||{}, items=history.items||[];
  if(!state.runtimeHistoryOpen)return '';
  const retention=history.retention||meta.retention||{};
  const page=Number(history.page||1), pages=Number(history.total_pages||0), total=Number(history.total||0);
  const policy=t('runtimeHistoryRetention',{
    raw:Number(retention.raw_days||7),
    days:Number(retention.history_days||90),
    max:Number(retention.history_max_attempts||2000)
  });
  const content=state.runtimeHistoryLoading
    ? `<div class="runtime-history-loading">${esc(t('runtimeHistoryLoading'))}</div>`
    : state.runtimeHistoryError
      ? `<div class="runtime-history-error">${esc(state.runtimeHistoryError)}</div>`
      : items.length
        ? `<div class="runtime-history-list">${items.map(runtimeHistoryRow).join('')}</div>`
        : `<div class="runtime-history-empty">${esc(t('runtimeHistoryEmpty'))}</div>`;
  const pager=(!state.runtimeHistoryLoading&&!state.runtimeHistoryError&&pages>1)
    ? `<div class="runtime-history-pager">
        <button data-runtime-history-page="${Math.max(1,page-1)}" ${page<=1?'disabled':''}>←</button>
        <span>${esc(t('runtimeHistoryPage',{page,pages,total}))}</span>
        <button data-runtime-history-page="${Math.min(pages,page+1)}" ${page>=pages?'disabled':''}>→</button>
      </div>`
    : (!state.runtimeHistoryLoading&&!state.runtimeHistoryError
        ? `<div class="runtime-history-page-label">${esc(t('runtimeHistoryPage',{page:pages?Math.min(page,pages):0,pages,total}))}</div>`
        : '');
  return `<section class="runtime-history-panel">
    <div class="runtime-history-head"><div><h3>${esc(t('runtimeHistoryTitle'))}</h3><p>${esc(policy)}</p></div></div>
    ${content}${pager}
  </section>`;
}
function workloadView() {
  const snapshot=state.snapshot||{}, w=snapshot.workload||{}, agents=w.agents||[], all=snapshot.all_items||{}, unassigned=w.unassigned_doing||[], released=w.released_holds||[];
  const runtime=snapshot.runtime_observability||{}, attempts=runtime.attempts||[], findings=runtime.findings||[], hooks=runtime.hooks||[], rc=runtime.counts||{}, historyMeta=runtime.history||{}, groups=runtime.session_groups||{};
  const currentIDs=new Set(groups.current_ids||[]);
  const needsCheckIDs=new Set(groups.needs_check_ids||[]);
  const currentAttempts=attempts.filter(a=>currentIDs.has(a.attempt_id));
  const needsCheckAttempts=attempts.filter(a=>needsCheckIDs.has(a.attempt_id));
  const recentTerminalAttempts=attempts.filter(a=>a.terminal);
  const findingsByAttempt={};
  findings.forEach(row=>{if(row.attempt_id)(findingsByAttempt[row.attempt_id]??=[]).push(row)});
  const latestByAgent={};
  attempts.forEach(attempt=>{if(attempt.agent_path&&!latestByAgent[attempt.agent_path])latestByAgent[attempt.agent_path]=attempt});
  const hookCards=hooks.map(runtimeHookCard).join('');
  const runtimePanel=`<section class="runtime-observability">
    <div class="runtime-section-head">
      <div><div class="eyebrow">Runtime</div><h2>${esc(t('runtimeObservability'))}</h2><p class="summary">${esc(t('runtimeObservabilityIntro'))}</p></div>
    </div>
    <div class="runtime-hook-panel">
      <div class="runtime-hook-explainer"><strong>${esc(t('runtimeHookStatus'))}</strong><p>${esc(t('runtimeHookNote'))}</p><p class="runtime-hook-history-note">${esc(t('runtimeHookHistoryPreserved'))}</p></div>
      <div class="runtime-hook-list">${hookCards||'-'}</div>
    </div>
    <div class="metrics runtime-metrics">
      <div class="metric"><strong>${Number(rc.current||0)}</strong><span>${esc(t('runtimeCurrentValid'))}</span></div>
      <div class="metric"><strong>${Number(rc.needs_check||0)}</strong><span>${esc(t('runtimeNeedsCheck'))}</span></div>
      <div class="metric"><strong>${Number(rc.terminal||0)}</strong><span>${esc(t('runtimeTerminalArchived'))}</span></div>
      <div class="metric"><strong>${Number(rc.unbound||0)}</strong><span>${esc(t('runtimeUnbound'))}</span></div>
      <div class="metric"><strong>${Number(rc.ambiguous||0)}</strong><span>${esc(t('runtimeAmbiguous'))}</span></div>
      <div class="metric"><strong>${Number(rc.stale||0)}</strong><span>${esc(t('runtimeStale'))}</span></div>
    </div>
    <div class="runtime-storage-toolbar">
      <button class="runtime-history-toggle" id="runtimeStorageToggle">${esc(state.runtimeStorageOpen?t('runtimeStorageClose'):t('runtimeStorageOpen'))}</button>
    </div>
    ${runtimeStoragePanel()}
    <div class="runtime-attempt-section">
      <div class="runtime-attempt-section-head"><h3>${esc(t('runtimeCurrentValid'))} · ${currentAttempts.length}</h3></div>
      ${currentAttempts.length?`<div class="runtime-attempt-grid">${currentAttempts.map(a=>runtimeAttemptCard(a,findingsByAttempt[a.attempt_id]||[])).join('')}</div>`:`<div class="runtime-empty compact">${esc(t('runtimeNoCurrentExecutions'))}</div>`}
    </div>
    <div class="runtime-attempt-section runtime-needs-check">
      <div class="runtime-attempt-section-head"><h3>${esc(t('runtimeNeedsCheck'))} · ${needsCheckAttempts.length}</h3></div>
      ${needsCheckAttempts.length?`<div class="runtime-attempt-grid">${needsCheckAttempts.map(a=>runtimeAttemptCard(a,findingsByAttempt[a.attempt_id]||[])).join('')}</div>`:`<div class="runtime-empty compact">${esc(t('runtimeNoNeedsCheck'))}</div>`}
    </div>
    <div class="runtime-attempt-section">
      <div class="runtime-attempt-section-head">
        <h3>${esc(t('runtimeRecentCompleted'))} · ${recentTerminalAttempts.length}</h3>
        ${Number(historyMeta.total||0)>0?`<button class="runtime-history-toggle" id="runtimeHistoryToggle">${esc(state.runtimeHistoryOpen?t('runtimeHistoryClose'):t('runtimeHistoryOpen'))} · ${Number(historyMeta.total||0)}</button>`:''}
      </div>
      ${recentTerminalAttempts.length?`<div class="runtime-attempt-grid">${recentTerminalAttempts.map(a=>runtimeAttemptCard(a,findingsByAttempt[a.attempt_id]||[])).join('')}</div>`:`<div class="runtime-empty compact">${esc(t('runtimeHistoryEmpty'))}</div>`}
    </div>
    ${runtimeHistoryPanel(historyMeta)}
  </section>`;

  const doingTotal=agents.reduce((n,a)=>n+(a.doing_count||0),0), blockingTotal=agents.reduce((n,a)=>n+(a.blocking_count||0),0), continuityTotal=agents.reduce((n,a)=>n+(a.ready_candidate_count||0),0);
  const cards=agents.map(a=>{
    const alias=String(a.agent||'').split('/').pop()||a.agent;
    const live=latestByAgent[a.agent];
    const liveLine=live?`<div class="worker-runtime-line"><span class="status ${esc(live.current_state||'runtime_unknown')}">${esc(runtimeStateLabel(live.current_state))}</span><span>${esc(live.provider||'-')} · ${esc(live.attempt_id||'-')}</span></div>`:'';
    const tasks=(a.doing||[]).map(id=>{const task=all[id]||{},act=task.activity||{},scope=a.change_scopes?.[id]||task.scope||'-';return `<div class="worker-task" data-id="${esc(id)}"><div class="task-id">${esc(id)}</div><div><div class="worker-task-title">${esc(titleOf(task)||id)}</div><div class="worker-health ${esc(act.health||'')}">${esc(healthLabel(act.health||'runtime_unknown'))}${act.last_activity_at?` · ${esc(ago(act.last_activity_at))}`:''}</div></div><div class="timer live-timer" data-id="${esc(id)}">${fmtSec(runningSeconds(task,'active'))}</div><div class="scope-text" title="${esc(scope)}">${esc(scope)}</div></div>`}).join('');
    const blocked=(a.blocking||[]).map(id=>`<span class="badge" data-id="${esc(id)}">${esc(t('blocks'))} ${esc(id)}</span>`).join('');
    const ready=(a.ready_candidates||[]).map(x=>`<span class="badge" data-id="${esc(x.id)}">${esc(t('continuity'))} ${esc(x.id)}</span>`).join('');
    return `<article class="worker-card"><div class="worker-head"><div><div class="worker-name">${esc(a.agent)}</div><div class="worker-alias">${esc(alias)}</div>${liveLine}</div><div class="worker-stats"><div><strong>${a.doing_count||0}</strong><span>${esc(t('doing'))}</span></div><div><strong>${a.blocking_count||0}</strong><span>${esc(t('downstreamBlocked'))}</span></div><div><strong>${a.ready_candidate_count||0}</strong><span>${esc(t('continuity'))}</span></div></div></div><div class="worker-body">${tasks||`<div class="worker-empty">${esc(t('noCurrentDoing'))}</div>`}${blocked?`<div class="mini-panel"><h3>${esc(t('downstreamBlocked'))}</h3><div class="mini-list">${blocked}</div></div>`:''}${ready?`<div class="mini-panel"><h3>${esc(t('readyContinuity'))}</h3><div class="mini-list">${ready}</div></div>`:''}</div></article>`;
  }).join('');
  const backlogPanel=`<section class="assigned-workload-section"><div class="runtime-section-head compact"><div><div class="eyebrow">Backlog / Git</div><h2>${esc(t('runtimeAssignedWorkload'))}</h2><p class="summary">${esc(t('runtimeAssignedWorkloadIntro'))}</p></div></div><div class="metrics"><div class="metric"><strong>${agents.length}</strong><span>${esc(t('workers'))}</span></div><div class="metric"><strong>${doingTotal}</strong><span>${esc(t('doing'))}</span></div><div class="metric"><strong>${blockingTotal}</strong><span>${esc(t('downstreamBlocked'))}</span></div><div class="metric"><strong>${continuityTotal}</strong><span>${esc(t('readyContinuity'))}</span></div></div>${unassigned.length?`<div class="unassigned-warning"><strong>${esc(t('unassignedDoing'))}:</strong> ${esc(unassigned.join(', '))}</div>`:''}${cards?`<div class="workload-grid">${cards}</div>`:`<div class="empty">${esc(t('noWorkload'))}</div>`}${released.length?`<div class="workload-secondary"><div class="mini-panel"><h3>${esc(t('releasedHold'))}</h3><div class="mini-list">${released.map(x=>`<span class="badge" data-id="${esc(x.id)}">${esc(x.id)}</span>`).join('')}</div></div></div>`:''}</section>`;
  return `<div class="page-head"><div><div class="eyebrow">${esc(t('operationsEyebrow'))}</div><h1>${esc(t('workload'))}</h1><p class="summary">${esc(t('workloadIntro'))}</p></div></div>${runtimePanel}${backlogPanel}`;
}
function issuesView() {
  const h=state.snapshot?.health||{}, rows=[];
  Object.entries(h).forEach(([k,v])=>{if(Array.isArray(v))v.forEach(x=>rows.push([k,x]));else if(v)rows.push([k,v])});
  return `<div class="page-head"><div><div class="eyebrow">${esc(t('diagnostics'))}</div><h1>${esc(t('issues'))}</h1><p class="summary">${esc(t('issuesIntro'))}</p></div></div>${rows.length?`<div class="markdown"><pre><code>${esc(rows.map(([k,v])=>`${k}: ${JSON.stringify(v,null,2)}`).join('\n\n'))}</code></pre></div>`:`<div class="empty">${esc(t('noIssues'))}</div>`}`;
}
function metaRow(k,v){return `<div class="meta-row"><span>${esc(k)}</span><span>${esc(v||'-')}</span></div>`}
function sourceLink(source,compact=false) {
  const label=source?.label||source?.raw||'';
  if(!label)return '';
  const cls=`source-link${compact?' compact':''}`;
  if(source?.url)return `<a class="${cls}" data-external-source href="${esc(source.url)}" target="_blank" rel="noopener noreferrer" title="${esc(t('openSource'))}"><span>${esc(label)}</span><span aria-hidden="true">↗</span></a>`;
  return `<span class="${cls} muted"><span>${esc(label)}</span></span>`;
}
function contractSections(task) {
  const doc=task.document||{}, req=doc.requirements||{}, kind=doc.contract_kind||'legacy', ss=doc.section_summaries||{};
  if(kind==='simple'){
    const definition=`<div class="contract-note">${esc(t('simpleNote'))}</div><h3>${esc(t('goal'))}</h3><div class="markdown">${markdown(req.goal)}</div>`;
    const acceptance=`<div class="markdown">${markdown(req.acceptance)}</div>`;
    return semanticSection('task-definition',t('taskDefinition'),ss.task_definition,definition)+semanticSection('acceptance',t('completionCriteria'),ss.acceptance,acceptance);
  }
  if(kind==='defined'){
    const background=semanticSection('background',t('backgroundProblem'),ss.background,`<div class="contract-note">${esc(t('definedNote'))}</div><div class="markdown">${markdown(req.background)}</div>`);
    const requirements=semanticSection('requirements',t('requirementsAndConstraints'),ss.requirements,`<h3>${esc(t('goal'))}</h3><div class="markdown">${markdown(req.goal)}</div><h3>${esc(t('requirements'))}</h3><div class="markdown">${markdown(req.requirements)}</div>`);
    const scope=semanticSection('scope',t('workScope'),ss.scope,`<h3>${esc(t('scopeIn'))}</h3><div class="markdown">${markdown(req.scope_in)}</div><h3>${esc(t('scopeOut'))}</h3><div class="markdown">${markdown(req.scope_out)}</div><h3>${esc(t('constraints'))}</h3><div class="markdown">${markdown(req.constraints)}</div>`);
    const acceptance=semanticSection('acceptance',t('completionCriteria'),ss.acceptance,`<div class="markdown">${markdown(req.acceptance)}</div>`);
    return background+requirements+scope+acceptance;
  }
  const legacy=doc.legacy_description||task.fields?.설명;
  return semanticSection('legacy-task',t('legacyDetails'),ss.legacy_task,`<div class="contract-note">${esc(t('legacyNote'))}</div><div class="markdown">${markdown(legacy)}</div>`);
}
function detailToc() {
  const icon=`<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 6h12M8 12h12M8 18h12M4 6h.01M4 12h.01M4 18h.01"/></svg>`;
  return `<button class="detail-toc-fab" type="button" aria-label="${esc(t('tocOpen'))}" title="${esc(t('tocTitle'))}">${icon}</button><div class="detail-toc-scrim" aria-hidden="true"></div><aside class="detail-toc" aria-label="${esc(t('tocTitle'))}"><div class="detail-toc-head"><div class="detail-toc-title">${esc(t('tocTitle'))}</div><button class="detail-toc-close" type="button" aria-label="${esc(t('tocClose'))}" title="${esc(t('tocClose'))}">×</button></div><nav class="detail-toc-nav"></nav></aside>`;
}
function openDetailSection(id,behavior='smooth') {
  const target=document.getElementById(id);
  if(!target)return;
  if(target.tagName?.toLowerCase()==='details')target.open=true;
  target.scrollIntoView({behavior,block:'start'});
  history.replaceState({},'',`${location.pathname}${location.search}#${encodeURIComponent(id)}`);
}
function bindDetailToc() {
  const layout=document.querySelector('.detail-layout');
  const toc=document.querySelector('.detail-toc-nav');
  const toggle=document.querySelector('.detail-toc-fab');
  const close=document.querySelector('.detail-toc-close');
  const scrim=document.querySelector('.detail-toc-scrim');
  const sections=[...document.querySelectorAll('.detail-section[id]')];
  if(!layout||!toc||!sections.length)return;
  const setOpen=open=>{
    layout.classList.toggle('toc-open',Boolean(open));
    toggle?.setAttribute('aria-expanded',open?'true':'false');
  };
  toggle?.setAttribute('aria-expanded','false');
  toggle?.addEventListener('click',()=>setOpen(!layout.classList.contains('toc-open')));
  close?.addEventListener('click',()=>setOpen(false));
  scrim?.addEventListener('click',()=>setOpen(false));
  toc.innerHTML=sections.map((section,i)=>`<a href="#${esc(section.id)}" data-toc-target="${esc(section.id)}" class="${i===0?'active':''}">${esc(section.dataset.tocLabel||section.querySelector('h2')?.textContent||section.id)}</a>`).join('');
  toc.querySelectorAll('[data-toc-target]').forEach(link=>link.addEventListener('click',e=>{
    e.preventDefault();
    openDetailSection(link.dataset.tocTarget);
    if(window.matchMedia('(max-width:1280px)').matches)setOpen(false);
  }));
  const setActive=id=>toc.querySelectorAll('[data-toc-target]').forEach(link=>link.classList.toggle('active',link.dataset.tocTarget===id));
  if('IntersectionObserver' in window){
    const observer=new IntersectionObserver(entries=>{
      const visible=entries.filter(x=>x.isIntersecting).sort((a,b)=>a.boundingClientRect.top-b.boundingClientRect.top);
      if(visible[0])setActive(visible[0].target.id);
    },{rootMargin:'-18% 0px -68% 0px',threshold:[0,0.01]});
    sections.forEach(section=>observer.observe(section));
  }
  const hash=decodeURIComponent(location.hash.replace(/^#/,''));
  if(hash&&document.getElementById(hash))requestAnimationFrame(()=>openDetailSection(hash,'auto'));
}
function bindDetailInteractions() {
  document.querySelectorAll('[data-detail-target]').forEach(link=>link.addEventListener('click',e=>{
    e.preventDefault();
    openDetailSection(link.dataset.detailTarget);
  }));
  document.querySelectorAll('[data-relation-id]').forEach(btn=>btn.addEventListener('click',e=>{
    e.preventDefault();
    e.stopPropagation();
    openTask(btn.dataset.relationId);
  }));
  document.querySelectorAll('[data-tag-filter]').forEach(btn=>btn.addEventListener('click',e=>{
    e.preventDefault(); e.stopPropagation();
    state.detail=null;
    if(!state.tagFilters.includes(btn.dataset.tagFilter))state.tagFilters=[...state.tagFilters,btn.dataset.tagFilter];
    state.view='backlog'; state.detailTask=null; state.listPage=1; state.selectedIndex=0;
    history.pushState({},'',backlogUrl()); refreshList();
  }));
}
function detailView(task) {
  const act=task.activity||{}, lc=task.lifecycle||{}, reason=task.attention_reason||null, warn=Boolean(reason)||['quiet','stale','worker_missing'].includes(act.health), events=lc.events||[];
  const hs=humanSummary(task);
  const timingInferred=Boolean(lc.lifecycle_inferred), timingIncomplete=Boolean(lc.timing_incomplete);
  const lifecyclePreview=events.length?`${lifecycleEventLabel(events[events.length-1].label)} · ${dateTimeLabel(events[events.length-1].at,true)}`:t('noLifecycle');
  const lifecycleBody=`${timingInferred?`<div class="timing-note"><strong>${esc(t('provisionalTiming'))}</strong><span>${esc(t('provisionalTimingDetail'))}</span></div>`:''}${timingIncomplete?`<div class="timing-note warning"><strong>${esc(t('incompleteHistory'))}</strong><span>${esc(t('incompleteHistoryDetail'))}</span></div>`:''}${events.length?`<div class="timeline">${events.map(e=>`<div class="timeline-event"><span class="timeline-dot"></span><span class="timeline-time">${esc(new Date(e.at).toLocaleString(localeCode()))}</span><div class="timeline-label"><strong>${esc(lifecycleEventLabel(e.label))}${e.provisional?` · ${esc(t('provisional'))}`:''}</strong><small>${e.interval&&e.interval!=='-'?`${esc(t('stayed'))} ${esc(e.interval)}`:''}${e.source&&e.source!=='git'?` · ${esc(e.source)}`:''}</small></div></div>`).join('')}</div>`:`<p class="summary">${esc(t('noLifecycle'))}</p>`}`;
  const lifecycle=detailDisclosure('lifecycle',t('lifecycle'),lifecyclePreview,lifecycleBody);
  const passiveAlert=!reason&&warn?`<div class="attention-banner ${['stale','worker_missing'].includes(act.health)?'danger':''}"><strong>${esc(healthLabel(act.health))}</strong><span>${esc(t('lastObservable',{ago:ago(act.last_activity_at),source:act.last_activity_source}))} ${esc(act.health==='worker_missing'?t('workerMissingDetail'):t('quietAdvisory'))}</span></div>`:'';
  const sectionSummaries=task.document?.section_summaries||{};
  const progressBody=`<h3>${esc(t('workNotes'))}</h3><div class="markdown">${markdown(task.document?.notes||task.fields?.메모)}</div><h3>${esc(t('result'))}</h3><div class="markdown">${markdown(task.document?.result||task.fields?.결과)}</div>`;
  const progress=semanticSection('progress-result',t('progressResult'),sectionSummaries.progress_result,progressBody);
  const verificationValue=task.document?.verification||task.fields?.검증;
  const verificationBody=`<div class="markdown">${markdown(verificationValue)}</div>`;
  const verification=semanticSection('verification',t('verificationDetail'),sectionSummaries.verification,verificationBody);
  const relatedBody=`<div class="relation-groups"><div><h3>${esc(t('dependsOn'))}</h3><div class="relation-list">${relationBadges(task.depends_on,t('dependsOn'))}</div></div><div><h3>${esc(t('related'))}</h3><div class="relation-list">${relationBadges(task.related,t('related'))}</div></div></div>`;
  const related=detailDisclosure('related-work',t('relatedWork'),[...(task.depends_on||[]),...(task.related||[])].join(', '),relatedBody);
  const metricGrid=`<div class="detail-metrics embedded"><div><div class="value live-active">${fmtSec(runningSeconds(task,'active'))}</div><div class="label">${esc(t('activeTime'))}</div></div><div><div class="value live-wait">${fmtSec(runningSeconds(task,'wait'))}</div><div class="label">${esc(t('waitTime'))}</div></div><div><div class="value live-queue">${fmtSec(runningSeconds(task,'queue'))}</div><div class="label">${esc(t('queueTime'))}</div></div><div><div class="value live-lead">${fmtSec(runningSeconds(task,'lead'))}</div><div class="label">${esc(t('leadTime'))}</div></div></div>`;
  const operationsBody=`${metricGrid}<h3>${esc(t('overview'))}</h3><div class="meta-grid">${metaRow(t('registrant'),task.registrant)}${metaRow(t('agent'),task.agent)}${metaRow(t('changeScope'),task.scope)}${metaRow(t('location'),task.archive_month?`archive/${task.archive_month}`:task.location)}${metaRow(t('updated'),dateTimeLabel(updatedAt(task)))}${metaRow(t('completed'),dateTimeLabel(completionAt(task)))}${metaRow(t('activity'),healthLabel(act.health))}${metaRow(t('lastSignal'),act.last_activity_at?ago(act.last_activity_at):'-')}${metaRow(t('signalSource'),act.last_activity_source)}</div><h3>${esc(t('execution'))}</h3><div class="meta-grid">${metaRow(t('runtimeProvider'),task.fields?.RuntimeProvider||'unknown')}${metaRow(t('dispatchStatus'),task.fields?.Dispatch상태||task.fields?.실행상태||'unknown')}${metaRow(t('executionEvidence'),task.fields?.실행근거||'unknown')}${metaRow(t('fallbackEvidence'),task.fields?.Fallback근거||'-')}</div>`;
  const operations=detailDisclosure('operations',t('operationsEvidence'),`${task.agent||'-'} · ${healthLabel(act.health)}`,operationsBody);
  const rawToggle=`<button id="rawToggle" class="raw-toggle ${state.raw?'active':''}">${esc(state.raw?t('rendered'):t('rawMarkdown'))}</button>`;
  const detailHead=`<div class="detail-head"><div class="detail-head-top"><div class="detail-id">${esc(task.id)}</div>${rawToggle}</div><h1>${esc(titleOf(task))}</h1><div class="detail-status"><span class="status ${esc(task.state)}">${esc(stateLabel(task.state))}</span>${task.agent?`<span class="badge">${esc(task.agent)}</span>`:''}<span class="badge">${esc(task.document?.schema||'legacy')}</span>${task.archive_month?`<span class="badge">archive/${esc(task.archive_month)}</span>`:''}</div>${task.source?.label?`<div class="detail-source-row"><span>${esc(t('source'))}</span>${sourceLink(task.source)}</div>`:''}${(task.tags||[]).length?`<div class="detail-tag-row">${(task.tags||[]).map(tag=>tagChip(tag,true)).join('')}</div>`:''}</div>`;
  const rawPanel=`<section class="section raw-markdown-view"><h2>${esc(t('rawMarkdown'))}</h2><pre class="raw">${esc(task.raw_markdown||'')}</pre></section>`;
  const rendered=`${humanSummaryCard(task)}${related}${operations}${lifecycle}${passiveAlert}${contractSections(task)}${progress}${verification}`;
  const body=`<div class="detail"><button class="back" id="backBtn">${esc(t('backToBacklog'))}</button>${detailHead}${state.raw?rawPanel:rendered}</div>`;
  return `<div class="detail-layout ${state.raw?'raw-mode':''}">${body}${state.raw?'':detailToc()}</div>`;
}
async function loadTaskDetail(id) {
  if(!id||!state.project)return;
  const targetProject=state.project, targetID=id;
  const params=new URLSearchParams();
  params.set('project',targetProject);
  if(state.backlog)params.set('backlog',state.backlog);
  try {
    const r=await fetch(`/api/tasks/${encodeURIComponent(targetID)}?${params.toString()}`,{cache:'no-store'});
    if(!r.ok){
      let detail=''; try { const body=await r.json(); detail=body.error||''; } catch(_) {}
      throw new Error(detail||`HTTP ${r.status}`);
    }
    const task=await r.json();
    if(state.project!==targetProject||state.detail!==targetID)return;
    state.detailTask=task;
    state.loadError='';
    render();
  } catch(e) {
    if(state.project!==targetProject||state.detail!==targetID)return;
    state.detailTask=null;
    state.loadError=String(e?.message||e||'Unknown error');
    render();
  }
}
function openTask(id) {
  if (!id) return;
  state.detail=id; state.detailTask=null; state.loadError=''; state.raw=false; state.view='backlog';
  const p=new URLSearchParams(); if(state.project)p.set('project',state.project); if(state.tagFilters.length)p.set('tags',state.tagFilters.join(',')); history.pushState({},'',`/tasks/${encodeURIComponent(id)}${p.toString()?`?${p.toString()}`:''}`); render(); loadTaskDetail(id);
}
function closeTask() {
  state.detail=null; state.detailTask=null; state.loadError=''; history.pushState({},'',state.view==='backlog'?backlogUrl():`/?view=${state.view}`); render();
}
function bindRows() {
  document.querySelectorAll('[data-id]').forEach(el => el.onclick = e => {
    if(e.target?.closest?.('[data-external-source]'))return;
    if (el.dataset.rowIndex != null) state.selectedIndex = Number(el.dataset.rowIndex) || 0;
    openTask(el.dataset.id);
  });
  document.querySelectorAll('[data-status-filter]').forEach(b=>b.addEventListener('click',()=>setStatusFilter(b.dataset.statusFilter)));
  document.querySelectorAll('[data-tag-filter]').forEach(b=>b.addEventListener('click',e=>{e.stopPropagation();setTagFilter(b.dataset.tagFilter)}));
  $('#tagExploreBtn')?.addEventListener('click',()=>{state.tagExplorerOpen=!state.tagExplorerOpen;render()});
  $('#tagClearBtn')?.addEventListener('click',clearTagFilters);
  $('#listSort')?.addEventListener('change', e => { state.listSort=e.target.value; state.listPage=1; state.selectedIndex=0; localStorage.setItem('task-mecca-list-sort-v2',state.listSort); refreshList(); });
  $('#listPageSize')?.addEventListener('change', e => {
    const value=e.target.value;
    state.listPage=1; state.selectedIndex=0;
    if(value==='auto'){
      state.listPageMode='auto';
      localStorage.setItem('task-mecca-list-page-mode-v1','auto');
    } else {
      state.listPageMode='manual';
      state.listPageSize=Number(value)||20;
      localStorage.setItem('task-mecca-list-page-mode-v1','manual');
      localStorage.setItem('task-mecca-list-page-size',String(state.listPageSize));
    }
    refreshList();
  });
  $('#prevPage')?.addEventListener('click', () => changeListPage(-1));
  $('#nextPage')?.addEventListener('click', () => changeListPage(1));
}
function changeListPage(delta) {
  if (state.view !== 'backlog') return;
  const info=pageInfo(), next=Math.min(info.pages,Math.max(1,state.listPage+delta));
  if (next===state.listPage) return;
  state.listPage=next; state.selectedIndex=0; refreshList(); window.scrollTo({top:0,behavior:'smooth'});
}
function highlightSelection(index) {
  const rows=[...document.querySelectorAll('.task-row[data-row-index]')];
  if (!rows.length) return;
  state.selectedIndex=Math.max(0,Math.min(index,rows.length-1));
  rows.forEach((el,i)=>el.classList.toggle('keyboard-selected',i===state.selectedIndex));
  rows[state.selectedIndex]?.scrollIntoView({block:'nearest'});
}
function selectedRowId() {
  const rows=[...document.querySelectorAll('.task-row[data-row-index]')];
  return rows[state.selectedIndex]?.dataset.id || rows[0]?.dataset.id || '';
}

let autoPageMeasureRaf = 0;
function measuredTaskRowHeight() {
  const rows=[...document.querySelectorAll('.task-list .task-row')].slice(0,5);
  const heights=rows.map(r=>r.getBoundingClientRect().height).filter(h=>Number.isFinite(h)&&h>20).sort((a,b)=>a-b);
  if(!heights.length)return 66;
  return heights[Math.floor(heights.length/2)];
}
function updateAutoListPageSize() {
  if(state.listPageMode!=='auto'||state.view!=='backlog'||state.detail)return;
  const list=document.querySelector('.task-list');
  if(!list)return;
  const shortcut=document.querySelector('.shortcut-bar');
  const shortcutHeight=shortcut?.getBoundingClientRect().height||28;
  const listTopAtPageTop=list.getBoundingClientRect().top+window.scrollY;
  const available=Math.max(0,window.innerHeight-listTopAtPageTop-shortcutHeight-12);
  const rowHeight=measuredTaskRowHeight();
  const next=Math.max(1,Math.min(50,Math.floor(available/Math.max(1,rowHeight))));
  if(next===state.autoListPageSize)return;
  const oldSize=Math.max(1,state.autoListPageSize||1);
  const firstIndex=(Math.max(1,state.listPage)-1)*oldSize;
  state.autoListPageSize=next;
  state.listPage=Math.floor(firstIndex/next)+1;
  state.selectedIndex=0;
  if(Date.now()-state.lastFetch<1500 && !state.pendingContentUpdate)refreshList();
}
function scheduleAutoListPageSize() {
  if(state.listPageMode!=='auto')return;
  cancelAnimationFrame(autoPageMeasureRaf);
  autoPageMeasureRaf=requestAnimationFrame(()=>requestAnimationFrame(updateAutoListPageSize));
}

function applySidebarState() {
  const shell=document.querySelector('.app-shell');
  shell?.classList.toggle('sidebar-collapsed', state.sidebarCollapsed);
  shell?.classList.toggle('sidebar-peek', state.sidebarCollapsed && state.sidebarPeek);
  const btn = $('#sidebarToggle');
  if (!btn) return;
  btn.textContent = state.sidebarCollapsed ? '›' : '‹';
  btn.setAttribute('aria-label', state.sidebarCollapsed ? t('expandSidebar') : t('collapseSidebar'));
  btn.title = state.sidebarCollapsed ? t('expandSidebar') : t('collapseSidebar');
}
function toggleSidebar() {
  state.sidebarCollapsed = !state.sidebarCollapsed;
  state.sidebarPeek = false;
  localStorage.setItem('task-mecca-sidebar-collapsed', state.sidebarCollapsed ? '1' : '0');
  applySidebarState();
  scheduleAutoListPageSize();
}

function render() {
  nav(); translateChrome(); renderAccess(); renderBacklogPicker(); applySidebarState(); updateNotificationIndicator(); renderGlobalUpdateIndicator(); renderContentUpdatePrompt(); renderReleaseUnreadPrompt();
  const c=$('#content'), data=currentProjectData();
  if(state.view==='release-notes'){
    c.innerHTML=releaseNotesView();
    bindReleaseNotesActions();
    renderReleaseNoteModal();
    return;
  }
  const manualReady=state.view==='manual';
  const snapshotView=['workload','attention','issues'].includes(state.view);
  const viewDataReady=manualReady || (snapshotView ? Boolean(state.snapshot) : Boolean(data));
  if (!viewDataReady && !state.detailTask) {
    if (state.view === 'hub' && state.hub) {
      c.innerHTML=hubView(); bindHubActions(); return;
    }
    if (state.loadError) {
      c.innerHTML=`<div class="load-error"><h2>Project dashboard could not be loaded</h2><p><strong>Project</strong> ${esc(state.project||'-')}</p><p>${esc(state.loadError)}</p><div class="project-actions"><button class="action-btn secondary" id="retryProjectBtn">Retry</button><button class="action-btn" id="backToHubBtn">Back to Projects</button></div></div>`;
      $('#retryProjectBtn')?.addEventListener('click',()=>state.detail?loadTaskDetail(state.detail):refresh());
      $('#backToHubBtn')?.addEventListener('click',()=>{state.project='';state.backlog='';state.listData=null;state.snapshot=null;state.view='hub';history.pushState({},'','/?view=hub');render();refresh();});
      return;
    }
    c.innerHTML=`<div class="loading">${esc(t('loading'))}</div>`; return;
  }
  const gate=accessBanner()+diagnosticBanner();
  if (state.detail) {
    const task=state.detailTask;
    if(!task){
      if(state.loadError){
        c.innerHTML=gate+`<div class="load-error"><h2>${esc(t('taskNotFound'))}</h2><p>${esc(state.loadError)}</p><div class="project-actions"><button class="action-btn secondary" id="detailBackBtn">${esc(t('backToBacklog'))}</button><button class="action-btn" id="detailRetryBtn">Retry</button></div></div>`;
        $('#detailBackBtn')?.addEventListener('click',closeTask);
        $('#detailRetryBtn')?.addEventListener('click',()=>{state.loadError='';render();loadTaskDetail(state.detail)});
      } else c.innerHTML=gate+`<div class="loading">${esc(t('loading'))}</div>`;
      return;
    }
    c.innerHTML=gate+detailView(task);
    $('#backBtn')?.addEventListener('click',closeTask);
    $('#rawToggle')?.addEventListener('click',()=>{
      const scrollY=window.scrollY;
      state.raw=!state.raw;
      render();
      requestAnimationFrame(()=>window.scrollTo({top:scrollY,left:0,behavior:'auto'}));
    });
    bindCopyButtons(); bindMermaidControls(); bindDetailToc(); bindDetailInteractions(); renderMermaidDiagrams();
    return;
  }
  c.innerHTML=(state.view==='hub'?hubView():gate+(state.view==='manual'?manualView():state.view==='workload'?workloadView():state.view==='attention'?attentionView():state.view==='issues'?issuesView():listView()));
  bindRows(); if(state.view==='hub') bindHubActions();
  if(state.view==='workload'){
    document.querySelectorAll('[data-runtime-hook-action]').forEach(button=>{
      button.addEventListener('click',e=>performRuntimeHookAction(e.currentTarget));
    });
    $('#runtimeHistoryToggle')?.addEventListener('click',()=>toggleRuntimeHistory());
    $('#runtimeStorageToggle')?.addEventListener('click',()=>toggleRuntimeStorage());
    $('#runtimeCleanupBtn')?.addEventListener('click',e=>performRuntimeCleanup(e.currentTarget));
    document.querySelectorAll('[data-runtime-history-page]').forEach(button=>{
      button.addEventListener('click',()=>loadRuntimeHistory(Number(button.dataset.runtimeHistoryPage||1)));
    });
    document.querySelectorAll('.runtime-attempt-disclosure[data-runtime-attempt-key]').forEach(details=>{
      details.addEventListener('toggle',()=>{
        state.runtimeAttemptDisclosure[details.dataset.runtimeAttemptKey]=details.open;
      });
    });
    document.querySelectorAll('.runtime-history[data-runtime-history-key]').forEach(details=>{
      details.addEventListener('toggle',()=>{
        state.runtimeTransitionDisclosure[details.dataset.runtimeHistoryKey]=details.open;
      });
    });
  }
  if(state.view==='backlog')scheduleAutoListPageSize();
  document.querySelectorAll('[data-manual-tab]').forEach(b=>b.onclick=()=>{state.manualTab=b.dataset.manualTab;render()});
  bindCopyButtons(); bindMermaidControls(); renderMermaidDiagrams();
}

async function loadManual(language = state.language, force = false) {
  if (!force && state.manualByLanguage[language]) return;
  try {
    const r=await fetch(`/api/manual?lang=${encodeURIComponent(language)}`,{cache:'no-store'});
    if(!r.ok)throw new Error(r.status);
    state.manualByLanguage[language]=await r.json();
  } catch(e) {
    state.manualByLanguage[language]={readme:t('manualUnavailable'),session_guide:t('manualUnavailable'),root_prompt:''};
  }
}
let refreshInFlight=null;
let refreshQueued=false;
let hubFetchInFlight=null;
let listRefreshInFlight=null;
let listRefreshQueued=false;

function listNotificationPayload(data) {
  const current={...(data?.attention_items||{})};
  (data?.items||[]).forEach(task=>{current[task.id]=task});
  return {all_items:current,notification_events:data?.notification_events||[],attention:data?.attention||[]};
}

async function refreshHub(force=false) {
  const fresh=state.hub && Date.now()-state.lastHubFetch<60000;
  if(!force && fresh)return state.hub;
  if(hubFetchInFlight)return hubFetchInFlight;
  hubFetchInFlight=(async()=>{
    try {
      const r=await fetch('/api/hub',{cache:'no-store'});
      if(!r.ok)throw new Error(`HTTP ${r.status}`);
      state.hub=await r.json();
      state.lastHubFetch=Date.now();
      return state.hub;
    } finally {
      hubFetchInFlight=null;
    }
  })();
  return hubFetchInFlight;
}

function listQueryString() {
  const params=new URLSearchParams();
  if(state.backlog)params.set('backlog',state.backlog);
  if(state.project)params.set('project',state.project);
  params.set('page',String(Math.max(1,state.listPage||1)));
  params.set('page_size',String(effectiveListPageSize()));
  if(!state.statusFilters.includes('all'))params.set('status',state.statusFilters.join(','));
  if(state.tagFilters.length)params.set('tags',state.tagFilters.join(','));
  if(state.query.trim())params.set('q',state.query.trim());
  params.set('sort',state.listSort||'id_desc');
  return params.toString();
}

async function refreshList() {
  if(!state.project)return;
  if(listRefreshInFlight){
    listRefreshQueued=true;
    return listRefreshInFlight;
  }
  const targetProject=state.project;
  const targetBacklog=state.backlog;
  listRefreshInFlight=(async()=>{
    try {
      const r=await fetch('/api/backlog/tasks?'+listQueryString(),{cache:'no-store'});
      if(!r.ok){
        let detail=''; try { const body=await r.json(); detail=body.error||''; } catch(_) {}
        throw new Error(detail||`HTTP ${r.status}`);
      }
      const data=await r.json();
      if(targetProject!==state.project||targetBacklog!==state.backlog)return;
      state.listData=data;
      state.loadError='';
      state.lastFetch=Date.now();
      state.listPage=Math.max(1,Number(data.page)||1);
      acceptContentRevision(data.revision||state.contentRevision);
      processTaskNotifications(listNotificationPayload(data));
      const candidates=data?.backlog_selection?.candidates||[];
      if(state.backlog && !candidates.some(x=>x.path===state.backlog)){
        state.backlog='';
        localStorage.removeItem('task-mecca-backlog-folder');
      }
      $('#connectionDot').style.background='var(--ok)';
      ensureAttentionStream();
      if(state.view==='backlog')render();
    } catch(e) {
      if(targetProject!==state.project)return;
      state.loadError=String(e?.message||e||'Unknown error');
      $('#connectionDot').style.background='var(--danger)';
      if(!state.listData)render();
    } finally {
      listRefreshInFlight=null;
      if(listRefreshQueued){
        listRefreshQueued=false;
        queueMicrotask(()=>refreshList());
      }
    }
  })();
  return listRefreshInFlight;
}

function closeAttentionStream() {
  if(state.eventSource){ try{state.eventSource.close()}catch(_){} }
  state.eventSource=null;
  state.eventStreamKey='';
}

function attentionPayloadRevision(payload) {
  const parts=[];
  (payload?.attention||[]).forEach(row=>parts.push(['a',row.id,row.type,row.health,row.runtime_state,row.last_activity_at,row.title,row.message,row.resume_condition].map(x=>String(x??'')).join('|')));
  (payload?.notification_events||[]).forEach(event=>parts.push('e|'+String(event?.id||'')));
  parts.sort();
  return parts.join('\n');
}

function ensureAttentionStream() {
  if(!state.project||typeof EventSource==='undefined')return;
  const params=new URLSearchParams();
  params.set('project',state.project);
  if(state.backlog)params.set('backlog',state.backlog);
  const key=state.project+'|'+state.backlog;
  if(state.eventSource&&state.eventStreamKey===key&&state.eventSource.readyState!==EventSource.CLOSED)return;
  closeAttentionStream();
  if(state.attentionRevisionKey!==key){
    state.attentionRevisionKey=key;
    state.attentionRevision='';
  }
  const source=new EventSource('/api/events?'+params.toString());
  state.eventSource=source;
  state.eventStreamKey=key;
  source.addEventListener('attention',event=>{
    if(state.eventStreamKey!==key)return;
    let payload=null;
    try{payload=JSON.parse(event.data)}catch(_){return}
    processTaskNotifications(payload);
    const nextRevision=attentionPayloadRevision(payload);
    const changed=Boolean(state.attentionRevision && nextRevision!==state.attentionRevision);
    state.attentionRevision=nextRevision;
    const count=(payload.attention||[]).length;
    const badge=$('#attentionCount');
    if(badge)badge.textContent=count||'';
    if(changed){
      if(state.view==='backlog')markContentUpdate('runtime');
      else queueMicrotask(()=>refresh());
    }
  });
}

async function refreshOnce() {
  const targetProject=state.project;
  const targetView=state.view;
  const params=new URLSearchParams();
  if(state.backlog)params.set('backlog',state.backlog);
  if(targetProject)params.set('project',targetProject);
  const qs=params.toString()?`?${params}`:'';

  if(targetView==='release-notes'){
    closeAttentionStream();
    await loadReleaseNotes(!state.releaseNotesLoaded);
    if(state.view==='release-notes')render();
    return;
  }

  if(targetView==='hub'){
    closeAttentionStream();
    try {
      await refreshHub(true);
      if(state.view!=='hub')return;
      state.snapshot=null;
      state.listData=null;
      state.loadError='';
      $('#connectionDot').style.background='var(--ok)';
      render();
    } catch(e) {
      if(state.view!=='hub')return;
      state.snapshot=null;
      state.listData=null;
      state.loadError=String(e?.message||e||'Unknown error');
      $('#connectionDot').style.background='var(--danger)';
      render();
    }
    return;
  }

  if(targetView==='backlog'){
    refreshHub(false).catch(()=>{});
    await refreshList();
    if(state.detail)await loadTaskDetail(state.detail);
    await checkContentRevision(true);
    return;
  }

  refreshHub(false).catch(()=>{});

  if(targetView==='manual'){
    await loadManual(state.language);
    if(targetProject!==state.project || state.view!==targetView)return;
    state.loadError='';
    $('#connectionDot').style.background='var(--ok)';
    ensureAttentionStream();
    render();
    return;
  }

  const viewEndpoint={
    workload:'/api/workload',
    attention:'/api/attention',
    issues:'/api/issues',
  }[targetView];

  if(!viewEndpoint){
    render();
    return;
  }

  try {
    const r=await fetch(viewEndpoint+qs,{cache:'no-store'});
    if(!r.ok){
      let detail='';
      try { const body=await r.json(); detail=body.error||''; } catch(_) {}
      throw new Error(detail||`HTTP ${r.status}`);
    }
    const snapshot=await r.json();
    if(targetProject!==state.project || state.view!==targetView)return;
    state.snapshot=snapshot;
    if(targetView==='attention')processTaskNotifications(snapshot);
    state.loadError='';
    state.lastFetch=Date.now();
    const candidates=snapshot?.backlog_selection?.candidates||[];
    if(state.backlog && candidates.length && !candidates.some(c=>c.path===state.backlog)){
      state.backlog='';
      localStorage.removeItem('task-mecca-backlog-folder');
    }
    $('#connectionDot').style.background='var(--ok)';
    ensureAttentionStream();
    render();
  } catch(e) {
    if(targetProject!==state.project || state.view!==targetView)return;
    state.snapshot=null;
    state.loadError=String(e?.message||e||'Unknown error');
    $('#connectionDot').style.background='var(--danger)';
    $('#snapshotAge').textContent=t('disconnected');
    render();
  }
}

async function refresh() {
  if(refreshInFlight){
    refreshQueued=true;
    return refreshInFlight;
  }
  refreshInFlight=refreshOnce();
  try {
    await refreshInFlight;
  } finally {
    refreshInFlight=null;
    if(refreshQueued){
      refreshQueued=false;
      queueMicrotask(()=>refresh());
    }
  }
}
function route(fromPop=false) {
  const previousDetail=state.detail;
  const previousProject=state.project;
  const m=location.pathname.match(/^\/tasks\/([^/]+)/);
  state.detail=m?decodeURIComponent(m[1]).toUpperCase():null;
  if(previousDetail!==state.detail)state.detailTask=null;
  state.loadError='';
  const p=new URLSearchParams(location.search);
  state.project=p.get('project')||'';
  if(previousProject!==state.project){
    state.contentRevision='';
    state.pendingContentUpdate=false;
    state.pendingContentReason='';
    state.runtimeHistoryOpen=false;
    state.runtimeHistoryLoading=false;
    state.runtimeHistoryError='';
    state.runtimeHistory={items:[],page:1,page_size:20,total:0,total_pages:0,retention:{}};
    state.runtimeAttemptDisclosure={};
    state.runtimeTransitionDisclosure={};
    state.runtimeStorageOpen=false;
    state.runtimeStorageLoading=false;
    state.runtimeStorageError='';
    state.runtimeStorage=null;
    if(state.project)queueMicrotask(()=>refreshVersionInfo(false));
  }
  if(state.project){
    state.lastProject=state.project;
    localStorage.setItem('task-mecca-last-project',state.project);
    ensureOpenProject(state.project);
  }
  if (!state.detail) {
    state.view=p.get('view')||(state.project?'backlog':'hub');
    if(state.view==='hub'||state.view==='release-notes') state.project='';
    if (state.view === 'backlog') {
      const legacy=p.get('state');
      const raw=p.get('filter');
      const allowed=['ready','doing','hold','blocked','done'];
      if (legacy && legacy !== 'all') state.statusFilters=allowed.includes(legacy)?[legacy]:['all'];
      else if (raw) {
        const picked=raw.split(',').map(x=>x.trim()).filter(x=>allowed.includes(x));
        state.statusFilters=picked.length?picked:['all'];
      } else state.statusFilters=['all'];
      const rawTags=p.get('tags')||'';
      state.tagFilters=rawTags.split(',').map(x=>x.trim()).filter(Boolean);
    }
  } else {
    state.view='backlog';
    const rawTags=p.get('tags')||'';
    state.tagFilters=rawTags.split(',').map(x=>x.trim()).filter(Boolean);
  }
  render();
  if(fromPop&&state.project&&state.view==='backlog'){
    queueMicrotask(()=>{
      refreshList();
      ensureAttentionStream();
      if(state.detail)loadTaskDetail(state.detail);
    });
  }
}
function renderLanguagePicker() {
  const picker=$('#languagePicker');
  if (!picker) return;
  picker.innerHTML=Object.entries(LANGUAGES).map(([code,meta])=>`<option value="${esc(code)}" ${state.language===code?'selected':''}>${esc(meta.label)}</option>`).join('');
  const label=$('#languageLabel'); if(label) label.textContent=t('language');
}
function translateChrome() {
  document.documentElement.lang=state.language;
  const search=$('#search'); if(search) search.placeholder=t('searchPlaceholder');
  const backlogLabel=$('#backlogLabel'); if(backlogLabel) backlogLabel.textContent=t('backlogFolder');
  const backlogPicker=$('#backlogPicker'); if(backlogPicker){backlogPicker.setAttribute('aria-label',t('selectBacklog')); backlogPicker.closest('.backlog-picker')?.setAttribute('title',t('selectBacklog'));}
  const languagePicker=$('#languagePicker'); if(languagePicker) languagePicker.setAttribute('aria-label',t('language'));
  const brandSub=document.querySelector('.brand-copy small'); if(brandSub) brandSub.textContent=t('observatory');
  const ops=$('#operationsLabel'); if(ops) ops.textContent=t('operations').toUpperCase();
  const help=$('#helpLabel'); if(help) help.textContent=t('help').toUpperCase();
  const pairs=[['#workloadText','workload'],['#attentionText','attention'],['#issuesText','issues'],['#manualText','manual'],['#releaseNotesText','releaseNotes']];
  pairs.forEach(([sel,key])=>{const el=$(sel);if(el)el.textContent=t(key)});
  [['[data-view="workload"]','workload'],['[data-view="attention"]','attention'],['[data-view="issues"]','issues'],['[data-view="manual"]','manual'],['[data-view="release-notes"]','releaseNotes']].forEach(([sel,key])=>{const el=document.querySelector(sel);if(el)el.title=t(key)});
  const sideBtn=$('#sidebarToggle');if(sideBtn){sideBtn.setAttribute('aria-label',state.sidebarCollapsed?t('expandSidebar'):t('collapseSidebar'));sideBtn.title=state.sidebarCollapsed?t('expandSidebar'):t('collapseSidebar')}
  const refresh=$('#refreshBtn'); if(refresh) refresh.title=t('refresh');
  const themeGroup=document.querySelector('.theme-switcher'); if(themeGroup){themeGroup.setAttribute('aria-label',t('theme'));themeGroup.title=t('theme');}
  document.querySelectorAll('[data-theme-choice]').forEach(btn=>{const key={system:'themeSystem',light:'themeLight',dark:'themeDark'}[btn.dataset.themeChoice];const titleKey={system:'followSystemTheme',light:'useLightTheme',dark:'useDarkTheme'}[btn.dataset.themeChoice];if(key)btn.textContent=t(key);if(titleKey)btn.title=t(titleKey)});
  const sb=$('#shortcutBar'); if(sb) sb.innerHTML=`<span><kbd>↑</kbd><kbd>↓</kbd> ${esc(t('move'))}</span><span><kbd>Enter</kbd>/<kbd>→</kbd> ${esc(t('open'))}</span><span><kbd>←</kbd>/<kbd>Esc</kbd> ${esc(t('back'))}</span><span><kbd>/</kbd> ${esc(t('search'))}</span><span><kbd>PgUp</kbd>/<kbd>PgDn</kbd> ${esc(t('page'))}</span>`;
  renderLanguagePicker();
}
async function setLanguage(value) {
  const next=LANGUAGES[value]?value:'en';
  if(next===state.language){ translateChrome(); return; }
  state.language=next;
  localStorage.setItem('task-mecca-language',next);
  await loadManual(next);
  translateChrome();
  render();
  renderReleaseNoteModal();
  renderReleaseUnreadPrompt();
  if(document.querySelector('.mermaid-wrap')) renderMermaidDiagrams(true);
}

function applyTheme(value) {
  state.theme=['system','light','dark'].includes(value)?value:'system';
  document.documentElement.dataset.theme=state.theme;
  localStorage.setItem('task-mecca-theme',state.theme);
  document.querySelectorAll('[data-theme-choice]').forEach(b=>{b.classList.toggle('active',b.dataset.themeChoice===state.theme);b.setAttribute('aria-pressed',b.dataset.themeChoice===state.theme?'true':'false')});
  if (document.querySelector('.mermaid-wrap')) renderMermaidDiagrams(true);
}
translateChrome();
applyTheme(state.theme);
document.querySelectorAll('[data-theme-choice]').forEach(b=>b.addEventListener('click',()=>applyTheme(b.dataset.themeChoice)));
window.matchMedia?.('(prefers-color-scheme: dark)').addEventListener?.('change',()=>{if(state.theme==='system')renderMermaidDiagrams(true)});
$('#languagePicker').addEventListener('change',e=>setLanguage(e.target.value));
$('#backlogPicker').addEventListener('change',e=>{
  const value=e.target.value;
  if(value===state.backlog)return;
  state.backlog=value;
  state.contentRevision='';
  state.pendingContentUpdate=false;
  state.pendingContentReason='';
  if(value)localStorage.setItem('task-mecca-backlog-folder',value);else localStorage.removeItem('task-mecca-backlog-folder');
  state.detail=null;state.listPage=1;state.selectedIndex=0;refresh();
});
let searchRefreshTimer=0;
$('#search').addEventListener('input',e=>{
  state.query=e.target.value; state.detail=null; state.detailTask=null; state.listPage=1; state.selectedIndex=0;
  clearTimeout(searchRefreshTimer);
  if(state.view==='backlog')searchRefreshTimer=setTimeout(refreshList,180); else render();
});
$('#refreshBtn').onclick=refreshVisibleContent;
$('#notificationBtn')?.addEventListener('click',()=>{const panel=$('#notificationPanel');panel?.classList.toggle('open');renderNotificationPanel();});
document.addEventListener('click',e=>{const panel=$('#notificationPanel');if(panel?.classList.contains('open')&&!panel.contains(e.target)&&!$('#notificationBtn')?.contains(e.target))panel.classList.remove('open')});
$('#sidebarToggle').onclick=toggleSidebar;
$('#sidebar')?.addEventListener('mouseenter',()=>{if(state.sidebarCollapsed){state.sidebarPeek=true;applySidebarState();}});
$('#sidebar')?.addEventListener('mouseleave',()=>{if(state.sidebarCollapsed){state.sidebarPeek=false;state.projectMenuOpen=false;applySidebarState();}});

document.addEventListener('keydown',e=>{
  if(e.key==='Escape'&&state.releaseNotePopup){e.preventDefault();dismissReleaseNotePopup();return}
  const tag=document.activeElement?.tagName?.toLowerCase();
  const editing=['input','textarea','select','button'].includes(tag)||document.activeElement?.isContentEditable;
  if(e.key==='/'&&document.activeElement!==$('#search')){e.preventDefault();$('#search').focus();return}
  if(e.key==='?'&&!editing&&!state.detail){e.preventDefault();navigateView('manual');return}
  if(state.detail){
    const detailLayout=document.querySelector('.detail-layout');
    if(e.key==='Escape'&&detailLayout?.classList.contains('toc-open')){
      e.preventDefault();
      detailLayout.classList.remove('toc-open');
      document.querySelector('.detail-toc-fab')?.setAttribute('aria-expanded','false');
      return;
    }
    if(e.key==='Escape'||e.key==='ArrowLeft'){e.preventDefault();closeTask()}
    return;
  }
  if(editing){if(e.key==='Escape')document.activeElement?.blur();return}
  if(['manual','workload','attention','issues','release-notes'].includes(state.view))return;
  if(e.key==='ArrowDown'){e.preventDefault();highlightSelection(state.selectedIndex+1)}
  else if(e.key==='ArrowUp'){e.preventDefault();highlightSelection(state.selectedIndex-1)}
  else if(e.key==='Home'){e.preventDefault();highlightSelection(0)}
  else if(e.key==='End'){e.preventDefault();highlightSelection(999999)}
  else if(e.key==='Enter'||e.key==='ArrowRight'){const id=selectedRowId();if(id){e.preventDefault();openTask(id)}}
  else if(e.key==='PageDown'&&state.view==='backlog'){e.preventDefault();changeListPage(1)}
  else if(e.key==='PageUp'&&state.view==='backlog'){e.preventDefault();changeListPage(-1)}
});
let autoPageResizeTimer=0;
window.addEventListener('resize',()=>{
  if(state.listPageMode!=='auto')return;
  clearTimeout(autoPageResizeTimer);
  autoPageResizeTimer=setTimeout(scheduleAutoListPageSize,100);
});
window.addEventListener('popstate',()=>route(true));
setInterval(()=>{
  const data=currentProjectData();
  if(data){
    $('#snapshotAge').textContent=`${t('updated')} ${ago(new Date(state.lastFetch).toISOString())}`;
    const byID={};
    (state.listData?.items||[]).forEach(task=>{byID[task.id]=task});
    Object.assign(byID,state.snapshot?.all_items||{});
    document.querySelectorAll('.live-timer').forEach(el=>{const task=byID[el.dataset.id];if(task)el.textContent=fmtSec(runningSeconds(task,'active'))});
    document.querySelectorAll('.runtime-elapsed').forEach(el=>{
      const ended=el.dataset.endedAt||'', started=el.dataset.startedAt||'';
      if(ended||!started){el.textContent=fmtSec(Number(el.dataset.elapsedMs||0)/1000);return}
      const ms=Date.now()-new Date(started).getTime();
      el.textContent=fmtSec(Number.isFinite(ms)?Math.max(0,ms/1000):Number(el.dataset.elapsedMs||0)/1000);
    });
    if(state.detail&&state.detailTask){
      const task=state.detailTask,a=document.querySelector('.live-active'),w=document.querySelector('.live-wait'),q=document.querySelector('.live-queue'),l=document.querySelector('.live-lead');
      if(a)a.textContent=fmtSec(runningSeconds(task,'active'));if(w)w.textContent=fmtSec(runningSeconds(task,'wait'));if(q)q.textContent=fmtSec(runningSeconds(task,'queue'));if(l)l.textContent=fmtSec(runningSeconds(task,'lead'));
    }
  }
},1000);
setInterval(()=>{
  if(state.project)checkContentRevision(false);
  else if(state.view==='hub')refreshHub(true).then(()=>{if(state.view==='hub')render()}).catch(()=>{});
},5000);
setInterval(()=>{
  if(state.project&&state.view==='workload'&&document.visibilityState!=='hidden'&&!refreshInFlight)refresh();
},3000);
setInterval(()=>refreshVersionInfo(true),300000);
if(window.isSecureContext&&'serviceWorker' in navigator)notificationWorker();
route();refresh();
refreshVersionInfo(false);
setTimeout(()=>refreshVersionInfo(true),800);
