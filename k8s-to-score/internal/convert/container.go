// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package convert

import (
	"fmt"
	"path"
	"sort"
	"strings"

	scoretypes "github.com/score-spec/score-go/types"
	corev1 "k8s.io/api/core/v1"
)

// Placeholder is the value written for config the converter can see the shape
// of but not the content of — a Secret value, or a ConfigMap that lives outside
// the Space. It is ConfigHub's placeholder sentinel, so `vet-placeholders`
// flags it if the score file is ever round-tripped back into a Unit.
const Placeholder = "confighubplaceholder"

// convertContainer maps one Kubernetes container onto a Score container. The
// resources it pulls in (volumes) are added to res; anything that cannot be
// represented is appended to the report.
func (c *converter) convertContainer(w *workload, k8sc corev1.Container, res scoretypes.WorkloadResources) scoretypes.Container {
	out := scoretypes.Container{
		Image:   k8sc.Image,
		Command: k8sc.Command,
		Args:    k8sc.Args,
	}

	if vars := c.convertVariables(w, k8sc); len(vars) > 0 {
		out.Variables = vars
	}
	if r := convertResources(k8sc.Resources); r != nil {
		out.Resources = r
	}
	if p := c.convertProbe(w, k8sc.LivenessProbe, k8sc.Name, "livenessProbe"); p != nil {
		out.LivenessProbe = p
	}
	if p := c.convertProbe(w, k8sc.ReadinessProbe, k8sc.Name, "readinessProbe"); p != nil {
		out.ReadinessProbe = p
	}

	files, volumes := c.convertMounts(w, k8sc, res)
	if len(files) > 0 {
		out.Files = files
	}
	if len(volumes) > 0 {
		out.Volumes = volumes
	}
	return out
}

// convertVariables flattens env and envFrom into Score's flat string map.
//
// Values that Kubernetes resolves at pod-admission time have no Score
// equivalent: a Secret value is not in the config data, and a fieldRef or
// resourceFieldRef is a property of the running pod. Those become placeholders
// with a warning rather than disappearing, so the operator sees what still
// needs a value.
func (c *converter) convertVariables(w *workload, k8sc corev1.Container) scoretypes.ContainerVariables {
	vars := scoretypes.ContainerVariables{}

	// envFrom first: explicit env entries win over envFrom, matching Kubernetes.
	for _, ef := range k8sc.EnvFrom {
		switch {
		case ef.ConfigMapRef != nil:
			cm, ok := c.inv.configMaps[key(w.ns, ef.ConfigMapRef.Name)]
			if !ok {
				c.warnf(w, "container %s: envFrom ConfigMap %q is not a Unit in this Space; its variables were not expanded",
					k8sc.Name, ef.ConfigMapRef.Name)
				continue
			}
			for _, k := range sorted(keysOf(cm.Data)) {
				name := ef.Prefix + k
				if !validEnvName(name) {
					// Kubernetes itself drops these from envFrom rather than
					// failing the pod, so the converted spec matches what the
					// container would actually see.
					c.warnf(w, "container %s: envFrom ConfigMap %q key %q is not a valid environment variable name and was skipped, as Kubernetes would skip it",
						k8sc.Name, ef.ConfigMapRef.Name, k)
					continue
				}
				vars[name] = cm.Data[k]
			}
		case ef.SecretRef != nil:
			s, ok := c.inv.secrets[key(w.ns, ef.SecretRef.Name)]
			if !ok {
				c.warnf(w, "container %s: envFrom Secret %q is not a Unit in this Space; its variables were not expanded",
					k8sc.Name, ef.SecretRef.Name)
				continue
			}
			written := 0
			for _, k := range sorted(append(keysOf(s.Data), keysOf(s.StringData)...)) {
				name := ef.Prefix + k
				if !validEnvName(name) {
					continue
				}
				vars[name] = Placeholder
				written++
			}
			if written > 0 {
				c.warnf(w, "container %s: envFrom Secret %q expanded to %d placeholder variable(s); supply real values or model the Secret as a Score resource",
					k8sc.Name, ef.SecretRef.Name, written)
			}
		}
	}

	for _, e := range k8sc.Env {
		switch {
		case e.ValueFrom == nil:
			vars[e.Name] = e.Value
		case e.ValueFrom.ConfigMapKeyRef != nil:
			ref := e.ValueFrom.ConfigMapKeyRef
			cm, ok := c.inv.configMaps[key(w.ns, ref.Name)]
			if ok {
				if v, found := cm.Data[ref.Key]; found {
					vars[e.Name] = v
					continue
				}
			}
			vars[e.Name] = Placeholder
			c.warnf(w, "container %s: variable %s reads ConfigMap %q key %q, which is not in this Space; wrote a placeholder",
				k8sc.Name, e.Name, ref.Name, ref.Key)
		case e.ValueFrom.SecretKeyRef != nil:
			ref := e.ValueFrom.SecretKeyRef
			vars[e.Name] = Placeholder
			c.warnf(w, "container %s: variable %s reads Secret %q key %q; wrote a placeholder — supply a real value or model the Secret as a Score resource",
				k8sc.Name, e.Name, ref.Name, ref.Key)
		case e.ValueFrom.FieldRef != nil:
			// metadata.name and metadata.namespace have Score equivalents via
			// the metadata substitution; the rest are pod runtime state.
			switch e.ValueFrom.FieldRef.FieldPath {
			case "metadata.name":
				vars[e.Name] = "${metadata.name}"
			default:
				vars[e.Name] = Placeholder
				c.warnf(w, "container %s: variable %s reads the pod field %q, which Score cannot express; wrote a placeholder",
					k8sc.Name, e.Name, e.ValueFrom.FieldRef.FieldPath)
			}
		default:
			vars[e.Name] = Placeholder
			c.warnf(w, "container %s: variable %s uses an unsupported valueFrom source; wrote a placeholder", k8sc.Name, e.Name)
		}
	}

	return vars
}

