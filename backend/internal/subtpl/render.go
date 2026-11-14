package subtpl

import (
	"gowave/internal/subtpl/ojson"
)

// Templates returns the parsed template of a type ("XRAY_JSON", "SINGBOX",
// "MIHOMO", "STASH", "CLASH") for the requested name, falling back to the
// default like getTemplateByNameAndTypeOrGetDefault.
type Templates func(templateType, name string) (*ojson.Object, error)

// RenderRequest mirrors IGenerateSubscription plus the SRR context fields
// the renderer reads.
type RenderRequest struct {
	ResponseType               string
	IsExtendedClient           bool
	OverrideTemplateName       string
	IgnoreHostXrayJSONTemplate bool
	Resolve                    ResolveOptions
}

// ContentTypes ports SUBSCRIPTION_CONFIG_TYPES.
var ContentTypes = map[string]string{
	"MIHOMO": "text/yaml", "CLASH": "text/yaml", "STASH": "text/yaml",
	"SINGBOX": "application/json", "XRAY_BASE64": "text/plain", "XRAY_JSON": "application/json",
	"BLOCK": "text/plain", "STATUS_CODE_404": "text/plain", "STATUS_CODE_451": "text/plain",
	"SOCKET_DROP": "text/plain", "BROWSER": "text/html",
}

// Render ports RenderTemplatesService.generateSubscription. Generator errors
// yield an empty body like upstream's catch blocks.
func Render(request RenderRequest, templates Templates) (body, contentType string) {
	hosts := ResolveProxyConfig(request.Resolve)
	template := func(kind string) *ojson.Object {
		t, err := templates(kind, request.OverrideTemplateName)
		if err != nil || t == nil {
			return nil
		}
		return t
	}
	switch request.ResponseType {
	case "XRAY_BASE64":
		return GenerateLinks(hosts, true, request.IsExtendedClient), ContentTypes["XRAY_BASE64"]
	case "CLASH":
		t := template("CLASH")
		if t == nil {
			return "", ContentTypes["CLASH"]
		}
		out, err := GenerateClash(hosts, t)
		if err != nil {
			out = ""
		}
		return out, ContentTypes["CLASH"]
	case "MIHOMO", "STASH":
		t := template(request.ResponseType)
		if t == nil {
			return "", ContentTypes[request.ResponseType]
		}
		out, err := GenerateMihomo(hosts, t, request.ResponseType == "STASH", request.ResponseType == "MIHOMO" && request.IsExtendedClient)
		if err != nil {
			out = ""
		}
		return out, ContentTypes[request.ResponseType]
	case "SINGBOX":
		t := template("SINGBOX")
		if t == nil {
			return "", ContentTypes["SINGBOX"]
		}
		return GenerateSingbox(hosts, t), ContentTypes["SINGBOX"]
	case "XRAY_JSON":
		t := template("XRAY_JSON")
		if t == nil {
			return "", ContentTypes["XRAY_JSON"]
		}
		return GenerateXrayJSON(XrayJSONParams{Hosts: hosts, IsExtendedClient: request.IsExtendedClient, Template: t, IgnoreHostXrayJSONTemplate: request.IgnoreHostXrayJSONTemplate}), ContentTypes["XRAY_JSON"]
	}
	return "", ""
}
