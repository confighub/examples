package argo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// WriteApply writes the files apply.sh reads, apply.sh, and handover.sh. It
// runs nothing and touches nothing outside dir.
func WriteApply(p *Plan, prefix, dir string) (string, error) {
	if len(p.Problems) > 0 {
		return "", fmt.Errorf("the plan has problems to fix first")
	}
	type outFile struct {
		name string
		data []byte
		mode os.FileMode
	}
	var files []outFile

	// The control objects, copied byte for byte so a review of what ConfigHub
	// holds is a review of what Argo reads today.
	for _, s := range p.controlSpaces(prefix) {
		for _, f := range s.Files {
			abs := f
			if p.Inputs.RepoRoot != "" && !filepath.IsAbs(abs) {
				abs = filepath.Join(p.Inputs.RepoRoot, filepath.FromSlash(f))
			}
			data, err := os.ReadFile(abs)
			if err != nil {
				return "", fmt.Errorf("reading %s, which %s syncs: %w", f, s.Parent, err)
			}
			files = append(files, outFile{filepath.Join("control", s.Space, filepath.Base(f)), data, 0o644})
		}
	}

	for _, c := range p.Components {
		if len(c.Stages) == 0 {
			continue
		}
		files = append(files, outFile{filepath.Join(c.Name, "change-workflow.yaml"), []byte(Workflow(c)), 0o644})
	}

	// apply.sh renders from the repository, so it needs the way back to it.
	repoRel := "."
	if p.Inputs.RepoRoot != "" {
		if abs, err := filepath.Abs(dir); err == nil {
			if rel, err := filepath.Rel(abs, p.Inputs.RepoRoot); err == nil {
				repoRel = rel
			} else {
				repoRel = p.Inputs.RepoRoot
			}
		}
	}

	files = append(files,
		outFile{"apply.sh", []byte(ApplyScript(p, prefix, repoRel)), 0o755},
		outFile{"handover.sh", []byte(HandoverScript(p, prefix, repoRel)), 0o755},
		outFile{".gitignore", []byte("render/\n"), 0o644},
	)

	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, f.data, f.mode); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, "apply.sh"), nil
}