// convertResources maps container requests and limits. Score carries only cpu
// and memory; other resources (GPUs, ephemeral storage) are dropped by the
// spec, not by this tool.
func convertResources(rr corev1.ResourceRequirements) *scoretypes.ContainerResources {
	limits := resourceLimits(rr.Limits)
	requests := resourceLimits(rr.Requests)
	if limits == nil && requests == nil {
		return nil
	}
	return &scoretypes.ContainerResources{Limits: limits, Requests: requests}
}

func resourceLimits(rl corev1.ResourceList) *scoretypes.ResourcesLimits {
	if len(rl) == 0 {
		return nil
	}
	var out scoretypes.ResourcesLimits
	if q, ok := rl[corev1.ResourceCPU]; ok {
		s := q.String()
		out.Cpu = &s
	}
	if q, ok := rl[corev1.ResourceMemory]; ok {
		s := q.String()
		out.Memory = &s
	}
	if out.Cpu == nil && out.Memory == nil {
		return nil
	}
	return &out
}

// convertProbe maps an httpGet or exec probe. Score supports only those two
// handlers: a tcpSocket or grpc probe has no representation and is reported.
func (c *converter) convertProbe(w *workload, p *corev1.Probe, container, which string) *scoretypes.ContainerProbe {
	if p == nil {
		return nil
	}
	switch {
	case p.HTTPGet != nil:
		port, ok := resolveProbePort(w, container, p.HTTPGet.Port.String(), p.HTTPGet.Port.IntValue())
		if !ok {
			c.warnf(w, "container %s: %s uses the named port %q, which no container port declares; probe dropped",
				container, which, p.HTTPGet.Port.String())
			return nil
		}
		probe := &scoretypes.ContainerProbe{HttpGet: &scoretypes.HttpProbe{
			Path: p.HTTPGet.Path,
			Port: port,
		}}
		if p.HTTPGet.Host != "" {
			h := p.HTTPGet.Host
			probe.HttpGet.Host = &h
		}
		if p.HTTPGet.Scheme == corev1.URISchemeHTTPS {
			s := scoretypes.HttpProbeSchemeHTTPS
			probe.HttpGet.Scheme = &s
		}
		for _, h := range p.HTTPGet.HTTPHeaders {
			probe.HttpGet.HttpHeaders = append(probe.HttpGet.HttpHeaders,
				scoretypes.HttpProbeHttpHeadersElem{Name: h.Name, Value: h.Value})
		}
		return probe
	case p.Exec != nil:
		return &scoretypes.ContainerProbe{Exec: &scoretypes.ExecProbe{Command: p.Exec.Command}}
	default:
		c.warnf(w, "container %s: %s uses a handler Score does not support (only httpGet and exec); probe dropped", container, which)
		return nil
	}
}

