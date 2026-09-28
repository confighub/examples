package flux

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// WriteApply writes the workflow files and the apply, handover and cleanup
// scripts. It runs
// nothing and touches nothing outside dir.
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
		outFile{"cleanup.sh", []byte(CleanupScript(p, prefix)), 0o755},
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
