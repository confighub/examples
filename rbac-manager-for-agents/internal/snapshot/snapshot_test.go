// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package snapshot

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestIsCanonicalSpace(t *testing.T) {
	cases := []struct {
		labels map[string]string
		want   bool
	}{
		{nil, false},
		{map[string]string{"Variant": "base"}, true},
		{map[string]string{"Variant": "prod"}, false},
		{map[string]string{"role": "base"}, true},
		{map[string]string{"role": "policy"}, true},
		{map[string]string{"role": "app"}, false},
		{map[string]string{"Environment": "prod"}, false},
	}
	for _, c := range cases {
		if got := isCanonicalSpace(c.labels); got != c.want {
			t.Errorf("isCanonicalSpace(%v) = %v, want %v", c.labels, got, c.want)
		}
	}
}

func TestUnitMetaState(t *testing.T) {
	cases := []struct {
		name      string
		u         UnitMeta
		gated     bool
		unapplied bool
	}{
		{"never applied", UnitMeta{HeadRevisionNum: 3, LiveRevisionNum: 0}, false, true},
		{"behind", UnitMeta{HeadRevisionNum: 5, LiveRevisionNum: 4}, false, true},
		{"in sync", UnitMeta{HeadRevisionNum: 5, LiveRevisionNum: 5}, false, false},
		{"gated", UnitMeta{HeadRevisionNum: 5, LiveRevisionNum: 5, GateCount: 2}, true, false},
	}
	for _, c := range cases {
		if got := c.u.Gated(); got != c.gated {
			t.Errorf("%s: Gated() = %v, want %v", c.name, got, c.gated)
		}
		if got := c.u.Unapplied(); got != c.unapplied {
			t.Errorf("%s: Unapplied() = %v, want %v", c.name, got, c.unapplied)
		}
	}
}

func TestDecodeResourceList(t *testing.T) {
	if got := decodeResourceList(""); got != nil {
		t.Errorf("empty input = %v, want nil", got)
	}
	if got := decodeResourceList("not base64!!!"); got != nil {
		t.Errorf("bad base64 = %v, want nil", got)
	}
	list := []rawResource{{ResourceType: "rbac.authorization.k8s.io/v1/Role", ResourceName: "ns/r", ResourceBody: "{}"}}
	raw, _ := json.Marshal(list)
	encoded := base64.StdEncoding.EncodeToString(raw)
	got := decodeResourceList(encoded)
	if len(got) != 1 || got[0].ResourceName != "ns/r" {
		t.Errorf("decodeResourceList round-trip = %+v", got)
	}
}
