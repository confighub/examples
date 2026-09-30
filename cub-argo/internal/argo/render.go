package argo

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
	w("Argo CD estate: %d clusters, %d components, %d variants\n", len(p.Clusters), len(p.Components), variants)
	if len(p.Inputs.Skipped) > 0 {
		// Named, not counted. A count under a banner saying "such as Helm
		// templates" reads as benign, and one of these was a file the estate
		// actually syncs that had a broken indent.
		w("Read %d objects; skipped %d file(s) that did not parse as Kubernetes YAML:\n", p.Inputs.Objects, len(p.Inputs.Skipped))
		for _, sk := range p.Inputs.Skipped {
			w("  %s\n", sk)
		}
	} else {
		w("Read %d objects\n", p.Inputs.Objects)
	}

	if len(p.Tree) > 0 {
		w("\nControl tree (stays as it is: this is the management record)\n")
		for _, n := range p.Tree {
			renderNode(&b, n, 1)
		}
	}

	if p.StageLabel != "" {
		w("\nStages by %s: %s\n", p.StageLabel, strings.Join(p.Stages, ", "))
	} else {
		w("\nOne stage, fleet (pass --stage-label and --stages to roll out in waves)\n")
	}

	for _, c := range p.Components {
		w("\n%s  (%s", c.Name, c.Kind)
		if c.Source != c.Name {
			w(" %s", c.Source)
		}
		if c.Owner != "" {
			w(", owned by %s", c.Owner)
		}
		w(", project %s, wave %d)\n", c.Project, c.Wave)
		if c.Generator != "" {
			w("  selects  %s\n", c.Generator)
		}
		w("  base     %s  reaches no cluster: its destination is empty\n", c.Base)
		w("  each variant differs from the base in %s\n", strings.Join(c.Departures, ", "))
		for _, st := range c.Stages {
			w("  stage %s\n", st.Name)
			for _, v := range st.Variants {
				label := v.Cluster
				if v.Key != "" {
					label = v.Key
				}
				w("    %-11s variant %s  ->  Target %s\n", label, v.Space, v.Target)
				w("    %-11s Application %s, namespace %s\n", "", v.Application, v.Namespace)
				w("    %-11s %s\n", "", v.Path)
			}
		}
		for _, f := range c.InFlight {
			w("  in flight  %s\n", f)
		}
		for _, n := range c.Notes {
			w("  note     %s\n", n)
		}
	}

	if len(p.Windows) > 0 {
		w("\nSync windows (a published release waits for Argo CD while one is closed)\n")
		for _, win := range p.Windows {
			tz := win.TimeZone
			if tz == "" {
				tz = "UTC"
			}
			manual := "manual sync blocked too"
			if win.ManualSync {
				manual = "manual sync allowed"
			}
			w("  %s  %s %s for %s (%s), %s\n", win.Project, win.Kind, win.Schedule, win.Duration, tz, manual)
			if len(win.Applications) > 0 {
				w("    covers %s\n", strings.Join(win.Applications, ", "))
			} else {
				w("    covers none of the planned Applications\n")
			}
		}
	}

	if len(p.Unselected) > 0 {
		w("\nClusters no ApplicationSet selects: %s\n", strings.Join(p.Unselected, ", "))
	}
	if len(p.Live) > 0 {
		w("\nLive: %d generated Applications in the input, so handover is needed\n", len(p.Live))
	}
	if len(p.Handover) > 0 {
		w("\nHandover, when this estate is live (apply will write it as handover.sh; plan runs nothing)\n")
		for i, t := range p.Handover {
			w("  %d. %s\n", i+1, t)
		}
	}
	if len(p.LeftOut) > 0 {
		w("\nLeft out\n")
		for _, l := range p.LeftOut {
			w("  - %s\n", l)
		}
	}
	if len(p.Problems) > 0 {
		w("\nProblems to fix first\n")
		for _, pr := range p.Problems {
			w("  - %s\n", pr)
		}
	}
	return b.String()
}

func renderNode(b *strings.Builder, n *Node, depth int) {
	indent := strings.Repeat("  ", depth)
	label := fmt.Sprintf("%s%s %s", indent, n.Kind, n.Name)
	fmt.Fprintf(b, "%-44s wave %3d  %s\n", label, n.Wave, n.Role)
	for _, c := range n.Children {
		renderNode(b, c, depth+1)
	}
}
