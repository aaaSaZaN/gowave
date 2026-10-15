package subtpl

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"remnawave-go/internal/subtpl/js"
)

// Now is the clock remark templates use; tests pin it.
var Now = func() time.Time { return time.Now().UTC() }

var templateRegex = regexp.MustCompile(`\{\{(\w+)(?::([^{}]*))?\}\}`)

const defaultDateFormat = "DD.MM.YYYY"

type resolver func(args map[string]string) string

// TemplateValues ports TemplateEngine.createUserValueMap.
func TemplateValues(user *User, settings *Settings, subPublicDomain string) map[string]resolver {
	trafficLeft := func() int64 {
		if user.TrafficLimitBytes == 0 {
			return 0
		}
		return user.TrafficLimitBytes - user.UsedTrafficBytes
	}
	formatDate := func(at *time.Time, args map[string]string) string {
		if at == nil {
			return ""
		}
		format := args["format"]
		if format == "" {
			format = defaultDateFormat
		}
		return js.DayjsFormat(*at, format)
	}
	nextReset := func() *time.Time { return nextTrafficResetAt(user.TrafficLimitStrategy, user.CreatedAt, Now()) }
	unix := func(t time.Time) string { return strconv.FormatInt(int64(math.Floor(float64(t.UnixMilli())/1000)), 10) }
	optional := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return map[string]resolver{
		"DAYS_LEFT": func(map[string]string) string {
			days := int64(user.ExpireAt.Sub(Now()) / (24 * time.Hour))
			if days < 0 {
				days = 0
			}
			return strconv.FormatInt(days, 10)
		},
		"TRAFFIC_USED":  func(map[string]string) string { return js.PrettyBytes(float64(user.UsedTrafficBytes), true, 3) },
		"TRAFFIC_LEFT":  func(map[string]string) string { return js.PrettyBytes(float64(trafficLeft()), true, 3) },
		"TOTAL_TRAFFIC": func(map[string]string) string { return js.PrettyBytes(float64(user.TrafficLimitBytes), true, 3) },
		"STATUS": func(args map[string]string) string {
			if label, ok := args[user.Status]; ok {
				return label
			}
			if user.Status == "" {
				return ""
			}
			return user.Status[:1] + strings.ToLower(user.Status[1:])
		},
		"USERNAME": func(map[string]string) string { return user.Username },
		"EMAIL":    func(map[string]string) string { return optional(user.Email) },
		"TELEGRAM_ID": func(map[string]string) string {
			if user.TelegramID == nil {
				return ""
			}
			return strconv.FormatInt(*user.TelegramID, 10)
		},
		"SUBSCRIPTION_URL":   func(map[string]string) string { return "https://" + subPublicDomain + "/" + user.ShortUUID },
		"TAG":                func(map[string]string) string { return optional(user.Tag) },
		"EXPIRE_UNIX":        func(map[string]string) string { return unix(user.ExpireAt) },
		"SHORT_UUID":         func(map[string]string) string { return user.ShortUUID },
		"ID":                 func(map[string]string) string { return strconv.FormatInt(user.ID, 10) },
		"TRAFFIC_USED_BYTES": func(map[string]string) string { return strconv.FormatInt(user.UsedTrafficBytes, 10) },
		"TRAFFIC_LEFT_BYTES": func(map[string]string) string { return strconv.FormatInt(trafficLeft(), 10) },
		"TOTAL_TRAFFIC_BYTES": func(map[string]string) string {
			return strconv.FormatInt(user.TrafficLimitBytes, 10)
		},
		"RESET_STRATEGY": func(args map[string]string) string {
			if label, ok := args[user.TrafficLimitStrategy]; ok {
				return label
			}
			return user.TrafficLimitStrategy
		},
		"LIFETIME_USED_BYTES": func(map[string]string) string { return strconv.FormatInt(user.LifetimeUsedTrafficBytes, 10) },
		"CREATED_AT_UNIX":     func(map[string]string) string { return unix(user.CreatedAt) },
		"LAST_TRAFFIC_RESET_AT_UNIX": func(map[string]string) string {
			if user.LastTrafficResetAt == nil {
				return "0"
			}
			return unix(*user.LastTrafficResetAt)
		},
		"LAST_TRAFFIC_RESET_AT": func(args map[string]string) string { return formatDate(user.LastTrafficResetAt, args) },
		"NEXT_TRAFFIC_RESET_AT_UNIX": func(map[string]string) string {
			if next := nextReset(); next != nil {
				return unix(*next)
			}
			return "0"
		},
		"NEXT_TRAFFIC_RESET_AT": func(args map[string]string) string { return formatDate(nextReset(), args) },
		"SS_HWID_LIMIT": func(map[string]string) string {
			if user.HwidDeviceLimit != nil {
				return strconv.Itoa(*user.HwidDeviceLimit)
			}
			if settings != nil && settings.HwidFallbackDeviceLimit != nil {
				return strconv.Itoa(*settings.HwidFallbackDeviceLimit)
			}
			return "0"
		},
		"DESCRIPTION": func(map[string]string) string { return optional(user.Description) },
	}
}

