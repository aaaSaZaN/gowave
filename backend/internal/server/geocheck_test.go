package server

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"gowave/internal/database"
)

func TestGeocheckPollResults(t *testing.T) {
	e := newProbeEnv(t)
	const nodeUUID = "55555555-5555-4555-8555-555555555555"
	const path = "/api/connections/geocheck/"
	if err := e.db.Create(&database.Node{UUID: nodeUUID, Name: "GeoCheck node", Address: "example.com"}).Error; err != nil {
		t.Fatal(err)
	}
	jobID := e.mustCreate(path+nodeUUID, map[string]string{}, "jobId")
	type pollResponse struct {
		IsCompleted bool           `json:"isCompleted"`
		IsFailed    bool           `json:"isFailed"`
		Result      map[string]any `json:"result"`
	}
	poll := func() pollResponse {
		t.Helper()
		status, body := e.do(http.MethodGet, path+jobID, nil)
		var envelope struct {
			Response pollResponse `json:"response"`
		}
		if err := json.Unmarshal(body, &envelope); status != http.StatusOK || err != nil {
			t.Fatalf("poll returned %d: %s (%v)", status, body, err)
		}
		return envelope.Response
	}
	if result := poll(); !reflect.DeepEqual(result, pollResponse{}) {
		t.Fatalf("unexpected pending job: %#v", result)
	}

	job := e.db.Model(&database.Job{}).Where("id = ?", jobID)
	image := map[string]any{"data": "AA==", "media_type": "image/png"}
	legacy := map[string]any{
		"success": true, "nodeUuid": nodeUUID, "image": image,
		"rawReport": map[string]any{"image": image, "connectivity": "report"},
	}
	if err := job.Updates(map[string]any{"state": "completed", "result": string(mustJSON(legacy))}).Error; err != nil {
		t.Fatal(err)
	}
	wantCompleted := pollResponse{IsCompleted: true, Result: map[string]any{
		"success": true, "nodeUuid": nodeUUID, "image": image,
		"rawReport": map[string]any{"connectivity": "report"}, "message": nil,
	}}
	if result := poll(); !reflect.DeepEqual(result, wantCompleted) {
		t.Fatalf("unexpected completed job: %#v", result)
	}

	if err := job.Updates(map[string]any{"state": "failed", "last_error": "node disabled"}).Error; err != nil {
		t.Fatal(err)
	}
	wantFailed := pollResponse{IsFailed: true, Result: map[string]any{
		"success": false, "nodeUuid": nodeUUID, "image": nil, "rawReport": nil, "message": "node disabled",
	}}
	if result := poll(); !reflect.DeepEqual(result, wantFailed) {
		t.Fatalf("unexpected failed job: %#v", result)
	}
}
