package notify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/projectguard"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Event struct{ ID, TaskID, Kind, Title, Message, ResumeCondition, At string }
type Channel interface {
	Name() string
	Deliver(Event) error
}

type TelegramConfig struct {
	Token          string          `json:"token,omitempty"`
	ChatID         int64           `json:"chat_id,omitempty"`
	BotUsername    string          `json:"bot_username,omitempty"`
	Enabled        bool            `json:"enabled"`
	Kinds          map[string]bool `json:"kinds,omitempty"`
	Delivered      []string        `json:"delivered,omitempty"`
	TaskPhases     map[string]int  `json:"task_phases,omitempty"`
	ActivatedAt    string          `json:"activated_at,omitempty"`
	ProjectEnabled *bool           `json:"project_enabled,omitempty"`
	SuppressBefore string          `json:"suppress_before,omitempty"`
	RecipientMode  string          `json:"recipient_mode,omitempty"`
}
type TelegramStatus struct {
	Configured     bool            `json:"configured"`
	Connected      bool            `json:"connected"`
	Enabled        bool            `json:"enabled"`
	BotUsername    string          `json:"bot_username,omitempty"`
	Kinds          map[string]bool `json:"kinds"`
	ProjectEnabled bool            `json:"project_enabled"`
	RecipientMode  string          `json:"recipient_mode"`
}

func projectNotificationsEnabled(cfg TelegramConfig) bool {
	return cfg.ProjectEnabled == nil || *cfg.ProjectEnabled
}

func recipientMode(cfg TelegramConfig) string {
	if cfg.RecipientMode == "shared" || cfg.RecipientMode == "individual" {
		return cfg.RecipientMode
	}
	return "individual"
}

type telegramChannel struct {
	cfg     TelegramConfig
	project string
}

func (t telegramChannel) Name() string { return "telegram" }
func (t telegramChannel) Deliver(e Event) error {
	if !t.cfg.Enabled || t.cfg.ChatID == 0 || !t.cfg.Kinds[e.Kind] {
		return nil
	}
	head := telegramProjectPrefix(t.project) + " " + e.TaskID
	title := telegramTaskTitle(e.Title, e.TaskID)
	if title != "" {
		head += " · " + title
	}
	labels := map[string]string{
		"registered": "📝 작업 등록", "started": "▶️ 작업 착수", "intervention": "🙋 사용자 개입 필요", "approval": "🔐 승인 필요",
		"stalled": "⏳ 작업 정체 확인 필요", "interrupted": "⚠️ 실행 중단/오류", "runtime_unknown": "❓ 실행 상태 확인 필요",
		"finalize": "📌 완료 처리 필요", "completed": "✅ 작업 완료",
	}
	label := labels[e.Kind]
	if label == "" {
		label = e.Kind
	}
	lines := []string{label, head}
	if e.Message != "" {
		lines = append(lines, e.Message)
	}
	if e.ResumeCondition != "" {
		lines = append(lines, "➡️ 다음 조치: "+e.ResumeCondition)
	}
	return telegramCall(t.cfg.Token, "sendMessage", map[string]any{"chat_id": t.cfg.ChatID, "text": strings.Join(lines, "\n")}, nil)
}

var telegramMu sync.Mutex
var telegramAPIBase = "https://api.telegram.org"