// resolveProbePort turns a probe port into an integer, resolving a named port
// against the container's declared ports.
func resolveProbePort(w *workload, container, name string, intVal int) (int, bool) {
	if intVal != 0 {
		return intVal, true
	}
	for _, k8sc := range w.pod.Containers {
		if k8sc.Name != container {
			continue
		}
		for _, cp := range k8sc.Ports {
			if cp.Name == name {
				return int(cp.ContainerPort), true
			}
		}
	}
	return 0, false
}

// convertMounts splits a container's volumeMounts into Score files (mounts
// whose content is in the config data — ConfigMaps and Secrets) and Score
// volumes (mounts backed by storage — PVCs, emptyDirs).
func (c *converter) convertMounts(w *workload, k8sc corev1.Container, res scoretypes.WorkloadResources) (scoretypes.ContainerFiles, scoretypes.ContainerVolumes) {
	files := scoretypes.ContainerFiles{}
	volumes := scoretypes.ContainerVolumes{}

	for _, m := range k8sc.VolumeMounts {
		if ct := claimTemplate(w, m.Name); ct != nil {
			name := c.addVolumeResource(res, ct.Name)
			volumes[m.MountPath] = volumeMount(name, m)
			continue
		}
		v := podVolume(w, m.Name)
		if v == nil {
			c.warnf(w, "container %s: volumeMount %q references no pod volume; mount dropped", k8sc.Name, m.Name)
			continue
		}
		switch {
		case v.ConfigMap != nil:
			c.configMapFiles(w, k8sc.Name, m, v.ConfigMap, files)
		case v.Secret != nil:
			c.secretFiles(w, k8sc.Name, m, v.Secret, files)
		case v.PersistentVolumeClaim != nil:
			name := c.addVolumeResource(res, v.PersistentVolumeClaim.ClaimName)
			volumes[m.MountPath] = volumeMount(name, m)
		case v.EmptyDir != nil:
			name := c.addVolumeResource(res, v.Name)
			volumes[m.MountPath] = volumeMount(name, m)
		default:
			c.warnf(w, "container %s: volume %q uses a source Score cannot express (only configMap, secret, persistentVolumeClaim and emptyDir map); mount dropped",
				k8sc.Name, m.Name)
		}
	}

	return files, volumes
}

func volumeMount(resourceName string, m corev1.VolumeMount) scoretypes.ContainerVolume {
	cv := scoretypes.ContainerVolume{Source: fmt.Sprintf("${resources.%s}", resourceName)}
	if m.SubPath != "" {
		sp := m.SubPath
		cv.Path = &sp
	}
	if m.ReadOnly {
		ro := true
		cv.ReadOnly = &ro
	}
	return cv
}

// addVolumeResource declares a `type: volume` resource once per backing volume
// and returns the Score resource name to reference it by.
func (c *converter) addVolumeResource(res scoretypes.WorkloadResources, name string) string {
	n := sanitizeName(name)
	if _, exists := res[n]; !exists {
		res[n] = scoretypes.Resource{Type: "volume"}
	}
	return n
}

