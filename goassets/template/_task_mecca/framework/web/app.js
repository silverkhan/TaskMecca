const state = {
 userAttention:[],userAttentionContext:'',userAttentionObservedAt:0,
 attentionScopes:{},attentionScopeObserved:{},commonUserAttention:{},commonUserRevision:{},commonAttentionOpen:false,sessionWarnings:[],operationsRequest:0,operationPayload:{},
  notificationCenterTab:'history',notificationHistoryPage:1,notificationHistoryNew:0,notificationHistoryVersions:{},notificationHistoryLoaded:false,notificationHistory:[],notificationHistoryRequest:0,notificationHistoryLoading:false,notificationHistoryErrors:[],diagnosticSnapshots:{},diagnosticRequests:{},
  snapshot: null,
  listData: null,
  listRevalidating:false,listDataContext:'',
  detailTask: null,
  eventSource: null,
  eventStreamKey: '',
  view: 'hub',
  hub: null,
  logStorage: null, logStorageBusy:false, logStorageProject:'', logStorageError:'', logStorageMessage:'',
  hubManagement: {projects:[],archives:[]},
  hubHistoryExpanded: localStorage.getItem('task-mecca-hub-history-expanded-v1') === '1',
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
  theme: (()=>{ const preference=localStorage.getItem('task-mecca-theme'); return ['dark','light','system'].includes(preference)?preference:'dark'; })(),
  palette: (()=>{ const saved=localStorage.getItem('task-mecca-palette'); const palette=['mecca','slate'].includes(saved)?saved:'mecca'; if(localStorage.getItem('task-mecca-palette-version')!=='2')localStorage.setItem('task-mecca-palette-version','2'); return palette; })(),
  manual: null,
  manualTab: 'quick',
  backlog: localStorage.getItem('task-mecca-backlog-folder') || '',
  listPage: 1,
  listPageMode: localStorage.getItem('task-mecca-list-page-mode-v1') || 'auto',
  listPageSize: Number(localStorage.getItem('task-mecca-list-page-size') || localStorage.getItem('task-mecca-done-page-size') || 20),
  autoListPageSize: 8,
  listSort: localStorage.getItem('task-mecca-list-sort-v2') || 'id_desc',
  selectedIndex: 0,
  sidebarMode: ['auto','expanded','compact'].includes(localStorage.getItem('task-mecca-sidebar-mode')) ? localStorage.getItem('task-mecca-sidebar-mode') : (localStorage.getItem('task-mecca-sidebar-collapsed') === '0' ? 'expanded' : 'auto'),
  sidebarCollapsed: true,
  sidebarPeek: false,
  mobileNavOpen: false,
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
  runtimeRootDisclosure: {},
  runtimeRootListOpen: false,
  runtimeRootListLoading: false,
  runtimeRootListError: '',
  runtimeRootList: {items:[],page:1,page_size:10,total:0,total_pages:0,counts:{}},
  runtimeHookStatus: null,
  runtimeHookStatusLoading: false,
  projectNotificationSettings: [],
  projectNotificationSettingsLoading: false,
  projectNotificationSettingsError: '',
  operationRevision: '',
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
    backlog:'백로그', operations:'운영', help:'도움말', workload:'서브에이전트 워크로드', attention:'확인 필요', issues:'이슈', manual:'사용자 매뉴얼', searchPlaceholder:'ID, 제목, 요구사항, 수용 기준 검색…', backlogFolder:'백로그', language:'언어', refresh:'새로고침', refreshWithUpdates:'새로고침 및 업데이트 확인', refreshChecking:'데이터와 신규 업데이트 확인 중…', refreshUpToDate:'최신 버전입니다', refreshUpdateFound:'새 업데이트: {version}', refreshCheckFailed:'업데이트 확인 실패', refreshDataFailed:'화면 데이터 새로고침 실패', themeSystem:'시스템', themeLight:'라이트', themeDark:'다크', accessUnchecked:'권한 미확인', loading:'Task Mecca 불러오는 중…', disconnected:'연결 끊김', move:'이동', open:'열기', back:'뒤로', search:'검색', page:'페이지', status:'상태', all:'전체', ready:'등록됨', working:'작업 중', hold:'보류', blocked:'차단', done:'완료', todo:'할 일', active:'활성', quiet:'조용함', stale:'정체', workerMissing:'워커 없음', runtimeUnknown:'런타임 미확인', awaitingFinalize:'완료 처리 필요', needsUser:'사용자 개입 필요', stalled:'정체 확인 필요', notificationSettings:'알림 설정', telegramNotifications:'Telegram 알림', telegramBotToken:'Bot Token', telegramConnect:'봇 확인', telegramFindChat:'연결 확인', telegramTest:'테스트 알림', telegramDisconnect:'연결 해제', telegramGuide:'BotFather에서 봇을 만든 뒤 Token을 입력하세요. 봇 확인 후 Telegram에서 해당 봇에 /start를 보내고 연결 확인을 누르세요.', telegramConnected:'Telegram 연결됨', telegramConfigured:'봇 확인됨 · /start 후 연결 확인', telegramNotConfigured:'Telegram 미연결', notifyRegistered:'작업 등록', notifyStarted:'작업 착수', notifyApproval:'승인 필요', notifyInterrupted:'실행 중단/오류', notifyRuntimeUnknown:'실행 상태 확인 필요', notifyFinalize:'완료 처리 필요', notificationTestSent:'테스트 알림을 전송했습니다.', telegramAllOn:'전체 켜기', telegramAllOff:'전체 끄기', browserNotifications:'이 기기의 브라우저 알림', browserNotificationsGuide:'Task Mecca를 열어 둔 이 브라우저/기기의 시스템 알림으로 표시합니다. Telegram 알림과는 별도로 설정됩니다.', allowBrowserNotifications:'브라우저 알림 권한 허용', notifyIntervention:'사용자 개입 필요', notifyCompleted:'태스크 완료', notifyStalled:'작업 정체', notificationsBlocked:'브라우저에서 알림이 차단되어 있습니다.', notificationsOn:'알림 켜짐', notificationsOff:'알림 꺼짐', notificationsPermissionNeeded:'브라우저 알림을 켜야 실제 알림을 받을 수 있습니다.', notificationsDeniedGuide:'브라우저 주소창의 사이트 설정에서 알림을 허용한 뒤 페이지를 새로고침하세요.', notificationTypesDisabled:'알림 유형이 모두 꺼져 있습니다.', sort:'정렬', perPage:'페이지당', updatedNewest:'최근 업데이트순', updatedOldest:'오래된 업데이트순', autoRows:'자동 ({n})', pageSummary:'{page} / {pages} 페이지 · {total}개', autoRowsSummary:' · 자동 {n}행', allStatuses:'전체 상태', items:'개', needsAttention:'확인 필요', updated:'업데이트', agent:'에이전트', activeTime:'활성 시간', task:'작업', id:'ID', repository:'저장소', noMatches:'선택한 필터에 맞는 백로그가 없습니다.', backlogUninitializedSummary:'백로그 생성 대기', backlogUninitializedTitle:'아직 등록된 작업이 없습니다.', backlogUninitializedBody:'첫 작업을 등록하면 백로그가 자동으로 생성됩니다.', manualIntro:'Task Mecca를 처음 사용하는 순서와 운영 규칙을 UI 안에서 확인합니다.', dashboardLaunch:'대시보드 실행', keyboard:'키보드', quickStart:'빠른 시작', detailedGuide:'상세 운영 가이드', manualLoading:'매뉴얼을 불러오는 중…', manualUnavailable:'매뉴얼을 불러올 수 없습니다.', operationsEyebrow:'운영', diagnostics:'진단', noAttention:'현재 확인이 필요한 작업이 없습니다.', advisory:'사용자 개입, 완료 처리 필요, 런타임 정체 등 대응이 필요한 작업을 표시합니다.', workers:'워커', doing:'진행', downstreamBlocked:'하위 차단', readyContinuity:'연속 작업 후보', noWorkload:'할당된 서브에이전트 워크로드가 없습니다.', noCurrentDoing:'현재 진행 중인 작업이 없습니다.', unassignedDoing:'미할당 진행 작업', releasedHold:'해제된 보류 소유권', continuity:'연속', blocks:'차단', scope:'변경범위', workloadIntro:'백로그/Git 기반의 할당 현황입니다. 관측 가능한 경우 runtime health를 함께 표시합니다.', issuesIntro:'현재 스냅샷의 doctor 및 hold-review 진단 결과입니다.', noIssues:'진단 이슈가 없습니다.', taskDefinition:'작업 정의', simpleNote:'Simple Task · 별도 요건정의 확인이 필요하지 않습니다.', goal:'목표', acceptance:'수용 기준', requirements:'요건 정의서', definedNote:'Defined Task · 등록 전 요건 정의를 확인한 작업입니다.', background:'배경 및 문제', scopeIn:'포함', scopeOut:'제외', constraints:'제약 및 보존 조건', legacyTask:'Legacy 작업', legacyNote:'Legacy 백로그 · 없는 요건 데이터를 만들지 않고 기존 필드를 표시합니다.', description:'설명', tocTitle:'이 작업의 목차', tocOpen:'목차 열기', tocClose:'목차 닫기', lifecycle:'Lifecycle', noLifecycle:'아직 사용할 수 있는 lifecycle 전환 기록이 없습니다.', stayed:'유지', provisional:'임시', overview:'개요', registrant:'등록자', changeScope:'변경범위', dependsOn:'선행', related:'연관', location:'위치', completed:'완료', activity:'활동', lastSignal:'마지막 신호', signalSource:'신호 근거', execution:'실행 정보', runtimeProvider:'RuntimeProvider', dispatchStatus:'Dispatch 상태', executionEvidence:'실행 근거', fallbackEvidence:'Fallback 근거', workNotes:'작업 노트', result:'결과', verification:'검증', rawMarkdown:'Raw Markdown', rendered:'렌더링', waitTime:'대기 시간', queueTime:'큐 시간', leadTime:'리드 시간', provisionalTiming:'임시 lifecycle 시간', incompleteHistory:'불완전한 lifecycle 이력', lastObservable:'마지막 관측 활동 {ago} ({source}).', workerMissingDetail:'작업은 진행 중이지만 할당된 워커가 runtime registry에 없습니다.', quietAdvisory:'참고용 경고입니다. 장시간 작업은 정상적으로 조용할 수 있습니다.', backToBacklog:'← 백로그', taskNotFound:'작업을 찾을 수 없습니다.', restrictedNow:'현재 제한됨', lastFullAccess:'마지막 Full Access', fullAccess:'Full Access', lastRestricted:'마지막 검증 제한', accessLastChecked:'마지막 권한 확인', accessNotChecked:'권한 미확인', networkOff:'네트워크 꺼짐', notChecked:'확인 안 됨', freshDispatch:'dispatch 직전에 항상 새 active preflight를 실행합니다.', dispatchDisabled:'서브에이전트 dispatch 중지', enableFullAccess:'현재 런타임 제한이 감지되었습니다. dispatch 전에 Full Access를 활성화하세요.', auto:'자동', manualMode:'수동', noBacklog:'선택된 백로그 없음', copyCode:'코드 복사', copied:'복사됨', copyFailed:'복사 실패', mermaid:'Mermaid', showSource:'소스 보기', hideSource:'소스 숨기기', renderingDiagram:'다이어그램 렌더링 중…', mermaidUnavailable:'Mermaid renderer를 사용할 수 없습니다.', mermaidUnavailableDetail:'로컬 및 CDN Mermaid 런타임을 불러오지 못했습니다. Source에서 원문을 확인하거나 복사할 수 있습니다.', mermaidFailed:'Mermaid 렌더링 실패', invalidMermaid:'유효하지 않은 Mermaid 문법', secondsAgo:'{n}초 전', minutesAgo:'{n}분 전', hoursAgo:'{n}시간 전', daysAgo:'{n}일 전', justNow:'방금', updatedColumn:'마지막 업데이트', activeColumn:'활성', agentColumn:'에이전트', statusColumn:'상태', taskColumn:'작업', expandSidebar:'사이드바 펼치기', collapseSidebar:'사이드바 접기', eventRegistered:'등록', eventAssigned:'배정', eventStarted:'착수', eventWaiting:'사용자 대기', eventResumed:'재개', eventHold:'보류', eventCompleted:'완료', observatory:'백로그 관제', theme:'테마', followSystemTheme:'시스템 테마 따르기', useLightTheme:'라이트 테마 사용', useDarkTheme:'다크 테마 사용', selectBacklog:'백로그 폴더 선택', provisionalTimingDetail:'전환을 직접 확인한 기록이 없어 파일·런타임 관측 시각을 사용합니다. 실제 전환 시각과 다를 수 있습니다.', incompleteHistoryDetail:'과거 lifecycle의 착수 근거가 없어 일부 시간은 정확히 복원할 수 없습니다.'
  },
  en: {
    backlog:'Backlog', operations:'Operations', help:'Help', workload:'Subagent Workload', attention:'Needs Attention', issues:'Issues', manual:'User Manual', searchPlaceholder:'Search ID, title, requirements, acceptance criteria…', backlogFolder:'Backlog', language:'Language', refresh:'Refresh', refreshWithUpdates:'Refresh and check for updates', refreshChecking:'Refreshing and checking for updates…', refreshUpToDate:'Already up to date', refreshUpdateFound:'Update available: {version}', refreshCheckFailed:'Could not check for updates', refreshDataFailed:'Could not refresh page data', themeSystem:'System', themeLight:'Light', themeDark:'Dark', accessUnchecked:'Access unchecked', loading:'Loading Task Mecca…', disconnected:'Disconnected', move:'move', open:'open', back:'back', search:'search', page:'page', status:'Status', all:'All', ready:'Ready', working:'Working', hold:'Hold', blocked:'Blocked', done:'Done', todo:'Todo', active:'Active', quiet:'Quiet', stale:'Stale', workerMissing:'Worker missing', runtimeUnknown:'Runtime unknown', awaitingFinalize:'Needs finalization', needsUser:'User action required', stalled:'Stalled', notificationSettings:'Notification settings', telegramNotifications:'Telegram notifications', telegramBotToken:'Bot Token', telegramConnect:'Verify bot', telegramFindChat:'Confirm chat', telegramTest:'Test notification', telegramDisconnect:'Disconnect', telegramGuide:'Create a bot with BotFather, paste its token, then send /start to the bot in Telegram and confirm the chat here.', telegramConnected:'Telegram connected', telegramConfigured:'Bot verified · send /start then confirm chat', telegramNotConfigured:'Telegram not connected', notifyRegistered:'Task registered', notifyStarted:'Task started', notifyApproval:'Approval required', notifyInterrupted:'Execution interrupted/error', notifyRuntimeUnknown:'Runtime status unknown', notifyFinalize:'Finalize required', notificationTestSent:'Test notification sent.', telegramAllOn:'Enable all', telegramAllOff:'Disable all', browserNotifications:'Browser notifications on this device', browserNotificationsGuide:'Shows system notifications on this browser/device while using Task Mecca. Configure these separately from Telegram notifications.', allowBrowserNotifications:'Allow browser notification permission', notifyIntervention:'User action required', notifyCompleted:'Task completed', notifyStalled:'Task stalled', notificationsBlocked:'Notifications are blocked by the browser.', notificationsOn:'Notifications on', notificationsOff:'Notifications off', notificationsPermissionNeeded:'Enable browser notifications to receive actual alerts.', notificationsDeniedGuide:'Allow notifications in the browser site settings, then refresh this page.', notificationTypesDisabled:'All notification types are turned off.', sort:'Sort', perPage:'Per page', updatedNewest:'Updated newest', updatedOldest:'Updated oldest', autoRows:'Auto ({n})', pageSummary:'Page {page} / {pages} · {total} items', autoRowsSummary:' · Auto {n} rows', allStatuses:'All statuses', items:'items', needsAttention:'Needs attention', updated:'Updated', agent:'Agent', activeTime:'Active time', task:'Task', id:'ID', repository:'Repository', noMatches:'No backlog items match the selected filters.', backlogUninitializedSummary:'Waiting for backlog creation', backlogUninitializedTitle:'No tasks have been registered yet.', backlogUninitializedBody:'The backlog will be created automatically when the first task is registered.', manualIntro:'Review the first-use flow and Task Mecca operating rules inside the UI.', dashboardLaunch:'Dashboard launch', keyboard:'Keyboard', quickStart:'Quick Start', detailedGuide:'Detailed Operations Guide', manualLoading:'Manual is loading…', manualUnavailable:'Manual unavailable.', operationsEyebrow:'Operations', diagnostics:'Diagnostics', noAttention:'No tasks currently need attention.', advisory:'Shows tasks that need user action, finalization, or runtime investigation.', workers:'Workers', doing:'Doing', downstreamBlocked:'Downstream blocked', readyContinuity:'Ready continuity', noWorkload:'No assigned subagent workload.', noCurrentDoing:'No current doing task.', unassignedDoing:'Unassigned doing', releasedHold:'Released hold ownership', continuity:'Continuity', blocks:'Blocks', scope:'Change scope', workloadIntro:'Durable allocation view from backlog/Git. Runtime health is shown when observable.', issuesIntro:'Doctor and hold-review findings from the current snapshot.', noIssues:'No diagnostic issues.', taskDefinition:'Task Definition', simpleNote:'Simple Task · no separate requirement-definition confirmation required.', goal:'Goal', acceptance:'Acceptance criteria', requirements:'Requirements', definedNote:'Defined Task · requirement definition confirmed before registration.', background:'Background & problem', scopeIn:'IN', scopeOut:'OUT', constraints:'Constraints & preservation', legacyTask:'Legacy task', legacyNote:'Legacy backlog · rendered from available fields without inventing missing requirement data.', description:'Description', tocTitle:'On this task', tocOpen:'Open table of contents', tocClose:'Close table of contents', lifecycle:'Lifecycle', noLifecycle:'No lifecycle transitions are available yet.', stayed:'Stayed', provisional:'provisional', overview:'Overview', registrant:'Registrant', changeScope:'Change scope', dependsOn:'Depends on', related:'Related', location:'Location', completed:'Completed', activity:'Activity', lastSignal:'Last signal', signalSource:'Signal source', execution:'Execution', runtimeProvider:'RuntimeProvider', dispatchStatus:'Dispatch status', executionEvidence:'Execution evidence', fallbackEvidence:'Fallback evidence', workNotes:'Work notes', result:'Result', verification:'Verification', rawMarkdown:'Raw Markdown', rendered:'Rendered', waitTime:'Wait time', queueTime:'Queue time', leadTime:'Lead time', provisionalTiming:'Provisional lifecycle timing', incompleteHistory:'Incomplete lifecycle history', lastObservable:'Last observable activity {ago} ({source}).', workerMissingDetail:'Task is doing but the assigned worker is absent from the available runtime registry.', quietAdvisory:'This is advisory; long-running work can be legitimately quiet.', backToBacklog:'← Backlog', taskNotFound:'Task not found.', restrictedNow:'Restricted now', lastFullAccess:'Last Full Access', fullAccess:'Full Access', lastRestricted:'Last check restricted', accessLastChecked:'Access last checked', accessNotChecked:'Access not checked', networkOff:'Net off', notChecked:'Not checked', freshDispatch:'dispatch always runs a fresh active preflight.', dispatchDisabled:'Subagent dispatch disabled', enableFullAccess:'Current runtime restriction detected. Enable Full Access before dispatch.', auto:'Auto', manualMode:'Manual', noBacklog:'No backlog selected', copyCode:'Copy code', copied:'Copied', copyFailed:'Copy failed', mermaid:'Mermaid', showSource:'Show source', hideSource:'Hide source', renderingDiagram:'Rendering diagram…', mermaidUnavailable:'Mermaid renderer unavailable.', mermaidUnavailableDetail:'Task Mecca could not load the local or CDN Mermaid runtime. Use Source to view or copy the diagram text.', mermaidFailed:'Mermaid render failed.', invalidMermaid:'Invalid Mermaid syntax', secondsAgo:'{n}s ago', minutesAgo:'{n}m ago', hoursAgo:'{n}h ago', daysAgo:'{n}d ago', justNow:'just now', updatedColumn:'Updated', activeColumn:'Active', agentColumn:'Agent', statusColumn:'Status', taskColumn:'Task', expandSidebar:'Expand sidebar', collapseSidebar:'Collapse sidebar', eventRegistered:'Registered', eventAssigned:'Assigned', eventStarted:'Started', eventWaiting:'Waiting for user', eventResumed:'Resumed', eventHold:'Hold', eventCompleted:'Completed', observatory:'Backlog Observatory', theme:'Theme', followSystemTheme:'Follow system theme', useLightTheme:'Use light theme', useDarkTheme:'Use dark theme', selectBacklog:'Select backlog folder', provisionalTimingDetail:'No direct transition report is available; timing uses file or runtime observation and may differ from the actual transition time.', incompleteHistoryDetail:'Some timing cannot be reconstructed exactly because the historical lifecycle has no observed start evidence.'
  }
};
Object.assign(I18N.ko,{
  runtimeHookProjectScope:'현재 프로젝트',runtimeHookGlobalScope:'이 기기의 전체 프로젝트',
  runtimeHookProjectConfigure:'{provider} · 현재 프로젝트에 설정',runtimeHookGlobalConfigure:'{provider} · 전역 설정',
  runtimeHookGlobalEnableConfirm:'{provider} Hook을 이 기기의 사용자 전역 설정에 추가합니다. 다른 프로젝트의 {provider} 세션에도 로드될 수 있지만, Task Mecca가 설치된 프로젝트에서만 기록합니다. 계속할까요?',
  runtimeHookGlobalDisableConfirm:'사용자 전역 설정에서 Task Mecca의 {provider} Hook만 제거합니다. 다른 Hook과 현재 프로젝트 설정은 유지합니다. 계속할까요?',
  runtimeHookTrustAction:'{provider} 승인 진행',runtimeHookTrustGuideTitle:'{provider}에서 직접 확인하세요',
  runtimeHookCodexTrustStep1:'Codex CLI에서 프로젝트를 연 다음 /hooks를 입력하세요. 새로 추가된 Task Mecca Hook을 찾아 Review/Trust를 완료하세요.',
  runtimeHookCodexTrustStep2:'Codex가 신뢰한 Hook 정의만 실행합니다. 이 버튼은 Codex의 승인을 대신하지 않습니다.',
  runtimeHookClaudeTrustStep1:'Claude Code에서 프로젝트를 열고 workspace trust 대화상자에서 신뢰를 확인하세요.',
  runtimeHookClaudeTrustStep2:'/hooks에서 설정 출처를 확인하세요. /hooks 자체가 workspace trust 승인은 아닙니다.',
  runtimeHookTrustStepVerify:'새 세션에서 실제 Hook 이벤트가 도착해야 ‘관측 확인됨’으로 바뀝니다. 이전 세션의 Start/Stop은 소급하지 않습니다.',
  runtimeHookOfficialDocs:'공식 Hook 안내 열기',
  runtimeHookScopeAmbiguous:'두 범위에 모두 설정되어 있습니다. 관측 이벤트는 확인했지만 어느 설정에서 실행됐는지는 구분하지 않습니다.',runtimeHookSourceUnresolved:'실행 경로 미확인',
  notificationsInsecureTitle:'시스템 알림을 사용할 수 없는 접속입니다.',
  notificationsInsecureGuide:'현재 페이지가 HTTPS 보안 연결이 아닙니다. 원격 브라우저의 시스템 알림 권한은 HTTPS에서만 사용할 수 있습니다. Task Mecca의 Tailscale HTTPS 주소로 접속한 뒤 다시 시도하세요.',
  notificationsIOSHomeTitle:'iPhone/iPad 알림 설정이 필요합니다.',
  notificationsIOSHomeGuide:'iOS에서는 사이트를 홈 화면에 추가한 뒤 홈 화면의 Task Mecca 웹 앱으로 열어야 알림 권한을 요청할 수 있습니다.',
  notificationsUnsupportedTitle:'이 브라우저는 시스템 알림을 지원하지 않습니다.',
  notificationsUnsupportedGuide:'현재 브라우저에서는 시스템 알림 API를 사용할 수 없습니다. 지원되는 브라우저 또는 HTTPS 환경을 사용하세요.',
  notificationPermissionError:'알림 권한 요청에 실패했습니다.',
});
Object.assign(I18N.en,{
  runtimeHookProjectScope:'Current project',runtimeHookGlobalScope:'All projects on this device',
  runtimeHookProjectConfigure:'Configure {provider} for this project',runtimeHookGlobalConfigure:'Configure {provider} globally',
  runtimeHookGlobalEnableConfirm:'Add the {provider} Hook to this device’s user-level settings? It may load in other {provider} projects, but Task Mecca records events only where Task Mecca is installed.',
  runtimeHookGlobalDisableConfirm:'Remove only the Task Mecca {provider} Hook from user-level settings? Other Hooks and this project’s configuration remain.',
  runtimeHookTrustAction:'Continue {provider} trust review',runtimeHookTrustGuideTitle:'Complete the review in {provider}',
  runtimeHookCodexTrustStep1:'Open the project in Codex CLI and enter /hooks. Find the new Task Mecca Hook, then complete Review/Trust.',
  runtimeHookCodexTrustStep2:'Codex runs only trusted Hook definitions. This button does not approve a Hook in Codex.',
  runtimeHookClaudeTrustStep1:'Open the project in Claude Code and accept its workspace trust dialog.',
  runtimeHookClaudeTrustStep2:'Use /hooks to inspect the configuration source. The /hooks menu does not itself grant workspace trust.',
  runtimeHookTrustStepVerify:'Start a new session. Only a Hook event actually received afterward changes the status to Observed; earlier Start/Stop events are not inferred.',
  runtimeHookOfficialDocs:'Open official Hooks documentation',
  runtimeHookScopeAmbiguous:'Both scopes are configured. A Hook event was received, but Task Mecca cannot attribute it to one scope.',runtimeHookSourceUnresolved:'Source unverified',
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
  upgrading:'업그레이드 중…', migrating:'마이그레이션 중…', updateLabel:'UPDATE', updateAgentContinuity:'업데이트해도 실행 중인 에이전트와 작업은 중단되지 않습니다.'
});
Object.assign(I18N.en,{
  updateAvailable:'Update available', currentVersion:'Current version', projectMigration:'Project migration required',
  newContentAvailable:'New content is available.', refreshToSee:'Your current reading position is preserved. Refresh when you want to see the update.',
  refreshNow:'Refresh', runtimeChanged:'Task status changed.', contentChanged:'Backlog content changed.',
  statusChanges:'Status changes', taskRegistered:'Registered', taskRemoved:'Removed from backlog', moreStatusChanges:'{n} more',
  upgrading:'Upgrading…', migrating:'Migrating…', updateLabel:'UPDATE', updateAgentContinuity:'Updating does not stop running agents or their work.'
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

Object.assign(I18N.ko,{terminal:'터미널'});
Object.assign(I18N.en,{terminal:'Terminal'});
Object.assign(I18N.ko,{projectNotifications:'프로젝트별 알림',projectNotificationsIntro:'감시 중인 프로젝트마다 이후 발생하는 Telegram 알림을 따로 켜거나 끕니다. 꺼진 동안의 과거 이벤트는 다시 보내지 않습니다.',projectNotificationEnabled:'활성',projectNotificationDisabled:'비활성',projectNotificationEmpty:'감시 중인 프로젝트가 없습니다.',projectNotificationPath:'경로',projectRecipientMode:'수신처',projectRecipientShared:'통합 수신처',projectRecipientIndividual:'개별 수신처',projectIndividualIncomplete:'개별 수신처가 설정되지 않았습니다. 통합 수신처로 자동 전환하지 않습니다.',projectSharedRecipientGuide:'통합 수신처를 설정하면 감시 중인 모든 프로젝트에 같은 수신처를 안전하게 저장합니다. Token과 수신처 ID는 이 화면·API 응답·기록에 표시하지 않습니다.',projectSharedRecipientConfigure:'통합 수신처 확인'});
Object.assign(I18N.en,{projectNotifications:'Project notifications',projectNotificationsIntro:'Enable or disable future Telegram notifications for each monitored project. Events from a disabled period are not replayed.',projectNotificationEnabled:'Enabled',projectNotificationDisabled:'Disabled',projectNotificationEmpty:'No monitored projects.',projectNotificationPath:'Path',projectRecipientMode:'Recipient',projectRecipientShared:'Shared recipient',projectRecipientIndividual:'Individual recipient',projectIndividualIncomplete:'Individual recipient is not configured; Task Mecca will not fall back to the shared recipient.',projectSharedRecipientGuide:'Shared recipient setup stores the same recipient safely for all monitored projects. Tokens and recipient IDs are never shown in this screen, API responses, or records.',projectSharedRecipientConfigure:'Verify shared recipient'});

function t(key, vars = {}) {
  const dict = I18N[state.language] || I18N.en;
  let value = dict[key] ?? I18N.en[key] ?? key;
  Object.entries(vars).forEach(([k,v]) => { value = value.replaceAll(`{${k}}`, String(v)); });
  return value;
}
const OPERATION_COPY = {
  ko: { heading:'세션 운영 확인 필요', monitoring:'Web 세션 감시 중', targets:'개 프로젝트 감시', lastScan:'최근 확인', scope:'감시 대상 보기', gap:'Web 감시 공백', no_signal:'세션 무신호', runtime_unknown:'실행 상태 미확인', interrupted:'확인된 실행 중단', errored:'확인된 실행 오류', shutdown:'확인된 세션 종료', evidence:'근거', seen:'마지막 관측', detected:'최초 발견', ended:'확인된 종료', recovered:'감시 재개', history:'과거 실행 관측', resolved:'현재 경고 해소', completedUnknown:'작업 완료 확인, 당시 실행 관측은 미확인', completionEvidence:'완료 확인 근거', completionAt:'작업 완료 시각', reconciliationAt:'경고 해소 시각', maintenance:'Telegram 전송 차단 · 유지보수 모드', action:'필요한 조치', project:'프로젝트', count:'건의 운영 사건', detail:'근거와 조치 보기' },
  en: { heading:'Session operation needs review', monitoring:'Web session monitoring', targets:'projects monitored', lastScan:'Last scan', scope:'View monitored projects', gap:'Web monitoring gap', no_signal:'No session signal', runtime_unknown:'Runtime unverified', interrupted:'Confirmed interruption', errored:'Confirmed execution error', shutdown:'Confirmed session shutdown', evidence:'Evidence', seen:'Last observed', detected:'First detected', ended:'Confirmed end', recovered:'Monitoring resumed', history:'Past runtime observations', resolved:'Current warning resolved', completedUnknown:'Task completion verified; runtime observation at the time remains unverified', completionEvidence:'Completion evidence', completionAt:'Task completed at', reconciliationAt:'Warning resolved at', maintenance:'Telegram transport blocked · maintenance mode', action:'Next action', project:'Project', count:'operation incidents', detail:'View evidence and action' },
};
const OPERATION_EVIDENCE = {
  ko: {
    'doing assignment has no linked runtime attempt':'진행 중인 배정은 있지만 연결된 runtime attempt가 없습니다.',
    'verified hook has no recent signal':'실제로 수신된 Hook에서 장시간 새 신호가 없습니다.',
    'runtime identity or hook execution unverified':'runtime identity 또는 Hook 실행 여부를 확인하지 못했습니다.',
    'terminal state lacks observed hook evidence':'종료 상태에 실제 Hook 수신 근거가 없습니다.',
    'web server scan gap; exact session stop time is unknown':'Web 서버의 감시 공백이 있었습니다. 그 사이의 정확한 세션 중단 시각은 알 수 없습니다.',
    'hook terminal state':'Hook이 종료 상태를 실제로 전달했습니다.',
    'hook completed state':'Hook이 정상 완료 상태를 실제로 전달했습니다.',
  },
  en: {
    'doing assignment has no linked runtime attempt':'A doing assignment has no linked runtime attempt.',
    'verified hook has no recent signal':'A previously observed Hook has no recent signal.',
    'runtime identity or hook execution unverified':'Runtime identity or Hook execution is unverified.',
    'terminal state lacks observed hook evidence':'The terminal state lacks an observed Hook event.',
    'web server scan gap; exact session stop time is unknown':'Web monitoring stopped; the exact session stop time is unknown.',
    'hook terminal state':'A Hook reported the terminal state.',
    'hook completed state':'A Hook reported normal completion.',
  },
};
function operationText(key) { return (OPERATION_COPY[state.language]||OPERATION_COPY.en)[key]||(key.startsWith('handoff_')?operationStageLabel(key):key); }
function operationEvidence(value) { return (OPERATION_EVIDENCE[state.language]||OPERATION_EVIDENCE.en)[value]||value; }
function operationAction(item) {
  if(state.language==='ko')return item.action;
  if(item.kind==='no_signal')return 'Controller: check the session and Hook delivery. Silence alone does not confirm that the Worker stopped.';
  if(item.kind==='monitor_gap')return 'Controller: verify session status with separate evidence for the monitoring gap.';
  if(item.quality==='confirmed')return 'Controller: review the termination evidence and decide whether to resume or reassign the work.';
  return 'Controller: compare assignment and runtime identity evidence. Do not infer Worker death or change backlog state automatically.';
}
function operationTime(value) { return value ? new Date(value).toLocaleString(localeCode()) : '—'; }
function operationStageLabel(stage) {
  const labels=state.language==='ko'?{assignment_pending:'배정 후 실행 연결 대기',worker_running:'워커 실행 중',worker_report_pending:'워커 완료 · 보고 대기',handoff_pending:'워커 보고 완료 · 인계 대기',controller_review:'컨트롤러 완료검토 중',review_complete:'검토 완료 · 최종 처리 대기',handoff_applied:'인계 처리 완료',handoff_failed:'인계 실패',handoff_stalled:'인계·검토 유예 초과'}:{assignment_pending:'Awaiting execution binding',worker_running:'Worker running',worker_report_pending:'Worker completed · report pending',handoff_pending:'Worker reported · handoff pending',controller_review:'Controller completion review',review_complete:'Review complete · finalization pending',handoff_applied:'Handoff applied',handoff_failed:'Handoff failed',handoff_stalled:'Handoff / review grace expired'};
  return labels[stage]||stage;
}
function renderOperationStages(stages) {
  const pending=stages.filter(item=>item.stage!=='worker_running'&&item.stage!=='handoff_applied'&&item.stage!=='controller_verified_complete'&&item.stage!=='runtime_observation_pending');
  if(!pending.length)return '';
  const ko=state.language==='ko';
  return `<details class="operation-scope operation-stages"><summary>${ko?'현재 인계·검토 단계':'Current handoff / review stages'} · ${pending.length}</summary>${pending.map(item=>`<div class="operation-stage"><strong>${esc(item.task_id)} · ${esc(operationStageLabel(item.stage))}</strong><span>${ko?'단계 시작':'Stage since'}: ${esc(operationTime(item.since))}${item.grace_until?` · ${ko?'유예 종료':'Grace until'}: ${esc(operationTime(item.grace_until))}`:''}</span><span>${esc(item.evidence)} · ${esc(item.assignment_id)}${item.handoff_id?` · ${esc(item.handoff_id)}`:''}</span></div>`).join('')}<p>${ko?'워커 완료와 검토 완료는 백로그 완료가 아닙니다. 최종 완료는 컨트롤러가 별도로 확인합니다.':'Worker or review completion is not backlog completion. The Controller verifies final completion separately.'}</p></details>`;
}
function renderOperationHistory(items) {
  if(!items.length)return '';
  return `<details class="operation-scope operation-history"><summary>${esc(operationText('history'))} · ${items.length}</summary>${items.map(item=>{
    if(item.recovery_evidence)return `<details class="operation-incident history"><summary><span class="operation-incident-type">${esc(operationText('resolved'))}</span><span class="operation-incident-subject">${esc(item.task_id)}</span><span class="operation-incident-expand">${esc(operationText('detail'))}</span></summary><dl><div><dt>${esc(operationText('evidence'))}</dt><dd>${esc(operationEvidence(item.evidence))}</dd></div><div><dt>${esc(operationText('reconciliationAt'))}</dt><dd>${esc(operationTime(item.recovered_at))}</dd></div><div class="operation-action"><dt>${esc(operationText('resolved'))}</dt><dd>${esc(item.recovery_evidence)}</dd></div></dl></details>`;
    const proof=item.resolution||{};
    return `<details class="operation-incident history"><summary><span class="operation-incident-type">${esc(operationText('resolved'))}</span><span class="operation-incident-subject">${esc(item.task_id)}</span><span class="operation-incident-expand">${esc(operationText('detail'))}</span></summary><dl><div class="operation-action"><dt>${esc(operationText('resolved'))}</dt><dd>${esc(proof.reason==='task_completed_runtime_observed'?(state.language==='ko'?'컨트롤러가 작업 완료를 확인했습니다. 관측된 실행 종료 근거를 보존합니다.':'Controller verified task completion; observed runtime completion is preserved.'):operationText('completedUnknown'))}</dd></div><div><dt>${esc(operationText('project'))}</dt><dd>${esc(item.project)}</dd></div><div><dt>${esc(operationText('evidence'))}</dt><dd>${esc(operationEvidence(item.evidence))}</dd></div><div><dt>${esc(operationText('seen'))}</dt><dd>${esc(operationTime(item.last_observed_at))}</dd></div><div><dt>${esc(operationText('detected'))}</dt><dd>${esc(operationTime(item.detected_at))}</dd></div><div><dt>${esc(operationText('completionAt'))}</dt><dd>${esc(operationTime(proof.completed_at))}</dd></div><div><dt>${esc(operationText('reconciliationAt'))}</dt><dd>${esc(operationTime(item.recovered_at))}</dd></div><div class="operation-action"><dt>${esc(operationText('completionEvidence'))}</dt><dd>${esc(proof.event_id)} · ${esc(proof.assignment_id)} · ${esc(proof.attempt_id)}<br>${esc(proof.task_path)}</dd></div></dl></details>`;
  }).join('')}</details>`;
}
function renderOperationBanner(payload) {
 const observed=Date.parse(payload.snapshot_at||''),prior=state.operationObservedAt||0;
 if(prior&&(!Number.isFinite(observed)||observed<prior))return;
 if(Number.isFinite(observed))state.operationObservedAt=observed;
 state.operationTombstones||={};
 for(const item of [...(payload.recent||[]),...(payload.resolved_observations||[])])if(item.id&&(item.recovered_at||item.resolution||item.recovery_evidence||item.history||item.active===false))state.operationTombstones[item.id]=true;
 state.operationPayload={projects:payload.projects||[],stages:payload.stages||[],resolved_observations:payload.resolved_observations||[],telegram_transport_disabled:Boolean(payload.telegram_transport_disabled)};
  state.sessionWarnings=(payload.active||[]).filter(item=>!state.operationTombstones[item.id]&&!item.recovered_at&&!item.resolution&&!item.recovery_evidence&&!item.policy_resolution&&item.kind!=='normal_exit'&&!item.history&&item.active!==false);
  renderUserAttention();
 if(state.view==='workload')render();
}
async function refreshOperations() {
  const request=++state.operationsRequest;
  try {
    const response=await fetch('/api/operations',{cache:'no-store'});
    if(!response.ok)return;
    const payload=await response.json();
    if(request!==state.operationsRequest)return;
    renderOperationBanner(payload);
    const projects=[...new Set((payload.projects||[]).filter(item=>!item.error).map(item=>item.path).filter(Boolean))];
    for(const key of Object.keys(state.commonUserAttention)){if(!projects.includes(state.commonUserAttention[key].project))delete state.commonUserAttention[key];}
    renderUserAttention();
    await Promise.all(projects.map(async project=>{
      try{
        const params=new URLSearchParams({project});
        const userRevisions={...state.commonUserRevision};
        const response=await fetch('/api/attention?'+params,{cache:'no-store'});
        if(!response.ok)return;
        const snapshot=await response.json();
        const snapshotScope=project+'|'+(snapshot.backlog_selection?.selected||'');
        if(request!==state.operationsRequest||(userRevisions[snapshotScope]||0)!==(state.commonUserRevision[snapshotScope]||0))return;
        const scope=snapshot.backlog_selection;
        if(scope&&Array.isArray(scope.candidates)){
          const paths=new Set([scope.selected,...scope.candidates.map(candidate=>candidate.path)].filter(Boolean));
          for(const key of Object.keys(state.commonUserAttention)){const source=state.commonUserAttention[key];if(source.project===project&&!paths.has(source.backlog))delete state.commonUserAttention[key];}
        }
        storeCommonUserAttention(snapshot,project,'');
        processTaskNotifications(snapshot,{project,backlog:'',updateCurrent:false});
        const selected=snapshot.backlog_selection?.selected||'';
        await Promise.all((snapshot.backlog_selection?.candidates||[]).filter(candidate=>candidate.path&&candidate.path!==selected).map(async candidate=>{
          const scoped=new URLSearchParams({project,backlog:candidate.path});
          const sourceKey=project+'|'+candidate.path,revision=state.commonUserRevision[sourceKey]||0;
          const result=await fetch('/api/attention?'+scoped,{cache:'no-store'});if(!result.ok)return;
          const data=await result.json();
          if(request!==state.operationsRequest||revision!==(state.commonUserRevision[sourceKey]||0))return;
          storeCommonUserAttention(data,project,candidate.path);
          processTaskNotifications(data,{project,backlog:candidate.path,updateCurrent:false});
        }));
      }catch(_){/* Current projections retry with the next successful refresh. */}
    }));
  } catch (_) { /* Current projections retry with the next successful refresh. */ }
  if(state.view==='notifications')loadNotificationHistory();
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
  if(version)await openUpgradeDetails(version);
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


// The update flow has one continuous visual surface: checking notes, an
// explicit decision, server-confirmed install phases, restart and errors.
let upgradeFlowMode='idle';
let upgradeFlowSession=0;
let upgradeFlowStartedAt=0;
let upgradeFlowTarget='';
let upgradeFlowTimer=null;
let upgradeFlowPollTimer=null;
let upgradeFlowPollBusy=false;
let upgradeFlowOwnRequest=false;
let upgradeFlowRestartWatching=false;
let upgradeFlowError='';

function upgradeFlowCopy(){
 const ko=state.language==='ko';
 return ko?{
  eyebrow:'TASK MECCA · SOFTWARE UPDATE',title:'Task Mecca 업데이트',
  notes:'업데이트 내용을 확인하고 있습니다',checking:'최신 버전과 설치 환경을 확인하고 있습니다',
  downloading:'업데이트 파일을 다운로드하고 있습니다',verifying:'다운로드 파일을 검증하고 있습니다',
  installing:'검증된 업데이트를 설치하고 있습니다',restarting:'웹 서비스를 재시작하고 있습니다',
  completed:'업데이트를 완료했습니다',failed:'업데이트를 완료하지 못했습니다',
  noNotes:'업데이트 상세 내용을 확인할 수 없습니다',noNotesHelp:'릴리스 노트를 불러오지 못했습니다. 계속 진행할지 선택해 주세요.',
  waiting:'진행 중입니다. 업데이트 버튼을 다시 누르지 않아도 됩니다.',
  restartHelp:'연결이 잠시 끊길 수 있습니다. 준비되면 자동으로 다시 연결합니다.',
  longWait:'네트워크 상태에 따라 시간이 걸릴 수 있습니다. 작업이 계속 진행 중입니다.',
  cancel:'취소',close:'닫기',continue:'업데이트 진행',retry:'다시 시도',
  elapsed:'경과',stages:['확인','다운로드','검증','설치','재시작'],
  restartError:'웹 서버의 재시작을 확인하지 못했습니다. 웹 상태를 확인한 뒤 다시 접속해 주세요.'
 }:{
  eyebrow:'TASK MECCA · SOFTWARE UPDATE',title:'Task Mecca update',
  notes:'Checking update details',checking:'Checking the latest version and installation environment',
  downloading:'Downloading the update',verifying:'Verifying the downloaded file',
  installing:'Installing the verified update',restarting:'Restarting the Web service',
  completed:'Update completed',failed:'The update could not be completed',
  noNotes:'Update details are unavailable',noNotesHelp:'Release notes could not be loaded. Choose whether to continue.',
  waiting:'In progress. You do not need to press Update again.',
  restartHelp:'The connection may briefly drop. This page will reconnect when ready.',
  longWait:'This can take longer on slower connections. The update is still running.',
  cancel:'Cancel',close:'Close',continue:'Continue update',retry:'Retry',
  elapsed:'Elapsed',stages:['Check','Download','Verify','Install','Restart'],
  restartError:'Could not confirm the Web service restart. Check its status, then reconnect.'
 };
}
const upgradeFlowPhases=['checking','downloading','verifying','installing','restarting'];
function upgradeFlowOverlay(){
 let overlay=$('#upgradeFlowOverlay');
 if(overlay)return overlay;
 overlay=document.createElement('div');
 overlay.id='upgradeFlowOverlay';
 overlay.className='upgrade-flow-overlay';
 overlay.innerHTML='<div class="upgrade-flow-backdrop"></div>'+
  '<section class="release-modal-card upgrade-flow-card" role="dialog" aria-modal="true" aria-labelledby="upgradeFlowTitle" aria-describedby="upgradeFlowDescription">'+
  '<div class="eyebrow upgrade-flow-eyebrow"></div>'+
  '<div class="upgrade-flow-heading"><span class="upgrade-flow-spinner" aria-hidden="true"></span><div><h2 id="upgradeFlowTitle"></h2><p class="upgrade-flow-version"></p></div></div>'+
  '<p id="upgradeFlowDescription" class="upgrade-flow-description" role="status" aria-live="polite" aria-atomic="true"></p>'+
  '<div class="upgrade-flow-meter" aria-hidden="true"><span></span></div>'+
  '<ol class="upgrade-flow-stages"></ol>'+
  '<p class="upgrade-flow-help"></p>'+
  '<p class="upgrade-flow-error" role="alert" hidden></p>'+
  '<div class="upgrade-flow-footer"><span class="upgrade-flow-elapsed" aria-hidden="true"></span><div class="upgrade-flow-actions"></div></div>'+
  '</section>';
 document.body.appendChild(overlay);
 document.body.classList.add('release-modal-open');
 const dialog=overlay.querySelector('[role="dialog"]');
 dialog.setAttribute('tabindex','-1');
 overlay.addEventListener('keydown',event=>{
  if(event.key==='Escape' && ['notes','no-notes','failed','completed'].includes(upgradeFlowMode)){
   event.preventDefault();closeUpgradeFlow();return;
  }
  if(event.key!=='Tab')return;
  const controls=[...dialog.querySelectorAll('button:not([disabled])')];
  if(!controls.length){event.preventDefault();dialog.focus();return;}
  const first=controls[0],last=controls[controls.length-1];
  if(event.shiftKey&&(document.activeElement===first||document.activeElement===dialog)){event.preventDefault();last.focus();}
  else if(!event.shiftKey&&(document.activeElement===last||document.activeElement===dialog)){event.preventDefault();first.focus();}
 });
 dialog.focus();
 return overlay;
}
function updateFlowElapsed(){
 const overlay=$('#upgradeFlowOverlay');if(!overlay)return;
 const seconds=Math.max(0,Math.floor((Date.now()-upgradeFlowStartedAt)/1000));
 const minutes=String(Math.floor(seconds/60)).padStart(2,'0');
 const secs=String(seconds%60).padStart(2,'0');
 const element=overlay.querySelector('.upgrade-flow-elapsed');
 if(element)element.textContent=upgradeFlowCopy().elapsed+' '+minutes+':'+secs;
 if(upgradeFlowMode!=='failed'&&upgradeFlowMode!=='completed'&&seconds>=20){
  const help=overlay.querySelector('.upgrade-flow-help');
  if(help && upgradeFlowMode!=='no-notes')help.textContent=upgradeFlowCopy().longWait;
 }
}
function setUpgradeFlowStage(phase,error=''){
 upgradeFlowMode=phase;
 upgradeFlowError=error;
 const overlay=upgradeFlowOverlay(),copy=upgradeFlowCopy();
 overlay.dataset.phase=phase;
 overlay.querySelector('.upgrade-flow-eyebrow').textContent=copy.eyebrow;
 overlay.querySelector('#upgradeFlowTitle').textContent=copy.title;
 overlay.querySelector('.upgrade-flow-version').textContent=upgradeFlowTarget;
 const message=copy[phase]||copy.checking;
 overlay.querySelector('#upgradeFlowDescription').textContent=message;
 const errorEl=overlay.querySelector('.upgrade-flow-error');
 errorEl.hidden=!error;
 errorEl.textContent=error;
 overlay.querySelector('.upgrade-flow-spinner').hidden=['no-notes','failed','completed'].includes(phase);
 overlay.querySelector('.upgrade-flow-meter').hidden=['no-notes','failed','completed'].includes(phase);
 overlay.querySelector('.upgrade-flow-help').textContent=phase==='no-notes'?copy.noNotesHelp:phase==='restarting'?copy.restartHelp:phase==='failed'||phase==='completed'?'':copy.waiting;
 const current=phase==='notes'?0:upgradeFlowPhases.indexOf(phase);
 overlay.querySelector('.upgrade-flow-stages').innerHTML=copy.stages.map((label,i)=>
  '<li class="'+(current<0?'':i<current?'done':i===current?'current':'pending')+'">'+
  '<span class="upgrade-step-dot">'+(i<current?'✓':String(i+1))+'</span><span>'+esc(label)+'</span></li>').join('');
 const actions=overlay.querySelector('.upgrade-flow-actions');
 if(phase==='notes'||phase==='no-notes'){
  actions.innerHTML='<button type="button" class="action-btn secondary" data-upgrade-flow-cancel>'+esc(copy.cancel)+'</button>'+
   (phase==='no-notes'?'<button type="button" class="action-btn" data-upgrade-flow-continue>'+esc(copy.continue)+'</button>':'');
  actions.querySelector('[data-upgrade-flow-cancel]').addEventListener('click',closeUpgradeFlow);
  actions.querySelector('[data-upgrade-flow-continue]')?.addEventListener('click',()=>performUpgrade());
 }else if(phase==='failed'||phase==='completed'){
  actions.innerHTML='<button type="button" class="action-btn secondary" data-upgrade-flow-close>'+esc(copy.close)+'</button>'+
   (phase==='failed'?'<button type="button" class="action-btn" data-upgrade-flow-retry>'+esc(copy.retry)+'</button>':'');
  actions.querySelector('[data-upgrade-flow-close]').addEventListener('click',closeUpgradeFlow);
  actions.querySelector('[data-upgrade-flow-retry]')?.addEventListener('click',()=>performUpgrade());
 }else actions.innerHTML='';
 updateFlowElapsed();
 renderGlobalUpdateIndicator();
 if(!upgradeFlowTimer && phase!=='completed' && phase!=='failed'){
  upgradeFlowTimer=setInterval(updateFlowElapsed,1000);
 }
}
function stopUpgradeFlowTimers(){
 if(upgradeFlowTimer){clearInterval(upgradeFlowTimer);upgradeFlowTimer=null;}
 if(upgradeFlowPollTimer){clearInterval(upgradeFlowPollTimer);upgradeFlowPollTimer=null;}
 upgradeFlowPollBusy=false;
}
function closeUpgradeFlow(){
 if(['checking','downloading','verifying','installing','restarting'].includes(upgradeFlowMode))return;
 upgradeFlowSession++;
 upgradeFlowOwnRequest=false;
 upgradeFlowRestartWatching=false;
 stopUpgradeFlowTimers();
 $('#upgradeFlowOverlay')?.remove();
 document.body.classList.toggle('release-modal-open',Boolean(state.releaseNotePopup));
 upgradeFlowMode='idle';
 upgradeFlowError='';
 renderGlobalUpdateIndicator();
}
async function openUpgradeDetails(version){
 if(upgradeFlowMode!=='idle')return;
 const session=++upgradeFlowSession;
 upgradeFlowStartedAt=Date.now();
 upgradeFlowTarget=String(version||'');
 setUpgradeFlowStage('notes');
 const detail=await loadReleaseNoteDetail(version);
 if(session!==upgradeFlowSession||upgradeFlowMode!=='notes')return;
 if(detail){
  closeUpgradeFlow();
  state.releaseNotePopup=detail;state.releaseNotePopupMode='available';
  renderReleaseNoteModal();
 }else{
  setUpgradeFlowStage('no-notes');
 }
}
async function observeUpgradeRestart(session){
 if(upgradeFlowRestartWatching || upgradeFlowOwnRequest)return;
 upgradeFlowRestartWatching=true;
 const target=String(upgradeFlowTarget.split(' → ').pop()||'');
 const ready=await waitForRestartedWeb(target);
 if(ready===false && session===upgradeFlowSession){
  stopUpgradeFlowTimers();
  setUpgradeFlowStage('failed',upgradeFlowCopy().restartError);
 }
 upgradeFlowRestartWatching=false;
}
function startUpgradeFlowPolling(session){
 if(upgradeFlowPollTimer)clearInterval(upgradeFlowPollTimer);
 upgradeFlowPollTimer=setInterval(async()=>{
  if(session!==upgradeFlowSession || upgradeFlowPollBusy || !upgradeFlowPhases.includes(upgradeFlowMode))return;
  upgradeFlowPollBusy=true;
  try{
   const r=await fetch('/api/upgrade-status',{cache:'no-store'});
   if(!r.ok)return;
   const status=await r.json();
   if(session!==upgradeFlowSession || !upgradeFlowPhases.includes(upgradeFlowMode))return;
   if(status.active && upgradeFlowPhases.includes(status.phase)){
    if(upgradeFlowPhases.indexOf(status.phase)>upgradeFlowPhases.indexOf(upgradeFlowMode)){
     setUpgradeFlowStage(status.phase);
    }
    if(status.phase==='restarting')void observeUpgradeRestart(session);
   }else if(!status.active && !upgradeFlowOwnRequest){
    if(status.phase==='failed'){
     stopUpgradeFlowTimers();
     setUpgradeFlowStage('failed',String(status.error||'Update failed'));
    }else if(status.phase==='completed'){
     stopUpgradeFlowTimers();
     setUpgradeFlowStage('completed');
     void refreshVersionInfo(true);
    }
   }
  }catch(_){/* Server restart may temporarily interrupt status polling. */}
  finally{upgradeFlowPollBusy=false;}
 },900);
}
async function resumeActiveUpgrade(){
 if(upgradeFlowMode!=='idle')return;
 try{
  const r=await fetch('/api/upgrade-status',{cache:'no-store'});
  if(!r.ok)return;
  const status=await r.json();
  if(!status.active || !upgradeFlowPhases.includes(status.phase) || upgradeFlowMode!=='idle')return;
  const cli=state.versionInfo?.cli||state.hub?.cli||{};
  upgradeFlowTarget=cli.current&&cli.latest
   ? String(cli.current)+' → '+String(cli.latest)
   : (state.language==='ko'?'진행 중인 업데이트에 다시 연결합니다':'Reconnecting to an active update');
  upgradeFlowStartedAt=Date.parse(status.started_at)||Date.now();
  const session=++upgradeFlowSession;
  state.releaseNotePopup=null;
  renderReleaseNoteModal();
  setUpgradeFlowStage(status.phase);
  startUpgradeFlowPolling(session);
  if(status.phase==='restarting')void observeUpgradeRestart(session);
 }catch(_){/* Older server versions have no status endpoint. */}
}

function renderGlobalUpdateIndicator() {
  const el=$('#globalUpdateIndicator');
  if(!el)return;
  const payload=state.versionInfo||{};
  const cli=payload.cli||state.hub?.cli||{};
  const project=payload.project||{};
  const projectMatches=state.project && project.path===state.project;
  if(cli.update_available){
    const from=String(cli.current||'-'), to=String(cli.latest||'-');
    const fullLabel=`${from} → ${to}`;
    const updateIcon='<svg class="update-arrow-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><circle cx="12" cy="12" r="9"/><path d="M12 17V7m0 0-4 4m4-4 4 4"/></svg>';
    el.innerHTML='<button type="button" class="global-update-pill available version-update-label" id="globalVersionUpdateBtn" '+(upgradeFlowMode!=='idle'?'disabled aria-busy="true" ':'')+'title="'+esc(t('updateNow')+' · '+fullLabel)+'" aria-label="'+esc(t('updateNow')+' · '+fullLabel)+'">'+
      '<span class="update-title">'+updateIcon+(cli.channel==='dev'?'<span class="badge update-channel">DEV</span>':'')+'</span>'+
      '<span class="update-versions"><span class="update-from">'+esc(from)+'</span><span class="update-separator">→</span><span class="update-to">'+esc(to)+'</span></span></button>';
    $('#globalVersionUpdateBtn')?.addEventListener('click',()=>openUpgradeDetails(cli.latest||''));
    return;
  }
  if(projectMatches && project.migration_available){
    const from=String(project.framework_version||'?');
    const to=String(cli.current||'?');
    const description=`Project Migration · ${from} → ${to}`;
    // Distinct from software-update (arrow-up) and refresh (circular arrow):
    // layered project assets moving to the target framework.
    const migrationIcon='<svg class="migration-transfer-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">'+
      '<path d="m2.5 6 7-3.5 7 3.5-7 3.5zM2.5 10l7 3.5 4.5-2.25M2.5 14l7 3.5 4-2"/>'+
      '<path d="M14 17h7m-3-3 3 3-3 3"/></svg>';
    el.innerHTML='<button type="button" class="global-update-pill migration" id="globalMigrateBtn" title="'+esc(description)+'" aria-label="'+esc(description)+'">'+
      '<span class="migration-title">'+migrationIcon+'<span>Project Migration</span></span>'+
      '<span class="migration-versions"><span class="migration-from">'+esc(from)+'</span><span class="migration-separator">→</span><span class="migration-to">'+esc(to)+'</span></span></button>';
    $('#globalMigrateBtn')?.addEventListener('click',e=>performProjectMigration(state.project,e.currentTarget));
    return;
  }
  el.innerHTML='<span class="global-update-pill version-label"><span>v'+esc(cli.current||'-')+'</span></span>';
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

let liveListRefreshScheduled=false;
function isLiveBacklogPage() {
  return Boolean(state.project && state.view==='backlog' && !state.detail && state.listPage===1);
}
function updateLiveListRelativeTimes() {
  if(!isLiveBacklogPage())return;
  document.querySelectorAll('.live-updated').forEach(el=>{el.textContent=ago(el.dataset.updatedAt)});
}
function scheduleLiveListRefresh() {
  if(liveListRefreshScheduled)return;
  liveListRefreshScheduled=true;
  const query=listQueryString();
  queueMicrotask(()=>{
    liveListRefreshScheduled=false;
    if(!isLiveBacklogPage() || query!==listQueryString())return;
    preserveViewportAndFocus(()=>refreshList(true));
  });
}

function markContentUpdate(reason='content',changes=[]) {
  if(!state.project)return;
  invalidateListPages();
  if(state.view!=='backlog'){
    queueMicrotask(()=>refresh());
    return;
  }
  if(isLiveBacklogPage()){ scheduleLiveListRefresh(); return; }
  const wasPending=state.pendingContentUpdate;
  const previousReason=state.pendingContentReason;
  state.pendingContentUpdate=true;
  if(reason==='runtime' || !state.pendingContentReason)state.pendingContentReason=reason;
  if(Array.isArray(changes) && changes.length)state.pendingContentChanges=changes;
  if(!wasPending || previousReason!==state.pendingContentReason || changes.length)renderContentUpdatePrompt();
  if(!state.detail)refreshListSummary();
}

function acceptContentRevision(revision='',states=null) {
  if(revision)state.contentRevision=revision;
  if(Array.isArray(states))state.contentStateSnapshot=states;
  state.pendingContentUpdate=false;
  state.pendingContentReason='';
  state.pendingContentChanges=[];
  renderContentUpdatePrompt();
}

async function maybeShowUpgradeRecovery() {
  try{
    const r=await fetch('/api/upgrade-recovery',{cache:'no-store'});
    if(!r.ok)return;
    const body=await r.json(), recovery=body?.recovery;
    if(!recovery?.id||recovery.status!=='rolled_back')return;
    const seen=localStorage.getItem('task-mecca-upgrade-recovery-seen');
    if(seen===recovery.id)return;
    localStorage.setItem('task-mecca-upgrade-recovery-seen',recovery.id);
    const attempted=recovery.attempted_version?(' '+recovery.attempted_version):'';
    const message=state.language==='ko'
      ? `Task Mecca${attempted} 업데이트 후 Web 서버를 정상적으로 시작하지 못해 이전 버전으로 자동 복구했습니다. 현재 Web은 계속 사용할 수 있습니다. 자세한 원인은 task-mecca web logs에서 확인할 수 있습니다.`
      : `Task Mecca${attempted} could not start the Web server after the update, so the previous version was restored automatically. Web remains available. See task-mecca web logs for details.`;
    alert(message);
  }catch(_){}
}

function maybeApplyStableBrandIntro() {
  const cli=state.versionInfo?.cli||state.hub?.cli||{};
  if((cli.channel||'stable')!=='stable')return;
  const marker='task-mecca-stable-brand-intro-v1';
  if(localStorage.getItem(marker)==='1')return;
  // The one-time branding migration must not overwrite an explicit choice.
  // The initial default is already Mecca Dark; existing preferences win.
  localStorage.setItem('task-mecca-palette-version','2');
  localStorage.setItem(marker,'1');
  applyPalette(state.palette);
  applyTheme(state.theme);
}

async function refreshVersionInfo(force=false) {
  const params=new URLSearchParams();
  if(state.project)params.set('project',state.project);
  if(force)params.set('refresh','1');
  try{
    const r=await fetch('/api/version?'+params.toString(),{cache:'no-store'});
    if(!r.ok)throw new Error(`HTTP ${r.status}`);
    const payload=await r.json();
    // A forced check must not silently turn a failed network lookup into
    // "already up to date". Preserve the previous known-good version state.
    if(force && (payload?.cli?.error || !payload?.cli?.latest)){
      return {ok:false,error:payload?.cli?.error || 'Latest version unavailable'};
    }
    state.versionInfo=payload;
    maybeApplyStableBrandIntro();
    renderGlobalUpdateIndicator();
    renderReleaseNotesBadge();
    queueMicrotask(()=>maybeShowCurrentReleaseNote());
    queueMicrotask(()=>maybeShowPendingFrameworkSync());
    queueMicrotask(()=>maybeShowUpgradeRecovery());
    return {ok:true,info:payload};
  }catch(error){
    return {ok:false,error:String(error?.message || error)};
  }
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

let manualRefreshInFlight=false;
let manualRefreshFeedbackTimer=null;
function showManualRefreshFeedback(message,kind='info',temporary=true){
  const element=$('#refreshFeedback');
  if(!element)return;
  clearTimeout(manualRefreshFeedbackTimer);
  element.textContent=message;
  element.className='refresh-feedback visible'+(kind==='error'?' error':kind==='pending'?' pending':'');
  if(temporary){
    manualRefreshFeedbackTimer=setTimeout(()=>{
      element.className='refresh-feedback';
      element.textContent='';
    },kind==='error'?4500:3200);
  }
}
async function refreshAndCheckUpdates(){
  if(manualRefreshInFlight)return;
  manualRefreshInFlight=true;
  const button=$('#refreshBtn');
  if(button){
    button.disabled=true;
    button.classList.add('checking');
    button.setAttribute('aria-busy','true');
  }
  showManualRefreshFeedback(t('refreshChecking'),'pending',false);
  // Independent operations: a failed data reload must not skip the update check.
  const [contentResult,versionResult]=await Promise.allSettled([
    refreshVisibleContent(),refreshVersionInfo(true)
  ]);
  const version=versionResult.status==='fulfilled'
    ? versionResult.value
    : {ok:false,error:String(versionResult.reason)};
  if(contentResult.status==='rejected'){
    showManualRefreshFeedback(t('refreshDataFailed'),'error');
  }else if(!version?.ok){
    showManualRefreshFeedback(t('refreshCheckFailed'),'error');
  }else if(version.info?.cli?.update_available){
    showManualRefreshFeedback(t('refreshUpdateFound',{version:version.info.cli.latest}));
  }else{
    showManualRefreshFeedback(t('refreshUpToDate'));
  }
  if(button){
    button.disabled=false;
    button.classList.remove('checking');
    button.removeAttribute('aria-busy');
  }
  manualRefreshInFlight=false;
}

async function performUpgrade() {
 // One click must create one transaction. Even if the DOM is re-rendered
 // during a long download, another click cannot start a second attempt.
 if(['notes','checking','downloading','verifying','installing','restarting'].includes(upgradeFlowMode))return;
 const session=++upgradeFlowSession;
 stopUpgradeFlowTimers();
 upgradeFlowStartedAt=Date.now();
 upgradeFlowOwnRequest=true;
 const cli=state.versionInfo?.cli||state.hub?.cli||{};
 upgradeFlowTarget=String(cli.current||'-')+' → '+String(cli.latest||'-');
 state.releaseNotePopup=null;
 state.releaseNotePopupMode='installed';
 renderReleaseNoteModal();
 setUpgradeFlowStage('checking');
 startUpgradeFlowPolling(session);
 try{
  const r=await fetch('/api/upgrade',{method:'POST',headers:{'X-Task-Mecca-Action':'1'}});
  const body=await r.json();
  if(r.status===409 && body.status?.active){
   upgradeFlowOwnRequest=false;
   if(upgradeFlowPhases.includes(body.status.phase))setUpgradeFlowStage(body.status.phase);
   if(body.status.phase==='restarting')void observeUpgradeRestart(session);
   return;
  }
  if(!r.ok)throw new Error(body.error||'Upgrade failed');
  if(session!==upgradeFlowSession)return;
  upgradeFlowOwnRequest=false;
  stopUpgradeFlowTimers();
  if(body.restart_required && body.to && body.to!==body.from){
   setUpgradeFlowStage('restarting');
   const recovered=await waitForRestartedWeb(body.to);
   if(recovered===false && session===upgradeFlowSession){
    stopUpgradeFlowTimers();
    setUpgradeFlowStage('failed',upgradeFlowCopy().restartError);
   }
   return;
  }
  setUpgradeFlowStage('completed');
  await refreshVersionInfo(true);
 }catch(error){
  if(session!==upgradeFlowSession)return;
  upgradeFlowOwnRequest=false;
  stopUpgradeFlowTimers();
  setUpgradeFlowStage('failed',String(error?.message||error));
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

async function loadRuntimeRootSessions(page=1) {
  state.runtimeRootListLoading=true;
  state.runtimeRootListError='';
  if(state.view==='workload')render();
  try{
    const params=new URLSearchParams();
    if(state.project)params.set('project',state.project);
    params.set('page',String(Math.max(1,Number(page)||1)));
    params.set('page_size','10');
    params.set('status','previous');
    params.set('include_storage','1');
    const r=await fetch('/api/runtime/root-sessions?'+params.toString(),{cache:'no-store'});
    const body=await r.json();
    if(!r.ok)throw new Error(body.error||`HTTP ${r.status}`);
    state.runtimeRootList=body;
  }catch(e){
    state.runtimeRootListError=String(e?.message||e||'Root Session load failed');
  }finally{
    state.runtimeRootListLoading=false;
  }
  if(state.view==='workload')render();
}
async function toggleRuntimeRootList() {
  state.runtimeRootListOpen=!state.runtimeRootListOpen;
  if(state.runtimeRootListOpen) {
    await loadRuntimeRootSessions(1);
    return;
  }
  render();
}
async function performRootSessionCleanup(button) {
  const rootSessionID=button?.dataset?.runtimeRootCleanup||'';
  const name=button?.dataset?.runtimeRootName||'Root Session';
  const bytes=Number(button?.dataset?.runtimeRootBytes||0);
  if(!rootSessionID)return;
  if(!window.confirm(t('runtimeRootCleanupConfirm',{name,bytes:fmtBytes(bytes)})))return;
  const original=button?.textContent||'';
  if(button){button.disabled=true;button.textContent=t('runtimeCleanupRunning');}
  try{
    const params=new URLSearchParams();
    if(state.project)params.set('project',state.project);
    const qs=params.toString()?`?${params}`:'';
    const r=await fetch('/api/runtime/root-sessions'+qs,{
      method:'POST',
      headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},
      body:JSON.stringify({action:'cleanup',root_session_id:rootSessionID})
    });
    const body=await r.json();
    if(!r.ok)throw new Error(body.error||'Root Session cleanup failed');
    alert(t('runtimeRootCleanupDone',{bytes:fmtBytes(body.result?.reclaimed_bytes||bytes)}));
    await refresh();
    if(state.runtimeRootListOpen)await loadRuntimeRootSessions(Number(state.runtimeRootList?.page||1));
    if(state.runtimeStorageOpen)await loadRuntimeStorage(true);
  }catch(e){
    alert(String(e?.message||e));
    if(button){button.disabled=false;button.textContent=original;}
  }
}

async function performRuntimeHookAction(button) {
  const provider=button?.dataset?.provider||'all';
  const action=button?.dataset?.runtimeHookAction||'enable';
  const scope=button?.dataset?.scope||'device';
  const providerLabel=String(provider).toUpperCase();
  const confirmKey=action==='disable'?'runtimeHookDisableConfirm':'runtimeHookEnableConfirm';
  const confirmation=scope==='device'
    ?t(action==='disable'?'runtimeHookDeviceDisableConfirm':'runtimeHookDeviceEnableConfirm',{provider:providerLabel})
    :scope==='global'
    ?t(action==='disable'?'runtimeHookGlobalDisableConfirm':'runtimeHookGlobalEnableConfirm',{provider:providerLabel})
    :t(confirmKey,{provider:providerLabel});
  if(!window.confirm(confirmation))return;
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
      body:JSON.stringify({provider,action,scope})
    });
    const body=await r.json();
    if(!r.ok)throw new Error(body.error||'Runtime hook action failed');
    await loadRuntimeHookStatus(true);
    await refresh();
    if(action==='enable')openRuntimeHookGuide(provider);
  }catch(e){
    alert(String(e?.message||e));
    if(button){button.disabled=false;button.textContent=original;}
  }
}

function openRuntimeHookGuide(provider) {
  setRuntimeHookGuide(provider,true);
  document.querySelector(`[data-runtime-hook-guide="${provider}"]`)?.scrollIntoView({block:'nearest',behavior:'auto'});
}
function setRuntimeHookGuide(provider,open) {
  state.runtimeHookGuideProvider=open?provider:null;
  document.querySelectorAll('[data-runtime-hook-guide]').forEach(guide=>{
    guide.hidden=!open||guide.dataset.runtimeHookGuide!==provider;
  });
  document.querySelectorAll('[data-runtime-hook-trust]').forEach(button=>{
    button.setAttribute('aria-expanded',String(open&&button.dataset.runtimeHookTrust===provider));
  });
}
function toggleRuntimeHookGuide(provider) {
  setRuntimeHookGuide(provider,state.runtimeHookGuideProvider!==provider);
}


async function performProjectMigration(project,button) {
  if(!project)return;
  const previousLabel=button?.textContent;
  if(button){button.disabled=true;button.textContent=t('migrating');}
  try{
  const body=await migrateProjectWithChoice(project,button);
  if(body.status==='cancelled')return;
    await refreshVersionInfo(false);
    await refreshVisibleContent();
    if(body.instruction_refresh_required||body.legacy_bootstrap)showMigrationResyncModal(body);
  }catch(e){
    alert(String(e?.message||e));
    if(button){button.disabled=false;renderGlobalUpdateIndicator();}
  }finally{
    if(button?.isConnected){button.disabled=false;button.textContent=previousLabel;}
  }
}
async function migrateProjectWithChoice(project,trigger) {
  let choice='',expectedPlan='';
  for(;;){
    const response=await fetch('/api/migrate',{method:'POST',headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},body:JSON.stringify({project,choice,expected_plan:expectedPlan})});
    const result=await response.json();
    if(response.ok)return result;
    if(result.status!=='choice_required')throw Error(result.error||'Migration failed');
    expectedPlan=result.plan_digest||'';
    choice=await migrationChoiceDialog(project,result.modified_files||[],trigger);
    if(choice==='cancel')return {status:'cancelled'};
  }
}
function migrationChoiceDialog(project,files,trigger) {
  const ko=state.language==='ko';
  return new Promise(resolve=>{
    const overlay=document.createElement('div');overlay.className='channel-switch-overlay hub-confirm-overlay';
    const options=[['overwrite',ko?'새 버전으로 덮어쓰기':'Overwrite with the new version'],['backup',ko?'기존 수정사항을 백업하고 진행':'Back up modifications and migrate'],['cancel',ko?'취소':'Cancel']];
    overlay.innerHTML=`<section class="channel-switch-modal migration-choice-modal" role="dialog" aria-modal="true" aria-labelledby="migrationChoiceTitle"><h2 id="migrationChoiceTitle">${esc(ko?'수정한 프레임워크 파일을 어떻게 처리할까요?':'How should modified framework files be handled?')}</h2><p>${esc(ko?'아직 변경된 파일은 없습니다. 백로그·프로젝트 설정·인증정보는 어떤 선택에서도 보존됩니다.':'No files have changed. Backlog, project settings and credentials are preserved for every choice.')}</p><div class="project-path"><code>${esc(project)}</code></div><ul class="migration-choice-files">${files.map(file=>`<li><code>${esc(file)}</code></li>`).join('')}</ul><div class="project-actions">${options.map(([value,label])=>`<button type="button" class="action-btn secondary" data-migration-choice="${value}">${esc(label)}</button>`).join('')}</div></section>`;
    let finished=false;
    const finish=value=>{if(finished)return;finished=true;overlay.remove();trigger?.focus();resolve(value);};
    overlay.querySelectorAll('[data-migration-choice]').forEach(button=>button.addEventListener('click',()=>finish(button.dataset.migrationChoice)));
    overlay.addEventListener('keydown',event=>{if(event.key==='Escape'){event.preventDefault();finish('cancel');}if(event.key==='Tab'){const buttons=[...overlay.querySelectorAll('button')];event.preventDefault();buttons[(buttons.indexOf(document.activeElement)+(event.shiftKey?-1:1)+buttons.length)%buttons.length].focus();}});
    document.body.append(overlay);overlay.querySelector('[data-migration-choice="cancel"]').focus();
  });
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
  const doc=task.document||{}, raw=doc.summary||{}, req=doc.requirements||{}, reason=effectiveAttentionReason(task);
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
function runtimeObservationHealthy(task) {
  if(task?.runtime_activity?.health==='active')return true;
  const id=String(task?.id||'').toUpperCase();
  const roots=state.workload?.root_sessions?.items||state.workload?.root_sessions||[];
  return Array.isArray(roots)&&roots.some(root=>(root.attempts||[]).some(a=>String(a.task_id||'').toUpperCase()===id && !a.terminal && ['starting','running'].includes(String(a.current_state||'').toLowerCase())));
}
function effectiveAttentionReason(task) {
  const reason=task?.attention_reason||null;
  if(!reason)return null;
  if(reason.type!=='runtime_unknown')return reason;
  const hooks=Array.isArray(state.runtimeHookStatus)?state.runtimeHookStatus:[];
  const metadata=task?.runtime_metadata||task?.document?.runtime||{};
  const provider=String(metadata?.runtime_provider||task?.fields?.RuntimeProvider||'').toLowerCase();
  const relevant=provider?hooks.filter(h=>String(h.provider||'').toLowerCase()===provider):hooks.filter(h=>Boolean(h.in_use));
  const hookReady=relevant.some(h=>Boolean(h.configured)&&!Boolean(h.needs_attention));
  // If hooks are healthy or Workload already observes this task running, the
  // generic "check runtime/hook" warning is misleading. Keep correlation
  // diagnostics internal; the user has no hook action to take.
  if(hookReady||runtimeObservationHealthy(task))return null;
  return reason;
}
function humanSummaryCard(task) {
  const s=humanSummary(task), reason=effectiveAttentionReason(task);
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
Object.assign(I18N.ko,{controllerReviewPending:'Controller 검토 대기',controllerReview:'Controller 검토 중',controllerFinalizing:'Controller 완료 처리 중',controllerRecovery:'Controller 복구 필요'});
Object.assign(I18N.en,{controllerReviewPending:'Controller review pending',controllerReview:'Controller review in progress',controllerFinalizing:'Controller finalization in progress',controllerRecovery:'Controller recovery needed'});
function healthLabel(h) { if(String(h).startsWith('controller_'))return ({controller_review_pending:t('controllerReviewPending'),controller_review:t('controllerReview'),controller_finalizing:t('controllerFinalizing'),controller_recovery:t('controllerRecovery')})[h]||h; return ({healthy:t('active'),quiet:t('quiet'),stale:t('stale'),worker_missing:t('workerMissing'),runtime_unknown:t('runtimeUnknown'),awaiting_finalize:t('controllerReviewPending'),needs_user:t('needsUser'),'n/a':'-'})[h] || h; }
function stateLabel(s) { if(String(s).startsWith('controller_'))return healthLabel(s); return ({doing:t('working'),ready:t('ready'),blocked:t('blocked'),hold:t('hold'),done:t('done'),todo:t('todo'),needs_user:t('needsUser'),awaiting_finalize:t('controllerReviewPending'),stalled:t('stalled')})[s] || s; }
function lifecycleEventLabel(label) { return ({Registered:t('eventRegistered'),Assigned:t('eventAssigned'),Started:t('eventStarted'),Waiting:t('eventWaiting'),Resumed:t('eventResumed'),Hold:t('eventHold'),Completed:t('eventCompleted')})[label] || label; }
function lifecycleEvidenceLabel(event) {
  const ko=state.language==='ko';
  const source=event.source==='durable_lifecycle'?(ko?'영속 기록':'Durable record'):
    event.source==='runtime_observed'?(ko?'파일 상태 관측':'File observation'):
    event.source==='execution_ledger'?(ko?'실행 증거':'Execution evidence'):
    event.source==='git'?(ko?'Git 이력':'Git history'):(event.source||'');
  const actor=event.actor?` · ${event.actor}`:'';
  const id=event.event_id?` · ID ${event.event_id}`:'';
  return `${source}${actor}${id}`;
}
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
function runtimeHookProviderGuide(hook,location='workload') {
  const provider=String(hook?.provider||'').toLowerCase();
  const guideId=`runtime-hook-guide-${location}-${provider}`;
  if(provider==='codex') {
    return `<div id="${guideId}" class="runtime-hook-provider-guide" data-runtime-hook-guide="codex" ${state.runtimeHookGuideProvider==='codex'?'':'hidden'}>
      <strong>${esc(t('runtimeHookTrustGuideTitle',{provider:'Codex'}))}</strong>
      <ol><li>${esc(t('runtimeHookCodexTrustStep1'))}</li><li>${esc(t('runtimeHookCodexTrustStep2'))}</li><li>${esc(t('runtimeHookTrustStepVerify'))}</li></ol>
      <a href="https://learn.chatgpt.com/docs/hooks#review-and-trust-hooks" target="_blank" rel="noopener noreferrer">${esc(t('runtimeHookOfficialDocs'))}</a>
    </div>`;
  }
  if(provider==='claude') {
    return `<div id="${guideId}" class="runtime-hook-provider-guide" data-runtime-hook-guide="claude" ${state.runtimeHookGuideProvider==='claude'?'':'hidden'}>
      <strong>${esc(t('runtimeHookTrustGuideTitle',{provider:'Claude Code'}))}</strong>
      <ol><li>${esc(t('runtimeHookClaudeTrustStep1'))}</li><li>${esc(t('runtimeHookClaudeTrustStep2'))}</li><li>${esc(t('runtimeHookTrustStepVerify'))}</li></ol>
      <a href="https://code.claude.com/docs/en/hooks#workspace-trust" target="_blank" rel="noopener noreferrer">${esc(t('runtimeHookOfficialDocs'))}</a>
    </div>`;
  }
  return '';
}
function runtimeHookOnboardingBanner() {
  if(!state.project || state.runtimeHookStatusLoading || !Array.isArray(state.runtimeHookStatus))return '';
  const hooks=state.runtimeHookStatus;
  if(!hooks.length)return '';
  const attention=hooks.filter(h=>Boolean(h.in_use)&&Boolean(h.needs_attention));
  const noneConfigured=hooks.every(h=>!Boolean(h.configured));
  if(!attention.length && !noneConfigured)return '';
  const targets=attention.length?attention:hooks;
  const providers=targets.map(h=>String(h.provider||'').toUpperCase()).filter(Boolean).join(' · ');
  const title=attention.length
    ? t('runtimeHookInUseTitle',{provider:providers})
    : t('runtimeHookOnboardingTitle');
  const body=attention.length
    ? t('runtimeHookInUseBody',{provider:providers})
    : t('runtimeHookOnboardingBody');
  const actions=targets.map(h=>`<button class="runtime-hook-onboarding-action" data-runtime-hook-action="enable" data-scope="device" data-provider="${esc(h.provider||'')}">${esc(t('runtimeHookApproveSetup'))}</button><button class="runtime-hook-onboarding-action secondary" data-runtime-hook-action="disable" data-scope="device" data-provider="${esc(h.provider||'')}">${esc(t('runtimeHookDisable'))}</button><button class="runtime-hook-onboarding-action secondary" data-runtime-hook-trust="${esc(h.provider||'')}" aria-expanded="${state.runtimeHookGuideProvider===h.provider}" aria-controls="runtime-hook-guide-onboarding-${esc(h.provider||'')}">${esc(t('runtimeHookTrustAction',{provider:String(h.provider||'').toUpperCase()}))}</button>`).join('');
  const guides=targets.map(h=>runtimeHookProviderGuide(h,'onboarding')).join('');
  return `<div class="global-access runtime-hook-onboarding">
    <div class="runtime-hook-onboarding-copy">
      <strong>${esc(title)}</strong>
      <span>${esc(body)}</span>
      <span class="runtime-hook-new-root">${esc(t('runtimeHookNewRootRequired'))}</span>
      ${guides}
    </div>
    <div class="runtime-hook-onboarding-actions">${actions}</div>
  </div>`;
}

async function loadRuntimeHookStatus(force=false) {
  if(!state.project)return;
  if(state.runtimeHookStatusLoading&&!force)return;
  state.runtimeHookStatusLoading=true;
  try{
    const params=new URLSearchParams();
    params.set('project',state.project);
    if(state.backlog)params.set('backlog',state.backlog);
    const r=await fetch('/api/runtime/hooks?'+params.toString(),{cache:'no-store'});
    if(!r.ok)throw new Error(`HTTP ${r.status}`);
    const body=await r.json();
    state.runtimeHookStatus=Array.isArray(body.hooks)?body.hooks:[];
  }catch(_){
    state.runtimeHookStatus=null;
  }finally{
    state.runtimeHookStatusLoading=false;
    render();
  }
}

Object.assign(I18N.ko,{currentUserAttention:'사용자 판단이 필요한 작업',userAttentionAction:'필요한 판단·조치',userAttentionFallback:'백로그를 열어 필요한 판단을 확인해 주세요.'});
Object.assign(I18N.en,{currentUserAttention:'Tasks awaiting your decision',userAttentionAction:'Decision or action needed',userAttentionFallback:'Open the backlog to review the decision needed.'});
function currentUserAttention(payload) {
  const items=payload?.all_items||{}, seen=new Set(), rows=[];
  for(const reason of payload?.attention||[]){
    if(reason.type!=='user_intervention'||reason.audience==='controller'||reason.resolved||reason.resolved_at||reason.history||reason.active===false||['resolved','closed','dismissed'].includes(reason.status||reason.state))continue;
    const task=items[reason.id];
    if(!task||!task.id||seen.has(task.id)||!['doing','hold'].includes(task.file_state)||(task.location&&task.location!=='active')||String(task.state||'').startsWith('controller_'))continue;
    seen.add(task.id);
    rows.push({id:task.id,title:titleOf(task),message:reason.message||'',action:reason.resume_condition||task.fields?.재개조건||task.document?.summary?.follow_up||t('userAttentionFallback')});
  }
  return rows.sort((a,b)=>a.id.localeCompare(b.id,undefined,{numeric:true}));
}
function updateCurrentUserAttention(payload) {
  if(!Array.isArray(payload?.attention))return;
  const key=state.project+'|'+state.backlog, observed=Date.parse(payload.snapshot_at||'');
  if(state.userAttentionContext===key&&state.userAttentionObservedAt>0&&(!Number.isFinite(observed)||observed<state.userAttentionObservedAt))return;
  if(state.userAttentionContext!==key)state.userAttentionObservedAt=0;
  state.userAttentionContext=key;
  if(Number.isFinite(observed))state.userAttentionObservedAt=observed;
  state.userAttention=currentUserAttention(payload);
  const sourceBacklog=payload.backlog_selection?.selected||state.backlog||state.attentionScopes[state.project+'|']||'';
  if(sourceBacklog)storeCommonUserAttention({...payload,backlog_selection:{...payload.backlog_selection,selected:sourceBacklog}},state.project,state.backlog);
  renderUserAttention();
}
function clearCurrentUserAttention() {
  state.userAttention=[];state.userAttentionContext='';state.userAttentionObservedAt=0;
  renderUserAttention();
}
function userAttentionMarkup(rows) {
  const params=new URLSearchParams({project:state.project});
  if(state.backlog)params.set('backlog',state.backlog);
  return `<h2 id="userAttentionHeading">${esc(t('currentUserAttention'))}<span>${rows.length}</span></h2><ul>${rows.map(row=>`<li><a data-user-attention-task="${esc(row.id)}" href="/tasks/${encodeURIComponent(row.id)}?${esc(params.toString())}"><span class="user-attention-id">${esc(row.id)}</span><strong>${esc(row.title)}</strong></a><div class="user-attention-decision">${row.message?`<p>${esc(row.message)}</p>`:''}<p><b>${esc(t('userAttentionAction'))}</b> ${esc(row.action)}</p></div></li>`).join('')}</ul>`;
}
Object.assign(I18N.ko,{commonAttention:'현재 알림',commonUserCount:'사용자 판단 {count}건',commonSessionCount:'세션 경고 {count}건',commonSessionHeading:'현재 세션 경고',commonSessionLink:'관련 세션 보기',commonTaskLink:'관련 작업 보기'});
Object.assign(I18N.en,{commonAttention:'Current notifications',commonUserCount:'User decisions: {count}',commonSessionCount:'Session warnings: {count}',commonSessionHeading:'Current session warnings',commonSessionLink:'View related session',commonTaskLink:'View related task'});
function storeCommonUserAttention(payload,project,backlog) {
  if(!project||!Array.isArray(payload?.attention))return;
  const requestedBacklog=backlog||'';
  backlog=payload.backlog_selection?.selected||requestedBacklog;
  const key=project+'|'+backlog,observed=Date.parse(payload.snapshot_at||''),prior=state.commonUserAttention[key];
    if(prior?.observed>0&&(!Number.isFinite(observed)||observed<prior.observed))return;
  const nextRows=currentUserAttention(payload);
  if(prior&&observed===prior.observed&&!prior.rows.length&&nextRows.length)return;
  if(payload.backlog_selection?.selected){
    const context=project+'|'+requestedBacklog,changed=state.attentionScopes[context]!==backlog;
    const priorScopeTime=state.attentionScopeObserved[context]||0;
    if(!priorScopeTime||(Number.isFinite(observed)&&(observed>priorScopeTime||(!changed&&observed===priorScopeTime)))){
      if(changed&&priorScopeTime)invalidateProjectPages(project,payload._listSnapshot?listContextKey():null);
      state.attentionScopes[context]=backlog;
      if(Number.isFinite(observed))state.attentionScopeObserved[context]=observed;
      if(changed&&project===state.project&&requestedBacklog===state.backlog)ensureAttentionStream();
    }
  }
  const signature=String(payload.content_revision||'')+'|'+attentionPayloadRevision(payload);
  const previousSignature=listSourceSignatures.get(key);
  if(previousSignature!==undefined&&previousSignature!==signature&&!payload._listSnapshot)invalidateProjectPages(project);
  listSourceSignatures.set(key,signature);
  state.commonUserRevision[key]=(state.commonUserRevision[key]||0)+1;
  state.commonUserAttention[key]={project,backlog:backlog||payload.backlog_selection?.selected||'',observed:Number.isFinite(observed)?observed:0,rows:nextRows};
  renderUserAttention();

  if(document.querySelector('[data-view="issues"]')&&project===state.project&&(requestedBacklog===state.backlog||backlog===state.backlog)){const selectedContext=state.backlog;queueMicrotask(()=>{if(project===state.project&&selectedContext===state.backlog)refreshDiagnostics(project,selectedContext,backlog+'|'+String(payload.content_revision||''));});}
}
function commonAttentionUsers() {
 const rows=[],seen=new Set();
 for(const source of Object.values(state.commonUserAttention)){
  for(const row of source.rows){const key=source.project+'|'+source.backlog+'|'+row.id;if(seen.has(key))continue;seen.add(key);rows.push({...row,project:source.project,backlog:source.backlog});}
 }
 return rows;
}
function commonAttentionMarkup(users,warnings,expanded=false) {
 const userRows=users.map(row=>{
  const params=new URLSearchParams({project:row.project});if(row.backlog)params.set('backlog',row.backlog);
  return `<li><a data-user-attention-task="${esc(row.id)}" href="/tasks/${encodeURIComponent(row.id)}?${esc(params.toString())}"><span class="user-attention-id">${esc(row.id)}</span><strong>${esc(row.title)}</strong></a><div class="user-attention-decision"><p class="common-attention-project">${esc(row.project)}</p>${row.message?`<p>${esc(row.message)}</p>`:''}<p><b>${esc(t('userAttentionAction'))}</b> ${esc(row.action)}</p></div></li>`;
 }).join('');
 const sessionRows=warnings.map(item=>{
  const params=new URLSearchParams({project:item.project||''});
  const taskLink=item.task_id?`<a href="/tasks/${encodeURIComponent(item.task_id)}?${esc(params.toString())}">${esc(t('commonTaskLink'))} · ${esc(item.task_id)}</a>`:'';
  params.set('view','workload');
  return `<li><div><strong>${esc(operationText(item.kind==='monitor_gap'?'gap':item.kind))}</strong><p>${esc(item.task_id||item.agent_path||item.attempt_id||item.project)}</p></div><div class="user-attention-decision"><p class="common-attention-project">${esc(item.project||'')}</p><p>${esc(operationEvidence(item.evidence||''))}</p>${item.agent_path?`<p>${esc(item.agent_path)}</p>`:''}${item.attempt_id?`<p>${esc(item.attempt_id)}</p>`:''}<p><b>${esc(operationText('action'))}</b> ${esc(operationAction(item)||'')}</p><div class="common-attention-links">${taskLink}<a href="/?${esc(params.toString())}${item.attempt_id?'#runtime-attempt-'+encodeURIComponent(item.attempt_id):''}">${esc(t('commonSessionLink'))}</a></div></div></li>`;
 }).join('');
 return `<details class="common-attention"${state.commonAttentionOpen||expanded?' open':''}><summary><strong>${esc(t('commonAttention'))}</strong><span>${esc(t('commonUserCount',{count:users.length}))}</span><span>${esc(t('commonSessionCount',{count:warnings.length}))}</span></summary><div class="common-attention-body"><a class="common-center-link" href="/?view=notifications" data-notification-center>${esc(t('notificationCenter'))} · ${esc(t('notificationCurrent'))}</a>${users.length?`<h2>${esc(t('currentUserAttention'))}</h2><ul>${userRows}</ul>`:''}${warnings.length?`<h2>${esc(t('commonSessionHeading'))}</h2><ul>${sessionRows}</ul>`:''}</div></details>`;
}
function renderUserAttention() {
 const el=$('#userAttention');if(!el)return;
 const users=commonAttentionUsers(),warnings=state.sessionWarnings;
 el.hidden=!users.length&&!warnings.length;
 if(el.hidden){el.innerHTML='';state.commonAttentionOpen=false;return;}
 const markup=commonAttentionMarkup(users,warnings);
 if(el.innerHTML===markup)return;
 el.innerHTML=markup;
 el.querySelector?.('details')?.addEventListener('toggle',event=>{state.commonAttentionOpen=event.target.open;});
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
let notificationWorkerPromise=null;
async function notificationWorker() {
  if(!('serviceWorker' in navigator)||!window.isSecureContext)return null;
  // Registration and ready may involve disk/worker startup on mobile.
  // Share one promise instead of repeating it for every event and toggle.
  if(!notificationWorkerPromise){
    notificationWorkerPromise=(async()=>{
      await navigator.serviceWorker.register('/sw.js',{scope:'/'});
      return navigator.serviceWorker.ready;
    })().catch(()=>{notificationWorkerPromise=null;return null});
  }
  return notificationWorkerPromise;
}

// Background Push is opt-in and distinct from the open-page Notification API.
let webPushHeartbeatAt=0;
let webPushFeedback='';
function pushSupport(){
 const capability=notificationCapability();
 if(capability.mode!=='supported')return capability.mode;
 if(!('serviceWorker' in navigator)||!('PushManager' in window))return 'no-push';
 return 'supported';
}
function pushKeyBytes(key){
 const raw=atob(key.replace(/-/g,'+').replace(/_/g,'/').padEnd(Math.ceil(key.length/4)*4,'='));
 return Uint8Array.from(raw,character=>character.charCodeAt(0));
}
function pushSubscriptionJSON(subscription){
 const json=subscription.toJSON();
 return {endpoint:json.endpoint,p256dh:json.keys?.p256dh||'',auth:json.keys?.auth||'',client:notificationBrowserClient()};
}
function currentPushKinds(project){
 const kinds={};
 for(const kind of notificationKinds){
  kinds[kind]=webNotificationEnabled(project,kind,kind);
 }
 return kinds;
}
async function pushRequest(project,body){
 const response=await fetch('/api/notifications/push?project='+encodeURIComponent(project),{
  method:'POST',headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},
  body:JSON.stringify(body),cache:'no-store'
 });
 const data=await response.json().catch(()=>({}));
 if(!response.ok)throw Error(data.error||'Push HTTP '+response.status);
 return data;
}
function pushProjects(){
 return [...new Set([state.project,...(state.projectNotificationSettings||[]).map(row=>row.path)].filter(Boolean))];
}
let cachedPushSubscriptionPromise=null;
async function activePushSubscription(){
 if(!cachedPushSubscriptionPromise){
  cachedPushSubscriptionPromise=(async()=>{
   const worker=await notificationWorker();
   return worker?.pushManager?.getSubscription()||null;
  })().catch(error=>{cachedPushSubscriptionPromise=null;throw error});
 }
 return cachedPushSubscriptionPromise;
}
async function enableBackgroundPush(){
 if(pushSupport()!=='supported')throw Error(state.language==='ko'?'이 브라우저에서는 백그라운드 알림을 사용할 수 없습니다. HTTPS 및 iOS 홈 화면 앱 조건을 확인하세요.':'Background Push is unavailable. Check HTTPS and iOS installed app requirements.');
 // Safari/iOS requires this request to originate from a direct user gesture.
 const permission=Notification.permission==='granted'?'granted':await Notification.requestPermission();
 if(permission!=='granted')throw Error(state.language==='ko'?'브라우저 알림 권한이 허용되지 않았습니다.':'Notification permission was not granted.');
 const worker=await notificationWorker();
 if(!worker?.pushManager)throw Error('PushManager unavailable');
 const project=state.project||pushProjects()[0];
 if(!project)throw Error('Select a project before enabling Push');
 const response=await fetch('/api/notifications/push?project='+encodeURIComponent(project),{cache:'no-store'});
 if(!response.ok)throw Error('Unable to obtain VAPID public key');
 const config=await response.json();
 let subscription=await worker.pushManager.getSubscription();
 const serverKey=pushKeyBytes(config.public_key);
 if(subscription?.options?.applicationServerKey){
  const current=new Uint8Array(subscription.options.applicationServerKey);
  if(current.length!==serverKey.length||current.some((v,i)=>v!==serverKey[i])){
   await subscription.unsubscribe();subscription=null;
  }
 }
 if(!subscription)subscription=await worker.pushManager.subscribe({userVisibleOnly:true,applicationServerKey:serverKey});
 cachedPushSubscriptionPromise=Promise.resolve(subscription);
 const details=pushSubscriptionJSON(subscription);
 for(const path of pushProjects()){
  await pushRequest(path,{action:'subscribe',...details,enabled:webNotificationSettings().enabled!==false&&webNotificationSettings().projects?.[path]?.enabled!==false,kinds:currentPushKinds(path)});
 }
 localStorage.setItem('task-mecca-push-enabled-v1','1');
 webPushFeedback=state.language==='ko'?'백그라운드 Push 구독이 연결됐습니다. 테스트 전송으로 확인하세요.':'Background Push subscription is registered. Run a test to verify delivery.';
}
async function syncBackgroundPush(force=false,strict=false,projects=null){
 if(pushSupport()!=='supported'||Notification.permission!=='granted'||localStorage.getItem('task-mecca-push-enabled-v1')!=='1')return;
 if(!force&&Date.now()-webPushHeartbeatAt<30000)return;
 webPushHeartbeatAt=Date.now();
 const sub=await activePushSubscription();
 if(!sub)return;
 const details=pushSubscriptionJSON(sub),projectsToSync=projects||pushProjects();
 // Parallel independent project writes; never serialize round trips on a click.
 const results=await Promise.allSettled(projectsToSync.map(project=>pushRequest(project,{
  action:'subscribe',...details,
  enabled:webNotificationSettings().enabled!==false&&webNotificationSettings().projects?.[project]?.enabled!==false,
  kinds:currentPushKinds(project)
 })));
 const failures=results.flatMap((result,index)=>result.status==='rejected'?[projectsToSync[index]+': '+(result.reason?.message||result.reason)]:[]);
 if(strict&&failures.length)throw Error(failures.join('\n'));
}
async function disableBackgroundPush(){
 const worker=await notificationWorker(),sub=await worker?.pushManager?.getSubscription();
 if(sub){
  for(const project of pushProjects()){
   await pushRequest(project,{action:'unsubscribe',endpoint:sub.endpoint});
  }
  await sub.unsubscribe();
 }
 cachedPushSubscriptionPromise=null;
 localStorage.removeItem('task-mecca-push-enabled-v1');
 webPushFeedback=state.language==='ko'?'백그라운드 Push 구독을 해제했습니다.':'Background Push subscription removed.';
}
async function testBackgroundPush(){
 const worker=await notificationWorker(),sub=await worker?.pushManager?.getSubscription();
 if(!sub||!state.project)throw Error('Enable Push for a selected project first');
 const response=await pushRequest(state.project,{action:'test',endpoint:sub.endpoint});
 if(!response.accepted)throw Error('Push service did not accept the test');
 webPushFeedback=state.language==='ko'?'Push 서비스가 테스트 요청을 접수했습니다. OS 알림 표시를 확인하세요.':'Push service accepted the test. Verify OS notification display.';
}
function pushStatusMarkup(){
 const mode=pushSupport(),connected=localStorage.getItem('task-mecca-push-enabled-v1')==='1';
 const reason=mode==='supported'
  ?(connected?'브라우저에 백그라운드 Push가 연결되어 있습니다.':'현재 페이지 알림과 별개로 탭이 닫혀도 받을 Push를 설정할 수 있습니다.')
  :mode==='ios-home'?'iPhone/iPad에서는 홈 화면에 추가한 웹앱에서만 Push를 사용할 수 있습니다.'
  :mode==='insecure'?'Push는 HTTPS 보안 접속에서만 사용할 수 있습니다.'
  :'이 브라우저에서는 Push API가 지원되지 않습니다.';
 const buttons=mode==='supported'
  ?'<div class="telegram-actions"><button type="button" class="action-btn" id="pushEnable">'+(connected?'재연결':'백그라운드 알림 켜기')+'</button>'+
    (connected?'<button type="button" class="secondary-btn" id="pushTest">테스트</button><button type="button" class="secondary-btn" id="pushDisable">해제</button>':'')+'</div>'
  :'';
 return '<div class="mini-panel push-settings"><strong>Web Push · 백그라운드 알림</strong><p class="muted">'+esc(state.language==='ko'?reason:
   mode==='supported'?(connected?'Push subscription is enabled.':'Enable background notifications even when the tab is closed.'):'Push requires HTTPS and a supported browser; iOS needs a Home Screen web app.')+
   '</p>'+buttons+(webPushFeedback?'<p role="status">'+esc(webPushFeedback)+'</p>':'')+
   '<p class="muted">'+esc(state.language==='ko'?'알림 권한, 운영체제 설정과 인터넷 연결이 필요합니다. 서버의 전송 성공은 기기 표시·읽음 보장이 아닙니다.':'Requires permission, OS settings and connectivity. Server acceptance does not confirm device display.')+'</p></div>';
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
function notificationBrowserClient(){
 const agent=navigator.userAgent||'';
 if(/SamsungBrowser/i.test(agent))return 'samsung';
 if(/Edg\//i.test(agent))return 'edge';
 if(/Firefox|FxiOS/i.test(agent))return 'firefox';
 if(/Chrome|CriOS/i.test(agent))return 'chrome';
 if(/Safari/i.test(agent))return 'safari';
 return 'other';
}
async function webNotificationDelivery(project,action,eventID='',token=''){
 const response=await fetch('/api/notifications/web?project='+encodeURIComponent(project),{
  method:'POST',
  headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},
  body:JSON.stringify({action,event_id:eventID,token,client:notificationBrowserClient()})
 });
 if(!response.ok)throw Error('Web notification coordination unavailable (HTTP '+response.status+')');
 return response.json();
}
async function sendBrowserNotification(kind,task,reason,key,eventKind=kind,eventID='',deliveryProject=state.project,sourceBacklog='',sourceValidity=null) {
  // Legacy snapshot-delta alerts have no canonical event ID and cannot be
  // deduplicated across devices. The server journal is the sole push source.
  if(notificationSeenSet().has(key)||browserNotificationsPending.has(key))return;
  if(!eventID)return; // do not push noncanonical, per-browser snapshot deltas
  if(!webNotificationEnabled(deliveryProject,eventKind,kind)){
    rememberNotification(key);
    return;
  }
  const capability=notificationCapability();
  if(capability.mode!=='supported'||Notification.permission!=='granted')return;
  const projectName=(deliveryProject||'').split(/[\\/]/).pop()||'Task Mecca';
  const title=kind==='completed'
    ? `${projectName} · ${task.id} 완료`
    : `${projectName} · ${task.id} · ${reason?.title||t(kind==='stalled'?'notifyStalled':'notifyIntervention')}`;
  const body=kind==='completed'
    ? (titleOf(task)||task.id)
    : [titleOf(task),reason?.message,reason?.resume_condition].filter(Boolean).join(' · ');
  const tag=`task-mecca:${deliveryProject}:${sourceBacklog}:${task.id}:${kind}`;
  const targetParams=new URLSearchParams({project:deliveryProject||''});if(sourceBacklog)targetParams.set('backlog',sourceBacklog);
  const target=`/tasks/${encodeURIComponent(task.id)}?${targetParams}`;
  browserNotificationsPending.add(key);
  let lease="",displayRequested=false;
  try {
    const registration=await notificationWorker();
    if(!webNotificationEnabled(deliveryProject,eventKind,kind)){rememberNotification(key);return;}
    if(notificationCapability().mode!=='supported'||Notification.permission!=='granted')return;
    const latest=sourceValidity&&state.notificationSnapshots?.[sourceValidity.sourceKey];
    if(latest){
      const current=latest.tasks?.[task.id];
      if((sourceValidity.reasonType&&current?.reason_type!==sourceValidity.reasonType)||(eventKind==='completed'&&current&&current.file_state!=='done')){rememberNotification(key);return;}
    }
    // Claim must be shared by every tab and device. It is obtained immediately
    // before display, after permissions and obsolete-reason checks.
    const claim=await webNotificationDelivery(deliveryProject,'claim',eventID);
    if(!claim.granted){
      if(claim.state!=='claimed'&&claim.state!=='failed')rememberNotification(key);
      return;
    }
    lease=claim.token;
    if(registration){
      await registration.showNotification(title,{body,tag,data:{url:target}});
    }else{
      const n=new Notification(title,{body,tag});
      n.onclick=()=>{ window.focus(); window.location.href=target; n.close(); };
    }
    displayRequested=true;
    recordBrowserDelivery(deliveryProject,task,eventKind,key,eventID);
    rememberNotification(key);
    // Ack loss is *not* a failed display. Server treats an expired claim as
    // uncertain rather than blindly replaying a possible OS notification.
    await webNotificationDelivery(deliveryProject,'ack',eventID,lease);
  } catch(_) {
    if(lease&&!displayRequested){
      try{await webNotificationDelivery(deliveryProject,'fail',eventID,lease)}catch(_){}
    }
  } finally { browserNotificationsPending.delete(key); }
}
function processTaskNotifications(snapshot,context={}) {
  const project=context.project??state.project,requestedBacklog=context.backlog??state.backlog;
  if(!project||!snapshot||(snapshot.project_path&&snapshot.project_path!==project))return;
  if(requestedBacklog&&snapshot.backlog_selection?.selected&&snapshot.backlog_selection.selected!==requestedBacklog)return;
  const sourceBacklog=snapshot.backlog_selection?.selected||requestedBacklog||state.attentionScopes[project+'|']||'';
  const sourceKey=project+'|'+sourceBacklog,observed=Date.parse(snapshot.snapshot_at||'');
  state.notificationSnapshots||={};
  const scopeAt=state.attentionScopeObserved[project+'|'+requestedBacklog]||0;
  if(scopeAt&&(!Number.isFinite(observed)||observed<scopeAt))return;
  const prior=state.notificationSnapshots[sourceKey],common=state.commonUserAttention[sourceKey],latest=Math.max(prior?.observed||0,common?.observed||0);
  if(latest&&(!Number.isFinite(observed)||observed<latest))return;
  const activeReasons=Object.values(snapshot.all_items||{}).filter(task=>task.attention_reason&&task.attention_reason.audience!=='controller').length;
  if(prior?.observed&&observed===prior.observed&&!prior.activeReasons&&activeReasons)return;
  state.notificationSnapshots[sourceKey]={observed:Number.isFinite(observed)?observed:0,activeReasons,tasks:Object.fromEntries(Object.values(snapshot.all_items||{}).map(task=>[task.id,{file_state:task.file_state,reason_type:task.attention_reason?.type}]))};
  if(context.updateCurrent!==false&&project===state.project)updateCurrentUserAttention(snapshot);
  const current=snapshot.all_items||{};
  const previous=state.previousTasksByProject[sourceKey]||null;
  const serverEvents=Array.isArray(snapshot.notification_events)?snapshot.notification_events:[];
  const completedByServer=new Set(),reasonsByServer=new Set();

  serverEvents.forEach(event=>{
    if(!event?.kind||!event.task_id||!event.id)return;
    if(['completion_pending','controller_completion_review'].includes(event.kind)||['completion_pending','controller_completion_review'].includes(event.reason_type))return;
    if(event.kind==='completed'&&current[event.task_id]?.file_state!=='done')return;
    if(event.kind==='completed')completedByServer.add(event.task_id);
    const task=current[event.task_id]||(snapshot.done_items||[]).find(x=>x.id===event.task_id);
    if(!task)return;
    const key=`server:${project}:${event.id}`;
    if(event.reason_type&&['intervention','approval','stalled','interrupted','runtime_unknown'].includes(event.kind)&&task.attention_reason?.type!==event.reason_type){rememberNotification(key);return;}
    const eventAt=Date.parse(event.at||'');
    if(Number.isFinite(eventAt) && Date.now()-eventAt>24*60*60*1000){
      rememberNotification(key);
      return;
    }
    if(event.reason_type)reasonsByServer.add(event.task_id+'|'+event.reason_type);
    const kind=event.kind==='stalled'?'stalled':event.kind==='completed'?'completed':'intervention';
    const reason=kind==='completed'?null:{title:event.reason_type||event.kind,message:event.message||'',resume_condition:event.resume_condition||''};
    sendBrowserNotification(kind,task,reason,key,event.kind,event.id,project,sourceBacklog,{sourceKey,reasonType:event.reason_type});
  });

  Object.values(current).forEach(task=>{
    const reason=task.attention_reason||null;
    if(reason&&!reasonsByServer.has(task.id+'|'+reason.type)&&reason.audience!=='controller'&&!['completion_pending','controller_completion_review'].includes(reason.type)&&!String(task.state||'').startsWith('controller_')){
      const kind=reason.type==='runtime_stalled'?'stalled':'intervention';
      const key=`${sourceKey}:${task.id}:${kind}:${reason.type||''}:${task.updated_at||task.mtime||''}`;
      if(notificationSeenSet().has(key.replace(sourceKey+':',project+':'))){rememberNotification(key);return;}
      sendBrowserNotification(kind,task,reason,key,kind,'',project,sourceBacklog,{sourceKey,reasonType:reason.type});
    }
    if(previous&&!completedByServer.has(task.id)){
      const before=previous[task.id];
      if(task.file_state==='done' && before && before.file_state!=='done'){
        const key=`${sourceKey}:${task.id}:completed:${task.completed_at||task.mtime||task.updated_at||''}`;
        if(notificationSeenSet().has(key.replace(sourceKey+':',project+':'))){rememberNotification(key);return;}
        sendBrowserNotification('completed',task,null,key,'completed','',project,sourceBacklog,{sourceKey});
      }
    }
  });
  const compact={...(previous||{})};
  Object.values(current).forEach(task=>compact[task.id]={file_state:task.file_state,state:task.state,updated_at:task.updated_at});
  state.previousTasksByProject[sourceKey]=compact;
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
function applyTelegramStatus(project,status){
 if(!status||typeof status!=='object')return;
 if(project===state.project)state.telegramStatus=status;
 const projectRow=(state.projectNotificationSettings||[]).find(row=>row.path===project);
 if(projectRow)projectRow.status=status;
}
async function refreshNotificationConfiguration(){
 // Do not block a successful POST behind a stale GET that may still be
 // pending. The POST response already updated the visible local cache.
 if(state.projectNotificationSettingsLoading){
   const pending=state.projectNotificationSettingsPromise;
   void Promise.resolve(pending).then(()=>refreshNotificationConfiguration()).catch(()=>{});
   return;
 }
 await loadProjectNotificationSettings();
 if(state.project)await loadTelegramStatus();
}
async function loadTelegramStatus() {
 const project=state.project;if(!project||(state.telegramLoading&&state.telegramLoadingProject===project))return;
 const request=(state.telegramStatusRequest||0)+1;state.telegramStatusRequest=request;state.telegramLoadingProject=project;state.telegramLoading=true;const revision=state.projectNotificationSettingsRevision||0;
 try{const response=await fetch('/api/notifications/telegram?project='+encodeURIComponent(project),{cache:'no-store'});const status=response.ok?await response.json():null;if(request===state.telegramStatusRequest&&project===state.project&&revision===(state.projectNotificationSettingsRevision||0))state.telegramStatus=status;}
 catch(_){if(request===state.telegramStatusRequest&&project===state.project&&revision===(state.projectNotificationSettingsRevision||0))state.telegramStatus=null;}
 finally{if(request===state.telegramStatusRequest)state.telegramLoading=false;}
}
async function telegramAction(action,payload={}) {
  const project=state.project;if(!project)return;
  if(action!=='test')state.projectNotificationSettingsRevision=(state.projectNotificationSettingsRevision||0)+1;
  const r=await fetch('/api/notifications/telegram?project='+encodeURIComponent(project),{
    method:'POST',headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},
    body:JSON.stringify({action,...payload})
  });
  const body=await r.json().catch(()=>({}));
  if(!r.ok)throw new Error(body.error||('HTTP '+r.status));
  if(action!=='test'){
    applyTelegramStatus(project,body);
    // The mutation response is authoritative. Avoid an extra two GET round trips
    // on every simple toggle; only connection/setup changes need a fresh inventory.
    if(!['kinds','project_enabled'].includes(action))void refreshNotificationConfiguration().catch(()=>{});
  }
  return body;
}
async function loadProjectNotificationSettings() {
  if(state.projectNotificationSettingsLoading){
    await state.projectNotificationSettingsPromise;
    if(state.projectNotificationSettingsLoadedRevision===(state.projectNotificationSettingsRevision||0))return;
  }
  const revision=state.projectNotificationSettingsRevision||0;
  state.projectNotificationSettingsLoading=true;
  state.projectNotificationSettingsError='';
  const promise=(async()=>{
    try{
      const response=await fetch('/api/notifications/projects',{cache:'no-store'});
      const body=await response.json();
      if(!response.ok)throw new Error(body.error||('HTTP '+response.status));
      if(revision!==(state.projectNotificationSettingsRevision||0))return;
      state.projectNotificationSettings=body.projects||[];
      state.projectNotificationSettingsLoadedRevision=revision;
    }catch(error){
      if(revision===(state.projectNotificationSettingsRevision||0))state.projectNotificationSettingsError=String(error?.message||error);
    }
  })();
  state.projectNotificationSettingsPromise=promise;
  try{await promise;}finally{state.projectNotificationSettingsLoading=false;}
}

async function setProjectNotificationEnabled(project,enabled) {
  state.projectNotificationSettingsRevision=(state.projectNotificationSettingsRevision||0)+1;
  const existing=(state.projectNotificationSettings||[]).find(row=>row.path===project)?.status;
  const prior=existing?{...existing,kinds:{...existing.kinds}}:null;
  if(existing)applyTelegramStatus(project,{...existing,project_enabled:enabled});
  try {
  const response=await fetch('/api/notifications/telegram?project='+encodeURIComponent(project),{
    method:'POST',headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},
    body:JSON.stringify({action:'project_enabled',project_enabled:enabled})
  });
  const body=await response.json().catch(()=>({}));
  if(!response.ok)throw new Error(body.error||('HTTP '+response.status));
  applyTelegramStatus(project,body);
  } catch(error) {
    if(prior)applyTelegramStatus(project,prior);
    throw error;
  }
}
async function projectTelegramAction(project,action,payload={}) {
  state.projectNotificationSettingsRevision=(state.projectNotificationSettingsRevision||0)+1;
  const existing=(state.projectNotificationSettings||[]).find(row=>row.path===project)?.status;
  const prior=existing?{...existing,kinds:{...existing.kinds}}:null;
  if(action==='kinds'&&existing)applyTelegramStatus(project,{...existing,kinds:{...payload.kinds}});
  try {
  const response=await fetch('/api/notifications/telegram?project='+encodeURIComponent(project),{
    method:'POST',headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},body:JSON.stringify({action,...payload})
  });
  const body=await response.json().catch(()=>({}));
  if(!response.ok)throw new Error(body.error||('HTTP '+response.status));
  applyTelegramStatus(project,body);
  if(!['kinds','project_enabled'].includes(action))void refreshNotificationConfiguration().catch(()=>{});
  return body;
  } catch(error) {
    if(prior)applyTelegramStatus(project,prior);
    throw error;
  }
}
async function configureSharedTelegram(project,token) {
  state.projectNotificationSettingsRevision=(state.projectNotificationSettingsRevision||0)+1;
  const response=await fetch('/api/notifications/telegram?project='+encodeURIComponent(project),{
    method:'POST',headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},body:JSON.stringify({action:'configure_shared',token})
  });
  const body=await response.json().catch(()=>({}));
  if(!response.ok)throw new Error(body.error||('HTTP '+response.status));
  applyTelegramStatus(project,body);
  void refreshNotificationConfiguration().catch(()=>{});
}
const browserNotificationsPending=new Set();
const notificationKinds=['registered','started','intervention','approval','stalled','interrupted','runtime_unknown','finalize','completed'];
Object.assign(I18N.ko,{projectNotifications:'알림센터',issues:'백로그 진단',notificationCenter:'알림센터',notificationCurrent:'현재 확인 사항',notificationHistory:'송신 이력',notificationHistoryEmpty:'이 단말과 프로젝트에서 확인 가능한 송신 기록이 없습니다.',notificationHistoryGuide:'Telegram은 서버 전송 기록, 웹은 이 브라우저의 표시 요청 결과입니다. 수신·읽음 확인을 뜻하지 않습니다.',notificationUnknown:'근거 없음',notificationAccepted:'표시 요청 성공',notificationSent:'전송 성공',notificationNotSent:'미전송',notificationChannel:'채널',notificationContent:'작업 내용',notificationTime:'일시',notificationLoading:'송신 기록을 불러오는 중…',notificationHistoryError:'송신 기록을 확인할 수 없습니다.',notificationWebProject:'이 단말의 웹 알림',backlogDiagnostics:'백로그 진단',diagnosticCopy:'Agent 복원 요청 복사',diagnosticCopyDone:'복원 요청을 복사했습니다. Agent에게 직접 전달해 주세요.',diagnosticUnavailable:'검사 불가',diagnosticNone:'현재 미해소 백로그 진단이 없습니다.'});
Object.assign(I18N.en,{projectNotifications:'Notification center',issues:'Backlog diagnostics',notificationCenter:'Notification center',notificationCurrent:'Current actions',notificationHistory:'Delivery history',notificationHistoryEmpty:'No delivery evidence is available for these projects or this browser.',notificationHistoryGuide:'Telegram shows server delivery evidence; Web shows this browser’s display-request result. Neither confirms receipt or reading.',notificationUnknown:'No evidence',notificationAccepted:'Display request accepted',notificationSent:'Sent',notificationNotSent:'Not sent',notificationChannel:'Channel',notificationContent:'Task content',notificationTime:'Time',notificationLoading:'Loading delivery evidence…',notificationHistoryError:'Delivery evidence is unavailable.',notificationWebProject:'Web on this browser',backlogDiagnostics:'Backlog diagnostics',diagnosticCopy:'Copy Agent recovery request',diagnosticCopyDone:'Recovery request copied. Send it to an Agent yourself.',diagnosticUnavailable:'Check unavailable',diagnosticNone:'No unresolved backlog diagnostics.'});
function notificationKindLabel(kind){return t(({registered:'notifyRegistered',started:'notifyStarted',intervention:'notifyIntervention',approval:'notifyApproval',stalled:'notifyStalled',interrupted:'notifyInterrupted',runtime_unknown:'notifyRuntimeUnknown',finalize:'notifyFinalize',completed:'notifyCompleted'})[kind]||kind);}
function webNotificationSettings(){try{const saved=JSON.parse(localStorage.getItem('task-mecca-web-notification-settings-v1')||'{}');return saved&&typeof saved==='object'?saved:{};}catch(_){return {};}}
function webNotificationEnabled(project,kind){const settings=webNotificationSettings(),specific=settings.projects?.[project];return settings.enabled!==false&&specific?.enabled!==false&&(specific?.kinds?.[kind]??state.notificationSettings[kind]??true)!==false;}
function changeWebNotificationSetting(project,kind,enabled){const settings=webNotificationSettings();settings.projects||={};settings.projects[project]||={};if(kind==='enabled')settings.projects[project].enabled=enabled;else{settings.projects[project].kinds||={};settings.projects[project].kinds[kind]=enabled;}localStorage.setItem('task-mecca-web-notification-settings-v1',JSON.stringify(settings));}
function browserDeliveryHistory(){try{const rows=JSON.parse(localStorage.getItem('task-mecca-browser-deliveries-v1')||'[]');return Array.isArray(rows)?rows:[];}catch(_){return [];}}
function recordBrowserDelivery(project,task,kind,key,eventID){const rows=browserDeliveryHistory();if(rows.some(row=>row.key===key))return;rows.push({key,event_id:eventID||key,project,task_id:task.id,kind,title:titleOf(task),sent_at:new Date().toISOString(),state:'display_requested'});try{localStorage.setItem('task-mecca-browser-deliveries-v1',JSON.stringify(rows));acceptHistoryUpdate();}catch(_){/* No durable evidence means unknown, never inferred success. */}}
function notificationDeliveryLabel(channel){if(channel?.state==='push_accepted')return state.language==='ko'?'Push 서비스 접수':'Push accepted by provider';if(!channel)return t('notificationUnknown');if(channel.state==='display_requested')return t('notificationAccepted');if(channel.state==='sent')return t('notificationSent');if(String(channel.state||'').startsWith('suppressed'))return state.language==='ko'?'미전송 · 설정/정책':'Not sent · settings/policy';if(channel.state==='consumed_legacy')return state.language==='ko'?'과거 기록 · 송신 근거 없음':'Legacy record · no delivery evidence';const labels=state.language==='ko'?{pending:'대기',retry:'재시도 대기',failed:'실패',suppressed:'억제됨',uncertain:'결과 불확실',unknown:'근거 없음'}:{pending:'Pending',retry:'Retry pending',failed:'Failed',suppressed:'Suppressed',uncertain:'Uncertain',unknown:'No evidence'};return labels[channel.state]||channel.state||t('notificationUnknown');}
function historyRowTime(row){return [row.telegram?.state==='sent'?row.telegram.sent_at:'',row.web?.sent_at].filter(value=>Number.isFinite(Date.parse(value))).sort((a,b)=>Date.parse(a)-Date.parse(b)).pop()||row.event_at||'';}
function historyRowMillis(row){const time=Date.parse(historyRowTime(row));return Number.isFinite(time)?time:0;}
function historyRowKey(row){return row.project+'|'+row.event_id;}
function mergedNotificationHistory(){const rows=new Map();for(const row of state.notificationHistory||[])rows.set(historyRowKey(row),{...row});for(const local of browserDeliveryHistory()){const key=historyRowKey(local),prior=rows.get(key);rows.set(key,{...local,...prior,web:prior?.web||local,event_at:prior?.event_at||local.sent_at});}return [...rows.values()].sort((a,b)=>historyRowMillis(b)-historyRowMillis(a)||historyRowKey(a).localeCompare(historyRowKey(b)));}
function notificationHistoryContext(){return state.project+'|'+state.backlog+'|'+(state.projectNotificationSettings||[]).map(row=>row.path).sort().join('|');}
function captureHistoryWindow(){state.notificationHistoryWindow=mergedNotificationHistory();state.notificationHistoryBaseline=new Map(state.notificationHistoryWindow.map(row=>[historyRowKey(row),historyRowMillis(row)]));state.notificationHistoryNew=0;}
function acceptHistoryUpdate(){const rows=mergedNotificationHistory();if(!state.notificationHistoryWindow)captureHistoryWindow();state.notificationHistoryNew=rows.filter(row=>!state.notificationHistoryBaseline.has(historyRowKey(row))||historyRowMillis(row)>state.notificationHistoryBaseline.get(historyRowKey(row))).length;if(state.notificationHistoryPage===1&&state.notificationCenterTab==='history')captureHistoryWindow();updateNotificationHistoryDOM();}
async function loadNotificationHistory(){
 const context=notificationHistoryContext();if(state.notificationHistoryLoading&&state.notificationHistoryContext===context)return state.notificationHistoryPromise;
 if(state.notificationHistoryContext&&state.notificationHistoryContext!==context){state.notificationHistoryPage=1;state.notificationHistoryWindow=null;state.notificationHistoryNew=0;state.notificationHistoryLoaded=false;}
 const request=++state.notificationHistoryRequest;state.notificationHistoryContext=context;state.notificationHistoryLoading=true;updateNotificationHistoryDOM();
 state.notificationHistoryPromise=(async()=>{try{
  const projects=[...new Set((state.projectNotificationSettings||[]).map(row=>row.path).concat(state.project?[state.project]:[]))];
  const results=await Promise.all(projects.map(async project=>{try{const params=new URLSearchParams({project});const revision=state.notificationHistoryVersions?.[project];if(revision)params.set('since',revision);const response=await fetch('/api/notifications/history?'+params,{cache:'no-store'});if(!response.ok)throw Error('HTTP '+response.status);return {project,data:await response.json()};}catch(_){return {project,error:true};}}));
  if(request!==state.notificationHistoryRequest||context!==notificationHistoryContext())return;
  state.notificationHistoryVersions||={};const byKey=new Map((state.notificationHistory||[]).filter(row=>projects.includes(row.project)).map(row=>[historyRowKey(row),row]));
  for(const result of results){if(result.error||result.data.unchanged)continue;const {project,data}=result;if(data.reset||!data.revision){for(const [key,row]of byKey)if(row.project===project)byKey.delete(key);}for(const id of data.removed||[])byKey.delete(project+'|'+id);for(const row of data.history||[])byKey.set(historyRowKey(row),row);if(data.revision)state.notificationHistoryVersions[project]=data.revision;}
  state.notificationHistory=[...byKey.values()];state.notificationHistoryErrors=results.filter(result=>result.error).map(result=>result.project);state.notificationHistoryLoaded=true;acceptHistoryUpdate();
 }finally{if(request===state.notificationHistoryRequest){state.notificationHistoryLoading=false;updateNotificationHistoryDOM();}}})();return state.notificationHistoryPromise;
}
function historyPagination(total){const pages=Math.max(1,Math.ceil(total/30)),page=state.notificationHistoryPage;return `<nav class="center-pagination" aria-label="${esc(state.language==='ko'?'송신 이력 페이지':'History pages')}"><button data-history-page="${page-1}" ${page<=1?'disabled':''}>${esc(state.language==='ko'?'이전':'Previous')}</button><span class="center-page-numbers">${Array.from({length:pages},(_,index)=>index+1).filter(number=>number===1||number===pages||Math.abs(number-page)<=2).map(number=>`<button data-history-page="${number}" ${number===page?'aria-current="page"':''}>${number}</button>`).join('')}</span><span class="center-page-position">${page} / ${pages}</span><button data-history-page="${page+1}" ${page>=pages?'disabled':''}>${esc(state.language==='ko'?'다음':'Next')}</button></nav>`;}
function notificationHistoryMarkup(){
 const rows=state.notificationHistoryWindow||[],page=state.notificationHistoryPage||1,visible=rows.slice((page-1)*30,page*30);
 const notice=state.notificationHistoryNew?`<button class="action-btn" data-history-latest>${esc(state.language==='ko'?`새 알림 ${state.notificationHistoryNew}건 · 최신 보기`:`${state.notificationHistoryNew} new · Latest`)}</button>`:'';
 const status=state.notificationHistoryLoading&&!state.notificationHistoryLoaded?`<p role="status">${esc(t('notificationLoading'))}</p>`:'';
 const error=state.notificationHistoryErrors.length?`<p class="load-error">${esc(t('notificationHistoryError'))} ${state.notificationHistoryErrors.map(esc).join(' · ')}</p>`:'';
 const table=visible.length?`${historyPagination(rows.length)}<div class="notification-history-scroll" role="region" aria-label="${esc(t('notificationHistory'))}" tabindex="0"><table class="notification-history-table"><thead><tr>${['notificationTime','operationProject','id','notificationType','telegramNotifications','browserNotifications','notificationContent'].map(key=>`<th scope="col">${esc(key==='operationProject'?(state.language==='ko'?'프로젝트':'Project'):key==='notificationType'?(state.language==='ko'?'유형':'Type'):t(key))}</th>`).join('')}</tr></thead><tbody>${visible.map(row=>{const params=new URLSearchParams({project:row.project});return `<tr data-history-row="${esc(historyRowKey(row))}"><td>${esc(operationTime(historyRowTime(row)))}</td><td title="${esc(row.project)}">${esc(String(row.project||'').split(/[\\/]/).pop())}</td><td>${row.task_id?`<a href="/tasks/${encodeURIComponent(row.task_id)}?${esc(params)}">${esc(row.task_id)}</a>`:'—'}</td><td>${esc(notificationKindLabel(row.kind))}</td><td>${esc(notificationDeliveryLabel(row.telegram))}</td><td>${esc(notificationDeliveryLabel(row.web))}</td><td>${esc(row.title||row.message||(state.language==='ko'?'내용 없음':'Content not recorded'))}${row.title&&row.message?`<details data-history-disclosure="${esc(historyRowKey(row))}"><summary>${esc(state.language==='ko'?'작업 내용':'Content')}</summary>${esc(row.message)}</details>`:''}</td></tr>`;}).join('')}</tbody></table></div>`:state.notificationHistoryLoaded&&!state.notificationHistoryErrors.length?`<div class="empty">${esc(t('notificationHistoryEmpty'))}</div>`:'';
 return notice+status+error+table;
}
function notificationCenterView(){return `<div class="notification-center-tabs" role="tablist"><button type="button" role="tab" data-center-tab="history" aria-selected="${state.notificationCenterTab==='history'}">${esc(t('notificationHistory'))}<span id="centerHistoryBadge">${state.notificationHistoryNew?` · ${state.notificationHistoryNew}`:''}</span></button><button type="button" role="tab" data-center-tab="settings" aria-selected="${state.notificationCenterTab==='settings'}">${esc(t('notificationSettings'))}</button></div>${state.notificationCenterTab==='settings'?'<section id="notificationSettingsBody" class="notification-settings-body"></section>':`<p class="summary">${esc(t('notificationHistoryGuide'))}</p><section id="centerHistory">${notificationHistoryMarkup()}</section>`}`;}
function bindNotificationHistoryActions(){document.querySelectorAll('[data-history-page]').forEach(button=>button.addEventListener('click',()=>{state.notificationHistoryPage=Number(button.dataset.historyPage);if(state.notificationHistoryPage===1)captureHistoryWindow();updateNotificationHistoryDOM();}));document.querySelectorAll('[data-history-latest]').forEach(button=>button.addEventListener('click',()=>{state.notificationHistoryPage=1;captureHistoryWindow();updateNotificationHistoryDOM();}));}
function updateNotificationHistoryDOM(){if(state.view!=='notifications')return;const badge=$('#centerHistoryBadge');if(badge)badge.textContent=state.notificationHistoryNew?' · '+state.notificationHistoryNew:'';const region=$('#centerHistory');if(!region)return;const scroll=region.querySelector('.notification-history-scroll'),left=scroll?.scrollLeft||0,top=window.scrollY,open=[...region.querySelectorAll('details[open]')].map(item=>item.dataset.historyDisclosure),active=document.activeElement,focusRow=active?.closest?.('[data-history-row]')?.dataset.historyRow,focusPage=active?.dataset?.historyPage;region.innerHTML=notificationHistoryMarkup();for(const details of region.querySelectorAll('details'))details.open=open.includes(details.dataset.historyDisclosure);const nextScroll=region.querySelector('.notification-history-scroll');if(nextScroll)nextScroll.scrollLeft=left;bindNotificationHistoryActions();const focus=focusRow?[...region.querySelectorAll('[data-history-row]')].find(row=>row.dataset.historyRow===focusRow)?.querySelector(active?.tagName==='SUMMARY'?'summary':'a'):focusPage?[...region.querySelectorAll('[data-history-page]')].find(button=>button.dataset.historyPage===focusPage):null;focus?.focus({preventScroll:true});window.scrollTo?.(0,top);}
function notificationSwitch(attrs,on,label,disabled=false){
 // Keep the switch as a single semantic button. The previous visible
 // "sr-only" span lacked a hiding CSS utility and squeezed the track on mobile.
 // The track and thumb are painted by CSS pseudo-elements instead.
 return '<button type="button" role="switch" class="notice-switch'+(on?' is-on':'')+'" aria-checked="'+Boolean(on)+'" aria-label="'+esc(label)+'" '+attrs+(disabled?' disabled':'')+'></button>';
}
function notificationOverviewMarkup(){
 const rows=state.projectNotificationSettings||[],ko=state.language==='ko',webOn=webNotificationSettings().enabled!==false;
 const telegramOn=rows.length>0&&rows.every(r=>r.status?.project_enabled!==false);
 const active=rows.filter(r=>r.status?.project_enabled!==false).length,connected=rows.filter(r=>r.status?.connected).length;
 const top=(name,detail,note,attributes,on,disabled=false)=>'<article class="notice-channel-card"><div class="notice-channel-heading"><div><strong>'+esc(name)+'</strong><small>'+esc(detail)+'</small></div>'+notificationSwitch(attributes,on,name,disabled)+'</div><p>'+esc(note)+'</p></article>';
 return '<section class="notice-overview">'+
 top(ko?'웹 알림':'Web notifications',ko?'현재 브라우저':'This browser',ko?'열린 탭과 백그라운드 Push의 전체 수신을 제어합니다.':'Control foreground and background Push.', 'id="centerGlobalWebChannel"',webOn)+
 top('Telegram',(ko?'연결된 프로젝트 ':'Connected projects ')+connected+'/'+rows.length,(ko?'채널 사용 프로젝트 ':'Enabled projects ')+active+'/'+rows.length,'id="centerGlobalTelegramChannel"',telegramOn,rows.length===0)+'</section>';
}
function projectChannelSettingsMarkup(){
 const rows=state.projectNotificationSettings||[],settings=webNotificationSettings(),ko=state.language==='ko',webGlobalOn=settings.enabled!==false;
 const groups=[
  {name:ko?'작업 진행':'Progress',kinds:['registered','started']},
  {name:ko?'확인 및 대응':'Needs attention',kinds:['intervention','approval','stalled','interrupted','runtime_unknown','finalize']},
  {name:ko?'작업 결과':'Results',kinds:['completed']}
 ];
 const content=rows.map(row=>{
  const project=row.path,web=settings.projects?.[project]||{},webOn=web.enabled!==false,tgOn=row.status?.project_enabled!==false;
  const tgReady=Boolean(row.status?.configured||row.status?.connected),name=row.name||project;
  const webLocked=!webGlobalOn||!webOn,tgLocked=!tgOn||!tgReady;
  const lockNotes=[];
  if(webLocked)lockNotes.push(ko?'Web 채널이 꺼져 있어 Web 유형을 변경할 수 없습니다.':'Web types are locked while the Web channel is off.');
  if(!tgOn)lockNotes.push(ko?'Telegram 채널이 꺼져 있어 Telegram 유형을 변경할 수 없습니다.':'Telegram types are locked while the channel is off.');
  const entry=(channel,kind,on,disabled=false)=>notificationSwitch('data-center-project="'+esc(project)+'" data-center-'+channel+'-kind="'+esc(kind)+'"',on,name+' · '+notificationKindLabel(kind)+' · '+channel,disabled);
  const kinds=groups.map(group=>'<div class="notice-type-group" role="group" aria-label="'+esc(group.name)+'"><div class="notice-type-caption">'+esc(group.name)+'</div>'+
   group.kinds.map(kind=>'<div class="notice-matrix-row"><span>'+esc(notificationKindLabel(kind))+'</span><span>'+entry('web',kind,web.kinds?.[kind]??state.notificationSettings[kind]??true,webLocked)+'</span><span>'+entry('telegram',kind,row.status?.kinds?.[kind]??true,tgLocked)+'</span></div>').join('')+'</div>').join('');
  return '<details class="notice-project" '+(project===state.project?'open':'')+'><summary><span class="notice-project-name">'+esc(name)+'</span><span class="notice-project-summary">Web '+(webOn?'ON':'OFF')+' · Telegram '+(tgOn?'ON':'OFF')+'</span></summary>'+
  '<div class="notice-project-content"><p class="notice-project-path">'+esc(project)+'</p>'+
  '<div class="notice-per-project-channels"><div><strong>Web</strong>'+notificationSwitch('data-center-project="'+esc(project)+'" data-center-channel="web"',webOn,name+' · Web',!webGlobalOn)+'</div><div><strong>Telegram</strong>'+notificationSwitch('data-center-project="'+esc(project)+'" data-center-channel="telegram"',tgOn,name+' · Telegram')+'</div></div>'+
  '<p class="notice-project-connection">'+esc(ko?'텔레그램 상태: ':'Telegram status: ')+esc(row.status?.connected?(ko?'연결됨':'Connected'):row.status?.configured?(ko?'봇 설정됨 · 채팅 연결 필요':'Bot configured · chat pending'):(ko?'봇 연결 필요':'Bot not connected'))+'</p>'+ (lockNotes.length?'<p class="notice-disabled-hint" role="status">'+esc(lockNotes.join(' '))+'</p>':'')+
  '<div class="notice-matrix" role="group" aria-label="'+esc(name)+'" data-web-locked="'+String(webLocked)+'" data-telegram-locked="'+String(tgLocked)+'"><div class="notice-matrix-head"><span>'+esc(ko?'알림 유형':'Event type')+'</span><span>Web'+(webLocked?'<small>OFF</small>':'')+'</span><span>Telegram'+(tgLocked?'<small>'+(tgOn?(ko?'미연결':'SETUP'):'OFF')+'</small>':'')+'</span></div>'+kinds+'</div></div></details>';
 }).join('');
 return '<section class="center-channel-settings"><div class="notice-section-heading"><h2>'+esc(ko?'프로젝트별 채널·유형':'Channels and event types')+'</h2><p>'+esc(ko?'프로젝트를 펼쳐 설정하세요. 채널을 꺼도 유형 선택은 유지됩니다.':'Expand a project. Turning a channel off preserves its event preferences.')+'</p></div>'+(content||'<p class="muted">'+esc(ko?'등록된 프로젝트가 없습니다.':'No projects.')+'</p>')+'</section>';
}
async function setAllTelegramChannelsEnabled(enabled){
 const rows=(state.projectNotificationSettings||[]).filter(row=>(row.status?.project_enabled!==false)!==enabled);
 // Dispatch writes concurrently. Keep individual server-confirmed failures
 // visible and never roll back a successful project's settings.
 const result=await Promise.allSettled(rows.map(row=>setProjectNotificationEnabled(row.path,enabled)));
 const errors=result.flatMap((entry,i)=>entry.status==='rejected'
  ?[(rows[i].name||rows[i].path)+': '+entry.reason?.message]:[]);
 if(errors.length)throw Error(errors.join('\n'));
}
async function commitWebNotificationSetting(apply,project=null){
 const key='task-mecca-web-notification-settings-v1',previous=localStorage.getItem(key);
 apply();
 const targets=project?[project]:null;
 try{await syncBackgroundPush(true,true,targets);}
 catch(error){
  if(previous==null)localStorage.removeItem(key);
  else localStorage.setItem(key,previous);
  // Restore the provider state asynchronously; do not prolong the failed
  // interaction with a second blocking round trip.
  void syncBackgroundPush(true,false,targets).catch(()=>{});
  throw error;
 }
}
async function setNotificationSwitch(input){
 if(input.disabled)return;
 const project=input.dataset.centerProject,channel=input.dataset.centerChannel,next=input.getAttribute('aria-checked')!=='true';
 // Disabled controls are not merely visual: even stale/programmatic events
 // cannot mutate settings while their parent channel is switched off.
 const projectRow=(state.projectNotificationSettings||[]).find(row=>row.path===project);
 const webSettings=webNotificationSettings();
 const webLocked=webSettings.enabled===false||webSettings.projects?.[project]?.enabled===false;
 const telegramLocked=projectRow?.status?.project_enabled===false;
 if((input.dataset.centerWebKind&&webLocked)||(channel==='web'&&webSettings.enabled===false))return;
 if(input.dataset.centerTelegramKind&&(telegramLocked||!projectRow?.status?.configured&&!projectRow?.status?.connected))return;
 input.setAttribute('aria-checked',String(next));
 input.classList.toggle('is-on',next);
 input.setAttribute('aria-busy','true');
 input.disabled=true;
 try{
  if(input.id==='centerGlobalTelegramChannel')await setAllTelegramChannelsEnabled(next);
  else if(input.id==='centerGlobalWebChannel')await commitWebNotificationSetting(()=>{
    const settings=webNotificationSettings();settings.enabled=next;localStorage.setItem('task-mecca-web-notification-settings-v1',JSON.stringify(settings));
   });
  else if(channel==='telegram')await setProjectNotificationEnabled(project,next);
  else if(channel==='web')await commitWebNotificationSetting(()=>changeWebNotificationSetting(project,'enabled',next),project);
  else if(input.dataset.centerTelegramKind){
   const row=state.projectNotificationSettings.find(r=>r.path===project);
   await projectTelegramAction(project,'kinds',{kinds:{...row?.status?.kinds,[input.dataset.centerTelegramKind]:next}});
  }else if(input.dataset.centerWebKind){
   await commitWebNotificationSetting(()=>changeWebNotificationSetting(project,input.dataset.centerWebKind,next),project);
  }
 }catch(error){alert(error.message);}
 finally{
  const projectOpen=[...document.querySelectorAll('.notice-project[open]')].map(d=>d.querySelector('.notice-project-path')?.textContent);
  renderNotificationPanel();
  [...document.querySelectorAll('.notice-project')].forEach(d=>{if(projectOpen.includes(d.querySelector('.notice-project-path')?.textContent))d.open=true;});
  const controls=[...document.querySelectorAll('.notice-switch')];
  const focus=controls.find(button=>input.id?button.id===input.id:
    button.dataset.centerProject===project&&
    (channel?button.dataset.centerChannel===channel:input.dataset.centerWebKind?button.dataset.centerWebKind===input.dataset.centerWebKind:button.dataset.centerTelegramKind===input.dataset.centerTelegramKind));
  focus?.focus?.({preventScroll:true});
 }
}
function bindCenterChannelSettings(panel){
 panel.querySelectorAll('#centerGlobalWebChannel,#centerGlobalTelegramChannel,[data-center-channel],[data-center-telegram-kind],[data-center-web-kind]').forEach(button=>button.addEventListener('click',()=>void setNotificationSwitch(button)));
}

function bindProjectRecipientControls(){
    document.querySelectorAll('[data-project-notification]').forEach(input=>input.addEventListener('change',async()=>{
      input.disabled=true;
      try { await setProjectNotificationEnabled(input.dataset.projectNotification,input.checked); renderNotificationPanel(); }
      catch(error) { input.checked=!input.checked; alert(error.message); }
      finally { input.disabled=false; }
    }));
    document.querySelectorAll('[data-recipient-mode]').forEach(select=>select.addEventListener('change',async()=>{
      select.disabled=true;
      try { await projectTelegramAction(select.dataset.recipientMode,'recipient_mode',{recipient_mode:select.value}); await loadProjectNotificationSettings(); renderNotificationPanel(); }
      catch(error) { alert(error.message); await loadProjectNotificationSettings(); renderNotificationPanel(); }
      finally { select.disabled=false; }
    }));
    document.querySelectorAll('[data-configure-individual]').forEach(button=>button.addEventListener('click',async()=>{
      const project=button.dataset.configureIndividual, token=[...document.querySelectorAll('[data-individual-token]')].find(input=>input.dataset.individualToken===project)?.value?.trim();
      if(!token)return;
      button.disabled=true;
      try { await projectTelegramAction(project,'configure',{token}); await loadProjectNotificationSettings(); renderNotificationPanel(); }
      catch(error) { alert(error.message); }
      finally { button.disabled=false; }
    }));
    document.querySelectorAll('[data-configure-individual]').forEach(button=>{
      const project=button.dataset.configureIndividual;
      const row=(state.projectNotificationSettings||[]).find(item=>item.path===project);
      if(!row?.status?.configured || row.status?.connected)return;
      const discover=document.createElement('button');
      discover.type='button'; discover.className='secondary-btn'; discover.textContent=t('telegramFindChat');
      discover.setAttribute('aria-label',`${row.name||project} · ${t('telegramFindChat')}`);
      discover.addEventListener('click',async()=>{
        discover.disabled=true;
        try { await projectTelegramAction(project,'discover'); await loadProjectNotificationSettings(); renderNotificationPanel(); }
        catch(error) { alert(error.message); }
        finally { discover.disabled=false; }
      });
      button.insertAdjacentElement('afterend',discover);
    });
    $('#sharedTelegramConfigure')?.addEventListener('click',async()=>{
      const token=$('#sharedTelegramToken')?.value?.trim(); if(!token)return;
      try { await configureSharedTelegram($('#sharedTelegramConfigure').dataset.sharedProject,token); renderNotificationPanel(); } catch(error) { alert(error.message); }
    });
    $('#sharedTelegramDiscover')?.addEventListener('click',async()=>{
      try { await projectTelegramAction($('#sharedTelegramDiscover').dataset.sharedProject,'discover_shared'); await loadProjectNotificationSettings(); renderNotificationPanel(); } catch(error) { alert(error.message); }
    });

}
function projectNotificationsView() {
  if(state.projectNotificationSettingsLoading)return `<div class="loading">${esc(t('loading'))}</div>`;
  if(state.projectNotificationSettingsError)return `<div class="load-error"><h2>${esc(t('projectNotifications'))}</h2><p>${esc(state.projectNotificationSettingsError)}</p></div>`;
  const rows=state.projectNotificationSettings||[];
  const shared=rows.some(row=>row.status?.recipient_mode==='shared');
  const sharedProject=rows.find(row=>row.status?.recipient_mode==='shared')?.path||rows[0]?.path||'';
  const sharedSetup=`<section class="telegram-settings"><strong>${esc(t('telegramNotifications'))}</strong><p class="muted">${esc(t('projectSharedRecipientGuide'))}</p><label>${esc(t('telegramBotToken'))}<input id="sharedTelegramToken" type="password" autocomplete="off" placeholder="${esc(t('telegramBotToken'))}"></label><button type="button" class="action-btn" id="sharedTelegramConfigure" data-shared-project="${esc(sharedProject)}">${esc(t('projectSharedRecipientConfigure'))}</button>${shared&&rows.some(row=>!row.status?.connected)?`<button type="button" class="secondary-btn" id="sharedTelegramDiscover" data-shared-project="${esc(sharedProject)}">${esc(t('telegramFindChat'))}</button>`:''}</section>`;
  const body=rows.length?rows.map(row=>{
    const enabled=row.status?.project_enabled!==false;
    const mode=row.status?.recipient_mode==='shared'?'shared':'individual';
    const stateLabel=mode==='individual'&&!row.status?.configured?t('projectIndividualIncomplete'):(row.status?.connected?t('telegramConnected'):(row.status?.configured?t('telegramConfigured'):t('telegramNotConfigured')));
    return `<article class="mini-panel project-notification-row"><div><strong>${esc(row.name||row.path)}</strong><p class="muted">${esc(t('projectNotificationPath'))}: <code>${esc(row.path)}</code></p><p class="muted">${esc(stateLabel)}</p></div><div class="project-notification-controls"><label>${esc(t('projectRecipientMode'))}<select data-recipient-mode="${esc(row.path)}"><option value="shared" ${mode==='shared'?'selected':''}>${esc(t('projectRecipientShared'))}</option><option value="individual" ${mode==='individual'?'selected':''}>${esc(t('projectRecipientIndividual'))}</option></select></label>${mode==='individual'?`<label class="sr-only">${esc(t('telegramBotToken'))}</label><input data-individual-token="${esc(row.path)}" type="password" autocomplete="off" placeholder="${esc(t('telegramBotToken'))}"><button type="button" class="secondary-btn" data-configure-individual="${esc(row.path)}">${esc(t('telegramConnect'))}</button>`:''}<span class="muted">${esc(state.language==='ko'?'채널 전환은 위의 프로젝트별 알림에서 설정하세요.':'Use the project channel switches above.')}</span></div></article>`;
  }).join(''):`<div class="empty">${esc(t('projectNotificationEmpty'))}</div>`;
  return `<div class="page-head"><div><div class="eyebrow">${esc(t('operationsEyebrow'))}</div><h1>${esc(t('projectNotifications'))}</h1><p class="summary">${esc(t('projectNotificationsIntro'))}</p></div></div><section class="assigned-workload-section">${sharedSetup}<div class="project-notification-list">${body}</div></section>`;
}
function restoreNotificationPanelFocus(){const previous=state.notificationPanelReturnFocus;(previous?.id?document.getElementById(previous.id):previous)?.focus();}
function settingsInputKey(input,index){const data=Object.entries(input.dataset||{}).sort(([a],[b])=>a.localeCompare(b));return input.id||data.length?input.id||JSON.stringify(data):'input:'+index;}
function preserveNotificationSettings(panel){const inputs=[...panel.querySelectorAll('input,textarea')],active=document.activeElement;return {text:Object.fromEntries(inputs.filter(input=>['password','text'].includes(input.type)||input.tagName==='TEXTAREA').map(input=>[settingsInputKey(input,inputs.indexOf(input)),input.value])),focus:active&&inputs.includes(active)?settingsInputKey(active,inputs.indexOf(active)):null,selection:active?.selectionStart,open:[...panel.querySelectorAll('details')].map(item=>item.open)};}
function restoreNotificationSettings(panel,draft){const inputs=[...panel.querySelectorAll('input,textarea')];inputs.forEach((input,index)=>{const key=settingsInputKey(input,index);if(Object.hasOwn(draft.text,key))input.value=draft.text[key];});[...panel.querySelectorAll('details')].forEach((item,index)=>{if(draft.open[index]!==undefined)item.open=draft.open[index];});const focus=inputs.find((input,index)=>settingsInputKey(input,index)===draft.focus);focus?.focus({preventScroll:true});if(focus&&draft.selection!=null&&['password','text'].includes(focus.type))focus.setSelectionRange(draft.selection,draft.selection);}
function renderNotificationPanel() {
  const panel=$('#notificationSettingsBody')||$('#notificationPanel'); if(!panel)return;
  const draft=preserveNotificationSettings(panel);
  panel.setAttribute('role',panel.id==='notificationSettingsBody'?'region':'dialog');panel.setAttribute('aria-label',t('notificationSettings'));panel.setAttribute('aria-modal','false');
  const capability=notificationCapability();
  const permission=capability.mode==='supported'?Notification.permission:capability.mode;
  const enabled=Object.values(state.notificationSettings).some(Boolean);
  let guide='';
  if(permission==='granted') guide=enabled
    ? `<div class="notification-state ok"><strong>${esc(t('notificationsOn'))}</strong><span>${esc(t('notificationSettings'))}</span></div>`
    : `<div class="notification-state off"><strong>${esc(t('notificationsOff'))}</strong><span>${esc(t('notificationTypesDisabled'))}</span></div>`;
  else if(permission==='denied') guide=`<div class="notification-state warning"><strong>${esc(t('notificationsBlocked'))}</strong><span>${esc(t('notificationsDeniedGuide'))}</span></div>`;
  else if(permission==='insecure') guide=`<div class="notification-state warning"><strong>${esc(t('notificationsInsecureTitle'))}</strong><span>${esc(t('notificationsInsecureGuide'))}</span></div>`;
  else if(permission==='ios-home') guide=`<div class="notification-state warning"><strong>${esc(t('notificationsIOSHomeTitle'))}</strong><span>${esc(t('notificationsIOSHomeGuide'))}</span></div>`;
  else if(permission==='unsupported') guide=`<div class="notification-state warning"><strong>${esc(t('notificationsUnsupportedTitle'))}</strong><span>${esc(t('notificationsUnsupportedGuide'))}</span></div>`;
  else guide=`<div class="notification-state warning"><strong>${esc(t('notificationsPermissionNeeded'))}</strong><span>${esc(t('allowBrowserNotifications'))}</span></div>`;

  const tg=state.telegramStatus;
  const telegram=tg?.configured
   ? '<section class="telegram-settings"><strong>'+esc(t('telegramNotifications'))+'</strong><p>'+esc(tg.connected?t('telegramConnected'):t('telegramConfigured'))+(tg.bot_username?' · @'+esc(tg.bot_username):'')+'</p><div class="telegram-actions">'+
     (!tg.connected?'<button type="button" class="action-btn" id="telegramDiscover">'+esc(t('telegramFindChat'))+'</button>':'<button type="button" class="action-btn" id="telegramTest">'+esc(t('telegramTest'))+'</button>')+
     '<button type="button" class="secondary-btn" id="telegramDisable">'+esc(t('telegramDisconnect'))+'</button></div></section>'
   : '<section class="telegram-settings"><strong>'+esc(t('telegramNotifications'))+'</strong><p>'+esc(t('telegramGuide'))+'</p><input id="telegramToken" type="password" autocomplete="off" placeholder="'+esc(t('telegramBotToken'))+'"><button type="button" class="action-btn" id="telegramConfigure">'+esc(t('telegramConnect'))+'</button></section>';
  panel.innerHTML='<div class="notification-panel-head"><strong>'+esc(t('notificationSettings'))+'</strong><a href="/?view=notifications">'+esc(t('notificationCenter'))+'</a><button type="button" id="notificationClose" aria-label="'+esc(state.language==='ko'?'알림 설정 닫기':'Close notification settings')+'">×</button></div>'+
  '<div class="notification-panel-body">'+guide+notificationOverviewMarkup()+
  '<div class="notice-push-wrap">'+pushStatusMarkup()+'</div>'+
  ((state.projectNotificationSettings||[]).length?'':telegram)+projectChannelSettingsMarkup()+
  (permission==='default'&&capability.canRequest?'<button type="button" class="action-btn notification-permission" id="notificationPermission">'+esc(t('allowBrowserNotifications'))+'</button>':'')+'</div>';

  const recipientMarkup=projectNotificationsView();
  panel.querySelector('.notification-panel-body')?.insertAdjacentHTML('beforeend',`<details class="center-project-settings"><summary>${esc(state.language==='ko'?'Telegram 수신처 연결':'Telegram recipient setup')}</summary>${recipientMarkup.includes('<section class="assigned-workload-section">')?recipientMarkup.slice(recipientMarkup.indexOf('<section class="assigned-workload-section">')):recipientMarkup}</details>`);
  if(panel.id==='notificationSettingsBody')panel.querySelector('.notification-panel-head')?.remove();
  bindCenterChannelSettings(panel);
  bindProjectRecipientControls();

  panel.querySelectorAll('[data-notification-setting]').forEach(input=>input.addEventListener('change',()=>{state.notificationSettings[input.dataset.notificationSetting]=input.checked;saveNotificationSettings();updateNotificationIndicator();renderNotificationPanel()}));
  panel.querySelectorAll('[data-telegram-kind]').forEach(input=>input.addEventListener('change',async()=>{try{const kinds={...state.telegramStatus.kinds,[input.dataset.telegramKind]:input.checked};await telegramAction('kinds',{kinds});renderNotificationPanel()}catch(e){alert(e.message)}}));
  $('#notificationClose')?.addEventListener('click',()=>{panel.classList.remove('open');restoreNotificationPanelFocus();});
  $('#notificationPermission')?.addEventListener('click',async()=>{try{await Notification.requestPermission();if(Notification.permission==='granted')await notificationWorker()}catch(_){alert(t('notificationPermissionError'))}updateNotificationIndicator();renderNotificationPanel()});
  $('#pushEnable')?.addEventListener('click',async()=>{try{await enableBackgroundPush()}catch(error){webPushFeedback=error.message}renderNotificationPanel()});
  $('#pushDisable')?.addEventListener('click',async()=>{try{await disableBackgroundPush()}catch(error){webPushFeedback=error.message}renderNotificationPanel()});
  $('#pushTest')?.addEventListener('click',async()=>{try{await testBackgroundPush()}catch(error){webPushFeedback=error.message}renderNotificationPanel()});
  $('#telegramConfigure')?.addEventListener('click',async()=>{const token=$('#telegramToken')?.value?.trim();if(!token)return;try{await telegramAction('configure',{token});renderNotificationPanel()}catch(e){alert(e.message)}});
  $('#telegramDiscover')?.addEventListener('click',async()=>{try{await telegramAction('discover');renderNotificationPanel()}catch(e){alert(e.message)}});
  $('#telegramTest')?.addEventListener('click',async()=>{try{await telegramAction('test');alert(t('notificationTestSent'))}catch(e){alert(e.message)}});
  const setAllTelegramKinds=async enabled=>{try{const kinds={};Object.keys(state.telegramStatus?.kinds||{}).forEach(kind=>kinds[kind]=enabled);await telegramAction('kinds',{kinds});renderNotificationPanel()}catch(e){alert(e.message)}};
  $('#telegramAllOn')?.addEventListener('click',()=>setAllTelegramKinds(true));
  $('#telegramAllOff')?.addEventListener('click',()=>setAllTelegramKinds(false));
  restoreNotificationSettings(panel,draft);
  $('#telegramDisable')?.addEventListener('click',async()=>{if(!confirm(t('telegramDisconnect')+'?'))return;try{await telegramAction('disable');renderNotificationPanel()}catch(e){alert(e.message)}});
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
 clearCurrentUserAttention();
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
  const remembered=listPageCaches.get(listContextKey());
  if(remembered)state.listPage=remembered.current;
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
function openWebTerminal() {
  const project=state.project||resolveProjectContext();
  const params=new URLSearchParams();
  if(project)params.set('project',project);
  params.set('lang',state.language);
  location.href='/terminal'+(params.toString()?'?'+params.toString():'');
}
function navigateView(view) {
 if(view==='hub'||view==='release-notes')clearCurrentUserAttention();
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
  if(view==='attention')view='notifications';
  const needsProject=['backlog','workload','issues','manual'].includes(view);
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
  if(view==='notifications')loadNotificationHistory();
  if(view==='issues')refreshDiagnostics();
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
  // Discovery is asynchronous and may temporarily omit projects. Only an
  // explicit close should remove a saved session; missing metadata uses the
  // path-based fallback below until the hub discovers the project again.
  const sessions=state.openProjects;
  const closed=projects.filter(p=>!sessions.includes(p.path));
  const sessionRows=sessions.map(path=>{
    const p=byPath.get(path)||{name:path.split(/[\\/]/).pop()||path,counts:{}};
    const pc=p.counts||{};
    const active=state.project===path && state.view!=='hub';
    const badge=(pc.working||0)+(pc.ready||0);
    const errors=(pc.blocked||0)+(pc.error||0),warnings=(pc.attention||0)+(pc.needs_attention||0)+(pc.hold||0);
    const status=errors?'error':warnings?'warning':pc.working?'working':pc.ready?'registered':pc.done && !pc.total_pending?'done':'unknown';
    const descriptions={error:'Error / Blocked',warning:'Needs attention',working:'Working',registered:'Registered',done:'Done',unknown:'Unknown / Idle'};
    const initial=(Array.from(String(p.name||'?').trim())[0]||'?').toLocaleUpperCase();
    const projectHint=`${p.name} · ${path} · ${descriptions[status]}`;
    return `<div class="session-row session-state-${status} ${active?'active':''}" data-session-project="${esc(path)}"><button class="session-open" type="button" title="${esc(projectHint)}" aria-label="${esc(projectHint)}"><span class="session-initial" aria-hidden="true">${esc(initial)}</span><span class="session-name">${esc(p.name)}</span><span class="session-count">${badge||''}</span></button><button class="session-close" type="button" data-close-project="${esc(path)}" aria-label="Close ${esc(p.name)}" title="Close">×</button></div>`;
  }).join('');
  const menu=state.projectMenuOpen?`<div class="project-open-menu">${closed.length?closed.map(p=>`<button type="button" data-add-project="${esc(p.path)}"><span>${esc(p.name)}</span><small>${esc(p.path)}</small></button>`).join(''):'<div class="project-open-empty">No closed projects</div>'}</div>`:'';
  $('#stateNav').innerHTML =
    `<div class="sidebar-label">SYSTEM</div><button class="nav-item ${state.view==='hub'?'active':''}" id="hubNavBtn" type="button"><span class="nav-main"><span class="nav-icon">⌂</span><span class="nav-text">Global Hub</span></span></button><div class="sidebar-label">BACKLOGS</div><div class="session-list">${sessionRows||'<div class="session-empty">No open backlogs</div>'}</div><button class="nav-item session-add" id="openProjectBtn" type="button"><span class="nav-main"><span class="nav-icon">＋</span><span class="nav-text">Open Project</span></span></button>${menu}`;
  const sidebarModes=[
    ['auto',state.language==='ko'?'자동':'Auto',state.language==='ko'?'마우스를 올리거나 포커스를 옮기면 임시로 펼침':'Temporarily expand on hover or focus'],
    ['expanded',state.language==='ko'?'펼침':'Expanded',state.language==='ko'?'항상 펼친 상태로 고정':'Keep the sidebar expanded'],
    ['compact',state.language==='ko'?'최소화':'Compact',state.language==='ko'?'항상 최소화된 상태로 고정':'Keep the sidebar compact']
  ];
  const modeSelector=document.createElement('div');
  modeSelector.className='sidebar-mode-selector';
  modeSelector.dataset.mode=state.sidebarMode;
  modeSelector.setAttribute('role','group');
  modeSelector.setAttribute('aria-label',state.language==='ko'?'사이드바 표시 방식':'Sidebar display mode');
  modeSelector.innerHTML=sidebarModes.map(([mode,label,description])=>`<button type="button" class="sidebar-mode-option theme-btn ${state.sidebarMode===mode?'active':''}" data-sidebar-mode="${mode}" aria-pressed="${state.sidebarMode===mode}" aria-label="${esc(label)}: ${esc(description)}" title="${esc(description)}">${esc(label)}</button>`).join('');
  $('#stateNav').prepend(modeSelector);
  modeSelector.querySelectorAll('[data-sidebar-mode]').forEach(button=>button.addEventListener('click',()=>{setSidebarMode(button.dataset.sidebarMode);render()}));
  $('#workloadCount').textContent = (state.snapshot?.workload?.agents || []).length || '';
  updateDiagnosticNavigation();
  document.querySelectorAll('[data-session-project]').forEach(row=>row.querySelector('.session-open')?.addEventListener('click',()=>switchProject(row.dataset.sessionProject)));
  document.querySelectorAll('[data-close-project]').forEach(btn=>btn.addEventListener('click',e=>{e.stopPropagation();closeProjectSession(btn.dataset.closeProject)}));
  document.querySelectorAll('[data-add-project]').forEach(btn=>btn.addEventListener('click',()=>{state.projectMenuOpen=false;switchProject(btn.dataset.addProject)}));
  $('#openProjectBtn')?.addEventListener('click',()=>{state.projectMenuOpen=!state.projectMenuOpen;render()});
  $('#hubNavBtn')?.addEventListener('click',()=>navigateView('hub'));
  $('#terminalNavBtn')?.addEventListener('click',openWebTerminal);
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
  const mark=$('#channelSwitchMark');
  if(mark&&!mark.dataset.channelGestureBound){
    mark.dataset.channelGestureBound='1';
    mark.addEventListener('click',recordChannelGestureTap);
  }
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
        const body=await migrateProjectWithChoice(row.path,button);
        if(body.status==='cancelled'){button.disabled=false;button.textContent=t('frameworkSyncAction',{channel:channelLabel(targetChannel)});return;}
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

function hubText(key) {
 const messages={
 ko:{collapseHistory:'보관 목록 접기',expandHistory:'보관 목록 펼치기',unregister:'감시 해제',register:'감시 등록',archive:'보관하기',restore:'다시 편입',forget:'목록에서 제거',copyPath:'전체 경로 복사',history:'보관한 프로젝트',historyInfo:'보관 중에는 감시·알림·런타임 갱신이 중지됩니다. 파일과 폴더는 그대로 유지합니다.',emptyHistory:'보관한 프로젝트가 없습니다.',cancel:'취소',confirm:'확인',failed:'작업을 완료하지 못했습니다. 경로와 권한을 확인한 뒤 다시 시도하세요.',success:'작업을 완료했습니다.',archived:'보관 시각',emptyProjects:'등록된 Task Mecca 프로젝트가 없습니다.',intro:'프로젝트 감시와 보관을 관리합니다. 파일은 변경하거나 삭제하지 않습니다.',add:'프로젝트 추가',pathLabel:'프로젝트 전체 경로'},
 en:{collapseHistory:'Collapse archive',expandHistory:'Expand archive',unregister:'Unregister monitoring',register:'Register monitoring',archive:'Archive',restore:'Restore to active',forget:'Remove from list',copyPath:'Copy full path',history:'Archived projects',historyInfo:'Archived projects stop monitoring, notifications and runtime updates. Files and folders stay unchanged.',emptyHistory:'No archived projects.',cancel:'Cancel',confirm:'Confirm',failed:'Action could not be completed. Check the path and permissions, then retry.',success:'Action completed.',archived:'Archived at',emptyProjects:'No registered Task Mecca projects.',intro:'Manage project monitoring and archiving without changing or deleting files.',add:'Add project',pathLabel:'Full project path'}
 };
 return (messages[state.language]||messages.en)[key]||key;
}
function hubFeedback(message,error=false) {
  const box=document.querySelector('#hubFeedback');
  if(box){box.textContent=message;box.setAttribute('role',error?'alert':'status');box.hidden=false;}
}
function confirmHubAction(action,path,trigger) {
  return new Promise(resolve=>{
    const warnings=state.language==='ko'?{archive:'활성 목록에서 보관으로 옮기고 감시와 알림을 중지합니다. 파일과 폴더는 그대로 둡니다.',forget:'보관 목록에서만 제거합니다. 감시는 중지 상태로 유지되며 파일과 폴더는 변경하지 않습니다.'}:{archive:'Move to the archive and stop monitoring and notifications. Files and folders remain unchanged.',forget:'Remove only this archive entry. Monitoring stays stopped; files and folders remain unchanged.'};
    const overlay=document.createElement('div');overlay.className='channel-switch-overlay hub-confirm-overlay';
    overlay.innerHTML=`<section class="channel-switch-modal" role="dialog" aria-modal="true" aria-labelledby="hubConfirmTitle" aria-describedby="hubConfirmWarning"><h2 id="hubConfirmTitle">${esc(hubText(action))}</h2><p id="hubConfirmWarning">${esc(warnings[action])}</p><div class="project-path"><code>${esc(path)}</code></div><div class="project-actions"><button type="button" class="action-btn secondary" data-hub-cancel>${esc(hubText('cancel'))}</button><button type="button" class="action-btn secondary danger-action" data-hub-confirm>${esc(hubText('confirm'))}</button></div></section>`;
    const finish=value=>{overlay.remove();const current=[...document.querySelectorAll('[data-project-action]')].find(button=>button.dataset.projectAction===action&&button.dataset.projectPath===path&&button.dataset.historyId===trigger?.dataset.historyId);(trigger?.isConnected?trigger:current)?.focus();resolve(value);};
    overlay.querySelector('[data-hub-cancel]').addEventListener('click',()=>finish(false));
    overlay.querySelector('[data-hub-confirm]').addEventListener('click',()=>finish(true));
    overlay.addEventListener('keydown',event=>{if(event.key==='Escape'){event.preventDefault();event.stopPropagation();finish(false);}if(event.key==='Tab'){const buttons=[...overlay.querySelectorAll('button')];event.preventDefault();buttons[(buttons.indexOf(document.activeElement)+(event.shiftKey?-1:1)+buttons.length)%buttons.length].focus();}});
    document.body.append(overlay);overlay.querySelector('[data-hub-cancel]').focus();
  });
}
// AID-118: log usage inspection. Only the explicit safe categories returned
// by the backend can be cleared. Event ledgers are always read-only.
function logStorageLabels() {
  const ko=state.language==='ko';
  return ko
    ? {title:'로그 데이터 관리',intro:'서비스 및 프로젝트 런타임 로그의 실제 파일 용량입니다. 실행 원장·알림 이력·라이프사이클 기록은 삭제할 수 없습니다.',project:'조회 프로젝트',reload:'용량 다시 확인',clear:'로그 비우기',protected:'보호됨',none:'기록 없음',total:'조회한 로그 총량',reclaim:'정리 가능한 용량',confirm:'로그 내용을 영구히 비웁니다. 다시 복구할 수 없습니다.',cancel:'취소',confirmAction:'비우기',success:'정리 완료',failed:'정리 실패',loading:'로그 용량 확인 중…',projectMissing:'등록된 프로젝트가 없습니다.',names:{'web-service':'웹 서비스 출력 로그','hook-diagnostics':'Hook 진단 이벤트','execution-ledger':'실행 이벤트 원장','notification-events':'알림 이력','lifecycle-observations':'라이프사이클 관측 이력'},notes:{'web-service':'웹 실행 로그입니다. 비운 뒤에도 새 로그는 계속 기록됩니다.','hook-diagnostics':'진단용 Hook 원본 관측 기록입니다. 비우면 이전 진단 자료를 복구할 수 없습니다.','execution-ledger':'실행 상태와 이력 복원에 필요한 원장으로 보호됩니다.','notification-events':'알림 중복 방지와 전송 이력을 위해 보호됩니다.','lifecycle-observations':'작업 라이프사이클 증거를 보존하기 위해 보호됩니다.'}}
    : {title:'Log storage',intro:'Actual disk usage of Web and project runtime logs. Execution, notification and lifecycle ledgers are protected.',project:'Project',reload:'Refresh usage',clear:'Clear log',protected:'Protected',none:'No log file',total:'Measured total',reclaim:'Clearable logs',confirm:'The log contents will be permanently cleared and cannot be restored.',cancel:'Cancel',confirmAction:'Clear',success:'Log cleared',failed:'Could not clear log',loading:'Checking log sizes…',projectMissing:'No registered projects',names:{'web-service':'Web service output','hook-diagnostics':'Hook diagnostic events','execution-ledger':'Execution event ledger','notification-events':'Notification history','lifecycle-observations':'Lifecycle observations'}};
}
function logBytes(value){
  const n=Math.max(0,Number(value)||0);
  if(n<1024)return n+' B';
  const units=['KB','MB','GB','TB'];let size=n,index=-1;
  do{size/=1024;index++}while(size>=1024&&index<units.length-1);
  return size.toFixed(size>=10?1:2)+' '+units[index];
}
function logStorageSection(){
  const words=logStorageLabels(),data=state.logStorage;
  const projects=(state.hub?.projects||[]).filter(p=>p?.path);
  const selected=state.logStorageProject||projects[0]?.path||state.lastProject||'';
  const choices=projects.map(p=>`<option value="${esc(p.path)}" ${p.path===selected?'selected':''}>${esc(p.name||p.path.split(/[\\/]/).pop()||p.path)}</option>`).join('');
  const status=state.logStorageError?`<p class="log-storage-alert" role="alert">${esc(state.logStorageError)}</p>`:state.logStorageMessage?`<p class="log-storage-alert" role="status">${esc(state.logStorageMessage)}</p>`:'';
  const rows=(data?.items||[]).map(item=>`<div class="log-storage-row">
      <div class="log-storage-details"><strong>${esc(words.names[item.id]||item.label)}</strong><small>${esc(item.scope==='global'?(state.language==='ko'?'전체 서비스':'Global service'):(state.language==='ko'?'선택한 프로젝트':'Selected project'))} · ${esc(words.notes?.[item.id]||item.note||'')}</small></div>
      <strong class="log-storage-size">${logBytes(item.size_bytes)}</strong>
      ${item.can_clear?`<button type="button" class="action-btn secondary log-clear-btn" data-clear-log="${esc(item.id)}" ${!item.size_bytes||state.logStorageBusy?'disabled':''}>${esc(words.clear)}</button>`:`<span class="log-storage-protected">${esc(words.protected)}</span>`}
    </div>`).join('');
  return `<section class="log-storage" aria-labelledby="logStorageHeading">
    <div class="log-storage-head"><div><h2 id="logStorageHeading">${esc(words.title)}</h2><p class="muted">${esc(words.intro)}</p></div></div>
    <div class="log-storage-controls"><label>${esc(words.project)} <select id="logStorageProject" ${state.logStorageBusy?'disabled':''}>${choices||`<option value="">${esc(words.projectMissing)}</option>`}</select></label>
      <button id="logStorageRefresh" type="button" class="action-btn secondary" ${state.logStorageBusy?'disabled':''}>${esc(words.reload)}</button></div>
    <div id="logStorageFeedback" aria-live="polite">${status}</div>
    ${data?`<div class="log-storage-totals"><span>${esc(words.total)} <strong>${logBytes(data.total_bytes)}</strong></span><span>${esc(words.reclaim)} <strong>${logBytes(data.reclaimable_bytes)}</strong></span></div><div class="log-storage-items">${rows}</div>`:`<p class="muted">${esc(words.loading)}</p>`}
  </section>`;
}
function refreshLogStoragePanel(){
  const element=$('#logStoragePanel');
  if(!element||state.view!=='hub')return;
  element.innerHTML=logStorageSection();
  bindLogStorageActions();
}
async function loadLogStorage(){
  if(state.logStorageBusy)return;
  state.logStorageBusy=true;
  state.logStorageError='';
  refreshLogStoragePanel();
  try {
    const project=state.logStorageProject||(state.hub?.projects||[])[0]?.path||state.lastProject||'';
    const r=await fetch('/api/storage/logs?project='+encodeURIComponent(project),{cache:'no-store'});
    const payload=await r.json();
    if(!r.ok)throw Error(payload.error||'HTTP '+r.status);
    state.logStorage=payload;
  }catch(error){state.logStorageError=String(error?.message||error);}
  finally{state.logStorageBusy=false;refreshLogStoragePanel();}
}
function confirmLogCleanup(id,label,size){
  return new Promise(resolve=>{
    const words=logStorageLabels();
    const overlay=document.createElement('div');
    overlay.className='channel-switch-overlay log-confirm-overlay';
    overlay.innerHTML=`<section class="channel-switch-modal" role="dialog" aria-modal="true" aria-labelledby="logCleanupTitle">
      <h2 id="logCleanupTitle">${esc(label)} · ${logBytes(size)}</h2><p>${esc(words.confirm)}</p>
      <div class="project-actions"><button type="button" class="action-btn secondary" data-log-cancel>${esc(words.cancel)}</button><button type="button" class="action-btn danger-action" data-log-confirm>${esc(words.confirmAction)}</button></div></section>`;
    const done=value=>{overlay.remove();resolve(value);};
    overlay.querySelector('[data-log-cancel]').addEventListener('click',()=>done(false));
    overlay.querySelector('[data-log-confirm]').addEventListener('click',()=>done(true));
    overlay.addEventListener('keydown',e=>{
      if(e.key==='Escape'){e.preventDefault();done(false);}
      if(e.key==='Tab'){const buttons=[...overlay.querySelectorAll('button')];e.preventDefault();buttons[(buttons.indexOf(document.activeElement)+(e.shiftKey?-1:1)+buttons.length)%buttons.length].focus();}
    });
    document.body.append(overlay);
    overlay.querySelector('[data-log-cancel]').focus();
  });
}
function bindLogStorageActions(){
  $('#logStorageRefresh')?.addEventListener('click',()=>loadLogStorage());
  $('#logStorageProject')?.addEventListener('change',event=>{
    state.logStorageProject=event.currentTarget.value;
    state.logStorage=null;
    state.logStorageMessage='';
    loadLogStorage();
  });
  document.querySelectorAll('[data-clear-log]').forEach(button=>button.addEventListener('click',async()=>{
    const id=button.dataset.clearLog;
    const item=(state.logStorage?.items||[]).find(entry=>entry.id===id && entry.can_clear);
    if(!item||state.logStorageBusy)return;
    if(!await confirmLogCleanup(id,logStorageLabels().names[id]||item.label,item.size_bytes))return;
    state.logStorageBusy=true;refreshLogStoragePanel();
    try{
      const project=state.logStorageProject||(state.hub?.projects||[])[0]?.path||state.lastProject||'';
      const r=await fetch('/api/storage/logs?project='+encodeURIComponent(project),{
        method:'POST',headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},
        body:JSON.stringify({id})
      });
      const payload=await r.json();
      if(!r.ok)throw Error(payload.error||'HTTP '+r.status);
      state.logStorage=payload;
      state.logStorageError='';
      state.logStorageMessage=logStorageLabels().success+' · '+logBytes(payload.released_bytes);
    }catch(err){state.logStorageError=logStorageLabels().failed+': '+String(err?.message||err);}
    finally{state.logStorageBusy=false;refreshLogStoragePanel();}
  }));
}

function hubView() {
  const h=state.hub||{};
  const cli=h.cli||{};
  const projects=Array.isArray(h.projects)?h.projects:[];
  const channelBadge=cli.channel==='dev' ? '<span class="badge warn">DEV</span>' : '';
  const currentVersionMark='<span class="badge hub-version-mark">'+esc(cli.current||'-')+'</span>';
  const cliStatus=cli.update_available
    ? currentVersionMark+'<span class="badge warn">→ '+esc(cli.latest||'-')+'</span>'
    : currentVersionMark;
  const managed=new Map((state.hubManagement?.projects||[]).map(p=>[p.path,p]));
  const cards=projects.map(p=>{
    const c=p?.counts||{};
    const name=p?.name||String(p?.path||'Project').split(/[\\/]/).pop()||'Project';
    const framework=p?.framework_version||'unknown';
    const status=managed.get(p.path)||{};
    const path=String(p?.path||'');
    return `<article class="project-card">
      <div class="project-card-head"><div><h2>${esc(name)}</h2><div class="project-path"><code>${esc(path)}</code><button class="icon-copy" type="button" data-copy-path="${esc(path)}" aria-label="${esc(hubText('copyPath'))}">${COPY_ICON}</button></div></div><span class="badge">${esc(framework)}</span></div>
      <div class="project-stats"><span><strong>${c.working||0}</strong> ${esc(t('working'))}</span><span><strong>${c.ready||0}</strong> ${esc(t('ready'))}</span><span><strong>${c.hold||0}</strong> ${esc(t('hold'))}</span></div>
      <div class="project-actions">${p?.migration_available?`<button class="action-btn secondary" data-migrate="${esc(p.path)}">Migrate</button>`:''}<button class="action-btn secondary" data-project-action="${status.monitoring===false?'register':'unregister'}" data-project-path="${esc(path)}">${esc(hubText(status.monitoring===false?'register':'unregister'))}</button><button class="action-btn secondary danger-action" data-project-action="archive" data-project-path="${esc(path)}">${esc(hubText('archive'))}</button><button class="action-btn" data-open-project="${esc(path)}">${esc(t('open'))}</button></div>
    </article>`;
  }).join('');
  const history=(state.hubManagement?.archives||[]).map(item=>`<article class="history-row"><div><strong>${esc(item.name||'Project')}</strong><div class="project-path"><code>${esc(item.path)}</code><button class="icon-copy" type="button" data-copy-path="${esc(item.path)}" aria-label="${esc(hubText('copyPath'))}">${COPY_ICON}</button></div><p>${esc(hubText('archived'))}: ${esc(item.archived_at||'—')}</p></div><div class="project-actions"><button class="action-btn secondary" data-project-action="restore" data-history-id="${esc(item.id)}" data-project-path="${esc(item.path)}">${esc(hubText('restore'))}</button><button class="action-btn secondary" data-project-action="forget" data-history-id="${esc(item.id)}" data-project-path="${esc(item.path)}">${esc(hubText('forget'))}</button></div></article>`).join('');
  const updateActions='';
  return `<div class="page-head"><div><h1>Global Hub</h1><p class="summary">${esc(hubText('intro'))}</p></div><div class="hub-cli"><strong>CLI</strong> ${channelBadge} ${cliStatus} ${updateActions}</div></div>
    ${cli.update_available?'<div class="timing-note"><strong>Upgrade</strong><span>업그레이드가 완료되면 Task Mecca Web이 자동으로 재시작되며, 현재 브라우저 페이지도 자동으로 새로고침됩니다.</span></div>':''}
    ${cli.error?`<div class="timing-note"><strong>Version check</strong><span>${esc(cli.error)}</span></div>`:''}
    <p id="hubFeedback" role="status" class="timing-note" hidden></p><div class="project-grid">${cards||`<div class="empty">${esc(hubText('emptyProjects'))}</div>`}</div><section class="hub-history"><h2><button id="hubHistoryToggle" type="button" class="hub-history-toggle" aria-expanded="${state.hubHistoryExpanded}" aria-controls="hubHistoryItems"><span>${esc(hubText('history'))}</span><span class="badge">${(state.hubManagement?.archives||[]).length}</span><span class="hub-history-action">${esc(hubText(state.hubHistoryExpanded?'collapseHistory':'expandHistory'))}</span><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 9 6 6 6-6"/></svg></button></h2><p class="muted">${esc(hubText('historyInfo'))}</p><div id="hubHistoryItems" ${state.hubHistoryExpanded?'':'hidden'}>${history||`<div class="empty">${esc(hubText('emptyHistory'))}</div>`}</div></section><div id="logStoragePanel">${logStorageSection()}</div>`;
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
        if(!targetVersion||normalizedVersion(body.version)===targetVersion){ location.reload(); return true; }
      }
    } catch(_) {}
    await new Promise(resolve=>setTimeout(resolve,500));
  }
  const c=$('#content');
  if(c)c.innerHTML=`<div class="load-error"><h2>Task Mecca Web 재시작을 확인하지 못했습니다</h2><p>CLI 업데이트 자체는 완료됐을 수 있습니다. 터미널에서 <code>task-mecca web status</code>로 상태를 확인하고, 이전 버전이 계속 실행 중이면 <code>task-mecca web restart</code>를 실행한 뒤 이 페이지를 새로고침하세요.</p></div>`;
  return false;
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
  bindLogStorageActions();
  if(!state.logStorage&&!state.logStorageBusy)queueMicrotask(()=>loadLogStorage());
  document.querySelectorAll('[data-open-project]').forEach(btn=>btn.addEventListener('click',()=>{
    const path=btn.dataset.openProject||'';
    if(path)switchProject(path);
  }));
  document.querySelectorAll('[data-migrate]').forEach(btn=>btn.addEventListener('click',e=>performProjectMigration(btn.dataset.migrate,e.currentTarget)));
  document.querySelectorAll('[data-copy-path]').forEach(btn=>btn.addEventListener('click',async()=>{try{await copyText(btn.dataset.copyPath||'');btn.innerHTML=CHECK_ICON;btn.setAttribute('aria-label',t('copied'));hubFeedback(t('copied'));setTimeout(()=>{btn.innerHTML=COPY_ICON;btn.setAttribute('aria-label',hubText('copyPath'));},1500);}catch(_){hubFeedback(t('copyFailed'),true);}}));
  document.querySelectorAll('[data-project-action]').forEach(btn=>btn.addEventListener('click',()=>manageHubProject(btn)));
  document.querySelector('#hubHistoryToggle')?.addEventListener('click',event=>{state.hubHistoryExpanded=!state.hubHistoryExpanded;localStorage.setItem('task-mecca-hub-history-expanded-v1',state.hubHistoryExpanded?'1':'0');event.currentTarget.setAttribute('aria-expanded',String(state.hubHistoryExpanded));event.currentTarget.querySelector('.hub-history-action').textContent=hubText(state.hubHistoryExpanded?'collapseHistory':'expandHistory');document.querySelector('#hubHistoryItems').hidden=!state.hubHistoryExpanded;});
  const changes=$('#hubUpdateChangesBtn');
  if(changes)changes.addEventListener('click',showAvailableUpdateNotes);
  const up=$('#upgradeBtn');
  if(up)up.addEventListener('click',showAvailableUpdateNotes);
}

