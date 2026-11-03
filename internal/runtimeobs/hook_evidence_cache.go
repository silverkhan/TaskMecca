package runtimeobs

import "sync"

// Hook evidence is immutable only while its source journal fingerprints
// remain unchanged. Never cache errors or partial scans. Use exact cutoff
// strings as part of the key: evidence before a newly installed hook must not
// make the new hook look verified.
type hookEvidenceCacheKey struct {
    project, provider, cutoff string
}
type hookEvidenceCacheValue struct {
    files []executionFileFingerprint
    activity, start, stop bool
    last string
}

var hookEvidenceCache = struct {
    sync.Mutex
    entries map[hookEvidenceCacheKey]hookEvidenceCacheValue
}{entries: make(map[hookEvidenceCacheKey]hookEvidenceCacheValue)}

const hookEvidenceCacheMaxEntries = 32

func cachedHookEvidence(key hookEvidenceCacheKey, files []executionFileFingerprint, valid bool) (map[string]bool,string,bool) {
    if !valid { return nil,"",false }
    hookEvidenceCache.Lock()
    defer hookEvidenceCache.Unlock()
    entry, exists := hookEvidenceCache.entries[key]
    if !exists || !sameExecutionFileFingerprints(entry.files, files) { return nil,"",false }
    return map[string]bool{"activity":entry.activity,"start":entry.start,"stop":entry.stop},entry.last,true
}

func rememberHookEvidence(key hookEvidenceCacheKey, files []executionFileFingerprint, events map[string]bool, last string) {
    hookEvidenceCache.Lock()
    defer hookEvidenceCache.Unlock()
    if len(hookEvidenceCache.entries) >= hookEvidenceCacheMaxEntries {
        // Bounded cache. Eviction has no impact on observation correctness.
        if _, exists := hookEvidenceCache.entries[key]; !exists {
            for old := range hookEvidenceCache.entries { delete(hookEvidenceCache.entries,old);break }
        }
    }
    hookEvidenceCache.entries[key] = hookEvidenceCacheValue{
        files: append([]executionFileFingerprint(nil),files...),
        activity:events["activity"],start:events["start"],stop:events["stop"],last:last,
    }
}