func telegramPath(project string) string {
	return filepath.Join(project, "_task_mecca", ".runtime", "notifications", "telegram.json")
}
func defaultKinds() map[string]bool {
	return map[string]bool{
		"registered": true, "started": true, "intervention": true, "approval": true, "stalled": true,
		"interrupted": true, "runtime_unknown": true, "finalize": true, "completed": true,
	}
}
func mergeDefaultKinds(kinds map[string]bool) map[string]bool {
	out := defaultKinds()
	for kind, enabled := range kinds {
		out[kind] = enabled
	}
	return out
}
func loadTelegram(project string) (TelegramConfig, error) {
	cfg := TelegramConfig{Kinds: defaultKinds(), TaskPhases: map[string]int{}}
	data, err := os.ReadFile(telegramPath(project))
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err = json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.Kinds = mergeDefaultKinds(cfg.Kinds)
	if cfg.TaskPhases == nil {
		cfg.TaskPhases = map[string]int{}
	}
	// Project controls replaced the old all-or-nothing enabled switch. A
	// previously connected destination that was globally disabled now follows
	// the new default (enabled for every project); explicit per-project false
	// values remain authoritative.
	if cfg.ProjectEnabled == nil && !cfg.Enabled && cfg.Token != "" && cfg.ChatID != 0 {
		cfg.Enabled = true
	}
	return cfg, nil
}
func saveTelegram(project string, cfg TelegramConfig) error {
	releaseGuard, guardErr := projectguard.AcquireWrite(project)
	if guardErr != nil {
		return guardErr
	}
	defer releaseGuard()

	path := telegramPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, append(data, '\n'), 0600); err != nil {
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}
func status(cfg TelegramConfig) TelegramStatus {
	return TelegramStatus{Configured: cfg.Token != "", Connected: cfg.Token != "" && cfg.ChatID != 0, Enabled: cfg.Enabled, BotUsername: cfg.BotUsername, Kinds: cfg.Kinds, ProjectEnabled: projectNotificationsEnabled(cfg), RecipientMode: recipientMode(cfg)}
}
func TelegramStatusFor(project string) (TelegramStatus, error) {
	releaseGuard, guardErr := projectguard.AcquireWrite(project)
	if guardErr != nil {
		return TelegramStatus{}, guardErr
	}
	defer releaseGuard()

	telegramMu.Lock()
	defer telegramMu.Unlock()
	cfg, err := loadTelegram(project)
	if err != nil {
		return TelegramStatus{}, err
	}
	// Persist the legacy global-disable migration when a project is inspected,
	// so reopening the dashboard cannot restore the old policy.
	if cfg.ProjectEnabled == nil && cfg.Enabled && cfg.Token != "" && cfg.ChatID != 0 {
		if err := saveTelegram(project, cfg); err != nil {
			return TelegramStatus{}, err
		}
	}
	return status(cfg), nil
}

type telegramEnvelope struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

// TelegramTransportDisabled is a process-only safety switch for maintenance.
// It does not alter any saved token, recipient, enabled kind or cutover.
func TelegramTransportDisabled() bool {
	return os.Getenv("TASK_MECCA_TELEGRAM_TRANSPORT") == "disabled"
}

