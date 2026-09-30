package argo

import "fmt"

// An Application is read on the cluster Argo CD runs on, and the objects it
// deploys are read on the cluster it deploys them to. Those are one cluster
// only when the destination is Argo's own. Reading a remote Application's
// objects on the management cluster would compare whatever happens to share
// their names there, and could pass.

// Destination is an Application's spec.destination: a server address, or the
// name of a cluster Argo CD knows.
type Destination struct {
	Server string `json:"server,omitempty"`
	Name   string `json:"name,omitempty"`
}

// InCluster reports whether the Application deploys to the cluster Argo CD
// itself runs on.
func (d Destination) InCluster() bool {
	return d.Server == "https://kubernetes.default.svc" || d.Name == "in-cluster"
}

func (d Destination) String() string {
	if d.Server != "" {
		return d.Server
	}
	return d.Name
}

// DestinationAccess is what the person running the check said about the
// destination: the kubectl context that reaches it, and optionally which Argo
// destination that context is.
type DestinationAccess struct {
	Context  string
	Declared string
}

// ResolveDestination picks the kubectl context that reads an Application's
// objects, or refuses. management is the context the Application itself was
// read on. serverOf gives a context's API server address. Identity comes only
// from the destination's server or cluster name and from what the person
// declared, never from a label or a display name.
func ResolveDestination(d Destination, management string, a DestinationAccess, serverOf func(string) (string, error)) (string, error) {
	if d.InCluster() {
		if a.Context != "" && a.Context != management {
			return "", fmt.Errorf("the Application deploys to the cluster Argo CD runs on, but --destination-context names %s; reading there would compare another cluster's objects", a.Context)
		}
		return management, nil
	}
	if a.Context == "" {
		return "", fmt.Errorf("the Application deploys to %s, not the cluster Argo CD runs on. Pass --destination-context for that cluster: its objects cannot be read on this one", d)
	}
	if a.Declared != "" {
		if a.Declared != d.Server && a.Declared != d.Name {
			return "", fmt.Errorf("--destination says %s is %s, but the Application deploys to %s", a.Context, a.Declared, d)
		}
		return a.Context, nil
	}
	server, err := serverOf(a.Context)
	if err != nil {
		return "", fmt.Errorf("reading the address of context %s: %w", a.Context, err)
	}
	if d.Server == "" || server != d.Server {
		return "", fmt.Errorf("cannot tell that context %s is %s: kubectl reaches it at %q. If they are the same cluster by different addresses, say so with --destination %s", a.Context, d, server, d)
	}
	return a.Context, nil
}
