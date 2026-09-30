package flux

import (
	"fmt"
	"strings"
)

// Render writes the plan as text for a person to read.
func Render(p *Plan) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }

	variants := 0
	for _, c := range p.Components {
		for _, st := range c.Stages {
			variants += len(st.Variants)
		}
	}
	w("Flux fleet: %d clusters, %d layers, %d variants\n", len(p.Clusters), len(p.Components), variants)
	if len(p.Inputs.Skipped) > 0 {
		w("Read %d objects; skipped %d files that are not Kubernetes YAML\n", p.Inputs.Objects, len(p.Inputs.Skipped))
	} else {
		w("Read %d objects\n", p.Inputs.Objects)
	}

	var cl []string
	for _, c := range p.Clusters {
		cl = append(cl, fmt.Sprintf("%s (clusters/%s)", c.Name, c.Dir))
	}
	w("\nClusters, one stage each, in order: %s\n", strings.Join(cl, ", "))
	for _, c := range p.Clusters {
		if c.LayersSpace != "" {
			w("  %s is handed over: its layers are Units in %s, read by its ConfigHub root, and this plan leaves them there\n", c.Name, c.LayersSpace)
		}
	}

	if len(p.Order) > 0 {
		var steps []string
		for _, s := range p.Order {
			steps = append(steps, strings.Join(s, ", "))
		}
		w("Reconcile order (dependsOn): %s\n", strings.Join(steps, " -> "))
	}

	for _, c := range p.Components {
		w("\n%s", c.Name)
		if len(c.DependsOn) > 0 {
			w("  (after %s)", strings.Join(c.DependsOn, ", "))
		}
		w("\n  base     %s  from %s\n", c.Base, c.BaseDir)
		for _, h := range c.HelmReleases {
			w("  helm     %s\n", h)
		}
		for _, st := range c.Stages {
			w("  stage %s\n", st.Name)
			for _, v := range st.Variants {
				w("    %-11s variant %s  ->  Target %s\n", v.Cluster, v.Space, v.Target)
				w("    %-11s Kustomization %s, path %s\n", "", v.Kustomization, v.Path)
				for _, d := range v.Departures {
					w("    %-11s   %s\n", "", d)
				}
			}
		}
		for _, f := range c.InFlight {
			w("  in flight  %s\n", f)
		}
		for _, n := range c.Notes {
			w("  note     %s\n", n)
		}
	}

	section := func(title string, lines []string) {
		if len(lines) == 0 {
			return
		}
		w("\n%s\n", title)
		for _, l := range lines {
			w("  - %s\n", l)
		}
	}
	var src []string
	for _, s := range p.Sources {
		line := fmt.Sprintf("%s %s  %s  (%s)", s.Kind, s.Name, s.URL, s.File)
		if len(s.Branches) > 0 {
			var parts []string
			for _, c := range p.Clusters {
				if br, ok := s.Branches[c.Name]; ok {
					parts = append(parts, c.Name+" "+br)
				}
			}
			line += "\n      branch: " + strings.Join(parts, ", ")
		}
		src = append(src, line)
	}
	section("Sources", src)
	section("Writes to Git by itself", p.Automation)
	section("Not in Git", p.NotInGit)
	section("Tenants (stay as they are)", p.Tenants)
	section("Bootstrap (stays outside ConfigHub)", p.Boundary)
	if len(p.Handover) > 0 {
		w("\nHandover (apply will write it as handover.sh; plan runs nothing)\n")
		for i, t := range p.Handover {
			w("  %d. %s\n", i+1, t)
		}
	}
	section("Left out", p.LeftOut)
	section("Problems to fix first", p.Problems)
	return b.String()
}
