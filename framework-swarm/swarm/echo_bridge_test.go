package swarm_test

import (
	"os"
	"path/filepath"
	"testing"

	swarm "github.com/remora-go/framework-swarm/swarm"
)

// fixture returns the path to a fixture file relative to the module root.
func fixture(t *testing.T, parts ...string) string {
	t.Helper()
	// Walk up from the test directory to find the go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			all := []string{dir}
			all = append(all, parts...)
			return filepath.Join(all...)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod from %s", dir)
		}
		dir = parent
	}
}

func TestZonesFromEchoFile(t *testing.T) {
	path := fixture(t, "fixtures", "invoice-reconciliation", "frameworkecho.json")
	zones, err := swarm.ZonesFromEchoFile(path)
	if err != nil {
		t.Fatalf("ZonesFromEchoFile: %v", err)
	}

	// We expect 5 OPPORTUNITY nodes
	if len(zones) != 5 {
		t.Errorf("got %d zones, want 5", len(zones))
	}

	// Zones should be sorted by PainWeight descending
	for i := 1; i < len(zones); i++ {
		if zones[i].PainWeight > zones[i-1].PainWeight {
			t.Errorf("zones not sorted by PainWeight: zones[%d]=%.2f > zones[%d]=%.2f",
				i, zones[i].PainWeight, i-1, zones[i-1].PainWeight)
		}
	}

	// All zones should have non-empty IDs and names
	for _, z := range zones {
		if z.ID == "" {
			t.Errorf("zone has empty ID: %+v", z)
		}
		if z.Name == "" {
			t.Errorf("zone has empty Name: %+v", z)
		}
		if z.PainWeight <= 0 || z.PainWeight > 1 {
			t.Errorf("zone %s has invalid PainWeight %.2f", z.ID, z.PainWeight)
		}
	}
}

func TestZonesFromEchoTree_NoOpportunities(t *testing.T) {
	// A tree with only PAIN nodes (no OPPORTUNITY) should fall back to pains
	echoJSON := []byte(`{
		"project_id": "test",
		"nodes": {
			"ax_001": {"id":"ax_001","layer":0,"type":"AXIOM","title":"Test axiom","status":"VALIDATED","confidence":100},
			"pn_001": {"id":"pn_001","layer":3,"type":"PAIN","title":"A real pain","status":"VALIDATED","confidence":85,"parent_id":"ax_001"},
			"pn_002": {"id":"pn_002","layer":3,"type":"PAIN","title":"Another pain","status":"VALIDATED","confidence":70,"parent_id":"ax_001"}
		}
	}`)
	zones, err := swarm.ZonesFromEchoTree(echoJSON)
	if err != nil {
		t.Fatalf("ZonesFromEchoTree (pain fallback): %v", err)
	}
	if len(zones) != 2 {
		t.Errorf("got %d zones from pains, want 2", len(zones))
	}
}

func TestZonesFromEchoTree_SkipsRejected(t *testing.T) {
	echoJSON := []byte(`{
		"project_id": "test",
		"nodes": {
			"op_001": {"id":"op_001","layer":4,"type":"OPPORTUNITY","title":"Good opportunity","status":"VALIDATED","confidence":90},
			"op_002": {"id":"op_002","layer":4,"type":"OPPORTUNITY","title":"Rejected one","status":"REJECTED","confidence":60}
		}
	}`)
	zones, err := swarm.ZonesFromEchoTree(echoJSON)
	if err != nil {
		t.Fatalf("ZonesFromEchoTree: %v", err)
	}
	if len(zones) != 1 {
		t.Errorf("got %d zones, want 1 (REJECTED should be skipped)", len(zones))
	}
	if zones[0].ID == "rejected_one" {
		t.Errorf("rejected node should have been excluded")
	}
}

