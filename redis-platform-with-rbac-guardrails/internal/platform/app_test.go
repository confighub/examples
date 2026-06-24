// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package platform

import "testing"

func TestPaymentsPlatformFixture(t *testing.T) {
	model, err := Load("../../fixtures/payments-platform.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(model); err != nil {
		t.Fatal(err)
	}
	snapshot := Snapshot(model)
	if snapshot["unitCount"] != 7 {
		t.Fatalf("unitCount = %v, want 7", snapshot["unitCount"])
	}
	findings := Findings(model)["findings"].([]Finding)
	if findings[0].BetterPlace == "" {
		t.Fatal("finding should tell the agent where the fix belongs")
	}
}
