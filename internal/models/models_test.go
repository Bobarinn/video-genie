package models

import (
	"encoding/json"
	"testing"
)

func TestJSONBMarshal(t *testing.T) {
	j := JSONB{
		"color_palette": []string{"red", "blue"},
		"mood":          "dramatic",
	}

	data, err := j.Value()
	if err != nil {
		t.Fatalf("failed to marshal JSONB: %v", err)
	}

	if data == nil {
		t.Fatal("expected non-nil data")
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(data.([]byte), &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result["mood"] != "dramatic" {
		t.Errorf("expected mood=dramatic, got %v", result["mood"])
	}
}

func TestJSONBScan(t *testing.T) {
	jsonData := []byte(`{"color": "blue", "size": 10}`)

	var j JSONB
	if err := j.Scan(jsonData); err != nil {
		t.Fatalf("failed to scan: %v", err)
	}

	if j["color"] != "blue" {
		t.Errorf("expected color=blue, got %v", j["color"])
	}

	if j["size"].(float64) != 10 {
		t.Errorf("expected size=10, got %v", j["size"])
	}
}

func TestProjectStatus(t *testing.T) {
	statuses := []ProjectStatus{
		ProjectStatusQueued,
		ProjectStatusPlanning,
		ProjectStatusGenerating,
		ProjectStatusRendering,
		ProjectStatusCompleted,
		ProjectStatusFailed,
	}

	for _, status := range statuses {
		if status == "" {
			t.Errorf("empty status found")
		}
	}
}

func TestClipStatus(t *testing.T) {
	statuses := []ClipStatus{
		ClipStatusPending,
		ClipStatusVoiced,
		ClipStatusImaged,
		ClipStatusRendered,
		ClipStatusFailed,
	}

	for _, status := range statuses {
		if status == "" {
			t.Errorf("empty status found")
		}
	}
}