func TestIdealFlowFromAlfaFile(t *testing.T) {
	path := fixture(t, "fixtures", "invoice-reconciliation", "alfa_spec.json")
	flow, err := swarm.IdealFlowFromAlfaFile(path)
	if err != nil {
		t.Fatalf("IdealFlowFromAlfaFile: %v", err)
	}

	if flow.Intent == "" {
		t.Error("IdealFlow.Intent should not be empty")
	}
	if len(flow.CriticalPath) != 5 {
		t.Errorf("got %d CriticalPath items, want 5", len(flow.CriticalPath))
	}
	if len(flow.Rules) != 5 {
		t.Errorf("got %d rules, want 5", len(flow.Rules))
	}
	if len(flow.CriticalVars) != 8 {
		t.Errorf("got %d CriticalVars, want 8", len(flow.CriticalVars))
	}

	// All critical path items should be normalised (no uppercase, no spaces)
	for _, step := range flow.CriticalPath {
		for _, r := range step {
			if r >= 'A' && r <= 'Z' {
				t.Errorf("CriticalPath item %q contains uppercase letter", step)
			}
			if r == ' ' {
				t.Errorf("CriticalPath item %q contains space", step)
			}
		}
	}
}

func TestIdealFlowForZones(t *testing.T) {
	echoPath := fixture(t, "fixtures", "invoice-reconciliation", "frameworkecho.json")
	alfaPath := fixture(t, "fixtures", "invoice-reconciliation", "alfa_spec.json")

	zones, err := swarm.ZonesFromEchoFile(echoPath)
	if err != nil {
		t.Fatalf("ZonesFromEchoFile: %v", err)
	}
	base, err := swarm.IdealFlowFromAlfaFile(alfaPath)
	if err != nil {
		t.Fatalf("IdealFlowFromAlfaFile: %v", err)
	}

	flow := swarm.IdealFlowForZones(base, zones)

	// CriticalPath should now be zone IDs, not Alfa step names
	if len(flow.CriticalPath) != len(zones) {
		t.Errorf("IdealFlowForZones: got %d path items, want %d (one per zone)",
			len(flow.CriticalPath), len(zones))
	}
	for i, step := range flow.CriticalPath {
		if step != zones[i].ID {
			t.Errorf("CriticalPath[%d] = %q, want zone ID %q", i, step, zones[i].ID)
		}
	}

	// Rules and vars should be preserved from the base flow
	if len(flow.Rules) != len(base.Rules) {
		t.Errorf("rules changed: got %d, want %d", len(flow.Rules), len(base.Rules))
	}
	if len(flow.CriticalVars) != len(base.CriticalVars) {
		t.Errorf("critical vars changed: got %d, want %d", len(flow.CriticalVars), len(base.CriticalVars))
	}
}

func TestNormaliseZoneID(t *testing.T) {
	// Test through ZonesFromEchoTree — the normalisation happens inside ZonesFromEchoTree
	echoJSON := []byte(`{
		"project_id": "test",
		"nodes": {
			"op_001": {"id":"op_001","layer":4,"type":"OPPORTUNITY","title":"Validar campos obligatorios al ingreso","status":"VALIDATED","confidence":95},
			"op_002": {"id":"op_002","layer":4,"type":"OPPORTUNITY","title":"Cruzar con ERP y escalár proveedores","status":"VALIDATED","confidence":85}
		}
	}`)
	zones, err := swarm.ZonesFromEchoTree(echoJSON)
	if err != nil {
		t.Fatalf("ZonesFromEchoTree: %v", err)
	}
	for _, z := range zones {
		for _, r := range z.ID {
			if r >= 'A' && r <= 'Z' {
				t.Errorf("zone ID %q contains uppercase", z.ID)
			}
			if r == ' ' || r == '-' {
				t.Errorf("zone ID %q contains space or hyphen", z.ID)
			}
		}
		// Must not start or end with underscore
		if len(z.ID) > 0 && (z.ID[0] == '_' || z.ID[len(z.ID)-1] == '_') {
			t.Errorf("zone ID %q starts or ends with underscore", z.ID)
		}
	}
}
