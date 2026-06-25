// Package k8s parses the bits of a Kubernetes workload manifest the cost model
// needs — replicas, container resource requests, and persistent storage — plus
// the resource.Quantity strings those use (CPU "250m", memory "64Mi", storage
// "20Gi"). Hand-rolled so the binary carries no Kubernetes dependency.
package k8s

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Container is one container's requested CPU (cores) and memory (GiB), and
// whether it declared each — a missing request can't be costed and trips the
// requests-required guardrail.
type Container struct {
	Name   string
	CPU    float64
	Memory float64
	HasCPU bool
	HasMem bool
}

// Workload is a Deployment or StatefulSet reduced to its cost inputs.
type Workload struct {
	Kind       string
	Name       string
	Replicas   int
	Containers []Container
	StorageGB  float64 // Σ volumeClaimTemplates storage (StatefulSet)
}

// MissingRequests reports whether any container omitted a CPU or memory request.
func (w *Workload) MissingRequests() bool {
	for _, c := range w.Containers {
		if !c.HasCPU || !c.HasMem {
			return true
		}
	}
	return len(w.Containers) == 0
}

type manifest struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Replicas *int `yaml:"replicas"`
		Template struct {
			Spec struct {
				Containers []struct {
					Name      string `yaml:"name"`
					Resources struct {
						Requests map[string]string `yaml:"requests"`
					} `yaml:"resources"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
		VolumeClaimTemplates []struct {
			Spec struct {
				Resources struct {
					Requests map[string]string `yaml:"requests"`
				} `yaml:"resources"`
			} `yaml:"spec"`
		} `yaml:"volumeClaimTemplates"`
	} `yaml:"spec"`
}

// Parse returns the first Deployment or StatefulSet in a (possibly multi-doc)
// manifest, reduced to its cost inputs. Returns an error if none is present.
func Parse(data []byte) (*Workload, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var m manifest
		if err := dec.Decode(&m); err != nil {
			break // EOF or undecodable trailing doc
		}
		if m.Kind != "Deployment" && m.Kind != "StatefulSet" {
			continue
		}
		w := &Workload{Kind: m.Kind, Name: m.Metadata.Name, Replicas: 1}
		if m.Spec.Replicas != nil {
			w.Replicas = *m.Spec.Replicas
		}
		for _, c := range m.Spec.Template.Spec.Containers {
			cn := Container{Name: c.Name}
			if v, ok := c.Resources.Requests["cpu"]; ok {
				if cores, err := ParseCPU(v); err == nil {
					cn.CPU, cn.HasCPU = cores, true
				}
			}
			if v, ok := c.Resources.Requests["memory"]; ok {
				if gb, err := ParseBytesGiB(v); err == nil {
					cn.Memory, cn.HasMem = gb, true
				}
			}
			w.Containers = append(w.Containers, cn)
		}
		for _, vct := range m.Spec.VolumeClaimTemplates {
			if v, ok := vct.Spec.Resources.Requests["storage"]; ok {
				if gb, err := ParseBytesGiB(v); err == nil {
					w.StorageGB += gb
				}
			}
		}
		return w, nil
	}
	return nil, fmt.Errorf("no Deployment or StatefulSet found")
}

// ParseCPU converts a Kubernetes CPU quantity to cores: "250m" → 0.25, "2" → 2.
func ParseCPU(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty cpu quantity")
	}
	if strings.HasSuffix(s, "m") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "m"), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid cpu %q: %w", s, err)
		}
		return n / 1000, nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid cpu %q: %w", s, err)
	}
	return n, nil
}

// binarySuffix → power-of-1024 multiplier; decimalSuffix → power-of-1000.
var binarySuffix = map[string]float64{"Ki": 1 << 10, "Mi": 1 << 20, "Gi": 1 << 30, "Ti": 1 << 40, "Pi": 1 << 50}
var decimalSuffix = map[string]float64{"k": 1e3, "K": 1e3, "M": 1e6, "G": 1e9, "T": 1e12, "P": 1e15}

// ParseBytesGiB converts a Kubernetes memory/storage quantity to GiB:
// "64Mi" → 0.0625, "20Gi" → 20, "1G" → 0.931, "1500000000" → ~1.397.
func ParseBytesGiB(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty memory quantity")
	}
	for suf, mul := range binarySuffix {
		if strings.HasSuffix(s, suf) {
			n, err := strconv.ParseFloat(strings.TrimSuffix(s, suf), 64)
			if err != nil {
				return 0, fmt.Errorf("invalid quantity %q: %w", s, err)
			}
			return n * mul / (1 << 30), nil
		}
	}
	for suf, mul := range decimalSuffix {
		if strings.HasSuffix(s, suf) {
			n, err := strconv.ParseFloat(strings.TrimSuffix(s, suf), 64)
			if err != nil {
				return 0, fmt.Errorf("invalid quantity %q: %w", s, err)
			}
			return n * mul / (1 << 30), nil
		}
	}
	// Bare byte count.
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid quantity %q: %w", s, err)
	}
	return n / (1 << 30), nil
}