async function manageHubProject(button) {
  const action=button.dataset.projectAction, path=button.dataset.projectPath||'', historyID=button.dataset.historyId||'';
  if(['archive','forget'].includes(action) && !await confirmHubAction(action,path,button))return;
  button.disabled=true;
  try {
    const body={action,path,archive_id:historyID};
    if(['archive','forget'].includes(action))body.confirm_path=path;
    const r=await fetch('/api/hub/projects',{method:'POST',headers:{'Content-Type':'application/json','X-Task-Mecca-Action':'1'},body:JSON.stringify(body)});
    const payload=await r.json();
    if(!r.ok)throw Error(payload.error||'Request failed');
    await refreshHub(true);render();
    hubFeedback(hubText('success')+' · '+path);
    document.querySelector('#hubFeedback')?.setAttribute('tabindex','-1');document.querySelector('#hubFeedback')?.focus();
  } catch(error) { await refreshHub(true);render();hubFeedback(hubText('failed')+' '+String(error?.message||error),true); } finally { button.disabled=false; }
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
function backlogListTools() {
  return `<div class="backlog-list-tools"><div class="search-wrap"><span class="search-icon" aria-hidden="true">⌕</span><input id="search" value="${esc(state.query)}" placeholder="${esc(t('searchPlaceholder'))}" aria-label="${esc(t('search'))}" autocomplete="off"/><kbd>/</kbd></div><label class="backlog-picker" title="${esc(t('selectBacklog'))}"><span id="backlogLabel">${esc(t('backlogFolder'))}</span><select id="backlogPicker" aria-label="${esc(t('selectBacklog'))}"></select></label></div>`;
}
function listControls(info) {
  const autoSelected = state.listPageMode === 'auto';
  const pageValue = autoSelected ? 'auto' : String(state.listPageSize);
  const pageText=t('pageSummary',{page:info.page,pages:info.pages,total:info.total})+(autoSelected?t('autoRowsSummary',{n:info.pageSize}):'')+(state.listRevalidating?(state.language==='ko'?' · 최신 상태 확인 중':' · Checking latest state'):'');
  return `${statusFilterBar()}${tagFilterBar()}<div class="backlog-list-controls">${backlogListTools()}<div class="list-controls"><div class="control-group"><label>${esc(t('sort'))}<select id="listSort"><option value="id_desc" ${state.listSort==='id_desc'?'selected':''}>ID ↓</option><option value="id_asc" ${state.listSort==='id_asc'?'selected':''}>ID ↑</option><option value="updated_desc" ${state.listSort==='updated_desc'?'selected':''}>${esc(t('updatedNewest'))}</option><option value="updated_asc" ${state.listSort==='updated_asc'?'selected':''}>${esc(t('updatedOldest'))}</option></select></label><label>${esc(t('perPage'))}<select id="listPageSize"><option value="auto" ${pageValue==='auto'?'selected':''}>${esc(t('autoRows',{n:info.pageSize}))}</option><option value="10" ${pageValue==='10'?'selected':''}>10</option><option value="20" ${pageValue==='20'?'selected':''}>20</option><option value="50" ${pageValue==='50'?'selected':''}>50</option></select></label></div><div class="pager"><button id="prevPage" ${info.page<=1?'disabled':''}>←</button><span>${esc(pageText)}</span><button id="nextPage" ${info.page>=info.pages?'disabled':''}>→</button></div></div></div>`;
}

function listView() {
  const data=currentProjectData()||{};
  const info = pageInfo(), rows = info.pageItems, c = data.counts || {};
  state.selectedIndex = Math.min(Math.max(0,state.selectedIndex), Math.max(0,rows.length-1));
  const meta = data.backlog_selection || {};
  const presence = data.backlog_presence || {};
  const uninitialized = presence.status === 'uninitialized';
  const filterLabel = state.statusFilters.includes('all') ? t('allStatuses') : state.statusFilters.map(stateLabel).join(' + ');
  const head=`<div class="task-list-head"><div>${esc(t('id'))}</div><div>${esc(t('taskColumn'))}</div><div>${esc(t('statusColumn'))}</div><div>${esc(t('agentColumn'))}</div><div>${esc(t('activeColumn'))}</div><div>${esc(t('updatedColumn'))}</div><div></div></div>`;
  const summary = uninitialized ? t('backlogUninitializedSummary') : `${meta.selected ? String(meta.selected).split(/[\\/]/).pop() : ''} · ${info.total} ${t('items')} · ${filterLabel}`;
  const emptyState = `<div class="empty"><strong>${esc(t('backlogUninitializedTitle'))}</strong><div>${esc(t('backlogUninitializedBody'))}</div></div>`;
  return `<div class="page-head"><div><div class="eyebrow">${esc(data.repo||t('repository'))}</div><h1>${esc(t('backlog'))}</h1><p class="summary">${esc(summary)}</p></div></div><div class="metrics"><div class="metric"><strong>${c.working||0}</strong><span>${esc(t('working'))}</span></div><div class="metric"><strong>${c.ready||0}</strong><span>${esc(t('ready'))}</span></div><div class="metric"><strong>${c.hold||0}</strong><span>${esc(t('hold'))}</span></div><div class="metric"><strong>${c.attention||0}</strong><span>${esc(t('needsAttention'))}</span></div></div>${uninitialized?`${listControls(info)}${emptyState}`:`${listControls(info)}${rows.length?`${head}<div class="task-list">${rows.map((task,i)=>{
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
    return `<div class="task-row ${i===state.selectedIndex?'keyboard-selected':''}" data-id="${esc(task.id)}" data-row-index="${i}" tabindex="-1"><div class="task-id">${esc(task.id)}</div><div class="task-main"><div class="task-mobile-id">${esc(task.id)}</div><div class="task-title">${esc(titleOf(task))}</div>${taskMeta?`<div class="task-meta-row">${taskMeta}</div>`:''}${previewLine?`<div class="task-preview">${esc(previewLine)}</div>`:''}</div><div class="state-col"><span class="status ${esc(task.state)}">${esc(stateLabel(task.state))}</span>${statusSummary?`<div class="task-state-summary">${esc(statusSummary)}</div>`:''}${['quiet','stale','worker_missing'].includes(h)?`<div class="task-sub">${esc(healthLabel(h))}</div>`:''}</div><div class="task-agent"><div>${esc(alias)}</div>${task.archive_month?`<div class="task-sub">archive/${esc(task.archive_month)}</div>`:''}</div><div class="task-active timer live-timer" data-id="${esc(task.id)}">${time}</div><div class="task-updated" title="${esc(dateTimeLabel(updated))}"><strong class="live-updated" data-updated-at="${esc(updated)}">${esc(ago(updated))}</strong><span>${esc(dateTimeLabel(updated,true))}</span></div><div class="chev">›</div></div>`;
  }).join('')}</div>`:`<div class="empty">${esc(t('noMatches'))}</div>`}`}`;
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
  runtimeRootSessions:'Root Sessions',
  runtimeRootActive:'현재 Root',
  runtimeRootNeedsCheck:'확인 필요 Root',
  runtimeRootPrevious:'이전 Root',
  runtimeRootCleanupEligible:'정리 가능 Root',
  runtimeRootAgents:'Agent {n}',
  runtimeRootAttempts:'Attempt {n}',
  runtimeRootCreated:'개설',
  runtimeRootObservedStart:'관측 시작',
  runtimeRootLastActivity:'마지막 활동',
  runtimeRootID:'Root Session ID',
  runtimeRootProvider:'Provider',
  runtimeRootCurrentCount:'현재 {n}',
  runtimeRootNeedsCount:'확인 필요 {n}',
  runtimeRootTerminalCount:'종료 {n}',
  runtimeRootShowPrevious:'이전 Root Sessions 보기',
  runtimeRootHidePrevious:'이전 Root Sessions 닫기',
  runtimeRootLoading:'Root Session 이력을 불러오는 중…',
  runtimeRootEmpty:'이전 Root Session이 없습니다.',
  runtimeRootPage:'{page} / {pages} 페이지 · {total}개 Root',
  runtimeRootCleanup:'Root 기록 정리',
  runtimeRootCleanupConfirm:'{name} Root Session과 그 하위 Runtime 기록 약 {bytes}를 정리할까요? canonical backlog/Git과 Hook 설정은 유지됩니다.',
  runtimeRootCleanupDone:'Root Session 기록 {bytes}를 정리했습니다.',
  runtimeRootInactiveTerminal:'7일 이상 미활동 · 정리 가능',
  runtimeRootTerminal:'종료됨',
  runtimeRootNeedsCheckStatus:'상태 확인 필요',
  runtimeRootActiveStatus:'활성',
  runtimeRootFallbackHint:'Provider 이름을 확인할 수 없어 Task Mecca fallback 이름을 사용 중입니다.',
});
Object.assign(I18N.ko,{
  runtimeHookOnboardingTitle:'Runtime 관측 설정이 아직 완료되지 않았습니다.',
  runtimeHookOnboardingBody:'아직 실사용 Provider를 확정할 수 없습니다. 사용할 Provider의 Hook만 설정해도 되며, 사용하지 않는 Agent의 Hook은 켤 필요가 없습니다.',
  runtimeHookOnboardingEnable:'{provider} Hook 설정',
  runtimeHookReapply:'{provider} 설정 다시 적용',
  runtimeHookInUseTitle:'현재 사용 중인 {provider}의 Runtime Hook을 확인하세요.',
  runtimeHookInUseBody:'백로그/현재 Root 근거상 이 Provider가 실사용 중이지만 Hook이 꺼져 있거나 현재 세션에서 관측되지 않습니다. 다른 Agent의 Hook이 켜져 있어도 이 경고는 해제되지 않습니다.',
  runtimeHookInUseUnconfigured:'{provider}가 현재 실사용 중이지만 Task Mecca Hook 설정이 없습니다.',
  runtimeHookInUseNotObserved:'{provider}가 현재 실사용 중이고 설정 파일도 존재하지만 현재 Runtime Hook 신호가 확인되지 않습니다. Agent 앱에서 Hook 활성화·신뢰 상태를 확인하세요.',
  runtimeHookNewRootRequired:'설정 또는 활성화 후에는 새 Root Session을 시작하세요. 이미 열려 있는 Root에는 SessionStart/기존 Agent Start 이벤트가 소급 적용되지 않습니다.',
  runtimeHookAgentGuide:'{provider} 앱에서 켜는 방법',
  runtimeHookDesktop:'Desktop',
  runtimeHookCLI:'CLI',
  runtimeHookDesktopCLI:'Desktop / CLI / IDE',
  runtimeHookCodexDesktopGuide:'프롬프트 입력창 좌측 하단의 Hooks에서 Task Mecca Hook을 활성화하고, 새로 추가되거나 변경된 Hook이면 Review/Trust까지 완료하세요.',
  runtimeHookCodexCLIGuide:'/hooks를 열어 Project Hook을 확인하고 Review/Trust/Enable 하세요. 전역 설정에 [features] hooks = false가 있으면 true로 바꾸거나 해당 비활성화를 제거해야 합니다.',
  runtimeHookClaudeAppGuide:'Task Mecca의 설정 버튼은 프로젝트 .claude/settings.json에 Hook을 추가합니다. 프로젝트를 trusted 상태로 연 뒤 /hooks에서 설정 출처를 확인하세요(/hooks는 확인용 읽기 전용). Claude Code의 terminal, IDE, Desktop은 같은 Hook 이벤트를 사용합니다.'
});
Object.assign(I18N.en,{
  runtimeHookOnboardingTitle:'Runtime observation is not configured yet.',
  runtimeHookOnboardingBody:'Task Mecca cannot identify the active provider yet. Configure only the provider you actually use; unused agent Hooks do not need to be enabled.',
  runtimeHookOnboardingEnable:'Configure {provider} Hooks',
  runtimeHookReapply:'Reapply {provider} setup',
  runtimeHookInUseTitle:'Check Runtime Hooks for the active provider: {provider}.',
  runtimeHookInUseBody:'Backlog/current Root evidence shows this provider is in use, but its Hook is disabled or not being observed in the current session. Enabling a different agent does not clear this warning.',
  runtimeHookInUseUnconfigured:'{provider} is currently in use but Task Mecca Hooks are not configured.',
  runtimeHookInUseNotObserved:'{provider} is currently in use and the project Hook file exists, but no current Runtime Hook signal is observed. Check Hook enablement and trust in the agent app.',
  runtimeHookNewRootRequired:'After setup or enablement, start a new Root Session. SessionStart and existing Agent Start events from an already-open Root are not backfilled.',
  runtimeHookAgentGuide:'Enable in {provider}',
  runtimeHookDesktop:'Desktop',
  runtimeHookCLI:'CLI',
  runtimeHookDesktopCLI:'Desktop / CLI / IDE',
  runtimeHookCodexDesktopGuide:'Use Hooks at the lower-left of the prompt composer to enable the Task Mecca Hook. Review/Trust newly added or changed Hooks when prompted.',
  runtimeHookCodexCLIGuide:'Open /hooks to inspect the Project Hook and Review/Trust/Enable it. If global config contains [features] hooks = false, set it to true or remove that disablement.',
  runtimeHookClaudeAppGuide:'Task Mecca setup writes the project .claude/settings.json. Open the project as trusted, then use /hooks to verify the source (/hooks is read-only). Claude Code terminal, IDE, and Desktop use the same Hook events.'
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
  runtimeRootSessions:'Root Sessions',
  runtimeRootActive:'Current roots',
  runtimeRootNeedsCheck:'Roots needing verification',
  runtimeRootPrevious:'Previous roots',
  runtimeRootCleanupEligible:'Cleanup eligible',
  runtimeRootAgents:'{n} agents',
  runtimeRootAttempts:'{n} attempts',
  runtimeRootCreated:'Created',
  runtimeRootObservedStart:'First observed',
  runtimeRootLastActivity:'Last activity',
  runtimeRootID:'Root Session ID',
  runtimeRootCurrentCount:'Current {n}',
  runtimeRootNeedsCount:'Needs check {n}',
  runtimeRootTerminalCount:'Terminal {n}',
  runtimeRootShowPrevious:'View previous Root Sessions',
  runtimeRootHidePrevious:'Close previous Root Sessions',
  runtimeRootLoading:'Loading Root Session history…',
  runtimeRootEmpty:'No previous Root Sessions.',
  runtimeRootPage:'Page {page} / {pages} · {total} roots',
  runtimeRootCleanup:'Clean Root records',
  runtimeRootCleanupConfirm:'Clean about {bytes} of runtime records for Root Session {name}? Canonical backlog/Git and Hook settings are preserved.',
  runtimeRootCleanupDone:'Cleaned {bytes} of Root Session runtime records.',
  runtimeRootInactiveTerminal:'Inactive for 7+ days · cleanup eligible',
  runtimeRootTerminal:'Terminal',
  runtimeRootNeedsCheckStatus:'Needs verification',
  runtimeRootActiveStatus:'Active',
  runtimeRootFallbackHint:'Task Mecca fallback name is shown because no provider session name was available.',
});

