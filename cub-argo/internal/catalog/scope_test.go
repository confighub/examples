package catalog

import (
	"strings"
	"testing"
)

// The scope of redis's default verdict names auth.existingSecret. A caller
// that sets it is out of that verdict's scope, and this says so.
func TestSetValueInScopeIsReported(t *testing.T) {
	var e Entry
	for _, x := range Find("redis", "0.34.11") {
		if x.Base == "default" {
			e = x
		}
	}
	if e.Lane == "" {
		t.Skip("redis default not in the snapshot")
	}
	if len(e.PathsIn()) == 0 {
		t.Fatalf("the scope names auth.existingSecret; got no paths from %v", e.MovedBy)
	}
	c := e.CheckScope([]string{"auth.existingSecret", "replicaCount"})
	if len(c.Touched) != 1 || !strings.Contains(c.Touched[0], "auth.existingSecret") {
		t.Errorf("want auth.existingSecret reported as touching the scope, got %v", c.Touched)
	}
	lines := c.Report("redis")
	if len(lines) == 0 || !strings.Contains(lines[0], "does not carry") {
		t.Errorf("want the consequence spelled out, got %v", lines)
	}
}

// Values the scope does not name are not reported, so the check does not cry
// wolf on every overlay.
func TestUnrelatedValuesAreNotReported(t *testing.T) {
	var e Entry
	for _, x := range Find("redis", "0.34.11") {
		if x.Base == "default" {
			e = x
		}
	}
	if e.Lane == "" {
		t.Skip("redis default not in the snapshot")
	}
	c := e.CheckScope([]string{"replicaCount", "resources.limits.memory"})
	if len(c.Touched) != 0 {
		t.Errorf("none of these are in the verdict's scope, got %v", c.Touched)
	}
}

// Scope written as prose cannot be checked, and saying nothing would read as
// agreement. It is reported as the reader's to check.
func TestUnparsableScopeIsHandedBack(t *testing.T) {
	e := Entry{Chart: "x", MovedBy: []struct {
		Values string `json:"values"`
		Effect string `json:"effect"`
	}{{Values: "authentication or TLS enabled", Effect: "more objects render"}}}
	c := e.CheckScope([]string{"replicaCount"})
	if len(c.Unparsed) != 1 {
		t.Fatalf("prose scope should be handed back, got %v", c.Unparsed)
	}
	lines := c.Report("x")
	if len(lines) != 1 || !strings.Contains(lines[0], "yourself") {
		t.Errorf("want it said plainly that this one is the reader's, got %v", lines)
	}
}
