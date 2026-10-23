package notify

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/url"
	"strings"
	"time"
)

type inlineKeyboard struct {
	URL         string `json:"url"`
	Text        string `json:"text"`
	CustomEmoji string `json:"customEmoji,omitempty"`
	Style       string `json:"style,omitempty"`
}

// fields is the event data decoded to generic JSON for template lookups.
type fields map[string]interface{}

func toFields(value interface{}) fields {
	data, err := json.Marshal(value)
	if err != nil {
		return fields{}
	}
	var result fields
	if json.Unmarshal(data, &result) != nil {
		return fields{}
	}
	return result
}

func (f fields) get(path ...string) interface{} {
	var value interface{} = map[string]interface{}(f)
	for _, key := range path {
		object, ok := value.(map[string]interface{})
		if !ok {
			return nil
		}
		value = object[key]
	}
	return value
}

// s returns an HTML-escaped string form of a field; Telegram rejects
// messages whose values break the HTML markup.
func (f fields) s(path ...string) string {
	switch value := f.get(path...).(type) {
	case nil:
		return ""
	case string:
		return html.EscapeString(value)
	case float64:
		return fmt.Sprint(int64(value))
	default:
		return html.EscapeString(fmt.Sprint(value))
	}
}

func (f fields) n(path ...string) float64 {
	switch value := f.get(path...).(type) {
	case float64:
		return value
	case string:
		var number float64
		fmt.Sscan(value, &number)
		return number
	}
	return 0
}

func (f fields) t(layout string, path ...string) string {
	raw, _ := f.get(path...).(string)
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(layout)
}

// prettyBytes ports prettyBytesUtil (xbytes, IEC units, two decimals).
// sticky keeps the unit fixed at prefixIndex (3 = GiB).
func prettyBytes(bytes float64, sticky bool, prefixIndex int) string {
	if bytes == 0 {
		return "0"
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"}
	index := 0
	if sticky {
		index = prefixIndex
	} else {
		for index < len(units)-1 && math.Abs(bytes) >= math.Pow(1024, float64(index+1)) {
			index++
		}
	}
	return fmt.Sprintf("%.2f %s", bytes/math.Pow(1024, float64(index)), units[index])
}

const separator = "➖➖➖➖➖➖➖➖➖"

func emoji(id, fallback string) string {
	return "<tg-emoji emoji-id='" + id + "'>" + fallback + "</tg-emoji>"
}

func header(icon, tag string) string { return icon + " <b>#" + tag + "</b>\n" + separator }

func nodeHeader(icon, tag, subtitle string) string {
	if subtitle == "" {
		return header(icon, tag)
	}
	return icon + " <b>#" + tag + "</b>\n<b>" + subtitle + "</b>\n" + separator
}

func userBasic(f fields) string { return "<b>Username:</b> <code>" + f.s("username") + "</code>" }

func userFull(f fields) string {
	return userBasic(f) + `
<b>Traffic limit:</b> <code>` + prettyBytes(f.n("trafficLimitBytes"), false, 0) + `</code>
<b>Valid until:</b> <code>` + f.t("02.01.2006 15:04", "expireAt") + ` UTC</code>
<b>Sub:</b> <code>` + f.s("shortUuid") + `</code>`
}

func userSquads(f fields) string {
	squads, _ := f.get("activeInternalSquads").([]interface{})
	var names []string
	for _, squad := range squads {
		if object, ok := squad.(map[string]interface{}); ok {
			names = append(names, html.EscapeString(fmt.Sprint(object["name"])))
		}
	}
	if len(names) == 0 {
		return "<b>Internal squads:</b> <code>-</code>"
	}
	return "<b>Internal squads:</b> <code>" + strings.Join(names, ", ") + "</code>"
}

func nodeBasic(f fields) string {
	return "\n<b>Name:</b> <code>" + f.s("name") + "</code>\n<b>Address:</b> <code>" + f.s("address") + "</code>\n"
}

func nodeConnection(f fields) string {
	return "<b>Name:</b> <code>" + f.s("name") + `</code>
<b>Reason:</b> <code>` + f.s("lastStatusMessage") + `</code>
<b>Last status change:</b> <code>` + f.t("02.01.2006 15:04", "lastStatusChange") + `</code>
<b>Address:</b> <code>` + f.s("address") + ":" + f.s("port") + "</code>"
}

func crmPayment(f fields) string {
	return "\n" + emoji("5264733042710181045", "🏢") + " <b>Provider:</b> <code>" + f.s("providerName") + "</code>\n" +
		emoji("5282843764451195532", "🖥️") + " <b>Node:</b> <code>" + f.s("nodeName") + "</code>\n" +
		emoji("5431897022456145283", "📆") + " <b>Due Date:</b> <code>" + f.t("02.01.2006", "nextBillingAt") + "</code>\n"
}

func crmLink(f fields) string {
	return `🔗 <a href="` + f.s("loginUrl") + `">Open Provider Panel</a>`
}

func daysPastDue(f fields) int {
	raw, _ := f.get("nextBillingAt").(string)
	due, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return 0
	}
	return int(math.Abs(time.Since(due).Hours() / 24))
}

