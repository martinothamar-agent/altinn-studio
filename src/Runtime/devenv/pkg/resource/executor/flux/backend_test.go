package fluxbackend

import (
	"errors"
	"strings"
	"testing"
	"time"

	"altinn.studio/devenv/pkg/flux"
	"altinn.studio/devenv/pkg/resource"
	"altinn.studio/devenv/pkg/resource/executor"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

var errTestKubeFailed = errors.New("kube failed")

func TestBackendSupportsOnlyFluxInstallation(t *testing.T) {
	t.Parallel()

	backend := New()
	if !backend.Supports(&resource.FluxInstallation{}) {
		t.Fatalf("Supports(FluxInstallation) = false")
	}
	for _, res := range []resource.Resource{
		&resource.KindCluster{},
		&resource.KubernetesObjectSet{},
		&resource.OCIArtifact{},
	} {
		if backend.Supports(res) {
			t.Fatalf("Supports(%T) = true, want false", res)
		}
	}
}

func TestApplyFluxInstallationInstallsAndWaitsForControllers(t *testing.T) {
	t.Parallel()

	fluxClient := &fakeFlux{}
	kube := &fakeKube{}
	backend := newTestBackend(kube, fluxClient)
	cluster := &resource.KindCluster{Name: "cluster"}
	installation := &resource.FluxInstallation{
		Cluster:    resource.Ref(cluster),
		Components: []string{"source-controller"},
	}

	if _, err := backend.Apply(t.Context(), executor.BackendContext{GraphID: "test"}, installation); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if got := strings.Join(fluxClient.installed, ","); got != "source-controller" {
		t.Fatalf("installed components = %q", got)
	}
	if got := strings.Join(kube.rollouts, ","); got != "flux-system/source-controller" {
		t.Fatalf("rollouts = %q", got)
	}
}

func TestObserveFluxInstallationReturnsClusterClientError(t *testing.T) {
	t.Parallel()

	backend := &Backend{
		newKube: func(string) (kubernetesOperations, error) {
			return nil, errTestKubeFailed
		},
		clusters: make(map[resource.ResourceID]clusterClients),
	}
	installation := &resource.FluxInstallation{Cluster: resource.Ref(&resource.KindCluster{Name: "cluster"})}

	_, err := backend.Observe(t.Context(), executor.BackendContext{GraphID: "test"}, installation)
	if !errors.Is(err, errTestKubeFailed) {
		t.Fatalf("Observe() error = %v, want %v", err, errTestKubeFailed)
	}
}

func newTestBackend(kube *fakeKube, fluxClient *fakeFlux) *Backend {
	return &Backend{
		newKube: func(string) (kubernetesOperations, error) {
			return kube, nil
		},
		newFlux: func(kubernetesOperations) (fluxOperations, error) {
			return fluxClient, nil
		},
		clusters: make(map[resource.ResourceID]clusterClients),
	}
}

type fakeKube struct {
	rollouts []string
}

func (f *fakeKube) Get(schema.GroupVersionResource, string, string) error {
	return nil
}

func (f *fakeKube) RolloutStatus(deployment, namespace string, _ time.Duration) error {
	f.rollouts = append(f.rollouts, namespace+"/"+deployment)
	return nil
}

type fakeFlux struct {
	installed []string
}

func (f *fakeFlux) Install(components []string, _ flux.InstallOptions) error {
	f.installed = append(f.installed, components...)
	return nil
}
