package pkgdb

import "testing"

func TestAPKCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.1.20-r5", "1.1.20-r6", -1}, // the musl CVE-2020-28928 case
		{"1.1.20-r6", "1.1.20-r5", 1},
		{"1.1.20-r5", "1.1.20-r5", 0},
		{"2.10.4-r2", "2.10.7-r0", -1}, // apk-tools CVE-2021-36159
		{"1.16.0-r0", "1.16.1-r2", -1}, // nginx fix
		{"1.2.11-r3", "1.2.11-r3", 0},
		{"7.61.0-r0", "7.64.0-r3", -1}, // curl
		{"1.0.0", "1.0.0_rc1", 1},      // release outranks pre-release
		{"1.0.0_alpha", "1.0.0_beta", -1},
		{"3.10", "3.9", 1}, // numeric, not lexical
	}
	for _, c := range cases {
		if got := apkCompare(c.a, c.b); got != c.want {
			t.Errorf("apkCompare(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestDpkgCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.3-1", "1.2.3-2", -1},
		{"1.2.3-1", "1.2.3-1", 0},
		{"2:1.0-1", "1:9.9-9", 1}, // epoch dominates
		{"1.0-1", "1.0~rc1-1", 1}, // ~ sorts before everything
		{"1.0~beta-1", "1.0-1", -1},
		{"1.10-1", "1.9-1", 1},                        // numeric segment compare
		{"7.68.0-1ubuntu2", "7.68.0-1ubuntu2.20", -1}, // curl debian/ubuntu
	}
	for _, c := range cases {
		if got := dpkgCompare(c.a, c.b); got != c.want {
			t.Errorf("dpkgCompare(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParseAPK(t *testing.T) {
	db := "C:Q1x\nP:musl\nV:1.1.20-r5\nA:x86_64\n\nP:busybox\nV:1.30.1-r3\nA:x86_64\n\n"
	pkgs := parseAPK([]byte(db))
	if len(pkgs) != 2 {
		t.Fatalf("got %d packages, want 2", len(pkgs))
	}
	if pkgs[0].Name != "musl" || pkgs[0].Version != "1.1.20-r5" || pkgs[0].Type != "apk" {
		t.Errorf("unexpected first package: %+v", pkgs[0])
	}
}

func TestParseDpkg(t *testing.T) {
	db := "Package: libc6\nStatus: install ok installed\nVersion: 2.28-10\nArchitecture: amd64\n\n" +
		"Package: removed-pkg\nStatus: deinstall ok config-files\nVersion: 1.0\n\n"
	pkgs := parseDpkg([]byte(db))
	if len(pkgs) != 1 {
		t.Fatalf("got %d packages, want 1 (only installed)", len(pkgs))
	}
	if pkgs[0].Name != "libc6" || pkgs[0].Version != "2.28-10" || pkgs[0].Type != "deb" {
		t.Errorf("unexpected package: %+v", pkgs[0])
	}
}