// configMapFiles turns a ConfigMap volume mount into Score files with literal
// content — the one place where ConfigHub's config-as-data model pays off
// directly, since the ConfigMap's values are right there in a sibling Unit.
func (c *converter) configMapFiles(w *workload, container string, m corev1.VolumeMount, src *corev1.ConfigMapVolumeSource, files scoretypes.ContainerFiles) {
	cm, ok := c.inv.configMaps[key(w.ns, src.Name)]
	if !ok {
		c.warnf(w, "container %s: mounted ConfigMap %q is not a Unit in this Space; mount at %s dropped",
			container, src.Name, m.MountPath)
		return
	}
	for _, f := range selectKeys(keysOf(cm.Data), src.Items, m.SubPath) {
		content := cm.Data[f.key]
		files[path.Join(m.MountPath, f.rel)] = scoretypes.ContainerFile{Content: &content}
	}
}

// secretFiles mirrors configMapFiles, but a Secret's values are not config
// data: the file keys are preserved with placeholder content and a warning.
func (c *converter) secretFiles(w *workload, container string, m corev1.VolumeMount, src *corev1.SecretVolumeSource, files scoretypes.ContainerFiles) {
	s, ok := c.inv.secrets[key(w.ns, src.SecretName)]
	if !ok {
		c.warnf(w, "container %s: mounted Secret %q is not a Unit in this Space; mount at %s dropped",
			container, src.SecretName, m.MountPath)
		return
	}
	names := sorted(append(keysOf(s.Data), keysOf(s.StringData)...))
	written := 0
	for _, f := range selectKeys(names, src.Items, m.SubPath) {
		content := Placeholder
		files[path.Join(m.MountPath, f.rel)] = scoretypes.ContainerFile{Content: &content}
		written++
	}
	if written > 0 {
		c.warnf(w, "container %s: mounted Secret %q wrote %d placeholder file(s) at %s; supply real content or model the Secret as a Score resource",
			container, src.SecretName, written, m.MountPath)
	}
}

// fileKey pairs a ConfigMap/Secret data key with the path it lands at relative
// to the mount point.
type fileKey struct {
	key string
	rel string
}

// selectKeys applies the volume's `items` projection and the mount's subPath to
// decide which keys are mounted and where. With no items, every key is mounted
// at its own name; with a subPath, only the matching key is mounted, directly
// at the mount path.
func selectKeys(all []string, items []corev1.KeyToPath, subPath string) []fileKey {
	var out []fileKey
	if len(items) > 0 {
		for _, it := range items {
			rel := it.Path
			if rel == "" {
				rel = it.Key
			}
			out = append(out, fileKey{key: it.Key, rel: rel})
		}
	} else {
		sorted := append([]string(nil), all...)
		sort.Strings(sorted)
		for _, k := range sorted {
			out = append(out, fileKey{key: k, rel: k})
		}
	}
	if subPath == "" {
		return out
	}
	// A subPath mount projects exactly one key onto the mount path itself.
	for _, f := range out {
		if f.rel == subPath {
			return []fileKey{{key: f.key, rel: ""}}
		}
	}
	return nil
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sorted(s []string) []string {
	sort.Strings(s)
	return s
}

// validEnvName reports whether a key can be an environment variable name — the
// C_IDENTIFIER rule Kubernetes enforces on envFrom. Keys that fail it (a
// ConfigMap holding a `settings.yaml` file, say) are skipped by the kubelet, so
// the converter skips them too rather than inventing a variable the container
// never sees.
func validEnvName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_',
			r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// sanitizeName coerces a Kubernetes name into a Score resource/port name:
// lowercase alphanumerics and single hyphens.
func sanitizeName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevHyphen := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		case !prevHyphen && b.Len() > 0:
			b.WriteRune('-')
			prevHyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
}
