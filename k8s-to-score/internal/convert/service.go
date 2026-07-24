// Copyright (C) ConfigHub, Inc.
// SPDX-License-Identifier: MIT

package convert

import (
	"fmt"
	"sort"

	scoretypes "github.com/score-spec/score-go/types"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// convertService folds every Service that selects a workload's pods into the
// single `service.ports` map Score gives a workload. Two Services exposing the
// same port name collapse to one entry; the collision is reported.
func (c *converter) convertService(w *workload) *scoretypes.WorkloadService {
	svcs := c.inv.servicesFor(w)
	if len(svcs) == 0 {
		return nil
	}
	sort.Slice(svcs, func(i, j int) bool { return svcs[i].Name < svcs[j].Name })

	ports := scoretypes.WorkloadServicePorts{}
	for _, s := range svcs {
		for _, p := range s.Spec.Ports {
			name := portName(p)
			if existing, dup := ports[name]; dup {
				if existing.Port != int(p.Port) {
					c.warnf(w, "Services %q and another both define port %q with different port numbers; kept %d",
						s.Name, name, existing.Port)
				}
				continue
			}
			sp := scoretypes.ServicePort{Port: int(p.Port)}
			if tp, ok := resolveTargetPort(w, p); ok && tp != int(p.Port) {
				sp.TargetPort = &tp
			}
			if p.Protocol == corev1.ProtocolUDP {
				proto := scoretypes.ServicePortProtocolUDP
				sp.Protocol = &proto
			}
			ports[name] = sp
		}
	}
	if len(ports) == 0 {
		return nil
	}
	return &scoretypes.WorkloadService{Ports: ports}
}

// portName is the key a port takes in Score's ports map. Kubernetes allows an
// unnamed port on a single-port Service; Score always needs a key.
func portName(p corev1.ServicePort) string {
	if p.Name != "" {
		return sanitizeName(p.Name)
	}
	return fmt.Sprintf("port-%d", p.Port)
}

// resolveTargetPort turns a Service targetPort into an integer, resolving a
// named target port against the workload's container ports.
func resolveTargetPort(w *workload, p corev1.ServicePort) (int, bool) {
	if v := p.TargetPort.IntValue(); v != 0 {
		return v, true
	}
	name := p.TargetPort.String()
	if name == "" || name == "0" {
		return 0, false
	}
	for _, k8sc := range w.pod.Containers {
		for _, cp := range k8sc.Ports {
			if cp.Name == name {
				return int(cp.ContainerPort), true
			}
		}
	}
	return 0, false
}

// convertIngresses turns the Ingress rules that route to a workload into Score
// `route` resources, one per host+path. A rule with no host gets a `dns`
// resource and references its output, which is how score-k8s expects a route to
// obtain a hostname it does not own.
func (c *converter) convertIngresses(w *workload, svc *scoretypes.WorkloadService, res scoretypes.WorkloadResources) {
	for _, ing := range c.inv.ingresses {
		if ing.Namespace != w.ns {
			continue
		}
		for _, rule := range ing.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			for _, p := range rule.HTTP.Paths {
				if p.Backend.Service == nil {
					continue
				}
				if c.inv.workloadForService(ing.Namespace, p.Backend.Service.Name) != w {
					continue
				}
				c.addRoute(w, ing, rule, p, svc, res)
			}
		}
	}
}

func (c *converter) addRoute(
	w *workload,
	ing *networkingv1.Ingress,
	rule networkingv1.IngressRule,
	p networkingv1.HTTPIngressPath,
	svc *scoretypes.WorkloadService,
	res scoretypes.WorkloadResources,
) {
	// score-k8s's route provisioner requires params.port to name a port the
	// workload's own service block declares. If it does not, the route would
	// fail at generate time, so drop it here with an explanation instead.
	port, ok := routePort(p.Backend.Service, svc)
	if !ok {
		c.warnf(w, "Ingress %q routes to a Service port that is not in this workload's service ports; route dropped", ing.Name)
		return
	}

	host := rule.Host
	if host == "" {
		// No host on the rule: let Score provision one.
		dns := c.uniqueName(res, sanitizeName(w.name)+"-dns")
		res[dns] = scoretypes.Resource{Type: "dns"}
		host = fmt.Sprintf("${resources.%s.host}", dns)
	}

	routePath := p.Path
	if routePath == "" {
		routePath = "/"
	}

	name := c.uniqueName(res, sanitizeName(ing.Name)+"-route")
	res[name] = scoretypes.Resource{
		Type: "route",
		Params: scoretypes.ResourceParams{
			"host": host,
			"path": routePath,
			"port": port,
		},
	}
}

// routePort picks the Score service port key that matches an Ingress backend.
// score-k8s accepts either the port name or the port number as the key, so a
// numeric backend port is returned as-is once it is confirmed present.
func routePort(backend *networkingv1.IngressServiceBackend, svc *scoretypes.WorkloadService) (any, bool) {
	if svc == nil {
		return nil, false
	}
	if backend.Port.Name != "" {
		key := sanitizeName(backend.Port.Name)
		if _, ok := svc.Ports[key]; ok {
			return key, true
		}
		return nil, false
	}
	for _, sp := range svc.Ports {
		if sp.Port == int(backend.Port.Number) {
			return int(backend.Port.Number), true
		}
	}
	return nil, false
}

// uniqueName suffixes a resource name until it does not collide — two Ingresses
// with the same name in different Units, or several paths on one Ingress.
func (c *converter) uniqueName(res scoretypes.WorkloadResources, base string) string {
	if _, exists := res[base]; !exists {
		return base
	}
	for i := 2; ; i++ {
		n := fmt.Sprintf("%s-%d", base, i)
		if _, exists := res[n]; !exists {
			return n
		}
	}
}
