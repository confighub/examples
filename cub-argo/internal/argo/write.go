package argo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// WriteApply writes the files apply.sh reads, apply.sh, handover.sh,
// move-applications.sh and argobot.sh. It
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
		// What each ApplicationSet's template renders for each cluster: a
		// cluster that joins after the retirement has no Application on the
		// cluster to read, so move-applications.sh makes its Unit from this.
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				if c.Kind != "ApplicationSet" || v.app == nil {
					continue
				}
				data, err := renderedApplication(v.app)
				if err != nil {
					return "", fmt.Errorf("writing what %s renders for %s: %w", c.Source, v.Cluster, err)
				}
				files = append(files, outFile{filepath.Join("apps", v.Space+".yaml"), data, 0o644})
			}
		}
	}

	// Each Application a parent syncs, repointed at its Space, in the file its
	// Unit was made from. handover.sh fills in the gateway and prints the
	// reviewed change that puts it in place.
	repointed, err := p.repointedUnits(prefix)
	if err != nil {
		return "", err
	}
	for name, data := range repointed {
		files = append(files, outFile{name, data, 0o644})
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
		outFile{"argobot.sh", []byte(ArgobotScript(prefix)), 0o755},
		outFile{".gitignore", []byte("render/\n"), 0o644},
	)
	if s := MoveScript(p, prefix); s != "" {
		files = append(files, outFile{"move-applications.sh", []byte(s), 0o755})
	}

	// What the templates render is written afresh each time, so a cluster
	// that left the plan, or an ApplicationSet that now has a templatePatch,
	// leaves nothing behind for move-applications.sh to read.
	if err := os.RemoveAll(filepath.Join(dir, "apps")); err != nil {
		return "", err
	}
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

// repointedUnits is each control-Space file holding an Application a variant
// stands for, with that Application's source pointed at its Space. The
// gateway is left as oci://<gateway>/, for handover.sh to fill in.
func (p *Plan) repointedUnits(prefix string) (map[string][]byte, error) {
	homes := p.unitHomes(prefix)
	spaceOf := map[string]string{}
	for _, c := range p.Components {
		if c.Kind == "ApplicationSet" {
			continue
		}
		for _, st := range c.Stages {
			for _, v := range st.Variants {
				if v.Path != "" && !strings.HasPrefix(v.Path, "(") {
					spaceOf[v.Application] = v.Space
				}
			}
		}
	}
	byFile := map[string]map[string]string{}
	homeOf := map[string]unitHome{}
	for app, space := range spaceOf {
		h, ok := homes["Application/"+app]
		if !ok {
			continue
		}
		if byFile[h.File] == nil {
			byFile[h.File] = map[string]string{}
		}
		byFile[h.File][app] = "oci://<gateway>/space/" + space
		homeOf[h.File] = h
	}
	out := map[string][]byte{}
	for file, urls := range byFile {
		abs := file
		if p.Inputs.RepoRoot != "" && !filepath.IsAbs(abs) {
			abs = filepath.Join(p.Inputs.RepoRoot, filepath.FromSlash(file))
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil, fmt.Errorf("reading %s, to repoint what it defines: %w", file, err)
		}
		changed, err := repointApplications(data, urls)
		if err != nil {
			return nil, fmt.Errorf("repointing what %s defines: %w", file, err)
		}
		out[filepath.Join("repointed", homeOf[file].Space, filepath.Base(file))] = changed
	}
	return out, nil
}

// repointApplications rewrites each named Application's spec.source to read
// its URL at tag latest, and leaves every other document and field as it is.
// The tool settings that built it from Git are dropped: the release is
// rendered already.
func repointApplications(data []byte, urls map[string]string) ([]byte, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var docs []*yaml.Node
	for {
		var n yaml.Node
		if err := dec.Decode(&n); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		docs = append(docs, &n)
	}
	done := map[string]bool{}
	for _, d := range docs {
		if len(d.Content) == 0 {
			continue
		}
		m := d.Content[0]
		if scalar(m, "kind") != "Application" {
			continue
		}
		name := scalar(mapping(m, "metadata"), "name")
		url, ok := urls[name]
		if !ok {
			continue
		}
		spec := mapping(m, "spec")
		if spec == nil {
			return nil, fmt.Errorf("Application %s has no spec", name)
		}
		src := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for _, kv := range [][2]string{{"repoURL", url}, {"path", "."}, {"targetRevision", "latest"}} {
			src.Content = append(src.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: kv[0]},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: kv[1]})
		}
		set := false
		for i := 0; i+1 < len(spec.Content); i += 2 {
			if spec.Content[i].Value == "source" {
				spec.Content[i+1] = src
				set = true
			}
		}
		if !set {
			return nil, fmt.Errorf("Application %s has no spec.source to repoint", name)
		}
		done[name] = true
	}
	for name := range urls {
		if !done[name] {
			return nil, fmt.Errorf("Application %s is not in the file", name)
		}
	}
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	for _, d := range docs {
		if err := enc.Encode(d); err != nil {
			return nil, err
		}
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func mapping(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func scalar(m *yaml.Node, key string) string {
	if n := mapping(m, key); n != nil && n.Kind == yaml.ScalarNode {
		return n.Value
	}
	return ""
}
