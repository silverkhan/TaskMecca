package activity

// Advice is a read-only hint for an agent's next activity classification.
// It neither writes evidence nor authorizes a lifecycle transition.
type Advice struct {
    Role Role `json:"role"`
    Event string `json:"event"`
    Type Type `json:"suggested_type"`
    Round int `json:"round"`
    RequiresExplanation bool `json:"requires_explanation"`
    Authoritative bool `json:"authoritative"`
}

func Suggest(role Role,event string,round int)Advice {
    if round<0{round=0}
    advice:=Advice{Role:role,Event:event,Type:Other,Round:round,RequiresExplanation:true,Authoritative:false}
    switch event {
    case "requirements","requirements_started","analysis_started":
        advice.Type=Requirements
    case "research","research_started":
        advice.Type=Research
    case "implementation","implementation_started","rework_started":
        advice.Type=Implementation
    case "targeted_test","targeted_test_started":
        advice.Type=TargetedTest
    case "full_regression","full_regression_started":
        advice.Type=FullRegression
    case "build","build_check","build_started":
        advice.Type=BuildCheck
    case "handoff","handoff_started":
        advice.Type=Handoff
    case "recovery","recovery_started":
        advice.Type=Recovery
    case "review","review_started","review_rejected","review_accepted":
        if role==RoleController||role==RoleSubController {
            if round>0{advice.Type=ControllerReReview}else{advice.Type=ControllerReview}
        }
    }
    if advice.Type!=Other{advice.RequiresExplanation=false}
    return advice
}
