package activity

import (
    "errors"
    "testing"
    "time"
)

func validEvent() Event {
    now:=time.Date(2026,11,3,7,0,0,0,time.UTC)
    return Event{SchemaVersion:1,EventID:"evt-1",TaskID:"B-589",Action:Start,Role:RoleWorker,AgentID:"worker-a",RuntimeAttemptID:"run-1",Round:0,Type:Requirements,Confidence:Confirmed,OccurredAt:now,RecordedAt:now}
}

func TestActivityContract(t *testing.T){
    e:=validEvent()
    if err:=e.Validate();err!=nil{t.Fatal(err)}
    cases:=map[string]func(*Event){
        "missing task":func(e *Event){e.TaskID=" "},
        "unknown role":func(e *Event){e.Role="root"},
        "invalid type":func(e *Event){e.Type="magic"},
        "negative round":func(e *Event){e.Round=-1},
        "missing attempt":func(e *Event){e.RuntimeAttemptID=""},
        "missing event":func(e *Event){e.EventID=""},
        "inferred without evidence":func(e *Event){e.Confidence=Inferred},
        "other without explanation":func(e *Event){e.Type=Other},
        "unapproved proposed type":func(e *Event){e.ProposedType="dependency_resolution"},
        "correction without source":func(e *Event){e.Action=Correct},
        "supersede without correction":func(e *Event){e.SupersedesID="evt-0"},
        "missing observed time":func(e *Event){e.OccurredAt=time.Time{}},
    }
    for name,mutate:=range cases{t.Run(name,func(t *testing.T){v:=e;mutate(&v);if err:=v.Validate();!errors.Is(err,ErrInvalidEvent){t.Fatalf("expected invalid event, got %v",err)}})}
}

func TestOtherClassificationAndCorrection(t *testing.T){
    e:=validEvent();e.Type=Other;e.Explanation="external dependency compatibility investigation";e.ProposedType="dependency_resolution";e.Confidence=Inferred;e.Evidence="agent report"
    if err:=e.Validate();err!=nil{t.Fatal(err)}
    e.Action=Correct;e.SupersedesID="evt-previous"
    if err:=e.Validate();err!=nil{t.Fatal(err)}
}

func TestAllKnownActivityTypes(t *testing.T){
    for _,kind:=range []Type{Requirements,Research,Implementation,TargetedTest,FullRegression,BuildCheck,ControllerReview,ControllerReReview,Handoff,Recovery,Other}{
       if !kind.Valid(){t.Fatalf("unexpected invalid type: %s",kind)}
    }
}