// ReplaceTemplate ports TemplateEngine.replace, including the
// "rwEncodeBase64:" prefix that yields "base64:<encoded>".
func ReplaceTemplate(template string, values map[string]resolver) string {
	const encodePrefix = "rwEncodeBase64:"
	encode := strings.HasPrefix(template, encodePrefix)
	body := strings.TrimPrefix(template, encodePrefix)
	result := templateRegex.ReplaceAllStringFunc(body, func(placeholder string) string {
		match := templateRegex.FindStringSubmatch(placeholder)
		resolve, ok := values[match[1]]
		if !ok {
			return placeholder
		}
		args := map[string]string{}
		if match[2] != "" {
			for _, pair := range strings.Split(match[2], "|") {
				if index := strings.IndexByte(pair, '='); index >= 0 {
					args[strings.TrimSpace(pair[:index])] = pair[index+1:]
				}
			}
		}
		return resolve(args)
	})
	if encode {
		return "base64:" + js.Base64(result)
	}
	return result
}

// nextTrafficResetAt ports getNextTrafficResetAt for a UTC process.
func nextTrafficResetAt(strategy string, createdAt, now time.Time) *time.Time {
	startOfDay := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) }
	pick := func(next, fallback time.Time) *time.Time {
		if next.After(now) {
			return &next
		}
		return &fallback
	}
	switch strategy {
	case "DAY":
		next := startOfDay(now).Add(5 * time.Minute)
		return pick(next, next.AddDate(0, 0, 1))
	case "WEEK":
		days := (int(time.Monday) - int(now.Weekday()) + 7) % 7
		next := startOfDay(now).AddDate(0, 0, days).Add(15 * time.Minute)
		return pick(next, next.AddDate(0, 0, 7))
	case "MONTH":
		next := time.Date(now.Year(), now.Month(), 1, 0, 20, 0, 0, time.UTC)
		return pick(next, addMonths(next, 1))
	case "MONTH_ROLLING":
		anchor := createdAt.UTC().Day()
		resetIn := func(month time.Time) time.Time {
			first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
			last := first.AddDate(0, 1, -1).Day()
			day := anchor
			if day > last {
				day = last
			}
			return time.Date(first.Year(), first.Month(), day, 0, 10, 0, 0, time.UTC)
		}
		next := resetIn(now)
		resolved := next
		if !next.After(now) {
			resolved = resetIn(addMonths(now, 1))
		}
		firstReset := resetIn(addMonths(createdAt.UTC(), 1))
		if resolved.Before(firstReset) {
			return &firstReset
		}
		return &resolved
	}
	return nil
}

// addMonths is dayjs#add(n, 'month'): the day clamps to the target month.
func addMonths(t time.Time, months int) time.Time {
	first := time.Date(t.Year(), t.Month()+time.Month(months), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	day := t.Day()
	if day > last {
		day = last
	}
	return time.Date(first.Year(), first.Month(), day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
}
