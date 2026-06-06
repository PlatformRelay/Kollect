// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package collect

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
)

// scaleTestMaxObjects is the ADR-0603 default synthetic object cap for task test.
// SAR-forbidden → Degraded (EC-P1-06) is not exercised here: envtest runs with
// cluster-admin-equivalent credentials; see KollectTargetReconciler unit/envtest paths.
const scaleTestMaxObjects = 500

func TestStore_Scale500(t *testing.T) {
	t.Parallel()

	start := time.Now()
	store := NewStore()
	for i := range scaleTestMaxObjects {
		store.Upsert(Item{
			TargetNamespace: "default",
			TargetName:      "scale-target",
			UID:             fmt.Sprintf("uid-%04d", i),
			Namespace:       "default",
			Name:            fmt.Sprintf("scale-%04d", i),
			Version:         "v1",
			Kind:            "Deployment",
			Attributes:      map[string]any{"name": fmt.Sprintf("scale-%04d", i)},
		})
	}

	if got := store.TotalCount(); got != scaleTestMaxObjects {
		t.Fatalf("TotalCount() = %d, want %d", got, scaleTestMaxObjects)
	}

	snap := store.SnapshotNamespace("default")
	if len(snap) != scaleTestMaxObjects {
		t.Fatalf("len(snapshot) = %d, want %d", len(snap), scaleTestMaxObjects)
	}

	elapsed := time.Since(start)
	t.Logf("store upsert+snapshot %d objects in %s", scaleTestMaxObjects, elapsed)
	if elapsed > 5*time.Second {
		t.Fatalf("store scale exceeded 5s budget: %s", elapsed)
	}
}

func TestEngine_ScaleEnvtest500(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping envtest scale in short mode")
	}

	deadline := time.Now().Add(28 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 28*time.Second)
	defer cancel()

	testEnv, cfg := startScaleEnvtest(t)
	defer stopScaleEnvtest(t, testEnv)

	if err := seedScaleNamespace(ctx, cfg); err != nil {
		t.Fatalf("seed namespace: %v", err)
	}

	if err := createScaleDeployments(ctx, cfg, scaleTestMaxObjects); err != nil {
		t.Fatalf("create deployments: %v", err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("dynamic client: %v", err)
	}

	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("kubernetes client: %v", err)
	}

	store := NewStore()
	engine, err := NewEngine(dyn, kube, store)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()
	if err := engine.Start(runCtx); err != nil {
		t.Fatalf("engine.Start: %v", err)
	}

	target := &kollectdevv1alpha1.KollectTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "scale-target", Namespace: "scale-test"},
		Spec: kollectdevv1alpha1.KollectTargetSpec{
			ProfileRef: "scale-profile",
		},
	}
	profile := &kollectdevv1alpha1.KollectProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "scale-profile"},
		Spec: kollectdevv1alpha1.KollectProfileSpec{
			TargetGVK: kollectdevv1alpha1.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			Attributes: []kollectdevv1alpha1.AttributeSpec{
				{Name: "name", Path: "{.metadata.name}"},
				{Name: "replicas", Path: "{.spec.replicas}"},
			},
		},
	}

	if err := engine.RegisterTarget(ctx, target, profile, RegisterTargetOptions{}); err != nil {
		t.Fatalf("RegisterTarget: %v", err)
	}

	waitStart := time.Now()
	for store.TotalCount() < scaleTestMaxObjects {
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s with %d/%d items collected",
				time.Since(waitStart), store.TotalCount(), scaleTestMaxObjects)
		}

		select {
		case <-ctx.Done():
			t.Fatalf("context cancelled with %d/%d items", store.TotalCount(), scaleTestMaxObjects)
		case <-time.After(50 * time.Millisecond):
		}
	}

	if got := engine.ItemCount("scale-test", "scale-target"); got != scaleTestMaxObjects {
		t.Fatalf("ItemCount = %d, want %d", got, scaleTestMaxObjects)
	}

	t.Logf("engine collected %d deployments in %s (envtest cap=%d)",
		scaleTestMaxObjects, time.Since(waitStart), scaleTestMaxObjects)
}

func startScaleEnvtest(t *testing.T) (*envtest.Environment, *rest.Config) {
	t.Helper()

	testEnv := &envtest.Environment{}
	testEnv.BinaryAssetsDirectory = resolveEnvtestAssetsDir()

	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("envtest start: %v", err)
	}

	return testEnv, cfg
}

func stopScaleEnvtest(t *testing.T, testEnv *envtest.Environment) {
	t.Helper()

	if err := testEnv.Stop(); err != nil {
		t.Fatalf("envtest stop: %v", err)
	}
}

func resolveEnvtestAssetsDir() string {
	if assets := os.Getenv("KUBEBUILDER_ASSETS"); assets != "" {
		if abs, err := filepath.Abs(assets); err == nil {
			return abs
		}

		return assets
	}

	return scaleEnvtestBinaryDir()
}

func scaleEnvtestBinaryDir() string {
	for _, basePath := range []string{
		filepath.Join("bin", "k8s"),
		filepath.Join("..", "..", "bin", "k8s"),
	} {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				abs, err := filepath.Abs(filepath.Join(basePath, entry.Name()))
				if err != nil {
					continue
				}

				return abs
			}
		}
	}

	return ""
}

func seedScaleNamespace(ctx context.Context, cfg *rest.Config) error {
	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("kubernetes client: %w", err)
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "scale-test",
			Labels: map[string]string{
				"kubernetes.io/metadata.name": "scale-test",
			},
		},
	}
	if _, err := kube.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create namespace: %w", err)
	}

	return nil
}

func createScaleDeployments(ctx context.Context, cfg *rest.Config, count int) error {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("dynamic client: %w", err)
	}

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	const workers = 25

	errCh := make(chan error, workers)
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for i := range count {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			name := fmt.Sprintf("scale-%04d", i)
			appLabel := fmt.Sprintf("scale-%04d", i)

			deploy := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "scale-test",
					Labels: map[string]string{
						"app": "kollect-scale",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": appLabel},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": appLabel},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:  "main",
								Image: "busybox:latest",
							}},
						},
					},
				},
			}

			obj, convErr := runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
			if convErr != nil {
				errCh <- fmt.Errorf("to unstructured %s: %w", name, convErr)

				return
			}

			if _, createErr := dyn.Resource(gvr).Namespace("scale-test").Create(
				ctx,
				&unstructured.Unstructured{Object: obj},
				metav1.CreateOptions{},
			); createErr != nil {
				errCh <- fmt.Errorf("create deployment %s: %w", name, createErr)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for createErr := range errCh {
		if createErr != nil {
			return createErr
		}
	}

	return nil
}

func int32Ptr(v int32) *int32 {
	return &v
}
