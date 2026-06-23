// Package pkgdb parses OS package databases out of a container filesystem and
// knows how to compare versions within each packaging ecosystem.
package pkgdb

// Package is one installed OS package.
type Package struct {
	Name    string
	Version string
	Type    string // "apk" | "deb" | "rpm"
	Arch    string
}

// Well-known paths inside a flattened image filesystem (no leading slash, the
// way the layer tar entries are named).
const (
	PathAPK          = "lib/apk/db/installed"
	PathDpkg         = "var/lib/dpkg/status"
	PathOSRelease    = "etc/os-release"
	PathOSReleaseLib = "usr/lib/os-release" // /etc/os-release is often a symlink to this
)

// WantPaths is the set of files the image exporter must capture.
func WantPaths() []string {
	return []string{PathAPK, PathDpkg, PathOSRelease, PathOSReleaseLib}
}

// Parse picks the right format based on which DB file is present and returns the
// installed packages plus their ecosystem type.
func Parse(files map[string][]byte) []Package {
	if b, ok := files[PathAPK]; ok && len(b) > 0 {
		return parseAPK(b)
	}
	if b, ok := files[PathDpkg]; ok && len(b) > 0 {
		return parseDpkg(b)
	}
	return nil
}

// Compare returns -1, 0, or 1 comparing version a to b within the given
// package type's version ordering. Unknown types fall back to a generic
// segment compare.
func Compare(typ, a, b string) int {
	switch typ {
	case "apk":
		return apkCompare(a, b)
	case "deb":
		return dpkgCompare(a, b)
	default:
		return genericCompare(a, b)
	}
}
