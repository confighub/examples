package pkgdb

import (
	"strconv"
	"strings"
)

// parseDpkg reads /var/lib/dpkg/status. Stanzas are blank-line separated;
// only entries whose Status contains "installed" are kept.
func parseDpkg(b []byte) []Package {
	var out []Package
	var cur Package
	installed := false
	flush := func() {
		if installed && cur.Name != "" && cur.Version != "" {
			cur.Type = "deb"
			out = append(out, cur)
		}
		cur = Package{}
		installed = false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" {
			flush()
			continue
		}
		k, v, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		switch k {
		case "Package":
			cur.Name = v
		case "Version":
			cur.Version = v
		case "Architecture":
			cur.Arch = v
		case "Status":
			installed = strings.Contains(v, "installed")
		}
	}
	flush()
	return out
}

// dpkgCompare implements Debian version comparison (epoch : upstream - revision,
// each part compared with the dpkg verrevcmp algorithm).
func dpkgCompare(a, b string) int {
	ea, ua, ra := dpkgSplit(a)
	eb, ub, rb := dpkgSplit(b)
	if ea != eb {
		return sign(ea - eb)
	}
	if c := verrevcmp(ua, ub); c != 0 {
		return c
	}
	return verrevcmp(ra, rb)
}

func dpkgSplit(v string) (epoch int, upstream, revision string) {
	if i := strings.IndexByte(v, ':'); i >= 0 {
		if n, err := strconv.Atoi(v[:i]); err == nil {
			epoch = n
			v = v[i+1:]
		}
	}
	if i := strings.LastIndexByte(v, '-'); i >= 0 {
		upstream, revision = v[:i], v[i+1:]
	} else {
		upstream, revision = v, "0"
	}
	return
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// order assigns the dpkg sort weight to a non-digit byte: '~' sorts before
// everything (even end-of-string), letters before other punctuation.
func order(c byte) int {
	switch {
	case c == 0:
		return 0
	case isDigit(c):
		return 0
	case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
		return int(c)
	case c == '~':
		return -1
	default:
		return int(c) + 256
	}
}

func verrevcmp(a, b string) int {
	ia, ib := 0, 0
	for ia < len(a) || ib < len(b) {
		firstDiff := 0
		for (ia < len(a) && !isDigit(a[ia])) || (ib < len(b) && !isDigit(b[ib])) {
			var ac, bc byte
			if ia < len(a) {
				ac = a[ia]
			}
			if ib < len(b) {
				bc = b[ib]
			}
			if order(ac) != order(bc) {
				return sign(order(ac) - order(bc))
			}
			ia++
			ib++
		}
		for ia < len(a) && a[ia] == '0' {
			ia++
		}
		for ib < len(b) && b[ib] == '0' {
			ib++
		}
		for ia < len(a) && isDigit(a[ia]) && ib < len(b) && isDigit(b[ib]) {
			if firstDiff == 0 {
				firstDiff = int(a[ia]) - int(b[ib])
			}
			ia++
			ib++
		}
		if ia < len(a) && isDigit(a[ia]) {
			return 1
		}
		if ib < len(b) && isDigit(b[ib]) {
			return -1
		}
		if firstDiff != 0 {
			return sign(firstDiff)
		}
	}
	return 0
}

// genericCompare is a last-resort dotted/dashed numeric-aware compare.
func genericCompare(a, b string) int {
	split := func(s string) []string {
		return strings.FieldsFunc(s, func(r rune) bool { return r == '.' || r == '-' || r == '_' })
	}
	as, bs := split(a), split(b)
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y string
		if i < len(as) {
			x = as[i]
		}
		if i < len(bs) {
			y = bs[i]
		}
		nx, errx := strconv.Atoi(x)
		ny, erry := strconv.Atoi(y)
		if errx == nil && erry == nil {
			if nx != ny {
				return sign(nx - ny)
			}
			continue
		}
		if c := strings.Compare(x, y); c != 0 {
			return sign(c)
		}
	}
	return 0
}
