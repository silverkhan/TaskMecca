package webui

import "testing"

func TestAttentionRevisionIncludesRequiredAction(t *testing.T) {
    base:=map[string]any{
        "attention":[]map[string]any{{
            "id":"A-38",
            "type":"runtime_stalled",
            "health":"stale",
            "runtime_state":"running",
            "last_activity_at":"2026-10-01T07:00:00Z",
            "title":"작업 정체 확인 필요",
            "message":"런타임 활동 확인 필요",
            "action":"워커 상태를 확인하세요.",
            "resume_condition":"활동이 재개되면 해제됩니다.",
        }},
        "notification_events":[]map[string]any{},
    }
    first:=attentionRevision(base)
    changed:=map[string]any{
        "attention":[]map[string]any{{
            "id":"A-38",
            "type":"runtime_stalled",
            "health":"stale",
            "runtime_state":"running",
            "last_activity_at":"2026-10-01T07:00:00Z",
            "title":"작업 정체 확인 필요",
            "message":"런타임 활동 확인 필요",
            "action":"남은 작업을 확인하고 재할당 여부를 결정하세요.",
            "resume_condition":"활동이 재개되면 해제됩니다.",
        }},
        "notification_events":[]map[string]any{},
    }
    second:=attentionRevision(changed)
    if first==second {
        t.Fatalf("action change must update attention revision: %q",first)
    }
}
