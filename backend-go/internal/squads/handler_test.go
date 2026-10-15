package squads

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http/httptest"
	"remnawave-go/internal/database"
	"remnawave-go/internal/testdb"
	"testing"
	"time"
)

func TestExternalSquadRemovalPreservesOtherMemberships(t *testing.T) {
	db := testdb.Open(t)
	router := chi.NewRouter()
	router.Delete("/api/external-squads/{uuid}/bulk-actions/remove-many-users", NewHandler(db).SquadBulkAction)
	first, second := uuid.NewString(), uuid.NewString()
	for _, squad := range []database.ExternalSquad{{UUID: first, Name: "first"}, {UUID: second, Name: "second"}} {
		if err := db.Create(&squad).Error; err != nil {
			t.Fatal(err)
		}
	}
	var ids []uint64
	for i, squadID := range []*string{&first, &second, nil} {
		user := database.User{Username: fmt.Sprintf("user%d", i), ShortUUID: fmt.Sprintf("user%d", i), ExpireAt: time.Now().UTC().Add(time.Hour), ExternalSquadUUID: squadID}
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		ids = append(ids, user.ID)
	}
	payload, err := json.Marshal(map[string]interface{}{"userIds": ids})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/external-squads/"+first+"/bulk-actions/remove-many-users", bytes.NewReader(payload)))
	status, body := w.Code, w.Body.String()
	if status >= 300 {
		t.Fatalf("remove: %d %s", status, body)
	}
	for i, want := range []*string{nil, &second, nil} {
		var user database.User
		if err := db.First(&user, ids[i]).Error; err != nil {
			t.Fatal(err)
		}
		if (want == nil) != (user.ExternalSquadUUID == nil) || (want != nil && user.ExternalSquadUUID != nil && *want != *user.ExternalSquadUUID) {
			t.Errorf("unexpected squad for user %d: %v", i, user.ExternalSquadUUID)
		}
	}
}
