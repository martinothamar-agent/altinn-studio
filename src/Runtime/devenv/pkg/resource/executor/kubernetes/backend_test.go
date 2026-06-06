package kubernetesbackend

import (
	"os"
	"strings"
	"testing"
	"time"

	"altinn.studio/devenv/pkg/flux"
	"altinn.studio/devenv/pkg/resource"
	"altinn.studio/devenv/pkg/resource/executor"
)

func TestBackendSupportsOnlyKubernetesObjectSet(t *testing.T) {
	t.Parallel()

	backend := New()
	if !backend.Supports(&resource.KubernetesObjectSet{}) {
		t.Fatalf("Supports(KubernetesObjectSet) = false")
	}
	for _, res := range []resource.Resource{
		&resource.KindCluster{},
		&resource.FluxInstallation{},
		&resource.OCIArtifact{},
	} {
		if backend.Supports(res) {
			t.Fatalf("Supports(%T) = true, want false", res)
		}
	}
}

func TestApplyKubernetesObjectSetAppliesManifestAndReadiness(t *testing.T) {
	t.Parallel()

	manifestPath := t.TempDir() + "/manifest.yaml"
	writeFile(t, manifestPath, "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: test\n")
	fluxClient := &fakeFlux{}
	kube := &fakeKube{}
	backend := newTestBackend(kube, fluxClient)
	cluster := &resource.KindCluster{Name: "cluster"}
	objects := &resource.KubernetesObjectSet{
		Name:    "app",
		Cluster: resource.Ref(cluster),
		Path:    manifestPath,
		Readiness: []resource.KubernetesReadinessCheck{
			{Kind: resource.KubernetesReadinessFluxKustomization, Namespace: "apps", Name: "app"},
			{Kind: resource.KubernetesReadinessDeploymentAvailable, Namespace: "apps", Name: "app"},
		},
	}

	if _, err := backend.Apply(t.Context(), executor.BackendContext{GraphID: "test"}, objects); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if !strings.Contains(kube.appliedManifest, "kind: Namespace") {
		t.Fatalf("applied manifest = %q", kube.appliedManifest)
	}
	if got := strings.Join(fluxClient.kustomizations, ","); got != "apps/app" {
		t.Fatalf("kustomizations = %q", got)
	}
	if got := strings.Join(kube.rollouts, ","); got != "apps/app" {
		t.Fatalf("rollouts = %q", got)
	}
}

func TestApplyKubernetesObjectSetRendersKustomizeDirectories(t *testing.T) {
	t.Parallel()

	kube := &fakeKube{}
	backend := newTestBackend(kube, &fakeFlux{})
	cluster := &resource.KindCluster{Name: "cluster"}
	objects := &resource.KubernetesObjectSet{
		Name:    "app",
		Cluster: resource.Ref(cluster),
		Path:    t.TempDir(),
	}

	if _, err := backend.Apply(t.Context(), executor.BackendContext{GraphID: "test"}, objects); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if !strings.HasPrefix(kube.appliedManifest, "rendered:") {
		t.Fatalf("applied manifest = %q, want rendered kustomize output", kube.appliedManifest)
	}
}

func TestApplyKubernetesObjectSetUsesInlineManifest(t *testing.T) {
	t.Parallel()

	kube := &fakeKube{}
	backend := newTestBackend(kube, &fakeFlux{})
	cluster := &resource.KindCluster{Name: "cluster"}
	objects := &resource.KubernetesObjectSet{
		Name:     "app",
		Cluster:  resource.Ref(cluster),
		Manifest: "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: inline\n",
	}

	if _, err := backend.Apply(t.Context(), executor.BackendContext{GraphID: "test"}, objects); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if !strings.Contains(kube.appliedManifest, "name: inline") {
		t.Fatalf("applied manifest = %q", kube.appliedManifest)
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
	appliedManifest string
	rollouts        []string
}

func (f *fakeKube) ApplyManifest(yamlContent string) (string, error) {
	f.appliedManifest = yamlContent
	return "applied", nil
}

func (f *fakeKube) KustomizeRender(path string) (string, error) {
	return "rendered:" + path, nil
}

func (f *fakeKube) RolloutStatus(deployment, namespace string, _ time.Duration) error {
	f.rollouts = append(f.rollouts, namespace+"/"+deployment)
	return nil
}

type fakeFlux struct {
	kustomizations []string
	helmReleases   []string
}

func (f *fakeFlux) ReconcileHelmRelease(name, namespace string, _ bool, _ flux.ReconcileOptions) error {
	f.helmReleases = append(f.helmReleases, namespace+"/"+name)
	return nil
}

func (f *fakeFlux) ReconcileKustomization(name, namespace string, _ bool, _ flux.ReconcileOptions) error {
	f.kustomizations = append(f.kustomizations, namespace+"/"+name)
	return nil
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
