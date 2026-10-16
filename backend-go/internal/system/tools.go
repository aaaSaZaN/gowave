package system

import (
	"encoding/json"
	"gowave/internal/auth"
	"gowave/internal/contracts"
	"gowave/internal/database"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Tools struct {
	db     *gorm.DB
	secret string
}

func NewTools(db *gorm.DB, secret string) *Tools { return &Tools{db, secret} }
func (t *Tools) Register(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(t.authenticate)
		r.Get("/api/backend-tools/queues", t.queues)
		r.Post("/api/backend-tools/queues/{id}/retry", t.retry)
		r.Post("/api/backend-tools/queues/{id}/cancel", t.cancel)
		r.Get("/api/backend-tools/swagger", t.docs)
		r.Get("/api/backend-tools/scalar", t.docs)
		r.Get("/api/backend-tools/openapi.json", t.openapi)
	})
}
func (t *Tools) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if raw := r.URL.Query().Get("ott"); raw != "" {
			claims, ok := t.claims(raw, "ott")
			if !ok {
				http.Error(w, "invalid or expired one-time token", 401)
				return
			}
			if _, err := auth.ConsumeChallenge(t.db, claims.ID, "ott", claims.Subject); err != nil {
				http.Error(w, "one-time token already consumed", 401)
				return
			}
			id := uuid.NewString()
			expires := time.Now().UTC().Add(2 * time.Hour)
			if err := t.db.Create(&database.AuthChallenge{ID: id, Purpose: "backend-tools", AdminUUID: claims.Subject, ExpiresAt: expires}).Error; err != nil {
				http.Error(w, "session unavailable", 503)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "rw-tools", Value: id, Path: "/api/backend-tools", Expires: expires, MaxAge: 7200, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"})
			target := *r.URL
			query := target.Query()
			query.Del("ott")
			target.RawQuery = query.Encode()
			http.Redirect(w, r, target.String(), 303)
			return
		}
		cookie, err := r.Cookie("rw-tools")
		if err != nil {
			http.Error(w, "backend tools login required", 401)
			return
		}
		var record database.AuthChallenge
		if err := t.db.Where("id=? AND purpose=?", cookie.Value, "backend-tools").First(&record).Error; err != nil || !record.ExpiresAt.After(time.Now()) {
			http.Error(w, "expired session", 401)
			return
		}
		var admin database.Admin
		if err := t.db.Where("uuid=? AND role='ADMIN'", record.AdminUUID).First(&admin).Error; err != nil {
			http.Error(w, "invalid administrator", 401)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			origin := r.Header.Get("Origin")
			if origin != "https://"+r.Host && origin != "http://"+r.Host {
				http.Error(w, "invalid origin", 403)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (t *Tools) claims(raw, scope string) (*jwt.RegisteredClaims, bool) {
	var claims struct {
		Scope string `json:"scope"`
		jwt.RegisteredClaims
	}
	token, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (interface{}, error) { return []byte(t.secret), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuer("backend-tools"))
	if err != nil || !token.Valid || claims.Scope != scope || claims.ID == "" || claims.Subject == "" {
		return nil, false
	}
	var admin database.Admin
	if t.db.Where("uuid=? AND role='ADMIN'", claims.Subject).First(&admin).Error != nil {
		return nil, false
	}
	return &claims.RegisteredClaims, true
}

var queuePage = template.Must(template.New("queue").Funcs(template.FuncMap{
	"time": func(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05") },
}).Parse(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Gowave queues</title>
<style>
:root{color-scheme:light dark;--bg:#fff;--fg:#1b1f24;--muted:#6b7280;--line:#e5e7eb;--accent:#2563eb;--bad:#b91c1c;--ok:#15803d}
@media (prefers-color-scheme:dark){:root{--bg:#111418;--fg:#e6e8eb;--muted:#9aa3ad;--line:#2a2f36;--accent:#60a5fa;--bad:#f87171;--ok:#4ade80}}
body{margin:0;padding:24px 16px;background:var(--bg);color:var(--fg);font:14px/1.45 system-ui,sans-serif}
h1{font-size:20px;margin:0 0 4px}p{color:var(--muted);margin:0 0 16px}
nav a{margin-right:12px;color:var(--accent);text-decoration:none}nav a.on{font-weight:600;text-decoration:underline}
.wrap{overflow-x:auto}table{border-collapse:collapse;width:100%;margin-top:12px}
th,td{border-bottom:1px solid var(--line);padding:6px 8px;text-align:left;vertical-align:top}
th{color:var(--muted);font-weight:500}code{font-size:12px}
.failed{color:var(--bad)}.completed{color:var(--ok)}
details summary{cursor:pointer;color:var(--bad)}pre{white-space:pre-wrap;max-width:60ch;margin:4px 0}
button{font:inherit;padding:2px 10px;border:1px solid var(--line);border-radius:6px;background:transparent;color:var(--fg);cursor:pointer}
</style></head><body>
<h1>Worker queues</h1><p>Eight database-backed workers; per-node work is serialized. Failed jobs stay here until retried or cancelled.</p>
<nav>{{range .States}}<a href="?state={{.Name}}" class="{{if .Active}}on{{end}}">{{.Name}} ({{.Count}})</a>{{end}}</nav>
<div class="wrap"><table><thead><tr><th>Kind</th><th>Entity</th><th>Node</th><th>State</th><th>Attempts</th><th>Next attempt</th><th>Updated</th><th>Error</th><th></th></tr></thead><tbody>
{{range .Jobs}}<tr><td><code>{{.Kind}}</code></td><td><code>{{.EntityID}}</code></td><td><code>{{.Target}}</code></td><td class="{{.State}}">{{.State}}</td><td>{{.Attempts}}</td><td>{{time .AvailableAt}}</td><td>{{time .UpdatedAt}}</td>
<td>{{if .LastError}}<details><summary>show</summary><pre>{{.LastError}}</pre></details>{{end}}</td>
<td>{{if eq .State "failed"}}<form method="post" action="/api/backend-tools/queues/{{.ID}}/retry" style="display:inline"><button>Retry</button></form>{{end}}
{{if or (eq .State "failed") (eq .State "pending")}}<form method="post" action="/api/backend-tools/queues/{{.ID}}/cancel" style="display:inline"><button>Cancel</button></form>{{end}}</td></tr>
{{else}}<tr><td colspan="9">No jobs in this state.</td></tr>{{end}}
</tbody></table></div></body></html>`))

var jobStates = []string{"failed", "pending", "running", "completed", "cancelled"}

func (t *Tools) queues(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state == "" {
		state = "failed"
	}
	var records []database.Job
	if err := t.db.Omit("payload").Where("state = ?", state).Order("updated_at DESC").Limit(500).Find(&records).Error; err != nil {
		http.Error(w, "queue unavailable", 503)
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"response": records})
		return
	}
	type stateCount struct {
		Name   string
		Count  int64
		Active bool
	}
	var counts []struct {
		State string
		Count int64
	}
	t.db.Model(&database.Job{}).Select("state, COUNT(*) AS count").Group("state").Scan(&counts)
	states := make([]stateCount, 0, len(jobStates))
	for _, name := range jobStates {
		entry := stateCount{Name: name, Active: name == state}
		for _, count := range counts {
			if count.State == name {
				entry.Count = count.Count
			}
		}
		states = append(states, entry)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'")
	queuePage.Execute(w, map[string]interface{}{"Jobs": records, "States": states})
}

func (t *Tools) retry(w http.ResponseWriter, r *http.Request) {
	t.transition(w, r, "failed", map[string]interface{}{"state": "pending", "attempts": 0, "last_error": "", "available_at": time.Now().UTC()})
}

// cancel withdraws a pending or failed job; running jobs finish their lease.
func (t *Tools) cancel(w http.ResponseWriter, r *http.Request) {
	t.transition(w, r, "", map[string]interface{}{"state": "cancelled", "lease_until": nil})
}

func (t *Tools) transition(w http.ResponseWriter, r *http.Request, from string, updates map[string]interface{}) {
	query := t.db.Model(&database.Job{}).Where("id = ?", chi.URLParam(r, "id"))
	if from != "" {
		query = query.Where("state = ?", from)
	} else {
		query = query.Where("state IN ?", []string{"pending", "failed"})
	}
	updates["updated_at"] = time.Now().UTC()
	result := query.Updates(updates)
	if result.Error != nil {
		http.Error(w, "update failed", 503)
		return
	}
	if result.RowsAffected != 1 {
		http.Error(w, "job not found in a state that allows this action", 404)
		return
	}
	http.Redirect(w, r, "/api/backend-tools/queues", 303)
}

// Swagger UI and Scalar load pinned assets from jsDelivr and render the
// OpenAPI document generated from the pinned contract.
const swaggerPage = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Gowave API (Swagger)</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css" integrity="sha384-wxLW6kwyHktdDGr6Pv1zgm/VGJh99lfUbzSn6HNHBENZlCN7W602k9VkGdxuFvPn" crossorigin="anonymous"></head><body><div id="ui"></div>
<script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js" integrity="sha384-wmyclcVGX/WhUkdkATwhaK1X1JtiNrr2EoYJ+diV3vj4v6OC5yCeSu+yW13SYJep" crossorigin="anonymous"></script>
<script>window.ui = SwaggerUIBundle({url: "/api/backend-tools/openapi.json", dom_id: "#ui", persistAuthorization: true});</script></body></html>`

const scalarPage = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Gowave API (Scalar)</title></head><body>
<script id="api-reference" data-url="/api/backend-tools/openapi.json"></script>
<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.25.11" integrity="sha384-p18x+z43d67nhmyocdVL3YvAL7i/8WyQmfF6pa0+GpO3P8/IRVWcKO++ZG+a3Euz" crossorigin="anonymous"></script></body></html>`

func (t *Tools) docs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://fonts.googleapis.com; font-src 'self' data: https://cdn.jsdelivr.net https://fonts.gstatic.com; img-src 'self' data: https:; connect-src 'self'")
	if strings.HasSuffix(r.URL.Path, "/scalar") {
		w.Write([]byte(scalarPage))
		return
	}
	w.Write([]byte(swaggerPage))
}

func (t *Tools) openapi(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(contracts.OpenAPI)
}
