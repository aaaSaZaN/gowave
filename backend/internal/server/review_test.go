package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
)

func TestMissingComputedProfileReturnsNotFound(t *testing.T) {
	e := newProbeEnv(t)
	status, body := e.do("GET", "/api/config-profiles/"+uuid.NewString()+"/computed-config", nil)
	if status != 404 {
		t.Fatalf("missing computed config: %d %s", status, body)
	}
}

func TestHwidTopUsersReturnsRankedPage(t *testing.T) {
	e := newProbeEnv(t)
	if err := e.db.Exec("INSERT INTO users (id, username, short_uuid) VALUES (10, 'first', 'first'), (20, 'second', 'second')").Error; err != nil {
		t.Fatal(err)
	}
	if err := e.db.Exec("INSERT INTO hwid_user_devices (hwid, user_id) VALUES ('a', 10), ('b', 20), ('c', 20)").Error; err != nil {
		t.Fatal(err)
	}
	for _, page := range []struct {
		start int
		id    int
		count int
	}{
		{start: 0, id: 20, count: 2},
		{start: 1, id: 10, count: 1},
	} {
		status, data := e.do(http.MethodGet, "/api/hwid/devices/top-users?size=1&start="+strconv.Itoa(page.start), nil)
		if status != http.StatusOK {
			t.Fatalf("page %d returned HTTP %d: %s", page.start, status, data)
		}
		var body struct {
			Response struct {
				Total int `json:"total"`
				Users []struct {
					ID           int `json:"id"`
					DevicesCount int `json:"devicesCount"`
				} `json:"users"`
			} `json:"response"`
		}
		if err := json.Unmarshal(data, &body); err != nil {
			t.Fatal(err)
		}
		if got := body.Response; got.Total != 2 ||
			len(got.Users) != 1 || got.Users[0].ID != page.id || got.Users[0].DevicesCount != page.count {
			t.Fatalf("page %d returned %+v", page.start, got)
		}
	}
}
