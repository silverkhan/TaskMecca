package backlog

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

// Production lock order: notification_state, then notificationJournalMu.
// Only readNotificationJournalSnapshot constructs a verified parsed snapshot.
// A caller cannot supply a parsed journal to be paired with unrelated bytes.
type notificationJournalSnapshot struct {
	info     os.FileInfo
	raw      []byte
	digest   [32]byte
	journal  notificationJournal
	verified bool
}
type notificationJournalCacheEntry struct {
	notificationJournalSnapshot
	checked time.Time
}

var notificationJournalCache = map[string]notificationJournalCacheEntry{}

const notificationJournalCacheLimit = 32
const notificationJournalCheckpoint = 30 * time.Second

func duplicateNotificationJournalValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		copy := make(map[string]any, len(value))
		for key, item := range value {
			copy[key] = duplicateNotificationJournalValue(item)
		}
		return copy
	case []any:
		copy := make([]any, len(value))
		for i, item := range value {
			copy[i] = duplicateNotificationJournalValue(item)
		}
		return copy
	default:
		return value
	}
}
func duplicateNotificationJournal(j notificationJournal) notificationJournal {
	out := notificationJournal{Version: j.Version}
	if j.Items != nil {
		out.Items = make(map[string]notificationObservation, len(j.Items))
		for id, item := range j.Items {
			out.Items[id] = item
		}
	}
	if j.Phases != nil {
		out.Phases = make(map[string]int, len(j.Phases))
		for id, phase := range j.Phases {
			out.Phases[id] = phase
		}
	}
	if j.Events != nil {
		out.Events = make([]map[string]any, len(j.Events))
		for i, event := range j.Events {
			if event != nil {
				out.Events[i] = duplicateNotificationJournalValue(event).(map[string]any)
			}
		}
	}
	return out
}
func sameNotificationJournalFile(a, b os.FileInfo) bool {
	return a != nil && b != nil && a.Mode().IsRegular() && b.Mode().IsRegular() && os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

// Bind the bytes read from an opened regular inode to its pre/post-read and
// named-path identities. An observed replacement or edit is an error, not a
// reason to attach new metadata to an older parse. No retry or rewrite occurs.
func readNotificationJournalBytes(path string) ([]byte, os.FileInfo, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, nil, errors.New("notification journal is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !sameNotificationJournalFile(before, opened) {
		return nil, nil, errors.New("notification journal identity changed before read")
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, err
	}
	afterOpen, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	afterPath, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !sameNotificationJournalFile(opened, afterOpen) || !sameNotificationJournalFile(afterOpen, afterPath) || int64(len(raw)) != afterOpen.Size() {
		return nil, nil, errors.New("notification journal changed during read")
	}
	return raw, afterOpen, nil
}
func notificationJournalSnapshotCurrent(path string, snapshot notificationJournalSnapshot) bool {
	raw, info, err := readNotificationJournalBytes(path)
	return err == nil && snapshot.verified && sameNotificationJournalFile(info, snapshot.info) && sha256.Sum256(raw) == snapshot.digest && bytes.Equal(raw, snapshot.raw)
}
func readNotificationJournalSnapshot(path string, expected []byte) (notificationJournalSnapshot, error) {
	raw, info, err := readNotificationJournalBytes(path)
	if err != nil {
		return notificationJournalSnapshot{}, err
	}
	if expected != nil && !bytes.Equal(raw, expected) {
		return notificationJournalSnapshot{}, errors.New("notification journal differs from expected committed bytes")
	}
	var journal notificationJournal
	if err = json.Unmarshal(raw, &journal); err != nil {
		return notificationJournalSnapshot{}, err
	}
	snapshot := notificationJournalSnapshot{info: info, raw: raw, digest: sha256.Sum256(raw), journal: journal, verified: true}
	// Validate the same bytes again after parsing, including named inode identity.
	if !notificationJournalSnapshotCurrent(path, snapshot) {
		return notificationJournalSnapshot{}, errors.New("notification journal changed while parsing")
	}
	return snapshot, nil
}

// The caller holds both production locks. Every hit revalidates disk bytes and
// identity; periodic expiry additionally forces a fresh parse. The entry's
// digest is always the digest of the bytes from which its journal was parsed.
func cachedNotificationJournal(path string) (notificationJournal, bool) {
	entry, found := notificationJournalCache[path]
	if !found {
		return notificationJournal{}, false
	}
	if time.Since(entry.checked) >= notificationJournalCheckpoint || !notificationJournalSnapshotCurrent(path, entry.notificationJournalSnapshot) {
		delete(notificationJournalCache, path)
		return notificationJournal{}, false
	}
	return duplicateNotificationJournal(entry.journal), true
}
func publishNotificationJournalSnapshot(path string, snapshot notificationJournalSnapshot) error {
	if !notificationJournalSnapshotCurrent(path, snapshot) {
		delete(notificationJournalCache, path)
		return errors.New("notification journal changed before cache publication")
	}
	if len(notificationJournalCache) >= notificationJournalCacheLimit {
		if _, ok := notificationJournalCache[path]; !ok {
			for old := range notificationJournalCache {
				delete(notificationJournalCache, old)
				break
			}
		}
	}
	notificationJournalCache[path] = notificationJournalCacheEntry{notificationJournalSnapshot: snapshot, checked: time.Now()}
	return nil
}

// Both cold readers and writers use this factory. nil expected means a cold
// current-disk read. A writer must pass its exact committed payload. Neither
// API accepts an independent parsed journal, so old-parse/new-digest pairing
// cannot be constructed at the publication call sites.
func rememberNotificationJournal(path string, expected []byte) (notificationJournal, error) {
	snapshot, err := readNotificationJournalSnapshot(path, expected)
	if err != nil {
		delete(notificationJournalCache, path)
		return notificationJournal{}, err
	}
	if err = publishNotificationJournalSnapshot(path, snapshot); err != nil {
		return notificationJournal{}, err
	}
	return duplicateNotificationJournal(snapshot.journal), nil
}
