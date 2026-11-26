package refmanifest

import (
	"encoding/json"
	"testing"
)

// As for identity.Thumbnail: only the spelling differs from the default decoder.
func TestSafetyControlsUnmarshal(t *testing.T) {
	t.Parallel()
	cpu, mem := 3, 4
	s := SafetyControls{MaxCPUPct: &cpu, MaxMemMB: &mem}
	if err := json.Unmarshal([]byte(`{"maxMemMB": null, "orientation": ["any"]}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.MaxCPUPct == nil || *s.MaxCPUPct != 3 || s.MaxMemMB != nil || len(s.Orientation) != 1 {
		t.Fatalf("got %+v", s)
	}
	if err := json.Unmarshal([]byte(`{"maxCpuPct": 5e1, "maxMemMB": 1024.0}`), &s); err != nil {
		t.Fatal(err)
	}
	if *s.MaxCPUPct != 50 || s.MaxMemMB == nil || *s.MaxMemMB != 1024 {
		t.Fatalf("got %+v", s)
	}
	if err := json.Unmarshal([]byte(`{"maxCpuPct": "50"}`), &SafetyControls{}); err == nil {
		t.Fatal("string decoded")
	}
}
