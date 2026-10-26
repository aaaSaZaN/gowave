package subscription

import (
	"encoding/json"
	"regexp"
	"strings"

	"gowave/internal/responserules"
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
	Description           string                    `json:"description,omitempty"`
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

type SRRResponseModifications struct {
	Headers                           responserules.Headers `json:"headers"`
	ApplyHeadersToEnd                 bool                  `json:"applyHeadersToEnd"`
	SubscriptionTemplate              string                `json:"subscriptionTemplate,omitempty"`
	IgnoreHostXrayJSONTemplate        bool                  `json:"ignoreHostXrayJsonTemplate"`
	IgnoreServeJSONAtBaseSubscription bool                  `json:"ignoreServeJsonAtBaseSubscription"`
	DisableHwidCheck                  bool                  `json:"disableHwidCheck"`
	AdditionalExtendedClientsRegex    []string              `json:"additionalExtendedClientsRegex,omitempty"`
	Encryption                        *SRREncryption        `json:"encryption,omitempty"`
	ExcludeHostsByTags                []string              `json:"excludeHostsByTags,omitempty"`
	RespondWithRemarks                []string              `json:"respondWithRemarks,omitempty"`
}

// SRREncryption is ResponseRuleEncryptionSchema.
type SRREncryption struct {
	Method string `json:"method"`
	Key    string `json:"key"`
}

type SRRMatcherResult struct {
	Matched      bool
	MatchedRule  *SRRRule
	ResponseType string
}

// MatchSRRRules ports ResponseRulesMatcherService.matchRules. A client type
// from the path bypasses the rules entirely.
func MatchSRRRules(rawRulesJSON string, headers map[string]string, overrideClientType string) SRRMatcherResult {
	if rawRulesJSON == "" || rawRulesJSON == "{}" || rawRulesJSON == "null" {
		return SRRMatcherResult{Matched: false}
	}
	var config SRRConfig
	if err := json.Unmarshal([]byte(rawRulesJSON), &config); err != nil {
		return SRRMatcherResult{Matched: false}
	}
	return config.Match(headers, overrideClientType)
}

// Match evaluates the rules against lower-cased request headers.
func (config *SRRConfig) Match(headers map[string]string, overrideClientType string) SRRMatcherResult {
	if overrideClientType != "" {
		if config.Settings != nil && config.Settings.DisableSubscriptionAccessByPath {
			return SRRMatcherResult{Matched: true, ResponseType: "BLOCK"}
		}
		return handleOverrideClientType(overrideClientType)
	}
	for i := range config.Rules {
		rule := &config.Rules[i]
		if !rule.Enabled {
			continue
		}
		if matchRule(rule, headers) {
			return SRRMatcherResult{Matched: true, MatchedRule: rule, ResponseType: rule.ResponseType}
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
	headerVal, ok := headers[strings.ToLower(cond.HeaderName)]
	if !ok {
		return false
	}
	compareVal := cond.Value
	if !cond.CaseSensitive {
		compareVal = strings.ToLower(compareVal)
		headerVal = strings.ToLower(headerVal)
	}
	switch cond.Operator {
	case "EQUALS":
		return headerVal == compareVal
	case "NOT_EQUALS":
		return headerVal != compareVal
	case "CONTAINS":
		return strings.Contains(headerVal, compareVal)
	case "NOT_CONTAINS":
		return !strings.Contains(headerVal, compareVal)
	case "STARTS_WITH":
		return strings.HasPrefix(headerVal, compareVal)
	case "NOT_STARTS_WITH":
		return !strings.HasPrefix(headerVal, compareVal)
	case "ENDS_WITH":
		return strings.HasSuffix(headerVal, compareVal)
	case "NOT_ENDS_WITH":
		return !strings.HasSuffix(headerVal, compareVal)
	case "REGEX", "NOT_REGEX":
		// Upstream builds new RegExp(value, caseSensitive ? '' : 'i') from the
		// already lower-cased value.
		pattern := compareVal
		if !cond.CaseSensitive {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(headerVal) == (cond.Operator == "REGEX")
	default:
		return false
	}
}

// handleOverrideClientType maps REQUEST_TEMPLATE_TYPE path values.
func handleOverrideClientType(clientType string) SRRMatcherResult {
	switch clientType {
	case "stash":
		return SRRMatcherResult{Matched: true, ResponseType: "STASH"}
	case "singbox":
		return SRRMatcherResult{Matched: true, ResponseType: "SINGBOX"}
	case "mihomo":
		return SRRMatcherResult{Matched: true, ResponseType: "MIHOMO"}
	case "json", "v2ray-json":
		return SRRMatcherResult{Matched: true, ResponseType: "XRAY_JSON"}
	case "clash":
		return SRRMatcherResult{Matched: true, ResponseType: "CLASH"}
	default:
		return SRRMatcherResult{Matched: true, ResponseType: "BLOCK"}
	}
}