func telegramCall(token, method string, body, out any) error {
	if TelegramTransportDisabled() {
		return errors.New("Telegram transport disabled for this process")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("telegram bot token is empty")
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(telegramAPIBase, "/")+"/bot"+token+"/"+method, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var env telegramEnvelope
	if err = json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return err
	}
	if !env.OK {
		if env.Description != "" {
			return errors.New(env.Description)
		}
		return fmt.Errorf("telegram %s failed", method)
	}
	if out != nil && len(env.Result) > 0 {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}
func ConfigureTelegram(project, token string, kinds map[string]bool) (TelegramStatus, error) {
	releaseGuard, guardErr := projectguard.AcquireWrite(project)
	if guardErr != nil {
		return TelegramStatus{}, guardErr
	}
	defer releaseGuard()

	telegramMu.Lock()
	defer telegramMu.Unlock()
	var me struct {
		Username string `json:"username"`
	}
	if err := telegramCall(token, "getMe", map[string]any{}, &me); err != nil {
		return TelegramStatus{}, err
	}
	cfg, err := loadTelegram(project)
	if err != nil {
		return TelegramStatus{}, err
	}
	cfg.Token = strings.TrimSpace(token)
	cfg.BotUsername = me.Username
	cfg.ChatID = 0
	cfg.Enabled = false
	cfg.ActivatedAt = ""
	if kinds != nil {
		cfg.Kinds = kinds
	}
	if err = saveTelegram(project, cfg); err != nil {
		return TelegramStatus{}, err
	}
	return status(cfg), nil
}

// ConfigureSharedTelegram validates the token once, then persists the same
// recipient setup for every monitored project. Credentials stay in 0600
// project runtime files and are never included in TelegramStatus or logs.
func ConfigureSharedTelegram(projects []string, token string, kinds map[string]bool) (TelegramStatus, error) {
	releaseGuard,guardErr:=projectguard.AcquireWrites(projects);if guardErr!=nil{return TelegramStatus{},guardErr};defer releaseGuard()
	telegramMu.Lock()
	defer telegramMu.Unlock()
	var me struct {
		Username string `json:"username"`
	}
	if err := telegramCall(token, "getMe", map[string]any{}, &me); err != nil {
		return TelegramStatus{}, err
	}
	for _, project := range projects {
		cfg, err := loadTelegram(project)
		if err != nil {
			return TelegramStatus{}, err
		}
		cfg.Token = strings.TrimSpace(token)
		cfg.BotUsername = me.Username
		cfg.ChatID = 0
		cfg.Enabled = false
		cfg.ActivatedAt = ""
		cfg.RecipientMode = "shared"
		if kinds != nil {
			cfg.Kinds = kinds
		}
		if err := saveTelegram(project, cfg); err != nil {
			return TelegramStatus{}, err
		}
	}
	return TelegramStatus{Configured: true, Connected: false, Enabled: false, BotUsername: me.Username, Kinds: mergeDefaultKinds(kinds), ProjectEnabled: true, RecipientMode: "shared"}, nil
}

func SetTelegramRecipientMode(project, mode string) (TelegramStatus, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "shared" && mode != "individual" {
		return TelegramStatus{}, errors.New("recipient mode must be shared or individual")
	}
	telegramMu.Lock()
	defer telegramMu.Unlock()
	cfg, err := loadTelegram(project)
	if err != nil {
		return TelegramStatus{}, err
	}
	// Selecting individual never borrows another project's recipient. A missing
	// local recipient is intentionally left unconfigured so delivery fails
	// closed rather than silently falling back to a shared destination.
	cfg.RecipientMode = mode
	if err := saveTelegram(project, cfg); err != nil {
		return TelegramStatus{}, err
	}
	return status(cfg), nil
}
func DiscoverTelegramChat(project string) (TelegramStatus, error) {
	releaseGuard, guardErr := projectguard.AcquireWrite(project)
	if guardErr != nil {
		return TelegramStatus{}, guardErr
	}
	defer releaseGuard()

	telegramMu.Lock()
	defer telegramMu.Unlock()
	cfg, err := loadTelegram(project)
	if err != nil {
		return TelegramStatus{}, err
	}
	if cfg.Token == "" {
		return TelegramStatus{}, errors.New("telegram bot is not configured")
	}
	var updates []struct {
		UpdateID int64 `json:"update_id"`
		Message  *struct {
			Text string `json:"text"`
			Chat struct {
				ID   int64  `json:"id"`
				Type string `json:"type"`
			} `json:"chat"`
		} `json:"message"`
	}
	if err = telegramCall(cfg.Token, "getUpdates", map[string]any{"limit": 100, "timeout": 0, "allowed_updates": []string{"message"}}, &updates); err != nil {
		return TelegramStatus{}, err
	}
	var chatID int64
	for i := len(updates) - 1; i >= 0; i-- {
		if updates[i].Message != nil && updates[i].Message.Chat.Type == "private" {
			chatID = updates[i].Message.Chat.ID
			if strings.HasPrefix(strings.TrimSpace(updates[i].Message.Text), "/start") {
				break
			}
		}
	}
	if chatID == 0 {
		return TelegramStatus{}, errors.New("no private Telegram chat found; send /start to the bot first")
	}
	cfg.ChatID = chatID
	cfg.Enabled = true
	cfg.ActivatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err = saveTelegram(project, cfg); err != nil {
		return TelegramStatus{}, err
	}
	return status(cfg), nil
}

// DiscoverSharedTelegram discovers the shared private chat once and writes it
// to every shared-project config. It never falls back to an individual route.
func DiscoverSharedTelegram(projects []string, sourceProject string) (TelegramStatus, error) {
	releaseGuard,guardErr:=projectguard.AcquireWrites(append(append([]string(nil),projects...),sourceProject));if guardErr!=nil{return TelegramStatus{},guardErr};defer releaseGuard()
	telegramMu.Lock()
	defer telegramMu.Unlock()
	source, err := loadTelegram(sourceProject)
	if err != nil {
		return TelegramStatus{}, err
	}
	if source.Token == "" || recipientMode(source) != "shared" {
		return TelegramStatus{}, errors.New("shared Telegram recipient is not configured")
	}
	var updates []struct {
		Message *struct {
			Text string `json:"text"`
			Chat struct {
				ID   int64  `json:"id"`
				Type string `json:"type"`
			} `json:"chat"`
		} `json:"message"`
	}
	if err = telegramCall(source.Token, "getUpdates", map[string]any{"limit": 100, "timeout": 0, "allowed_updates": []string{"message"}}, &updates); err != nil {
		return TelegramStatus{}, err
	}
	var chatID int64
	for i := len(updates) - 1; i >= 0; i-- {
		if updates[i].Message != nil && updates[i].Message.Chat.Type == "private" {
			chatID = updates[i].Message.Chat.ID
			if strings.HasPrefix(strings.TrimSpace(updates[i].Message.Text), "/start") {
				break
			}
		}
	}
	if chatID == 0 {
		return TelegramStatus{}, errors.New("no private Telegram chat found; send /start to the bot first")
	}
	activatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	for _, project := range projects {
		cfg, loadErr := loadTelegram(project)
		if loadErr != nil {
			return TelegramStatus{}, loadErr
		}
		if recipientMode(cfg) != "shared" || cfg.Token == "" {
			continue
		}
		cfg.ChatID, cfg.Enabled, cfg.ActivatedAt = chatID, true, activatedAt
		if saveErr := saveTelegram(project, cfg); saveErr != nil {
			return TelegramStatus{}, saveErr
		}
	}
	source.ChatID, source.Enabled, source.ActivatedAt = chatID, true, activatedAt
	return status(source), nil
}
func TestTelegram(project string) error {
	releaseGuard, guardErr := projectguard.AcquireWrite(project)
	if guardErr != nil {
		return guardErr
	}
	defer releaseGuard()

	telegramMu.Lock()
	defer telegramMu.Unlock()
	cfg, err := loadTelegram(project)
	if err != nil {
		return err
	}
	if cfg.Token == "" || cfg.ChatID == 0 {
		return errors.New("telegram bot is not connected")
	}
	return telegramCall(cfg.Token, "sendMessage", map[string]any{"chat_id": cfg.ChatID, "text": "🔔 " + telegramProjectPrefix(project) + " 테스트 알림\nTelegram 알림 연결이 정상입니다."}, nil)
}
// DisableAllTelegram deletes saved Telegram recipient configurations for all
// monitored projects. It does not touch browser notifications or delivery logs.
// A missing file is idempotently ignored; partial errors report the count.
func DisableAllTelegram(projects []string) (int, error) {
 if len(projects) == 0 { return 0, nil }
 release, err := projectguard.AcquireWrites(projects)
 if err != nil { return 0, err }
 defer release()
 telegramMu.Lock()
 defer telegramMu.Unlock()
 // Check access and validity before any deletion.
 seen := make(map[string]bool, len(projects))
 paths := make([]string, 0, len(projects))
 for _, project := range projects {
  path := telegramPath(project)
  if seen[path] { continue }
  seen[path] = true
  if _, statErr := os.Stat(path); statErr != nil {
   if errors.Is(statErr, os.ErrNotExist) { continue }
   return 0, statErr
  }
  cfg, loadErr := loadTelegram(project)
  if loadErr != nil { return 0, loadErr }
  // A project may store "notifications off" without a bot token. Removing
  // such a file would unexpectedly erase that explicit user preference.
  if cfg.Token == "" { continue }
  paths = append(paths, path)
 }
 removed := 0
 for _, path := range paths {
  if err := os.Remove(path); err != nil {
   return removed, fmt.Errorf("cleared %d Telegram configurations before error: %w", removed, err)
  }
  removed++
 }
 return removed, nil
}

func DisableTelegram(project string) error {
	telegramMu.Lock()
	defer telegramMu.Unlock()
	err := os.Remove(telegramPath(project))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func UpdateTelegramKinds(project string, kinds map[string]bool) (TelegramStatus, error) {
	telegramMu.Lock()
	defer telegramMu.Unlock()
	cfg, err := loadTelegram(project)
	if err != nil {
		return TelegramStatus{}, err
	}
	if cfg.Token == "" {
		return TelegramStatus{}, errors.New("telegram bot is not configured")
	}
	cfg.Kinds = kinds
	if err = saveTelegram(project, cfg); err != nil {
		return TelegramStatus{}, err
	}
	return status(cfg), nil
}

// SetProjectNotificationsEnabled keeps the configured recipient and event
// kinds intact. Re-enabling records a cutover: events from the disabled
// period are consumed rather than replayed.
func SetProjectNotificationsEnabled(project string, enabled bool) (TelegramStatus, error) {
	telegramMu.Lock()
	defer telegramMu.Unlock()
	cfg, err := loadTelegram(project)
	if err != nil {
		return TelegramStatus{}, err
	}
	value := enabled
	cfg.ProjectEnabled = &value
	if enabled {
		cfg.SuppressBefore = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if err := saveTelegram(project, cfg); err != nil {
		return TelegramStatus{}, err
	}
	return status(cfg), nil
}

func suppressedByProjectControl(e Event, cfg TelegramConfig) bool {
	if !projectNotificationsEnabled(cfg) {
		return true
	}
	if strings.TrimSpace(cfg.SuppressBefore) == "" {
		return false
	}
	at, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(e.At))
	if err != nil {
		return true
	}
	cutover, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(cfg.SuppressBefore))
	if err != nil {
		return false
	}
	return !at.After(cutover)
}

func notificationAfterActivation(at, activatedAt string) bool {
	eventAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(at))
	if err != nil {
		return false
	}
	cutover, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(activatedAt))
	if err != nil {
		return false
	}
	return eventAt.After(cutover)
}
func deliveryPhaseRank(kind string) int {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "registered":
		return 10
	case "intervention", "approval", "stalled", "interrupted", "runtime_unknown":
		return 15
	case "started":
		return 20
	case "finalize":
		return 30
	case "completed":
		return 40
	default:
		return 0
	}
}
func shouldSuppressRegression(kind string, phase int) bool {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "registered":
		return phase >= 10
	case "started":
		return phase >= 20
	case "finalize":
		return phase >= 30
	case "completed":
		return phase >= 40
	case "intervention", "approval", "stalled", "interrupted", "runtime_unknown":
		return phase >= 30
	default:
		return phase >= 40
	}
}
func advanceDeliveryPhase(cfg *TelegramConfig, e Event) {
	if cfg.TaskPhases == nil {
		cfg.TaskPhases = map[string]int{}
	}
	task := strings.ToUpper(strings.TrimSpace(e.TaskID))
	if task == "" {
		return
	}
	rank := deliveryPhaseRank(e.Kind)
	if rank > cfg.TaskPhases[task] {
		cfg.TaskPhases[task] = rank
	}
}