// render produces the Telegram message for an event, or "" when upstream
// sends none.
func (n *Notifier) render(event Event) (string, []inlineKeyboard) {
	f := toFields(event.Data)
	meta := toFields(event.Meta)
	switch event.Name {
	case "user.created":
		return "\n" + header(emoji("5361979468887893611", "🆕"), "created") + "\n" + userFull(f) + "\n" + userSquads(f), nil
	case "user.modified":
		return "\n" + header(emoji("5334882760735598374", "📝"), "modified") + "\n" + userFull(f) + "\n" + userSquads(f), nil
	case "user.revoked":
		return "\n" + header(emoji("5264727218734524899", "🔄"), "revoked") + "\n" + userFull(f) + "\n" + userSquads(f), nil
	case "user.deleted":
		return "\n" + header(emoji("5258130763148172425", "🗑️"), "deleted") + "\n" + userBasic(f), nil
	case "user.disabled":
		return "\n" + header(emoji("5210952531676504517", "❌"), "disabled") + "\n" + userBasic(f), nil
	case "user.enabled":
		return "\n" + header(emoji("5427009714745517609", "✅"), "enabled") + "\n" + userBasic(f), nil
	case "user.limited":
		return "\n" + header(emoji("5447644880824181073", "⚠️"), "limited") + "\n" + userBasic(f), nil
	case "user.expired":
		return "\n" + header(emoji("5382194935057372936", "⏱️"), "expired") + "\n" + userBasic(f), nil
	case "user.traffic_reset":
		return "\n" + header(emoji("5264727218734524899", "🔄"), "traffic_reset") + "\n" + userBasic(f) +
			"\n<b>Traffic:</b> <code>" + prettyBytes(f.n("userTraffic", "usedTrafficBytes"), false, 0) + "</code>", nil
	case "user.first_connected":
		return "\n" + header(emoji("5379999674193172777", "🔭"), "first_connected") + "\n" + userBasic(f), nil
	case "user.expiration":
		hours, ok := meta.get("expiration").(float64)
		if !ok {
			return "", nil
		}
		tag := fmt.Sprintf("expired_%d_hours_ago", int(hours))
		if hours < 0 {
			tag = fmt.Sprintf("expires_in_%d_hours", int(-hours))
		}
		return "\n" + header(emoji("5382194935057372936", "⏱️"), tag) + "\n" + userBasic(f), nil
	case "user.bandwidth_usage_threshold_reached":
		return "\n" + header(emoji("5447644880824181073", "⚠️"), "bandwidth_usage_threshold_reached") + "\n" + userBasic(f) +
			"\n<b>Traffic:</b> <code>" + prettyBytes(f.n("userTraffic", "usedTrafficBytes"), false, 0) + "</code>" +
			"\n<b>Limit:</b> <code>" + prettyBytes(f.n("trafficLimitBytes"), false, 0) + "</code>" +
			"\n<b>Threshold:</b> <code>" + f.s("lastTriggeredThreshold") + "%</code>", nil
	case "user.not_connected":
		if event.Meta == nil {
			return "", nil
		}
		return "\n" + header(emoji("5382194935057372936", "⏱️"), "not_connected_after_"+meta.s("notConnectedAfterHours")+"_hours") + "\n" + userBasic(f), nil

	case "node.created":
		return "\n" + header(`<tg-emoji emoji-id="5282843764451195532">🖥️</tg-emoji>`, "nodeCreated") + "\n" + nodeBasic(f), nil
	case "node.modified":
		return "\n" + header(emoji("5334882760735598374", "📝"), "nodeModified") + "\n" + nodeBasic(f), nil
	case "node.disabled":
		return "\n" + header(emoji("5447644880824181073", "⚠️"), "nodeDisabled") + "\n" + nodeBasic(f), nil
	case "node.enabled":
		return "\n" + header(emoji("5206607081334906820", "🟩"), "nodeEnabled") + "\n" + nodeBasic(f), nil
	case "node.deleted":
		return "\n" + nodeHeader(emoji("5370971163310693562", "💀"), "nodeDeleted", "Node deleted") + "\n" + nodeBasic(f), nil
	case "node.connection_lost":
		return "\n" + nodeHeader(emoji("5447183459602669338", "🚨"), "nodeConnectionLost", "Connection to node lost") + "\n" + nodeConnection(f), nil
	case "node.connection_restored":
		return "\n" + nodeHeader(emoji("5449683594425410231", "❇️"), "nodeConnectionRestored", "Connection to node restored") + "\n" + nodeConnection(f), nil
	case "node.traffic_notify":
		return "\n" + nodeHeader(emoji("5431577498364158238", "📊"), "nodeTrafficNotify", "Bandwidth limit reached") + "\n" +
			emoji("5447410659077661506", "🌐") + " <code>" + prettyBytes(f.n("trafficUsedBytes"), true, 3) + "</code> <b>/</b> <code>" + prettyBytes(f.n("trafficLimitBytes"), true, 3) + "</code>\n" +
			"<b>Name:</b> <code>" + f.s("name") + "</code>\n<b>Address:</b> <code>" + f.s("address") + ":" + f.s("port") + "</code>\n" +
			"<b>Traffic reset day:</b> <code>" + f.s("trafficResetDay") + "</code>\n<b>Percent:</b> <code>" + f.s("notifyPercent") + " %</code>", nil

	case "service.panel_started":
		return "\n" + emoji("5418304400152096012", "🌊") + " <b>#panel_started</b>\n" + separator + "\n" +
			emoji("5461117441612462242", "✅") + " GoWave v" + f.s("panelVersion") + " is up and running.\n\n" +
			emoji("5415680458602081205", "📚") + " Documentation: https://aaasazan.github.io/gowave-docs/", []inlineKeyboard{
			{URL: "https://github.com/aaaSaZaN/gowave", Text: "Leave a star", CustomEmoji: "5258039825805624495"},
		}
	case "service.login_attempt_failed", "service.login_attempt_success":
		success := event.Name == "service.login_attempt_success"
		mark := emoji("5472267631979405211", "❌") + "<b>#login_attempt_failed</b>"
		if success {
			mark = emoji("5461117441612462242", "✅") + " <b>#login_attempt_success</b>"
		}
		message := "\n" + emoji("5330115548900501467", "🔑") + " " + mark + "\n" + separator + "\n" +
			emoji("5256143829672672750", "👥") + " <code>" + f.s("loginAttempt", "username") + "</code>\n"
		if !success {
			// Never forward the typed password: an admin's near-miss of the real
			// one would end up in the chat history.
			message += emoji("5330115548900501467", "🔑") + " <b>Password:</b> <code>" + maskSecret(f.get("loginAttempt", "password")) + "</code>\n"
		}
		message += emoji("5447410659077661506", "🌐") + " <b>IP:</b> <code>" + f.s("loginAttempt", "ip") + "</code>\n" +
			emoji("5460756166143405924", "💻") + " <b>User agent:</b> <code>" + f.s("loginAttempt", "userAgent") + "</code>\n" +
			emoji("5443038326535759644", "💬") + " <b>Description:</b> <code>" + f.s("loginAttempt", "description") + "</code>"
		return message, nil
	case "service.subpage_config_changed":
		return "\n" + header(emoji("5334882760735598374", "📝"), "subpage_config_changed") + "\n<b>Action:</b> <code>" + f.s("subpageConfig", "action") +
			"</code>\n<b>UUID:</b> <code>" + f.s("subpageConfig", "uuid") + "</code>", nil
	case "service.api_token_created", "service.api_token_deleted":
		tag := strings.TrimPrefix(event.Name, "service.")
		scopes, _ := f.get("apiToken", "scopes").([]interface{})
		raw, _ := f.get("apiToken", "expireAt").(string)
		expire, _ := time.Parse(time.RFC3339Nano, raw)
		return "\n" + header(emoji("5334882760735598374", "📝"), tag) + "\n<b>Name:</b> <code>" + f.s("apiToken", "name") + "</code>\n" +
			fmt.Sprintf(`<b>Expire at:</b> <tg-time unix="%d" format="DT">%s</tg-time>`, expire.Unix(), expire.UTC().Format("02.01.2006 15:04:05")) + "\n" +
			fmt.Sprintf("<b>Scopes:</b> <code>%d</code>\n", len(scopes)) + "<b>UUID:</b> <code>" + f.s("apiToken", "uuid") + "</code>", nil
	case "errors.bandwidth_usage_threshold_reached_max_notifications":
		return "\n" + emoji("5276089339967716971", "📢") + " <b>#bandwidth_usage_threshold_reached_max_notifications</b>\n" + separator +
			"\n<b>Description:</b> <code>" + f.s("description") + "</code>", nil

	case "crm.infra_billing_node_payment_in_7_days":
		return "\n" + emoji("5431897022456145283", "📆") + " <b>Payment Reminder</b>\n\n" + crmPayment(f) + "\n\n" + crmLink(f), nil
	case "crm.infra_billing_node_payment_in_48hrs":
		return "\n" + emoji("5447644880824181073", "⚠️") + " <b>Payment Alert - 2 Days Warning</b>\n\n" + crmPayment(f) + "\n\n" +
			emoji("5219943216781995020", "⚡") + " <i>Payment is due in 2 days!</i>\n\n" + crmLink(f), nil
	case "crm.infra_billing_node_payment_in_24hrs":
		return "\n" + emoji("5395695537687123235", "🚨") + " <b>URGENT: Payment Due Tomorrow!</b>\n\n" + crmPayment(f) + "\n\n" +
			emoji("5420315771991497307", "🔥") + " <i>Payment is due tomorrow!</i>\n\n" + crmLink(f), nil
	case "crm.infra_billing_node_payment_due_today":
		return "\n" + emoji("5411225014148014586", "🔴") + " <b>CRITICAL: Payment Due TODAY!</b>\n\n" + crmPayment(f) + "\n\n" +
			emoji("5219943216781995020", "⚡") + " <i>Payment must be completed today!</i>\n\n" + crmLink(f), nil
	case "crm.infra_billing_node_payment_overdue_24hrs":
		return "\n" + emoji("5472267631979405211", "❌") + " <b>OVERDUE: First Notice</b>\n\n" + crmPayment(f) +
			emoji("5447644880824181073", "⚠️") + fmt.Sprintf(" <b>Days Overdue:</b> <code>%d day(s)</code>\n\n", daysPastDue(f)) +
			emoji("5395695537687123235", "🚨") + "<i>Payment is overdue!</i>\n\n" + crmLink(f), nil
	case "crm.infra_billing_node_payment_overdue_48hrs":
		return "\n" + emoji("5420315771991497307", "🔥") + " <b>OVERDUE: Second Notice</b>\n\n" + crmPayment(f) +
			emoji("5447644880824181073", "⚠️") + fmt.Sprintf(" <b>Days Overdue:</b> <code>%d day(s)</code>\n\n", daysPastDue(f)) +
			emoji("5219943216781995020", "⚡") + " <i>Critical: Service suspension imminent!</i>\n\n" + crmLink(f), nil
	case "crm.infra_billing_node_payment_overdue_7_days":
		return "\n" + emoji("5370971163310693562", "💀") + " <b>FINAL NOTICE: Service Termination Risk</b>\n\n" + crmPayment(f) +
			emoji("5447644880824181073", "⚠️") + fmt.Sprintf(" <b>Days Overdue:</b> <code>%d day(s)</code>\n\n", daysPastDue(f)) + crmLink(f), nil

	case "torrent_blocker.report":
		username := f.s("user", "username")
		lines := []string{
			emoji("5469913852462242978", "🧨️") + " #torrentBlocked #" + username,
			emoji("5215186239853964761", "🖥") + " <code>" + f.s("node", "name") + "</code> (<code>" + f.s("node", "address") + "</code>)",
			emoji("5372981976804366741", "🤖") + " <code>" + username + "</code> (<code>" + f.s("user", "id") + "</code>)",
			"",
			"<blockquote expandable>",
			`<tg-emoji emoji-id="5271604874419647061">🔗</tg-emoji> <b>Connection</b>`,
			"<b>IP:</b> <code>" + f.s("report", "actionReport", "ip") + "</code>",
			"<b>Protocol:</b> <code>" + orDefault(f.s("report", "xrayReport", "protocol"), "unknown") + "</code> (<code>" + f.s("report", "xrayReport", "network") + "</code>)",
			"<b>Dest:</b> <code>" + f.s("report", "xrayReport", "destination") + "</code>",
			"<b>Inbound:</b> <code>" + orDefault(f.s("report", "xrayReport", "inboundTag"), "—") + "</code>",
			"",
			`<tg-emoji emoji-id="5472267631979405211">⛔</tg-emoji> <b>Block info</b>`,
			fmt.Sprintf("<b>Duration:</b> <code>%d min</code>", int(f.n("report", "actionReport", "blockDuration")/60)),
			"<b>Unblock at:</b> <code>" + f.t("02.01.2006 15:04:05", "report", "actionReport", "willUnblockAt") + "</code>",
			"</blockquote>",
		}
		var keyboard []inlineKeyboard
		if n.dashboardURL != "" {
			keyboard = []inlineKeyboard{{URL: n.dashboardURL + "dashboard/open/user/" + url.PathEscape(fmt.Sprint(f.get("user", "id"))), Text: "View user", CustomEmoji: "5282843764451195532", Style: "primary"}}
		}
		return strings.Join(lines, "\n"), keyboard
	}
	return "", nil
}

