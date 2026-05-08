package policy

type DefaultPolicy struct {
	Mode PermissionMode
}

func New(mode PermissionMode) DefaultPolicy {
	if mode == "" {
		mode = DefaultPermissionMode
	}
	return DefaultPolicy{Mode: mode}
}

func (p DefaultPolicy) EvaluateCommand(req CommandRequest) Decision {
	assessment := assessCommand(req.Command)
	return p.decide("shell command", assessment.Risk, assessment.Reason, assessment.Details)
}

func (p DefaultPolicy) EvaluatePath(req PathRequest) Decision {
	assessment := assessPath(req)
	return p.decide("path access", assessment.Risk, assessment.Reason, assessment.Details)
}

func (p DefaultPolicy) EvaluateNetwork(req NetworkRequest) Decision {
	assessment := assessNetwork(req)
	return p.decide("network access", assessment.Risk, assessment.Reason, assessment.Details)
}

func (p DefaultPolicy) EvaluateMCP(req MCPRequest) Decision {
	details := map[string]any{
		"server": req.Server,
		"name":   req.Name,
		"kind":   req.Kind,
	}
	if len(req.Arguments) > 0 {
		details["argument_count"] = len(req.Arguments)
	}
	return p.decide("mcp call", RiskLevelMedium, "MCP calls require auditable policy review", details)
}

func (p DefaultPolicy) decide(subject string, risk RiskLevel, reason string, details map[string]any) Decision {
	mode := p.Mode
	if mode == "" {
		mode = DefaultPermissionMode
	}

	action := DecisionAllow
	switch mode {
	case PermissionModeReadonly:
		if risk != RiskLevelLow {
			action = DecisionBlock
		}
	case PermissionModeSuggest:
		if risk == RiskLevelCritical {
			action = DecisionBlock
		} else if risk != RiskLevelLow {
			action = DecisionRequestApproval
		}
	case PermissionModeWorkspaceWrite:
		if risk == RiskLevelCritical {
			action = DecisionBlock
		} else if risk == RiskLevelHigh {
			action = DecisionRequestApproval
		}
	case PermissionModeFullAuto:
		if risk == RiskLevelCritical {
			action = DecisionBlock
		}
	default:
		action = DecisionRequestApproval
		reason = "unknown permission mode requires approval"
		risk = RiskLevelMedium
	}

	return Decision{
		Action:  action,
		Reason:  decisionReason(subject, reason, mode),
		Risk:    risk,
		Details: details,
	}
}

type assessment struct {
	Risk    RiskLevel
	Reason  string
	Details map[string]any
}

func decisionReason(subject, reason string, mode PermissionMode) string {
	if reason == "" {
		return subject + " evaluated by " + string(mode) + " policy"
	}
	return reason
}
