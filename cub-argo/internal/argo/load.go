// Package argo reads an Argo CD estate, from a repository directory or from
// `kubectl get` output, and plans the fleet ConfigHub would govern.
package argo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Doc is one Kubernetes object from the input and the file it came from.
// File is empty for objects read from stdin.
type Doc struct {
	File  string
	Value map[string]any
}

// Input is everything the plan reads.
type Input struct {
	Docs []Doc
	// Dirs are the directories read, as absolute paths.
	Dirs []string
	// Skipped are files under a directory that are not Kubernetes YAML, such
	// as Helm chart templates, relative to the directory they were found in.
	Skipped []string
}

// Load reads each input: a YAML file, "-" for stdin, or a directory, which is
// walked for *.yaml and *.yml files. In a directory, a file that does not parse
// as YAML is skipped and listed rather than failing the whole read.
func Load(stdin io.Reader, inputs []string) (*Input, error) {
	in := &Input{}
	for _, arg := range inputs {
		if arg == "-" {
			data, err := io.ReadAll(stdin)
			if err != nil {
				return nil, err
			}
			docs, err := parse(data, "")
			if err != nil {
				return nil, fmt.Errorf("stdin: %w", err)
			}
			in.Docs = append(in.Docs, docs...)
			continue
		}
		abs, err := filepath.Abs(arg)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			data, err := os.ReadFile(abs)
			if err != nil {
				return nil, err
			}
			docs, err := parse(data, abs)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", arg, err)
			}
			in.Docs = append(in.Docs, docs...)
			continue
		}
		in.Dirs = append(in.Dirs, abs)
		err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if p != abs && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if ext := filepath.Ext(p); ext != ".yaml" && ext != ".yml" {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			docs, err := parse(data, p)
			if err != nil {
				rel, _ := filepath.Rel(abs, p)
				in.Skipped = append(in.Skipped, rel)
				return nil
			}
			in.Docs = append(in.Docs, docs...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(in.Skipped)
	return in, nil
}

// parse reads every object in a multi-document YAML or JSON stream, flattening
// kubectl's List output into its items.
func parse(data []byte, file string) ([]Doc, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var out []Doc
	for {
		var v map[string]any
		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		if v == nil {
			continue
		}
		if strings.HasSuffix(str(v["kind"]), "List") {
			for _, item := range list(v["items"]) {
				if obj := obj(item); obj != nil {
					out = append(out, Doc{File: file, Value: obj})
				}
			}
			continue
		}
		out = append(out, Doc{File: file, Value: v})
	}
}

// obj, list, str and get read the untyped values yaml.v3 decodes into.
func obj(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func str(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func get(v any, keys ...string) any {
	for _, k := range keys {
		m := obj(v)
		if m == nil {
			return nil
		}
		v = m[k]
	}
	return v
}

func strMap(v any) map[string]string {
	out := map[string]string{}
	for k, val := range obj(v) {
		out[k] = str(val)
	}
	return out
}