// maskSecret shows only the length of a submitted secret.
func maskSecret(value interface{}) string {
	secret, _ := value.(string)
	if secret == "" {
		return "-"
	}
	return fmt.Sprintf("hidden, %d chars", len([]rune(secret)))
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

var allEvents = []string{
	"user.created", "user.modified", "user.deleted", "user.revoked", "user.disabled", "user.enabled", "user.limited",
	"user.expired", "user.traffic_reset", "user.first_connected", "user.bandwidth_usage_threshold_reached",
	"user.not_connected", "user.expiration",
	"user_hwid_devices.added", "user_hwid_devices.deleted",
	"node.created", "node.modified", "node.disabled", "node.enabled", "node.deleted", "node.connection_lost",
	"node.connection_restored", "node.traffic_notify",
	"service.panel_started", "service.login_attempt_failed", "service.login_attempt_success",
	"service.subpage_config_changed", "service.api_token_created", "service.api_token_deleted",
	"errors.bandwidth_usage_threshold_reached_max_notifications",
	"crm.infra_billing_node_payment_in_7_days", "crm.infra_billing_node_payment_in_48hrs",
	"crm.infra_billing_node_payment_in_24hrs", "crm.infra_billing_node_payment_due_today",
	"crm.infra_billing_node_payment_overdue_24hrs", "crm.infra_billing_node_payment_overdue_48hrs",
	"crm.infra_billing_node_payment_overdue_7_days",
	"torrent_blocker.report",
}

func knownEvent(name string) bool {
	for _, event := range allEvents {
		if event == name {
			return true
		}
	}
	return false
}
