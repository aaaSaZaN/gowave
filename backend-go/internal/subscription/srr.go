package subscription

import (
	"encoding/json"
	"regexp"
	"strings"
)

type SRRConfig struct {
	Version  string       `json:"version"`
	Settings *SRRSettings `json:"settings"`
	Rules    []SRRRule    `json:"rules"`
}

type SRRSettings struct {
	DisableSubscriptionAccessByPath bool `json:"disableSubscriptionAccessByPath"`
}

type SRRRule struct {
	Name                  string                    `json:"name"`
	Description           string                    `json:"description"`
	Enabled               bool                      `json:"enabled"`
	Operator              string                    `json:"operator"` // "AND" or "OR"
	Conditions            []SRRCondition            `json:"conditions"`
	ResponseType          string                    `json:"responseType"`
	ResponseModifications *SRRResponseModifications `json:"responseModifications"`
}

type SRRCondition struct {
	HeaderName    string `json:"headerName"`
	Operator      string `json:"operator"` // EQUALS, NOT_EQUALS, CONTAINS, NOT_CONTAINS, STARTS_WITH, NOT_STARTS_WITH, ENDS_WITH, NOT_ENDS_WITH, REGEX, NOT_REGEX
	Value         string `json:"value"`
	CaseSensitive bool   `json:"caseSensitive"`
}

type SRRHeaderMod struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SRRResponseModifications struct {
	Headers                           []SRRHeaderMod `json:"headers"`
	ApplyHeadersToEnd                 bool           `json:"applyHeadersToEnd"`
	SubscriptionTemplate              string         `json:"subscriptionTemplate"`
	IgnoreHostXrayJSONTemplate        bool           `json:"ignoreHostXrayJsonTemplate"`
	IgnoreServeJSONAtBaseSubscription bool           `json:"ignoreServeJsonAtBaseSubscription"`
	DisableHwidCheck                  bool           `json:"disableHwidCheck"`
}

type SRRMatcherResult struct {
	Matched      bool
	MatchedRule  *SRRRule
	ResponseType string
}

func MatchSRRRules(rawRulesJSON string, headers map[string]string, overrideClientType string) SRRMatcherResult {
	if overrideClientType != "" {
		return handleOverrideClientType(overrideClientType)
	}

	if rawRulesJSON == "" || rawRulesJSON == "{}" || rawRulesJSON == "null" {
		return SRRMatcherResult{Matched: false}
	}

	var config SRRConfig
	if err := json.Unmarshal([]byte(rawRulesJSON), &config); err != nil {
		return SRRMatcherResult{Matched: false}
	}

	for i := range config.Rules {
		rule := &config.Rules[i]
		if !rule.Enabled {
			continue
		}

		if matchRule(rule, headers) {
			return SRRMatcherResult{
				Matched:      true,
				MatchedRule:  rule,
				ResponseType: rule.ResponseType,
			}
		}
	}

	return SRRMatcherResult{Matched: false}
}

func matchRule(rule *SRRRule, headers map[string]string) bool {
	if len(rule.Conditions) == 0 {
		return true
	}

	op := strings.ToUpper(rule.Operator)
	if op == "OR" {
		for i := range rule.Conditions {
			if matchCondition(&rule.Conditions[i], headers) {
				return true
			}
		}
		return false
	}

	// Default to AND
	for i := range rule.Conditions {
		if !matchCondition(&rule.Conditions[i], headers) {
			return false
		}
	}
	return true
}

func matchCondition(cond *SRRCondition, headers map[string]string) bool {
	targetHeaderKey := strings.ToLower(cond.HeaderName)
	headerVal, ok := headers[targetHeaderKey]
	if !ok {
		return false
	}

	compareVal := cond.Value
	checkVal := headerVal

	if !cond.CaseSensitive {
		compareVal = strings.ToLower(compareVal)
		checkVal = strings.ToLower(checkVal)
	}

	switch strings.ToUpper(cond.Operator) {
	case "EQUALS":
		return checkVal == compareVal
	case "NOT_EQUALS":
		return checkVal != compareVal
	case "CONTAINS":
		return strings.Contains(checkVal, compareVal)
	case "NOT_CONTAINS":
		return !strings.Contains(checkVal, compareVal)
	case "STARTS_WITH":
		return strings.HasPrefix(checkVal, compareVal)
	case "NOT_STARTS_WITH":
		return !strings.HasPrefix(checkVal, compareVal)
	case "ENDS_WITH":
		return strings.HasSuffix(checkVal, compareVal)
	case "NOT_ENDS_WITH":
		return !strings.HasSuffix(checkVal, compareVal)
	case "REGEX":
		pattern := cond.Value
		if !cond.CaseSensitive {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(headerVal)
	case "NOT_REGEX":
		pattern := cond.Value
		if !cond.CaseSensitive {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return !re.MatchString(headerVal)
	default:
		return false
	}
}

func handleOverrideClientType(clientType string) SRRMatcherResult {
	ct := strings.ToUpper(clientType)
	switch ct {
	case "STASH":
		return SRRMatcherResult{Matched: true, ResponseType: "STASH"}
	case "SINGBOX", "SING-BOX":
		return SRRMatcherResult{Matched: true, ResponseType: "SINGBOX"}
	case "MIHOMO":
		return SRRMatcherResult{Matched: true, ResponseType: "MIHOMO"}
	case "XRAY_JSON", "V2RAY_JSON", "JSON":
		return SRRMatcherResult{Matched: true, ResponseType: "XRAY_JSON"}
	case "CLASH":
		return SRRMatcherResult{Matched: true, ResponseType: "CLASH"}
	default:
		return SRRMatcherResult{Matched: true, ResponseType: "BLOCK"}
	}
}