func notificationKindRank(kind string) int {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "registered":
		return 10
	case "started":
		return 20
	case "completed":
		return 40
	default:
		return 30
	}
}
func notificationEventLess(a, b Event) bool {
	at, aErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(a.At))
	bt, bErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(b.At))
	if aErr == nil && bErr == nil {
		if !at.Equal(bt) {
			return at.Before(bt)
		}
	} else if aErr == nil {
		return true
	} else if bErr == nil {
		return false
	} else if a.At != b.At {
		return a.At < b.At
	}
	ar, br := notificationKindRank(a.Kind), notificationKindRank(b.Kind)
	if ar != br {
		return ar < br
	}
	if a.TaskID != b.TaskID {
		return a.TaskID < b.TaskID
	}
	return a.ID < b.ID
}
func deliverLegacy(project string, events []Event) []error {
	releaseGuard, guardErr := projectguard.AcquireWrite(project)
	if guardErr != nil {
		return []error{guardErr}
	}
	defer releaseGuard()

	telegramMu.Lock()
	defer telegramMu.Unlock()
	cfg, err := loadTelegram(project)
	if err != nil {
		return []error{err}
	}
	if !cfg.Enabled || cfg.ChatID == 0 {
		return nil
	}
	seen := map[string]bool{}
	for _, id := range cfg.Delivered {
		seen[id] = true
	}
	errs := []error{}
	dirty := false
	ch := telegramChannel{cfg: cfg, project: project}

	// Reconstruct lifecycle floors from already-consumed event IDs so existing
	// Telegram configs gain monotonic delivery semantics without replaying history.
	beforePhases := len(cfg.TaskPhases)
	for _, e := range events {
		if e.ID != "" && seen[e.ID] {
			task := strings.ToUpper(strings.TrimSpace(e.TaskID))
			old := cfg.TaskPhases[task]
			advanceDeliveryPhase(&cfg, e)
			if cfg.TaskPhases[task] != old {
				dirty = true
			}
		}
	}
	if len(cfg.TaskPhases) != beforePhases {
		dirty = true
	}

	// Existing installations predate the channel activation watermark. Their
	// current event set is historical baseline, not a queue to replay.
	if strings.TrimSpace(cfg.ActivatedAt) == "" {
		cfg.ActivatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		for _, e := range events {
			if e.ID != "" && !seen[e.ID] {
				seen[e.ID] = true
				cfg.Delivered = append(cfg.Delivered, e.ID)
				advanceDeliveryPhase(&cfg, e)
			}
		}
		if len(cfg.Delivered) > 250 {
			cfg.Delivered = append([]string{}, cfg.Delivered[len(cfg.Delivered)-250:]...)
		}
		if err := saveTelegram(project, cfg); err != nil {
			return []error{err}
		}
		return nil
	}

	sort.SliceStable(events, func(i, j int) bool { return notificationEventLess(events[i], events[j]) })
	for _, e := range events {
		if e.ID == "" || seen[e.ID] {
			continue
		}
		task := strings.ToUpper(strings.TrimSpace(e.TaskID))
		phase := cfg.TaskPhases[task]
		// A later lifecycle stage is a durable delivery floor. Consume any newly
		// discovered older-stage event instead of surfacing a chronological
		// regression such as Completed -> Registered or Finalize -> Started.
		if shouldSuppressRegression(e.Kind, phase) {
			seen[e.ID] = true
			cfg.Delivered = append(cfg.Delivered, e.ID)
			dirty = true
			continue
		}
		// Events that predate channel activation, or occurred while a kind was
		// disabled, are consumed without delivery so enabling/reconnecting cannot
		// backfill them later. They still advance the task lifecycle floor.
		if !notificationAfterActivation(e.At, cfg.ActivatedAt) || !cfg.Kinds[e.Kind] {
			seen[e.ID] = true
			cfg.Delivered = append(cfg.Delivered, e.ID)
			advanceDeliveryPhase(&cfg, e)
			dirty = true
			continue
		}
		if err := ch.Deliver(e); err != nil {
			errs = append(errs, err)
			continue
		}
		seen[e.ID] = true
		cfg.Delivered = append(cfg.Delivered, e.ID)
		advanceDeliveryPhase(&cfg, e)
		dirty = true
	}
	if len(cfg.Delivered) > 250 {
		cfg.Delivered = append([]string{}, cfg.Delivered[len(cfg.Delivered)-250:]...)
		dirty = true
	}
	if dirty {
		if err := saveTelegram(project, cfg); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
