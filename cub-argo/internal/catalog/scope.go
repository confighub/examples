package catalog

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// A verdict's scope is written for a person: "authentication or TLS enabled",
// "certificates supplied from outside the render". Some of it names a values
// path, though, and where it does and the caller has set that path, that is
// worth saying out loud.
//
// This is a hint, never a gate. It finds what it can name and says so; a scope
// it cannot parse is reported as the reader's to check.

// valuesPath matches something that looks like a Helm values key:
// redis-ha.enabled, deletionPolicy, auth.existingSecret.
var valuesPath = regexp.MustCompile(`\b[a-z][A-Za-z0-9_-]*(?:\.[A-Za-z0-9_-]+)+\b|\b[a-z][a-z0-9]*[A-Z][A-Za-z0-9]*\b`)

// PathsIn are the values paths a verdict's scope names, which may be none.
func (e Entry) PathsIn() []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range e.MovedBy {
		for _, p := range valuesPath.FindAllString(m.Values, -1) {
			// Sentence words that happen to match, and file names.
			if strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".io") || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// ScopeCheck is what a caller's own values have to say about a verdict's scope.
type ScopeCheck struct {
	// Touched are the values paths the caller sets that the scope also names.
	// Each one is a reason to doubt that the verdict carries.
	Touched []string
	// Unparsed is the scope this could not turn into paths, which stays the
	// reader's to check.
	Unparsed []string
}

// CheckScope compares the values a caller sets with what the verdict says
// would move it. Set values are dotted paths, as valuesInline flattens to.
func (e Entry) CheckScope(set []string) ScopeCheck {
	var c ScopeCheck
	named := e.PathsIn()
	has := map[string]bool{}
	for _, p := range named {
		has[strings.ToLower(p)] = true
	}
	for _, s := range set {
		l := strings.ToLower(s)
		for p := range has {
			if l == p || strings.HasPrefix(l, p+".") || strings.HasPrefix(p, l+".") {
				c.Touched = appendOnce(c.Touched, fmt.Sprintf("%s (the verdict names %s)", s, p))
			}
		}
	}
	for _, m := range e.MovedBy {
		if len(valuesPath.FindAllString(m.Values, -1)) == 0 {
			c.Unparsed = append(c.Unparsed, m.Values)
		}
	}
	sort.Strings(c.Touched)
	return c
}

func appendOnce(l []string, s string) []string {
	for _, x := range l {
		if x == s {
			return l
		}
	}
	return append(l, s)
}

// Report says what the check found, in lines a person reads.
func (c ScopeCheck) Report(chart string) []string {
	var out []string
	for _, t := range c.Touched {
		out = append(out, fmt.Sprintf(
			"%s sets %s, which the Catalog's verdict says takes a variant out of its scope. The verdict does not carry here until that is checked",
			chart, t))
	}
	if len(c.Unparsed) > 0 {
		out = append(out, fmt.Sprintf(
			"%s: the rest of the verdict's scope is written for a reader, not a check — %s. Compare it with your own values yourself",
			chart, strings.Join(c.Unparsed, "; ")))
	}
	return out
}
