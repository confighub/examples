package argo

import (
	"fmt"
	"strings"
	"testing"
)

const remoteApp = `{"spec":{"destination":{"server":"https://prod-1.example:6443","namespace":"apptique"},
 "syncPolicy":{"automated":{"prune":true}}},
 "status":{"resources":[{"group":"apps","kind":"Deployment","namespace":"apptique","name":"frontend"}]}}`

const releaseOneReplica = `apiVersion: apps/v1
kind: Deployment
metadata: {name: frontend, namespace: apptique}
spec: {replicas: 1}
`

func deployWith(replicas int) string {
	return fmt.Sprintf(`{"kind":"Deployment","metadata":{"name":"frontend","namespace":"apptique"},"spec":{"replicas":%d}}`, replicas)
}

// Two clusters hold a Deployment of the same name with different fields. The
// check must read the one the Application deploys to, not the one Argo runs
// on (confighub/helm-expt#2022).
func TestFieldsAreReadOnTheDestination(t *testing.T) {
	mgmt := fake(map[string]string{
		"get application": remoteApp,
		"unit data":       releaseOneReplica,
		"get deployment":  deployWith(4), // same name, on the management cluster
	})
	var readOn string
	workloads := func(d Destination) (Runner, error) {
		readOn = d.Server
		return fake(map[string]string{"get deployment": deployWith(1)}), nil
	}
	r, err := RunCheck(mgmt, Check{Application: "prod-1-apptique", Space: "s", Unit: "u", Namespace: "apptique"}, true, workloads)
	if err != nil {
		t.Fatal(err)
	}
	if readOn != "https://prod-1.example:6443" {
		t.Errorf("objects should be read at the Application's destination, got %q", readOn)
	}
	if !r.OK() || r.Fields.Compared != 1 {
		t.Errorf("prod-1 holds replicas 1 as released; the management cluster's 4 is another object: %+v", r.Fields)
	}
}

// A destination that cannot be reached is not a clean check.
func TestUnreachableDestinationIsNotClean(t *testing.T) {
	mgmt := fake(map[string]string{"get application": remoteApp, "unit data": releaseOneReplica})
	workloads := func(Destination) (Runner, error) {
		return func(string, ...string) ([]byte, error) {
			return nil, fmt.Errorf("kubectl -n apptique get: Unable to connect to the server: dial tcp: i/o timeout")
		}, nil
	}
	r, err := RunCheck(mgmt, Check{Application: "a", Space: "s", Unit: "u"}, true, workloads)
	if err != nil {
		t.Fatal(err)
	}
	if r.OK() || len(r.Fields.Unreadable) != 1 {
		t.Errorf("an unreachable destination must not pass: %+v", r.Fields)
	}
}

// A refusal to resolve the destination fails the check rather than falling
// back to the management cluster.
func TestUnresolvedDestinationFailsTheCheck(t *testing.T) {
	mgmt := fake(map[string]string{"get application": remoteApp, "unit data": releaseOneReplica, "get deployment": deployWith(1)})
	workloads := func(d Destination) (Runner, error) {
		return nil, fmt.Errorf("no context for %s", d)
	}
	if _, err := RunCheck(mgmt, Check{Application: "a", Space: "s", Unit: "u"}, true, workloads); err == nil {
		t.Error("an unresolved destination must fail, not read the management cluster")
	}
}

func TestResolveDestination(t *testing.T) {
	servers := map[string]string{"kind-prod-1": "https://prod-1.example:6443", "kind-other": "https://127.0.0.1:55001"}
	serverOf := func(ctx string) (string, error) {
		if s, ok := servers[ctx]; ok {
			return s, nil
		}
		return "", fmt.Errorf("no context %s", ctx)
	}
	remote := Destination{Server: "https://prod-1.example:6443"}
	for _, tc := range []struct {
		name   string
		d      Destination
		access DestinationAccess
		want   string
		refuse string
	}{
		{"in-cluster reads on management", Destination{Server: "https://kubernetes.default.svc"}, DestinationAccess{}, "kind-mgmt", ""},
		{"in-cluster by name", Destination{Name: "in-cluster"}, DestinationAccess{}, "kind-mgmt", ""},
		{"in-cluster with another context", Destination{Name: "in-cluster"}, DestinationAccess{Context: "kind-other"}, "", "another cluster"},
		{"remote without a context", remote, DestinationAccess{}, "", "Pass --destination-context"},
		{"remote, addresses match", remote, DestinationAccess{Context: "kind-prod-1"}, "kind-prod-1", ""},
		{"remote, address differs", remote, DestinationAccess{Context: "kind-other"}, "", "cannot tell"},
		{"remote, declared", remote, DestinationAccess{Context: "kind-other", Declared: "https://prod-1.example:6443"}, "kind-other", ""},
		{"remote, declared wrongly", remote, DestinationAccess{Context: "kind-other", Declared: "staging-1"}, "", "but the Application deploys to"},
		{"remote by name, declared", Destination{Name: "prod-1"}, DestinationAccess{Context: "kind-other", Declared: "prod-1"}, "kind-other", ""},
		{"remote by name, undeclared", Destination{Name: "prod-1"}, DestinationAccess{Context: "kind-prod-1"}, "", "cannot tell"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveDestination(tc.d, "kind-mgmt", tc.access, serverOf)
			if tc.refuse != "" {
				if err == nil || !strings.Contains(err.Error(), tc.refuse) {
					t.Errorf("want a refusal containing %q, got %q, %v", tc.refuse, got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Errorf("want %q, got %q, %v", tc.want, got, err)
			}
		})
	}
}
