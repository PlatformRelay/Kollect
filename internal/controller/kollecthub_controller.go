// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kollectdevv1alpha1 "github.com/konih/kollect/api/v1alpha1"
	"github.com/konih/kollect/internal/metrics"
)

const defaultHubReplicas int32 = 1

// KollectHubReconciler reconciles a KollectHub object.
type KollectHubReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Options RuntimeOptions
}

// +kubebuilder:rbac:groups=kollect.dev,resources=kollecthubs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kollect.dev,resources=kollecthubs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kollect.dev,resources=kollecthubs/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile ensures a minimal hub consumer Deployment exists for the configured transport.
func (r *KollectHubReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	finish := trackReconcile("kollecthub")
	var retErr error
	defer func() { finish(retErr) }()

	log := logf.FromContext(ctx)

	var hub kollectdevv1alpha1.KollectHub
	if err := r.Get(ctx, req.NamespacedName, &hub); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	replicas := defaultHubReplicas
	if hub.Spec.Replicas != nil && *hub.Spec.Replicas > 0 {
		replicas = *hub.Spec.Replicas
	}

	dep := r.desiredDeployment(&hub, replicas)
	if err := r.ensureDeployment(ctx, dep); err != nil {
		metrics.ReconcileErrorsTotal.WithLabelValues("KollectHub", metrics.ErrorClassTransient).Inc()
		log.Error(err, "ensure hub deployment")
		retErr = err

		return ctrl.Result{}, err
	}

	hub.Status.ObservedGeneration = hub.Generation
	apimeta.RemoveStatusCondition(&hub.Status.Conditions, conditionDegraded)
	apimeta.SetStatusCondition(&hub.Status.Conditions, metav1.Condition{
		Type:               conditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "HubDeploymentReady",
		Message:            fmt.Sprintf("hub transport %q deployment reconciled", hub.Spec.Transport.Type),
		ObservedGeneration: hub.Generation,
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, &hub); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KollectHubReconciler) desiredDeployment(
	hub *kollectdevv1alpha1.KollectHub,
	replicas int32,
) *appsv1.Deployment {
	name := hub.Name + "-consumer"
	labels := map[string]string{
		"app.kubernetes.io/name":      "kollect-hub",
		"app.kubernetes.io/component": "hub-consumer",
		"kollect.dev/hub":             hub.Name,
		"kollect.dev/transport-type":  hub.Spec.Transport.Type,
	}

	env := []corev1.EnvVar{
		{Name: "KOLLECT_HUB_NAME", Value: hub.Name},
		{Name: "KOLLECT_TRANSPORT_TYPE", Value: hub.Spec.Transport.Type},
	}

	if hub.Spec.Transport.Redis != nil {
		env = append(env, corev1.EnvVar{
			Name:  "KOLLECT_REDIS_URL",
			Value: hub.Spec.Transport.Redis.URL,
		})
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "kollect-system",
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "hub-consumer",
						Image:           "ghcr.io/konih/kollect:latest",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"/manager"},
						Args: []string{
							"--hub-consumer",
							"--health-probe-bind-address=:8081",
							"--metrics-bind-address=:8080",
							"--metrics-secure=false",
						},
						Env: env,
						Ports: []corev1.ContainerPort{{
							Name:          "health",
							ContainerPort: 8081,
						}},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromString("health"),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/readyz",
									Port: intstr.FromString("health"),
								},
							},
						},
					}},
				},
			},
		},
	}
}

func (r *KollectHubReconciler) ensureDeployment(ctx context.Context, desired *appsv1.Deployment) error {
	var existing appsv1.Deployment
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), &existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}

	if err != nil {
		return err
	}

	patch := client.MergeFrom(existing.DeepCopy())
	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Selector = desired.Spec.Selector
	existing.Spec.Template = desired.Spec.Template
	existing.Labels = desired.Labels

	return r.Patch(ctx, &existing, patch)
}

// SetupWithManager sets up the controller with the Manager.
func (r *KollectHubReconciler) SetupWithManager(mgr ctrl.Manager) error {
	opts := r.Options.controllerOptions(r.Options.MaxConcurrentHub)
	if opts.MaxConcurrentReconciles == 0 {
		opts.MaxConcurrentReconciles = DefaultRuntimeOptions().MaxConcurrentHub
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kollectdevv1alpha1.KollectHub{}).
		WithOptions(opts).
		Named("kollecthub").
		Complete(r)
}