Object.assign(I18N.ko,{
  runtimeHookApproveSetup:'승인 설정',
  runtimeHookDisabled:'관측 꺼짐',
  runtimeHookDisabledGuide:'이 기기에서 해당 Provider의 새 관측 기록이 중단되었습니다. 기존 프로젝트 Hook이 남아 있어도 기록하지 않습니다.',
  runtimeHookDeviceEnableConfirm:'{provider}의 사용자 전역 Task Mecca Hook을 설정하고 이 기기의 관측을 다시 켭니다. 신뢰 승인은 앱에서 직접 해야 합니다. 계속할까요?',
  runtimeHookDeviceDisableConfirm:'이 기기에서 {provider} 관측을 끕니다. 기존 기록과 다른 Provider·다른 Hook은 유지됩니다. 계속할까요?'
});
Object.assign(I18N.en,{
  runtimeHookApproveSetup:'Set up approval',
  runtimeHookDisabled:'Observation off',
  runtimeHookDisabledGuide:'New observations for this provider are paused on this device, even if a project Hook remains installed.',
  runtimeHookDeviceEnableConfirm:'Install the user-level Task Mecca {provider} Hook and resume observation on this device? Trust approval still happens in the app.',
  runtimeHookDeviceDisableConfirm:'Pause {provider} observation on this device? Existing history, other providers and other Hooks remain unchanged.'
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
  return `<details id="runtime-attempt-${esc(encodeURIComponent(disclosureKey))}" class="runtime-attempt-card runtime-attempt-disclosure" data-runtime-attempt-key="${esc(disclosureKey)}" ${isOpen?'open':''}>
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
  return stateValue==='disabled'?t('runtimeHookDisabled'):stateValue==='observed'?t('runtimeHookObserved'):stateValue==='source_unresolved'?t('runtimeHookSourceUnresolved'):stateValue==='verification_required'?t('runtimeHookVerificationRequired'):t('runtimeHookUnconfigured');
}
function runtimeHookStateClass(stateValue) {
  return stateValue==='observed'?'ok':stateValue==='verification_required'||stateValue==='source_unresolved'?'warn':'';
}
function runtimeHookGuidance(hook) {
  if(hook?.observation_enabled===false)return t('runtimeHookDisabledGuide');
  const events=hook?.observed_events||{};
  const activityOnly=Boolean(events.activity)&&!events.start;
  if(hook?.in_use&&hook?.needs_attention) {
    if(!hook?.configured)return t('runtimeHookInUseUnconfigured',{provider:String(hook?.provider||'').toUpperCase()});
    return t('runtimeHookInUseNotObserved',{provider:String(hook?.provider||'').toUpperCase()});
  }
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
  const last=hook?.last_observed_at||'';
  return `<article class="runtime-hook-card">
    <div class="runtime-hook-card-head">
      <div class="runtime-hook-identity"><strong>${esc(provider)}</strong></div>
      <span class="badge ${runtimeHookStateClass(hook?.state)}">${esc(runtimeHookStateLabel(hook?.state))}</span>
    </div>
    <p class="runtime-hook-guidance">${esc(runtimeHookGuidance(hook))}</p>
    <div class="runtime-hook-events">
      ${runtimeHookEventChip(t('runtimeHookActivity'),Boolean(events.activity))}
      ${runtimeHookEventChip(t('runtimeHookStart'),Boolean(events.start))}
      ${runtimeHookEventChip(t('runtimeHookStop'),Boolean(events.stop))}
    </div>
    ${last?`<div class="runtime-hook-last">${esc(t('runtimeHookLastObserved'))} · ${esc(ago(last))}</div>`:''}
    ${hook?.in_use&&hook?.state!=='observed'?`<div class="runtime-hook-apply-note">${esc(t('runtimeHookNewRootRequired'))}</div>`:''}
    <div class="runtime-hook-actions"><button class="runtime-hook-action" data-runtime-hook-action="enable" data-scope="device" data-provider="${esc(hook?.provider||'')}">${esc(t('runtimeHookApproveSetup'))}</button><button class="runtime-hook-action secondary" data-runtime-hook-action="disable" data-scope="device" data-provider="${esc(hook?.provider||'')}">${esc(t('runtimeHookDisable'))}</button><button class="runtime-hook-action secondary" data-runtime-hook-trust="${esc(hook?.provider||'')}" aria-expanded="${state.runtimeHookGuideProvider===hook?.provider}" aria-controls="runtime-hook-guide-workload-${esc(hook?.provider||'')}">${esc(t('runtimeHookTrustAction',{provider}))}</button></div>
    ${runtimeHookProviderGuide(hook)}
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
function runtimeRootStatusLabel(status) {
  return {
    active:t('runtimeRootActiveStatus'),
    needs_check:t('runtimeRootNeedsCheckStatus'),
    terminal:t('runtimeRootTerminal'),
    inactive_terminal:t('runtimeRootInactiveTerminal'),
  }[status]||status||'-';
}
function runtimeRootStatusClass(status) {
  return status==='active'?'ok':status==='needs_check'?'warn':status==='inactive_terminal'?'warn':'';
}
function runtimeRootCard(root,attempts,findingsByAttempt,attemptRootMap) {
  const key=String(root.root_session_id||root.provider_session_id||root.display_name||'root');
  const remembered=Object.prototype.hasOwnProperty.call(state.runtimeRootDisclosure,key);
  const defaultOpen=root.status==='active'||root.status==='needs_check';
  const open=remembered?Boolean(state.runtimeRootDisclosure[key]):defaultOpen;
  const sessionAttempts=attempts.filter(a=>attemptRootMap[a.attempt_id]===root.root_session_id);
  const createdLabel=root.created_at_source==='provider_metadata'?t('runtimeRootCreated'):t('runtimeRootObservedStart');
  const providerID=root.provider_session_id||'-';
  const fallbackHint=root.display_name_source==='task_mecca_fallback'
    ? `<span class="runtime-root-fallback" title="${esc(t('runtimeRootFallbackHint'))}">fallback</span>`
    : '';
  const cleanupBadge=root.cleanup_eligible?`<span class="badge warn">${esc(t('runtimeRootCleanupEligible'))}</span>`:'';
  return `<details class="runtime-root-card" data-runtime-root-key="${esc(key)}" ${open?'open':''}>
    <summary class="runtime-root-summary">
      <div class="runtime-root-title-wrap">
        <div class="runtime-root-title">${esc(root.display_name||'Root Session')} ${fallbackHint}</div>
        <div class="runtime-root-id"><span>${esc(t('runtimeRootProvider'))} <strong>${esc(String(root.provider||'-').toUpperCase())}</strong></span><span>${esc(t('runtimeRootID'))} <strong title="${esc(providerID)}">${esc(providerID)}</strong></span></div>
      </div>
      <div class="runtime-root-badges">
        <span class="badge ${runtimeRootStatusClass(root.status)}">${esc(runtimeRootStatusLabel(root.status))}</span>
        ${cleanupBadge}
      </div>
      <div class="runtime-root-meta">
        <span>${esc(createdLabel)} <strong>${esc(dateTimeLabel(root.created_at,true))}</strong></span>
        <span>${esc(t('runtimeRootLastActivity'))} <strong>${esc(root.last_activity_at?ago(root.last_activity_at):'-')}</strong></span>
      </div>
      <div class="runtime-root-counts">
        <span>${esc(t('runtimeRootAgents',{n:Number(root.agent_count||0)}))}</span>
        <span>${esc(t('runtimeRootCurrentCount',{n:Number(root.current_count||0)}))}</span>
        <span>${esc(t('runtimeRootNeedsCount',{n:Number(root.needs_check_count||0)}))}</span>
        <span>${esc(t('runtimeRootTerminalCount',{n:Number(root.terminal_count||0)}))}</span>
      </div>
      <span class="runtime-root-chevron" aria-hidden="true">›</span>
    </summary>
    <div class="runtime-root-body">
      ${sessionAttempts.length?`<div class="runtime-attempt-grid">${sessionAttempts.map(a=>runtimeAttemptCard(a,findingsByAttempt[a.attempt_id]||[])).join('')}</div>`:`<div class="runtime-empty compact">-</div>`}
    </div>
  </details>`;
}
function runtimeRootListRow(root) {
  const statusClass=runtimeRootStatusClass(root.status);
  const cleanup=root.cleanup_eligible
    ? `<button class="runtime-root-cleanup-btn" data-runtime-root-cleanup="${esc(root.root_session_id||'')}" data-runtime-root-name="${esc(root.display_name||'Root Session')}" data-runtime-root-bytes="${Number(root.cleanup_bytes||root.storage_bytes||0)}">${esc(t('runtimeRootCleanup'))}</button>`
    : '';
  return `<div class="runtime-root-list-row">
    <div class="runtime-root-list-main">
      <strong>${esc(root.display_name||'Root Session')}</strong>
      <span>${esc(t('runtimeRootProvider'))} ${esc(String(root.provider||'-').toUpperCase())}</span>
      <small>${esc(t('runtimeRootID'))} ${esc(root.provider_session_id||'-')}</small>
    </div>
    <span class="badge ${statusClass}">${esc(runtimeRootStatusLabel(root.status))}</span>
    <div class="runtime-root-list-meta">
      <span>${esc(root.last_activity_at?ago(root.last_activity_at):'-')}</span>
      <span>${esc(fmtBytes(root.storage_bytes||0))}</span>
    </div>
    ${cleanup}
  </div>`;
}
function runtimeRootListPanel() {
  if(!state.runtimeRootListOpen)return '';
  if(state.runtimeRootListLoading)return `<section class="runtime-root-list-panel"><div class="runtime-history-loading">${esc(t('runtimeRootLoading'))}</div></section>`;
  if(state.runtimeRootListError)return `<section class="runtime-root-list-panel"><div class="runtime-history-error">${esc(state.runtimeRootListError)}</div></section>`;
  const data=state.runtimeRootList||{}, items=data.items||[], page=Number(data.page||1), pages=Number(data.total_pages||0), total=Number(data.total||0);
  const rows=items.length?items.map(runtimeRootListRow).join(''):`<div class="runtime-history-empty">${esc(t('runtimeRootEmpty'))}</div>`;
  const pager=pages>1?`<div class="runtime-history-pager">
    <button data-runtime-root-page="${Math.max(1,page-1)}" ${page<=1?'disabled':''}>←</button>
    <span>${esc(t('runtimeRootPage',{page,pages,total}))}</span>
    <button data-runtime-root-page="${Math.min(pages,page+1)}" ${page>=pages?'disabled':''}>→</button>
  </div>`:`<div class="runtime-history-page-label">${esc(t('runtimeRootPage',{page:pages?1:0,pages,total}))}</div>`;
  return `<section class="runtime-root-list-panel"><div class="runtime-root-list">${rows}</div>${pager}</section>`;
}
function workloadView() {
  const snapshot=state.snapshot||{}, w=snapshot.workload||{}, agents=w.agents||[], all=snapshot.all_items||{}, unassigned=w.unassigned_doing||[], released=w.released_holds||[];
  const runtime=snapshot.runtime_observability||{}, attempts=runtime.attempts||[], findings=runtime.findings||[], hooks=runtime.hooks||[], rc=runtime.counts||{}, historyMeta=runtime.history||{};
  const rootSessions=runtime.root_sessions||[], rootCounts=runtime.root_session_counts||{}, attemptRootMap=runtime.attempt_root_sessions||{};
  const currentRoots=rootSessions.filter(r=>r.status==='active'||r.status==='needs_check');
  const recentPreviousRoots=rootSessions.filter(r=>r.status==='terminal'||r.status==='inactive_terminal');
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
    <div class="metrics runtime-metrics runtime-root-metrics">
      <div class="metric"><strong>${Number(rootCounts.active||0)}</strong><span>${esc(t('runtimeRootActive'))}</span></div>
      <div class="metric"><strong>${Number(rootCounts.needs_check||0)}</strong><span>${esc(t('runtimeRootNeedsCheck'))}</span></div>
      <div class="metric"><strong>${Number(rootCounts.terminal||0)}</strong><span>${esc(t('runtimeRootPrevious'))}</span></div>
      <div class="metric"><strong>${Number(rootCounts.cleanup_eligible||0)}</strong><span>${esc(t('runtimeRootCleanupEligible'))}</span></div>
    </div>
    <div class="runtime-storage-toolbar">
      <button class="runtime-history-toggle" id="runtimeStorageToggle">${esc(state.runtimeStorageOpen?t('runtimeStorageClose'):t('runtimeStorageOpen'))}</button>
    </div>
    ${runtimeStoragePanel()}
    <div class="runtime-root-section">
      <div class="runtime-attempt-section-head"><h3>${esc(t('runtimeRootSessions'))}</h3></div>
      ${currentRoots.length?`<div class="runtime-root-stack">${currentRoots.map(root=>runtimeRootCard(root,attempts,findingsByAttempt,attemptRootMap)).join('')}</div>`:`<div class="runtime-empty compact">${esc(t('runtimeNoCurrentExecutions'))}</div>`}
    </div>
    <div class="runtime-root-section runtime-root-previous">
      <div class="runtime-attempt-section-head">
        <h3>${esc(t('runtimeRootPrevious'))} · ${Number(rootCounts.terminal||0)}</h3>
        ${Number(rootCounts.terminal||0)>0?`<button class="runtime-history-toggle" id="runtimeRootListToggle">${esc(state.runtimeRootListOpen?t('runtimeRootHidePrevious'):t('runtimeRootShowPrevious'))}</button>`:''}
      </div>
      ${recentPreviousRoots.length?`<div class="runtime-root-stack">${recentPreviousRoots.map(root=>runtimeRootCard(root,attempts,findingsByAttempt,attemptRootMap)).join('')}</div>`:`<div class="runtime-empty compact">${esc(t('runtimeRootEmpty'))}</div>`}
      ${runtimeRootListPanel()}
    </div>
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
  return `<div class="page-head"><div><div class="eyebrow">${esc(t('operationsEyebrow'))}</div><h1>${esc(t('workload'))}</h1><p class="summary">${esc(t('workloadIntro'))}</p></div></div>${runtimePanel}${renderOperationStages(state.operationPayload.stages||[])}${backlogPanel}`;
}
function diagnosticEntries(snapshot){const rows=[];for(const [type,value] of Object.entries(snapshot?.health||{})){if(Array.isArray(value))for(const item of value)rows.push({type,item});else if(value&&typeof value==='object'&&Object.keys(value).length)rows.push({type,item:value});}for(const item of snapshot?.diagnostics||[])rows.push({type:item.component||'doctor',item});return rows;}
function currentDiagnosticSource(){return state.diagnosticSnapshots[state.project+'|'+state.backlog];}
function updateDiagnosticNavigation(){const button=document.querySelector('[data-view="issues"]');if(!button)return;const source=currentDiagnosticSource(),rows=diagnosticEntries(source?.snapshot);button.hidden=!state.project||(!source?.error&&!rows.length);const count=$('#issueCount');if(count)count.textContent=source?.error?'!':rows.length||'';}
async function refreshDiagnostics(project=state.project,backlog=state.backlog,signature=''){if(!project)return;const key=project+'|'+backlog,prior=state.diagnosticSnapshots[key];if(signature&&prior?.signature===signature&&!prior.error&&Date.now()-(prior.checkedAt||0)<15000)return;const request=(state.diagnosticRequests[key]||0)+1;state.diagnosticRequests[key]=request;const sourceAtStart=state.attentionScopes[key]||backlog;const current=()=>state.diagnosticRequests[key]===request&&(!sourceAtStart||!state.attentionScopes[key]||state.attentionScopes[key]===sourceAtStart);try{const params=new URLSearchParams({project});if(backlog)params.set('backlog',backlog);const response=await fetch('/api/issues?'+params,{cache:'no-store'});if(!response.ok)throw Error('HTTP '+response.status);const snapshot=await response.json();if(!current())return;const previous=state.diagnosticSnapshots[key],at=Date.parse(snapshot.snapshot_at||'');if(previous?.observed&&(!Number.isFinite(at)||at<previous.observed))return;state.diagnosticSnapshots[key]={snapshot,signature,checkedAt:Date.now(),observed:Number.isFinite(at)?at:0};}catch(_){if(!current())return;state.diagnosticSnapshots[key]={snapshot:prior?.snapshot,error:true};}updateDiagnosticNavigation();if(state.project===project&&state.backlog===backlog&&state.view==='issues')render();}
function diagnosticRecoveryRequest(entry,project=state.project,backlog=state.backlog){return ['Task Mecca 백로그 복원 요청','프로젝트: '+project,'백로그: '+(backlog||'자동 선택'),'진단 유형: '+entry.type,'문제 파일·항목·근거: '+JSON.stringify(entry.item,null,2),'요청: 위 진단의 현재 근거를 먼저 확인하고 원본 계약과 사용자 변경을 보존하는 복원 방안을 제시해 주세요. 이 복사만으로 Agent 실행·자동 복원·작업 재개를 승인하거나 수행한 것은 아닙니다.'].join('\n');}
function backlogDiagnosticsView(){const source=currentDiagnosticSource(),rows=diagnosticEntries(source?.snapshot);return `<div class="page-head"><div><h1>${esc(t('backlogDiagnostics'))}</h1><p class="summary">${esc(state.project)}${state.backlog?' · '+esc(state.backlog):''}</p></div></div>${source?.error?`<p class="load-error">${esc(t('diagnosticUnavailable'))} · ${esc(state.language==='ko'?'검사를 다시 시도해 주세요.':'Retry the diagnostic check.')} <button type="button" class="action-btn secondary" id="retryDiagnostics">${esc(t('refresh'))}</button></p>`:''}${rows.length?rows.map((entry,index)=>`<section class="diagnostic-entry"><h2>${esc(entry.type)}</h2><pre><code>${esc(JSON.stringify(entry.item,null,2))}</code></pre><button type="button" class="action-btn secondary" data-copy-diagnostic="${index}">${esc(t('diagnosticCopy'))}</button></section>`).join(''):`<div class="empty">${esc(source?t('diagnosticNone'):t('loading'))}</div>`}<p class="diagnostic-copy-status" role="status"></p>`;}
function bindDiagnosticActions(){document.querySelectorAll('[data-copy-diagnostic]').forEach(button=>button.addEventListener('click',async()=>{const entry=diagnosticEntries(currentDiagnosticSource()?.snapshot)[Number(button.dataset.copyDiagnostic)];if(!entry)return;try{await navigator.clipboard.writeText(diagnosticRecoveryRequest(entry));$('.diagnostic-copy-status').textContent=t('diagnosticCopyDone');}catch(_){$('.diagnostic-copy-status').textContent=t('copyFailed');}}));$('#retryDiagnostics')?.addEventListener('click',()=>refreshDiagnostics());}

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
  const lifecycleBody=`${timingInferred?`<div class="timing-note"><strong>${esc(t('provisionalTiming'))}</strong><span>${esc(t('provisionalTimingDetail'))}</span></div>`:''}${timingIncomplete?`<div class="timing-note warning"><strong>${esc(t('incompleteHistory'))}</strong><span>${esc(t('incompleteHistoryDetail'))}</span></div>`:''}${events.length?`<div class="timeline">${events.map((e,i)=>{
    const isCurrent=i===events.length-1 && e.state!=='done' && Boolean(e.at) && Number.isFinite(new Date(e.at).getTime());
    const duration=isCurrent?fmtSec(Math.max(0,(Date.now()-new Date(e.at).getTime())/1000)):(e.interval||'-');
    const durationHTML=duration&&duration!=='-'?`${esc(t('stayed'))} <span${isCurrent?` class="live-lifecycle-interval" data-state-at="${esc(e.at)}"`:''}>${esc(duration)}</span>`:'';
    return `<div class="timeline-event"><span class="timeline-dot"></span><span class="timeline-time">${esc(new Date(e.at).toLocaleString(localeCode()))}</span><div class="timeline-label"><strong>${esc(lifecycleEventLabel(e.label))}${e.provisional?` · ${esc(t('provisional'))}`:''}</strong><small>${durationHTML}${lifecycleEvidenceLabel(e)?` · ${esc(lifecycleEvidenceLabel(e))}`:''}</small></div></div>`;
  }).join('')}</div>`:`<p class="summary">${esc(t('noLifecycle'))}</p>`}`;
  const lifecycle=`<section class="section detail-section lifecycle-section" id="lifecycle" data-toc-label="${esc(t('lifecycle'))}"><h2>${esc(t('lifecycle'))}</h2>${lifecycleBody}</section>`;
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
  const rendered=`${humanSummaryCard(task)}${lifecycle}${related}${operations}${passiveAlert}${contractSections(task)}${progress}${verification}`;
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
  const p=new URLSearchParams(); if(state.project)p.set('project',state.project); if(state.backlog)p.set('backlog',state.backlog); if(state.tagFilters.length)p.set('tags',state.tagFilters.join(',')); history.pushState({},'',`/tasks/${encodeURIComponent(id)}${p.toString()?`?${p.toString()}`:''}`); render(); loadTaskDetail(id);
}
function closeTask() {
  state.detail=null; state.detailTask=null; state.loadError=''; history.pushState({},'',state.view==='backlog'?backlogUrl():`/?view=${state.view}`); render();
}
function bindRows() {
  bindBacklogListTools();
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
function updateAutoListPageSize(force=false) {
  if(state.listPageMode!=='auto'||state.view!=='backlog'||state.detail)return;
  if(state.listData&&!force)return;
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
  if(state.listData && !state.pendingContentUpdate)refreshList();
}
function scheduleAutoListPageSize(force=false) {
  if(state.listPageMode!=='auto')return;
  cancelAnimationFrame(autoPageMeasureRaf);
  autoPageMeasureRaf=requestAnimationFrame(()=>requestAnimationFrame(()=>updateAutoListPageSize(force)));
}

function applySidebarState() {
  state.sidebarCollapsed=state.sidebarMode!=='expanded';
  const shell=document.querySelector('.app-shell');
  shell?.classList.toggle('sidebar-collapsed',state.sidebarCollapsed);
  shell?.classList.toggle('sidebar-peek',state.sidebarMode==='auto' && state.sidebarPeek);
  shell?.setAttribute('data-sidebar-mode',state.sidebarMode);
  const isMobile=window.matchMedia('(max-width:680px)').matches;
  const mobileOpen=isMobile && state.mobileNavOpen;
  shell?.classList.toggle('mobile-nav-open',mobileOpen);
  document.body.classList.toggle('mobile-nav-visible',mobileOpen);
  const scrim=$('#mobileSidebarScrim');
  if(scrim)scrim.hidden=!mobileOpen;
  const panel=$('#mobileSidebarPanel');
  if(panel){
    if(mobileOpen){
      panel.setAttribute('role','dialog');
      panel.setAttribute('aria-modal','true');
      panel.setAttribute('aria-label',state.language==='ko'?'모바일 메뉴':'Mobile navigation');
    }else{
      panel.removeAttribute('role');
      panel.removeAttribute('aria-modal');
      panel.removeAttribute('aria-label');
    }
  }
  const btn=$('#sidebarToggle');
  if(!btn)return;
  if(isMobile){
    btn.textContent=state.mobileNavOpen?'×':'☰';
    btn.title=state.mobileNavOpen?(state.language==='ko'?'메뉴 닫기':'Close menu'):(state.language==='ko'?'메뉴 열기':'Open menu');
    btn.setAttribute('aria-expanded',String(state.mobileNavOpen));
  }else{
    btn.textContent=state.sidebarMode==='expanded'?'‹':'›';
    btn.title=state.sidebarMode==='expanded'?(state.language==='ko'?'최소화 고정':'Pin compact'):(state.language==='ko'?'펼침 고정':'Pin expanded');
    btn.setAttribute('aria-expanded',String(state.sidebarMode==='expanded'||state.sidebarPeek));
  }
  btn.setAttribute('aria-label',btn.title);
}
function setSidebarMode(mode) {
  if(!['auto','expanded','compact'].includes(mode))return;
  state.sidebarMode=mode;
  // When Auto is selected while the pointer or focus is inside the sidebar,
  // keep the temporary overlay open until the user actually leaves.
  const sidebar=$('#sidebar');
  state.sidebarPeek=mode==='auto' && Boolean(sidebar?.matches(':hover')||sidebar?.contains(document.activeElement));
  localStorage.setItem('task-mecca-sidebar-mode',mode);
  applySidebarState();
  scheduleAutoListPageSize();
}
function closeMobileNav(restoreFocus=false){
  if(!state.mobileNavOpen)return;
  state.mobileNavOpen=false;
  state.projectMenuOpen=false;
  applySidebarState();
  if(restoreFocus)$('#sidebarToggle')?.focus({preventScroll:true});
}
function toggleSidebar() {
  if(window.matchMedia('(max-width:680px)').matches){
    if(state.mobileNavOpen){
      closeMobileNav();
    }else{
      state.mobileNavOpen=true;
      applySidebarState();
      const panel=$('#mobileSidebarPanel');
      const selected=panel?.querySelector('.nav-item.active');
      (selected||panel?.querySelector('#hubNavBtn')||panel?.querySelector('button'))?.focus({preventScroll:true});
    }
    return;
  }
  setSidebarMode(state.sidebarMode==='expanded'?'compact':'expanded');
}

let notificationSettingsDraft=null;
function render() {
  const settingsElement=$('#notificationSettingsBody');
  const settingsDraft=settingsElement?.dataset.context===notificationHistoryContext()?preserveNotificationSettings(settingsElement):notificationSettingsDraft?.context===notificationHistoryContext()?notificationSettingsDraft.draft:null;
 renderUserAttention();
  if(document.querySelector('.hub-confirm-overlay'))return;
  if(document.activeElement?.closest?.('#userAttention'))return;
  const hubHistoryFocus=document.activeElement?.id==='hubHistoryToggle';
  const searchFocus=document.activeElement?.id==='search' ? {start:document.activeElement.selectionStart,end:document.activeElement.selectionEnd} : null;
  nav(); translateChrome(); renderAccess(); applySidebarState(); updateNotificationIndicator(); renderGlobalUpdateIndicator(); renderContentUpdatePrompt(); renderReleaseUnreadPrompt();
  const c=$('#content'), data=currentProjectData();
  if(state.view==='release-notes'){
    c.innerHTML=releaseNotesView();
    bindReleaseNotesActions();
    renderReleaseNoteModal();
    return;
  }
  const manualReady=state.view==='manual'||state.view==='notifications'||state.view==='issues';
  const snapshotView=['workload','attention'].includes(state.view);
  const viewDataReady=manualReady || (snapshotView ? Boolean(state.snapshot) : Boolean(data));
  if (!viewDataReady && !state.detailTask) {
    if (state.view === 'hub' && state.hub) {
      c.innerHTML=hubView(); bindHubActions(); if(hubHistoryFocus)document.querySelector('#hubHistoryToggle')?.focus({preventScroll:true}); return;
    }
    if (state.loadError) {
      c.innerHTML=`<div class="load-error"><h2>Project dashboard could not be loaded</h2><p><strong>Project</strong> ${esc(state.project||'-')}</p><p>${esc(state.loadError)}</p><div class="project-actions"><button class="action-btn secondary" id="retryProjectBtn">Retry</button><button class="action-btn" id="backToHubBtn">Back to Projects</button></div></div>`;
      $('#retryProjectBtn')?.addEventListener('click',()=>state.detail?loadTaskDetail(state.detail):refresh());
      $('#backToHubBtn')?.addEventListener('click',()=>{state.project='';state.backlog='';state.listData=null;state.snapshot=null;state.view='hub';history.pushState({},'','/?view=hub');render();refresh();});
      return;
    }
    c.innerHTML=`<div class="loading">${esc(t('loading'))}</div>`; return;
  }
  const gate=accessBanner()+runtimeHookOnboardingBanner()+diagnosticBanner();
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
  c.innerHTML=(state.view==='hub'?hubView():gate+(state.view==='manual'?manualView():state.view==='notifications'?notificationCenterView():state.view==='workload'?workloadView():state.view==='attention'?attentionView():state.view==='issues'?backlogDiagnosticsView():listView()));
  bindRows(); if(state.view==='hub') bindHubActions();
  if(searchFocus && state.view==='backlog' && !state.detail){const search=$('#search');search?.focus({preventScroll:true});search?.setSelectionRange(searchFocus.start,searchFocus.end);}
  if(state.view==='hub'&&hubHistoryFocus)document.querySelector('#hubHistoryToggle')?.focus({preventScroll:true});
  document.querySelectorAll('[data-runtime-hook-action]').forEach(button=>{
    button.addEventListener('click',e=>performRuntimeHookAction(e.currentTarget));
  });
  document.querySelectorAll('[data-runtime-hook-trust]').forEach(button=>{
    button.addEventListener('click',()=>toggleRuntimeHookGuide(button.dataset.runtimeHookTrust));
  });
  if(state.view==='workload'){
    $('#runtimeHistoryToggle')?.addEventListener('click',()=>toggleRuntimeHistory());
    $('#runtimeStorageToggle')?.addEventListener('click',()=>toggleRuntimeStorage());
    $('#runtimeCleanupBtn')?.addEventListener('click',e=>performRuntimeCleanup(e.currentTarget));
    $('#runtimeRootListToggle')?.addEventListener('click',()=>toggleRuntimeRootList());
    document.querySelectorAll('[data-runtime-root-page]').forEach(button=>{
      button.addEventListener('click',()=>loadRuntimeRootSessions(Number(button.dataset.runtimeRootPage||1)));
    });
    document.querySelectorAll('[data-runtime-root-cleanup]').forEach(button=>{
      button.addEventListener('click',e=>performRootSessionCleanup(e.currentTarget));
    });
    document.querySelectorAll('.runtime-root-card[data-runtime-root-key]').forEach(details=>{
      details.addEventListener('toggle',()=>{
        state.runtimeRootDisclosure[details.dataset.runtimeRootKey]=details.open;
      });
    });
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
  if(state.view==='issues')bindDiagnosticActions();
  if(state.view==='notifications'){
    document.querySelectorAll('[data-center-tab]').forEach(button=>button.addEventListener('click',()=>{const body=$('#notificationSettingsBody');if(body)notificationSettingsDraft={context:notificationHistoryContext(),draft:preserveNotificationSettings(body)};state.notificationCenterTab=button.dataset.centerTab;if(state.notificationCenterTab==='history'&&state.notificationHistoryPage===1)captureHistoryWindow();render();document.querySelector(`[data-center-tab="${state.notificationCenterTab}"]`)?.focus({preventScroll:true});if(state.notificationCenterTab==='history')loadNotificationHistory();}));
    if(state.notificationCenterTab==='settings'){renderNotificationPanel();const body=$('#notificationSettingsBody');if(body){body.dataset.context=notificationHistoryContext();if(settingsDraft)restoreNotificationSettings(body,settingsDraft);}}
    bindNotificationHistoryActions();
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
let listRefreshContext='';
let listRefreshAbort=null;

function listNotificationPayload(data) {
  const current={...(data?.attention_items||{})};
  (data?.items||[]).forEach(task=>{current[task.id]=task});
  return {_listSnapshot:true,content_revision:data?.revision,snapshot_at:data?.snapshot_at,backlog_selection:data?.backlog_selection,project_path:data?.project_path,all_items:current,notification_events:data?.notification_events||[],attention:data?.attention||[]};
}

async function refreshHub(force=false) {
  const fresh=state.hub && Date.now()-state.lastHubFetch<60000;
  if(!force && fresh)return state.hub;
  if(hubFetchInFlight)return hubFetchInFlight;
  hubFetchInFlight=(async()=>{
    try {
      const [r, management]=await Promise.all([fetch('/api/hub',{cache:'no-store'}),fetch('/api/hub/projects',{cache:'no-store'})]);
      if(!r.ok)throw new Error(`HTTP ${r.status}`);
      state.hub=await r.json();
      state.hubManagement=management.ok?await management.json():{projects:[],history:[]};
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

// Cache rows only. Notification/current-attention snapshots always come from a
// fresh response; cached pages must never resurrect resolved user alerts.
const listPageCaches=new Map();
const listCacheEpochs=new Map();
const listSourceSignatures=new Map();
function invalidateProjectPages(project,keepKey=null){
 for(const [key,cache] of listPageCaches){if(cache.project!==project||key===keepKey)continue;const source=listSourceKey(cache.project,cache.backlog);listCacheEpochs.set(source,listCacheEpoch(source)+1);listPageCaches.delete(key);listSummaryLatest.delete(key);}
}
let listSummarySerial=0;
let listSummaryInFlight=null,listSummaryQueued=false;
const listSummaryObserved=new Map();
const listSummaryLatest=new Map();
function cachedListRows(data,key){const latest=listSummaryLatest.get(key);return latest?{...data,counts:latest.counts,tag_catalog:latest.tag_catalog,access:latest.access}:data;}
function listContextKey(query=listQueryString()) {
 const params=new URLSearchParams(query); params.delete('page');
 for(const field of ['status','tags'])if(params.has(field))params.set(field,params.get(field).split(',').sort().join(','));
 params.sort(); return params.toString();
}
function listSourceKey(project=state.project,backlog=state.backlog){return JSON.stringify([project,backlog]);}
function listCacheEpoch(source=listSourceKey()){return listCacheEpochs.get(source)||0;}
function invalidateListPages(){
 const source=listSourceKey();listCacheEpochs.set(source,listCacheEpoch(source)+1);
 for(const [key,cache] of listPageCaches)if(cache.project===state.project){listPageCaches.delete(key);listSummaryLatest.delete(key);}
}
function pageCache(key,project=state.project,backlog=state.backlog){
 let cache=listPageCaches.get(key);
 if(!cache){cache={project,backlog,pages:new Map(),pending:new Map(),current:1};listPageCaches.set(key,cache);}
 // Bound inactive contexts as well as pages within each context.
 if(listPageCaches.size>12){const oldest=listPageCaches.keys().next().value;if(oldest!==key)listPageCaches.delete(oldest);}
 return cache;
}
function pruneListPages(cache,current,pages){
 cache.current=current;
 for(const page of cache.pages.keys())if(page<1||page>pages||Math.abs(page-current)>1)cache.pages.delete(page);
}
function storeListPage(key,data,epoch,source){
 if(listCacheEpoch(source)!==epoch)return;
 const cache=pageCache(key);const page=Math.max(1,Number(data.page)||1);
 const prior=cache.pages.get(page);
 if(prior && Date.parse(prior.snapshot_at)>Date.parse(data.snapshot_at))return;
 if(data.revision&&cache.revision&&data.revision!==cache.revision)cache.pages.clear();
 if(data.revision)cache.revision=data.revision;
 if(Math.abs(page-cache.current)<=1)cache.pages.set(page,data);
 pruneListPages(cache,cache.current,Math.max(1,Number(data.pages)||1));
}
function prefetchListPages(query,data,epoch,source){
 if(!Number(data.pages)||!Number(data.page_size))return;
 const key=listContextKey(query),cache=pageCache(key);
 const current=Number(data.page)||1,pages=Number(data.pages)||1;
 pruneListPages(cache,current,pages);
 for(const page of [current-1,current+1]){
  if(page<1||page>pages||cache.pages.has(page)||cache.pending.has(page))continue;
  const params=new URLSearchParams(query);params.set('page',String(page));params.set('prefetch','1');
  const promise=(async()=>{try{
   const r=await fetch('/api/backlog/tasks?'+params,{cache:'no-store'});if(!r.ok)return;
   const response=await r.json();
   if(listCacheEpoch(source)!==epoch||listPageCaches.get(key)!==cache)return;
   if(cache.revision&&response.revision!==cache.revision)return;
   storeListPage(key,response,epoch,source);
  }catch(_){}finally{cache.pending.delete(page);}})();cache.pending.set(page,promise);
 }
}
async function refreshListSummary(){
 if(!state.project||state.view!=='backlog'||state.detail)return;
 if(listSummaryInFlight){listSummaryQueued=true;return listSummaryInFlight;}
 const query=listQueryString(),key=listContextKey(query),source=listSourceKey(),epoch=listCacheEpoch(source),serial=++listSummarySerial;
 listSummaryInFlight=(async()=>{try{
  const r=await fetch('/api/backlog/tasks?'+query+'&projection=summary',{cache:'no-store'});if(!r.ok)return;
  const data=await r.json();
  if(serial!==listSummarySerial||listContextKey()!==key||listCacheEpoch(source)!==epoch||state.view!=='backlog'||state.detail)return;
  const observed=Date.parse(data.snapshot_at)||0;if(observed<(listSummaryObserved.get(key)||0))return;listSummaryObserved.set(key,observed);listSummaryLatest.set(key,data);
  if(state.listData){state.listData={...state.listData,counts:data.counts,tag_catalog:data.tag_catalog,access:data.access};}
  processTaskNotifications(listNotificationPayload(data));
  render();
 }catch(_){}finally{listSummaryInFlight=null;if(listSummaryQueued){listSummaryQueued=false;queueMicrotask(()=>{if(!isLiveBacklogPage())refreshListSummary();});}}})();return listSummaryInFlight;
}
async function refreshList(preserveSelection=false) {
 if(!state.project)return;
 const currentContext=listContextKey();
 // A former project's in-flight fetch must never delay opening a new one.
 // Abort only when the query context changes; same-context refreshes keep
 // their existing coalescing semantics.
 if(listRefreshInFlight && listRefreshContext!==currentContext){
  listRefreshAbort?.abort();
  listRefreshInFlight=null;
  listRefreshAbort=null;
  listRefreshContext='';
  listRefreshQueued=false;
 }
 if(state.listData&&state.listDataContext&&state.listDataContext!==currentContext){state.listData=null;state.listRevalidating=true;if(state.view==='backlog')render();}
 if(listRefreshInFlight){
  const cache=listPageCaches.get(listContextKey()),cached=cache?.pages.get(state.listPage||1);
  if(cached&&!preserveSelection){state.listData=cachedListRows(cached,listContextKey());state.listDataContext=listContextKey();state.listRevalidating=true;if(state.view==='backlog')render();}
  listRefreshQueued=true;return listRefreshInFlight;
 }
 const targetProject=state.project,targetBacklog=state.backlog;
 const targetQuery=listQueryString(),targetView=state.view,targetDetail=state.detail;
 const key=listContextKey(targetQuery),source=listSourceKey(),epoch=listCacheEpoch(source);
 const cache=pageCache(key,targetProject,targetBacklog),requestedPage=Math.max(1,state.listPage||1);
 cache.current=requestedPage;pruneListPages(cache,requestedPage,Infinity);
 const selectedID=preserveSelection?state.listData?.items?.[state.selectedIndex]?.id:null;
 const isCurrent=()=>targetProject===state.project&&targetBacklog===state.backlog&&targetQuery===listQueryString()&&targetView===state.view&&targetDetail===state.detail&&listCacheEpoch(source)===epoch&&listPageCaches.get(key)===cache;
 const cached=cache.pages.get(requestedPage);
 if(cached && !preserveSelection){state.listData=cachedListRows(cached,listContextKey());state.listDataContext=listContextKey();state.listRevalidating=true;state.loadError='';if(state.view==='backlog')render();}
 const controller=new AbortController();
 listRefreshAbort=controller;listRefreshContext=key;
 const request=(async()=>{try{
  const r=await fetch('/api/backlog/tasks?'+targetQuery,{cache:'no-store',signal:controller.signal});
  if(!r.ok){let detail='';try{detail=(await r.json()).error||'';}catch(_){}throw new Error(detail||`HTTP ${r.status}`);}
  const data=await r.json();if(!isCurrent())return;
  cache.current=Math.max(1,Number(data.page)||1);storeListPage(key,data,epoch,source);
  const observed=Date.parse(data.snapshot_at)||0;if(observed<(listSummaryObserved.get(key)||0)){if(observed)listRefreshQueued=true;return;}listSummaryObserved.set(key,observed);listSummaryLatest.set(key,data);
  state.listData=data;state.listDataContext=key;state.listRevalidating=false;
  const selectedIndex=(data.items||[]).findIndex(task=>task.id===selectedID);if(selectedIndex>=0)state.selectedIndex=selectedIndex;
  state.loadError='';state.lastFetch=Date.now();state.listPage=Math.max(1,Number(data.page)||1);
  acceptContentRevision(data.revision||state.contentRevision);
  processTaskNotifications(listNotificationPayload(data));
  const candidates=data.backlog_selection?.candidates||[];
  if(state.backlog&& !candidates.some(x=>x.path===state.backlog)){state.backlog='';localStorage.removeItem('task-mecca-backlog-folder');}
  $('#connectionDot').style.background='var(--ok)';if(state.view==='backlog')render();
  // Start adjacent requests only after the foreground rows have rendered.
  const acceptedQuery=listQueryString();
  requestAnimationFrame(()=>requestAnimationFrame(()=>{if(targetProject===state.project&&targetBacklog===state.backlog&&acceptedQuery===listQueryString()&&targetView===state.view&&targetDetail===state.detail&&listCacheEpoch(source)===epoch&&listPageCaches.get(key)===cache)prefetchListPages(acceptedQuery,data,epoch,source);}));
 }catch(e){if(!isCurrent())return;state.listRevalidating=false;state.loadError=String(e?.message||e||'Unknown error');$('#connectionDot').style.background='var(--danger)';if(!state.listData)render();}
 finally{
  // A canceled earlier request must not clear or queue over a newer one.
  if(listRefreshInFlight===request){
   listRefreshInFlight=null;listRefreshAbort=null;listRefreshContext='';
   if(listRefreshQueued){listRefreshQueued=false;queueMicrotask(()=>refreshList());}
  }
 }})();
 listRefreshInFlight=request;
 return request;
}

function closeAttentionStream() {
  if(state.eventSource){ try{state.eventSource.close()}catch(_){} }
  state.eventSource=null;
  state.eventStreamKey='';
}

function attentionPayloadRevision(payload) {
  const parts=[];
  (payload?.attention||[]).forEach(row=>parts.push(['a',row.id,row.type,row.health,row.runtime_state,row.last_activity_at,row.title,row.message,row.resume_condition].map(x=>String(x??'')).join('|')));
  // Durable delivery events do not change backlog content; only attention semantics do.
  parts.sort();
  return parts.join('\n');
}

function ensureAttentionStream() {
  if(!state.project||typeof EventSource==='undefined')return;
  const params=new URLSearchParams();
  params.set('project',state.project);
  const requestedKey=state.project+'|'+state.backlog;
  const sourceBacklog=state.backlog||state.attentionScopes[requestedKey];
  if(!sourceBacklog)return;
  params.set('backlog',sourceBacklog);
  const key=requestedKey+'|'+sourceBacklog;
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
    if(state.eventStreamKey!==key||state.project+'|'+state.backlog!==requestedKey||(state.backlog||state.attentionScopes[requestedKey])!==sourceBacklog)return;
    let payload=null;
    try{payload=JSON.parse(event.data)}catch(_){return}
    processTaskNotifications({...payload,backlog_selection:{...payload.backlog_selection,selected:sourceBacklog}});
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
 const targetBacklog=state.backlog;
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

  if(targetView==='notifications'){
    closeAttentionStream();
    await loadProjectNotificationSettings();
    await loadTelegramStatus();
    await loadNotificationHistory();
    if(targetProject!==state.project || targetBacklog!==state.backlog || state.view!==targetView)return;
    state.loadError=state.projectNotificationSettingsError;
    if(!state.loadError)$('#connectionDot').style.background='var(--ok)';
    render();
    return;
  }

  if(targetView==='manual'){
    await loadManual(state.language);
    if(targetProject!==state.project || targetBacklog!==state.backlog || state.view!==targetView)return;
    state.loadError='';
    $('#connectionDot').style.background='var(--ok)';
    ensureAttentionStream();
    render();
    return;
  }

  if(targetView==='notifications'){loadNotificationHistory();render();return;}
  if(targetView==='issues'){await refreshDiagnostics(targetProject,targetBacklog);return;}

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
    if(targetProject!==state.project || targetBacklog!==state.backlog || state.view!==targetView)return;
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
    if(targetProject!==state.project || targetBacklog!==state.backlog || state.view!==targetView)return;
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
 const previousAttentionContext=state.project+'|'+state.backlog;
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
    state.runtimeRootDisclosure={};
    state.runtimeRootListOpen=false;
    state.runtimeRootListLoading=false;
    state.runtimeRootListError='';
    state.runtimeRootList={items:[],page:1,page_size:10,total:0,total_pages:0,counts:{}};
    state.runtimeHookStatus=null;
    state.runtimeHookStatusLoading=false;
    if(state.project){
      queueMicrotask(()=>refreshVersionInfo(false));
      queueMicrotask(()=>loadRuntimeHookStatus(false));
    }
  }
  if(state.project){
    state.lastProject=state.project;
    localStorage.setItem('task-mecca-last-project',state.project);
    ensureOpenProject(state.project);
  }
  if (!state.detail) {
    state.view=p.get('view')||(state.project?'backlog':'hub');
    if(state.view==='attention')state.view='notifications';
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
  if(previousAttentionContext!==state.project+'|'+state.backlog)clearCurrentUserAttention();
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
  const pairs=[['#workloadText','workload'],['#attentionText','attention'],['#issuesText','issues'],['#projectNotificationsText','projectNotifications'],['#terminalText','terminal'],['#manualText','manual'],['#releaseNotesText','releaseNotes']];
  pairs.forEach(([sel,key])=>{const el=$(sel);if(el)el.textContent=t(key)});
  [['[data-view="workload"]','workload'],['[data-view="attention"]','attention'],['[data-view="issues"]','issues'],['[data-view="notifications"]','projectNotifications'],['#terminalNavBtn','terminal'],['[data-view="manual"]','manual'],['[data-view="release-notes"]','releaseNotes']].forEach(([sel,key])=>{const el=document.querySelector(sel);if(el)el.title=t(key)});
  const centerNav=$('#projectNotificationsNav');if(centerNav)centerNav.setAttribute('aria-label',t('notificationCenter'));
  const sideBtn=$('#sidebarToggle');if(sideBtn){sideBtn.setAttribute('aria-label',state.sidebarCollapsed?t('expandSidebar'):t('collapseSidebar'));sideBtn.title=state.sidebarCollapsed?t('expandSidebar'):t('collapseSidebar')}
  const refresh=$('#refreshBtn'); if(refresh){refresh.title=t('refreshWithUpdates');refresh.setAttribute('aria-label',t('refreshWithUpdates'));}
  const themeGroup=document.querySelector('.theme-switcher'); if(themeGroup){themeGroup.setAttribute('aria-label',t('theme'));themeGroup.title=t('theme');}
  const palette=$('#palettePicker');if(palette){palette.value=state.palette;palette.setAttribute('aria-label',t('theme'));palette.closest('.palette-picker')?.setAttribute('title',t('theme'));}
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
  await refreshOperations();
  if(document.querySelector('.mermaid-wrap')) renderMermaidDiagrams(true);
}

function syncThemeBrowserChrome() {
  const effective=effectiveTheme(),mecca=state.palette==='mecca';
  const background=mecca
    ?(effective==='dark'?'#0a0e1b':'#f7f3f8')
    :(effective==='dark'?'#111318':'#f7f8fa');
  document.documentElement.style.colorScheme=effective;
  // The inline background only protects the first paint. Once app.js is
  // loaded, stylesheet colors and the body gradient take over.
  document.documentElement.style.removeProperty('background-color');
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content',background);
}
function applyPalette(value) {
  state.palette=['mecca','slate'].includes(value)?value:'mecca';
  document.documentElement.dataset.palette=state.palette;
  localStorage.setItem('task-mecca-palette',state.palette);
  syncThemeBrowserChrome();
  const picker=$('#palettePicker');if(picker)picker.value=state.palette;
  if (document.querySelector('.mermaid-wrap')) renderMermaidDiagrams(true);
}

function applyTheme(value) {
  state.theme=['system','light','dark'].includes(value)?value:'dark';
  document.documentElement.dataset.theme=effectiveTheme();
  document.documentElement.dataset.themePreference=state.theme;
  localStorage.setItem('task-mecca-theme',state.theme);
  syncThemeBrowserChrome();
  document.querySelectorAll('[data-theme-choice]').forEach(b=>{b.classList.toggle('active',b.dataset.themeChoice===state.theme);b.setAttribute('aria-pressed',b.dataset.themeChoice===state.theme?'true':'false')});
  if (document.querySelector('.mermaid-wrap')) renderMermaidDiagrams(true);
}
translateChrome();
applyPalette(state.palette);
applyTheme(state.theme);
document.querySelectorAll('[data-theme-choice]').forEach(b=>b.addEventListener('click',()=>applyTheme(b.dataset.themeChoice)));
$('#palettePicker')?.addEventListener('change',e=>applyPalette(e.target.value));
window.matchMedia?.('(prefers-color-scheme: dark)').addEventListener?.('change',()=>{if(state.theme==='system'){document.documentElement.dataset.theme=effectiveTheme();syncThemeBrowserChrome();renderMermaidDiagrams(true)}});
$('#languagePicker').addEventListener('change',e=>setLanguage(e.target.value));
let searchRefreshTimer=0;
function bindBacklogListTools() {
  if(state.view!=='backlog'||state.detail)return;
  renderBacklogPicker();
  $('#backlogPicker')?.addEventListener('change',e=>{
  const value=e.target.value;
  if(value===state.backlog)return;
  state.backlog=value;
  state.contentRevision='';
  state.pendingContentUpdate=false;
  state.pendingContentReason='';
  if(value)localStorage.setItem('task-mecca-backlog-folder',value);else localStorage.removeItem('task-mecca-backlog-folder');
  state.detail=null;state.listPage=1;state.selectedIndex=0;refresh();
});
  $('#search')?.addEventListener('input',e=>{
  state.query=e.target.value; state.detail=null; state.detailTask=null; state.listPage=1; state.selectedIndex=0;
  clearTimeout(searchRefreshTimer);
  searchRefreshTimer=setTimeout(()=>{if(state.view==='backlog'&&!state.detail)refreshList();},180);
});
}
$('#refreshBtn').onclick=refreshAndCheckUpdates;
$('#notificationBtn')?.addEventListener('click',()=>{state.notificationCenterTab='settings';navigate(null,'notifications');});
document.addEventListener('click',e=>{const panel=$('#notificationPanel');if(panel?.classList.contains('open')&&!panel.contains(e.target)&&!$('#notificationBtn')?.contains(e.target))panel.classList.remove('open')});
$('#sidebarToggle').onclick=toggleSidebar;
$('#mobileSidebarScrim')?.addEventListener('click',()=>closeMobileNav(true));
$('#sidebar')?.addEventListener('click',event=>{
  if(!window.matchMedia('(max-width:680px)').matches)return;
  const item=event.target.closest('.nav-item,.session-open,[data-add-project]');
  // Opening the project picker does not navigate and must keep the panel visible.
  if(item && item.id!=='openProjectBtn')closeMobileNav();
});
window.addEventListener('resize',()=>{
  if(state.mobileNavOpen && !window.matchMedia('(max-width:680px)').matches)closeMobileNav();
});
bindChannelGesture();
$('#sidebar')?.addEventListener('mouseenter',()=>{if(state.sidebarMode==='auto'){state.sidebarPeek=true;applySidebarState();}});
$('#sidebar')?.addEventListener('mouseleave',()=>{if(state.sidebarMode==='auto'){state.sidebarPeek=false;state.projectMenuOpen=false;applySidebarState();}});
$('#sidebar')?.addEventListener('focusin',()=>{if(state.sidebarMode==='auto'){state.sidebarPeek=true;applySidebarState();}});
$('#sidebar')?.addEventListener('focusout',e=>{if(state.sidebarMode==='auto'&&!e.currentTarget.contains(e.relatedTarget)){state.sidebarPeek=false;applySidebarState();}});

document.addEventListener('keydown',e=>{
  if(state.mobileNavOpen && window.matchMedia('(max-width:680px)').matches){
    if(e.key==='Escape'){
      e.preventDefault();
      closeMobileNav(true);
      return;
    }
    if(e.key==='Tab'){
      const panel=$('#mobileSidebarPanel');
      const focusables=Array.from(panel?.querySelectorAll('button:not([disabled]),a[href],input:not([disabled]),select:not([disabled])')||[])
        .filter(item=>item.getClientRects().length>0 && !item.closest('[hidden]'));
      if(focusables.length){
        const first=focusables[0],last=focusables[focusables.length-1],active=document.activeElement;
        if(e.shiftKey && (active===first || !panel.contains(active))){
          e.preventDefault();last.focus();
        }else if(!e.shiftKey && (active===last || !panel.contains(active))){
          e.preventDefault();first.focus();
        }
      }
    }
  }
  if(document.querySelector('.hub-confirm-overlay'))return;
  if(document.activeElement?.closest?.('#userAttention'))return;
  if(e.key==='Escape'&&$('#notificationPanel')?.classList.contains('open')){e.preventDefault();$('#notificationPanel').classList.remove('open');restoreNotificationPanelFocus();return;}
  if(e.key==='Escape'&&state.releaseNotePopup){e.preventDefault();dismissReleaseNotePopup();return}
  const tag=document.activeElement?.tagName?.toLowerCase();
  const editing=['input','textarea','select','button'].includes(tag)||document.activeElement?.isContentEditable;
  if(e.key==='/'&&!editing&&state.view==='backlog'&&!state.detail){const search=$('#search');if(search){e.preventDefault();search.focus();return}}
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
  autoPageResizeTimer=setTimeout(()=>scheduleAutoListPageSize(true),100);
});
window.addEventListener('popstate',()=>route(true));
setInterval(()=>{
  const data=currentProjectData();
  updateLiveListRelativeTimes();
  if(data){
    $('#snapshotAge').textContent=`${t('updated')} ${ago(new Date(state.lastFetch).toISOString())}`;
    const byID={};
    (state.listData?.items||[]).forEach(task=>{byID[task.id]=task});
    Object.assign(byID,state.snapshot?.all_items||{});
    document.querySelectorAll('.live-timer').forEach(el=>{const task=byID[el.dataset.id];if(task)el.textContent=fmtSec(runningSeconds(task,'active'))});
    document.querySelectorAll('.live-lifecycle-interval').forEach(el=>{
      const started=new Date(el.dataset.stateAt||'').getTime();
      if(Number.isFinite(started))el.textContent=fmtSec(Math.max(0,(Date.now()-started)/1000));
    });
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
function preserveViewportAndFocus(work) {
  const x=window.scrollX, y=window.scrollY;
  const active=document.activeElement;
  const activeID=active?.id||'';
  const selection=active&&typeof active.selectionStart==='number'
    ? {start:active.selectionStart,end:active.selectionEnd}
    : null;
  return Promise.resolve().then(work).finally(()=>{
    requestAnimationFrame(()=>{
      window.scrollTo({left:x,top:y,behavior:'auto'});
      const target=activeID?document.getElementById(activeID):null;
      if(target&&document.contains(target)){
        try { target.focus({preventScroll:true}); } catch(_) {}
        if(selection&&typeof target.setSelectionRange==='function'){
          try { target.setSelectionRange(selection.start,selection.end); } catch(_) {}
        }
      }
    });
  });
}
setInterval(()=>{
  if(state.project&&state.view==='workload'&&document.visibilityState!=='hidden'&&!refreshInFlight){
    preserveViewportAndFocus(()=>refresh());
  }
},30000);
setInterval(()=>{
  if(document.visibilityState!=='hidden')refreshVersionInfo(true);
},60000);
if(window.isSecureContext&&'serviceWorker' in navigator)notificationWorker();
setInterval(()=>{if(document.visibilityState!=='hidden')void syncBackgroundPush()},60000);
route();
const initialForeground=refresh();
Promise.resolve(initialForeground).finally(()=>refreshOperations());
setInterval(refreshOperations,15000);
refreshVersionInfo(false);
void resumeActiveUpgrade();
setTimeout(()=>refreshVersionInfo(true),800);
