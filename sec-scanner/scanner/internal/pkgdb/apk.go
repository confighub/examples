package pkgdb

import (
	"regexp"
	"strconv"
	"strings"
)

// parseAPK reads /lib/apk/db/installed. Records are blank-line-separated; each
// line is "<K>:<value>" with P=name, V=version, A=arch.
func parseAPK(b []byte) []Package {
	var out []Package
	var cur Package
	flush := func() {
		if cur.Name != "" && cur.Version != "" {
			cur.Type = "apk"
			out = append(out, cur)
		}
		cur = Package{}
	}
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" {
			flush()
			continue
		}
		if len(line) < 2 || line[1] != ':' {
			continue
		}
		val := line[2:]
		switch line[0] {
		case 'P':
			cur.Name = val
		case 'V':
			cur.Version = val
		case 'A':
			cur.Arch = val
		}
	}
	flush()
	return out
}

var apkRev = regexp.MustCompile(`-r(\d+)$`)

// preRank orders apk pre-release suffixes below a plain release.
var preRank = map[string]int{
	"_alpha": 0, "_beta": 1, "_pre": 2, "_rc": 3,
}

// apkCompare compares two apk versions. Practical subset of apk's vercmp:
// optional epoch, dotted numeric main version (segments may carry a trailing
// letter), pre-release suffixes (_alpha/_beta/_pre/_rc sort below release),
// and the -rN package revision.
func apkCompare(a, b string) int {
	ea, a := apkEpoch(a)
	eb, b := apkEpoch(b)
	if ea != eb {
		return sign(ea - eb)
	}
	ra, baseA := apkSplitRev(a)
	rb, baseB := apkSplitRev(b)
	if c := apkBaseCompare(baseA, baseB); c != 0 {
		return c
	}
	return sign(ra - rb)
}

func apkEpoch(v string) (int, string) {
	if i := strings.IndexByte(v, ':'); i >= 0 {
		if n, err := strconv.Atoi(v[:i]); err == nil {
			return n, v[i+1:]
		}
	}
	return 0, v
}

func apkSplitRev(v string) (int, string) {
	if m := apkRev.FindStringSubmatch(v); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n, v[:len(v)-len(m[0])]
	}
	return 0, v
}

func apkBaseCompare(a, b string) int {
	mainA, preA := apkSplitPre(a)
	mainB, preB := apkSplitPre(b)
	if c := apkMainCompare(mainA, mainB); c != 0 {
		return c
	}
	// A version with no pre-release outranks one with a pre-release.
	switch {
	case preA == "" && preB == "":
		return 0
	case preA == "":
		return 1
	case preB == "":
		return -1
	}
	ra, oka := preRank[preKeyword(preA)]
	rb, okb := preRank[preKeyword(preB)]
	if oka && okb && ra != rb {
		return sign(ra - rb)
	}
	return sign(strings.Compare(preA, preB))
}

func apkSplitPre(v string) (string, string) {
	if i := strings.IndexByte(v, '_'); i >= 0 {
		return v[:i], v[i:]
	}
	return v, ""
}

func preKeyword(pre string) string {
	for k := range preRank {
		if strings.HasPrefix(pre, k) {
			return k
		}
	}
	return pre
}

func apkMainCompare(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y string
		if i < len(as) {
			x = as[i]
		}
		if i < len(bs) {
			y = bs[i]
		}
		if c := apkSegCompare(x, y); c != 0 {
			return c
		}
	}
	return 0
}

// apkSegCompare compares "20" vs "20a": leading integer first, then any letter.
func apkSegCompare(x, y string) int {
	nx, sx := splitNumPrefix(x)
	ny, sy := splitNumPrefix(y)
	if nx != ny {
		return sign(nx - ny)
	}
	return sign(strings.Compare(sx, sy))
}

func splitNumPrefix(s string) (int, string) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	n := 0
	if i > 0 {
		n, _ = strconv.Atoi(s[:i])
	}
	return n, s[i:]
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}
