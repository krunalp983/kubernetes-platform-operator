package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	platformv1alpha1 "github.com/A1D4863/kubernetes-platform-operator/api/v1alpha1"
)

func TestReconcileCreatesManagedResources(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := platformv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add platform api to scheme: %v", err)
	}
	if err := autoscalingv2.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add autoscaling api to scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add apps api to scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add core api to scheme: %v", err)
	}

	minReplicas := int32(2)
	targetCPU := int32(70)
	env := &platformv1alpha1.ApplicationEnvironment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "platform.example.com/v1alpha1",
			Kind:       "ApplicationEnvironment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "default",
		},
		Spec: platformv1alpha1.ApplicationEnvironmentSpec{
			Environment: "dev",
			Image: platformv1alpha1.ApplicationEnvironmentImageSpec{
				Repository: "nginx",
				Tag:        "1.27.1",
			},
			Service: platformv1alpha1.ApplicationEnvironmentServiceSpec{
				Port: 9090,
			},
			Config: map[string]string{
				"B": "2",
				"A": "1",
			},
			Autoscaling: &platformv1alpha1.ApplicationEnvironmentAutoscalingSpec{
				Enabled:                        true,
				MinReplicas:                    &minReplicas,
				MaxReplicas:                    6,
				TargetCPUUtilizationPercentage: &targetCPU,
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(env).WithObjects(env).Build()
	r := &ApplicationEnvironmentReconciler{
		Client:   c,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(50),
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "sample", Namespace: "default"}})
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}

	dep := &appsv1.Deployment{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "sample", Namespace: "default"}, dep); err != nil {
		t.Fatalf("expected deployment: %v", err)
	}
	if got := dep.Spec.Template.Spec.Containers[0].Image; got != "nginx:1.27.1" {
		t.Fatalf("unexpected image: %s", got)
	}
	if got := dep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort; got != 9090 {
		t.Fatalf("unexpected port: %d", got)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 1 {
		t.Fatalf("unexpected replicas: %#v", dep.Spec.Replicas)
	}

	svc := &corev1.Service{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "sample-svc", Namespace: "default"}, svc); err != nil {
		t.Fatalf("expected service: %v", err)
	}
	if svc.Spec.Ports[0].Port != 9090 {
		t.Fatalf("unexpected service port: %d", svc.Spec.Ports[0].Port)
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "sample-config", Namespace: "default"}, cm); err != nil {
		t.Fatalf("expected configmap: %v", err)
	}
	if cm.Data["A"] != "1" || cm.Data["B"] != "2" {
		t.Fatalf("unexpected configmap data: %#v", cm.Data)
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "sample", Namespace: "default"}, hpa); err != nil {
		t.Fatalf("expected hpa: %v", err)
	}
	if hpa.Spec.MaxReplicas != 6 {
		t.Fatalf("unexpected hpa max replicas: %d", hpa.Spec.MaxReplicas)
	}

	updated := &platformv1alpha1.ApplicationEnvironment{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: "sample", Namespace: "default"}, updated); err != nil {
		t.Fatalf("expected updated environment: %v", err)
	}
	if updated.Status.ObservedGeneration != updated.Generation {
		t.Fatalf("observed generation mismatch: got %d want %d", updated.Status.ObservedGeneration, updated.Generation)
	}
	if len(updated.Status.Conditions) == 0 {
		t.Fatal("expected status conditions to be populated")
	}
	if got := conditionStatus(updated.Status.Conditions, platformv1alpha1.ConditionTypeDegraded); got != metav1.ConditionFalse {
		t.Fatalf("expected Degraded=False, got %s", got)
	}
	if got := conditionStatus(updated.Status.Conditions, platformv1alpha1.ConditionTypeProgressing); got != metav1.ConditionTrue {
		t.Fatalf("expected Progressing=True while deployment is not yet ready, got %s", got)
	}
}

func TestDesiredReplicasAndPortDefaults(t *testing.T) {
	env := &platformv1alpha1.ApplicationEnvironment{}
	if got := desiredReplicas(env); got != 1 {
		t.Fatalf("default replicas should be 1, got %d", got)
	}
	if got := desiredServicePort(env); got != 8080 {
		t.Fatalf("default port should be 8080, got %d", got)
	}

	zero := int32(0)
	env.Spec.Replicas = &zero
	if got := desiredReplicas(env); got != 1 {
		t.Fatalf("replicas lower bound should coerce to 1, got %d", got)
	}

	three := int32(3)
	env.Spec.Replicas = &three
	if got := desiredReplicas(env); got != 3 {
		t.Fatalf("expected explicit replicas to be used, got %d", got)
	}

	env.Spec.Service.Port = 70000
	if got := desiredServicePort(env); got != 8080 {
		t.Fatalf("out-of-range port should default, got %d", got)
	}

	env.Spec.Service.Port = 8181
	if got := desiredServicePort(env); got != 8181 {
		t.Fatalf("expected explicit port to be used, got %d", got)
	}
}

func TestStableMapKeys(t *testing.T) {
	keys := stableMapKeys(map[string]string{"c": "3", "a": "1", "b": "2"})
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Fatalf("keys not sorted: %#v", keys)
	}
}

func TestReconcileNotFoundReturnsNoError(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := platformv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add platform api to scheme: %v", err)
	}

	r := &ApplicationEnvironmentReconciler{
		Client:   fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "missing", Namespace: "default"}})
	if err != nil {
		t.Fatalf("expected no error for missing resource, got: %v", err)
	}
}

func TestReconcileRemovesHPAWhenAutoscalingDisabled(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := platformv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add platform api to scheme: %v", err)
	}
	if err := autoscalingv2.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add autoscaling api to scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add apps api to scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add core api to scheme: %v", err)
	}

	minReplicas := int32(1)
	targetCPU := int32(75)
	env := &platformv1alpha1.ApplicationEnvironment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "platform.example.com/v1alpha1", Kind: "ApplicationEnvironment"},
		ObjectMeta: metav1.ObjectMeta{Name: "cleanup", Namespace: "default"},
		Spec: platformv1alpha1.ApplicationEnvironmentSpec{
			Environment: "dev",
			Image:       platformv1alpha1.ApplicationEnvironmentImageSpec{Repository: "nginx", Tag: "1.27.1"},
			Autoscaling: &platformv1alpha1.ApplicationEnvironmentAutoscalingSpec{
				Enabled:                        true,
				MinReplicas:                    &minReplicas,
				MaxReplicas:                    3,
				TargetCPUUtilizationPercentage: &targetCPU,
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(env).WithObjects(env).Build()
	r := &ApplicationEnvironmentReconciler{Client: c, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	key := types.NamespacedName{Name: "cleanup", Namespace: "default"}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("first reconcile failed: %v", err)
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	if err := c.Get(context.Background(), key, hpa); err != nil {
		t.Fatalf("expected hpa to exist after enabled autoscaling: %v", err)
	}

	updated := &platformv1alpha1.ApplicationEnvironment{}
	if err := c.Get(context.Background(), key, updated); err != nil {
		t.Fatalf("failed to re-fetch environment: %v", err)
	}
	updated.Spec.Autoscaling.Enabled = false
	if err := c.Update(context.Background(), updated); err != nil {
		t.Fatalf("failed to disable autoscaling: %v", err)
	}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatalf("second reconcile failed: %v", err)
	}

	err := c.Get(context.Background(), key, hpa)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected hpa to be deleted, got err: %v", err)
	}
}

func TestAutoscalingDefaultsAndBounds(t *testing.T) {
	env := &platformv1alpha1.ApplicationEnvironment{}
	if got := desiredHPAMinReplicas(env); got != 1 {
		t.Fatalf("default min replicas should be 1, got %d", got)
	}
	if got := desiredHPAMaxReplicas(env); got != 5 {
		t.Fatalf("default max replicas should be 5, got %d", got)
	}
	if got := *desiredCPUUtilization(env); got != 80 {
		t.Fatalf("default cpu should be 80, got %d", got)
	}

	min := int32(4)
	cpu := int32(150)
	env.Spec.Autoscaling = &platformv1alpha1.ApplicationEnvironmentAutoscalingSpec{
		MinReplicas:                    &min,
		MaxReplicas:                    2,
		TargetCPUUtilizationPercentage: &cpu,
	}

	if got := desiredHPAMaxReplicas(env); got != 4 {
		t.Fatalf("max replicas should be coerced to min replicas, got %d", got)
	}
	if got := *desiredCPUUtilization(env); got != 80 {
		t.Fatalf("invalid cpu target should default to 80, got %d", got)
	}

	validCPU := int32(60)
	env.Spec.Autoscaling.TargetCPUUtilizationPercentage = &validCPU
	if got := *desiredCPUUtilization(env); got != 60 {
		t.Fatalf("expected cpu target 60, got %d", got)
	}
}

func TestImageAndPullPolicyDefaults(t *testing.T) {
	env := &platformv1alpha1.ApplicationEnvironment{}
	env.Spec.Image.Repository = "example/app"
	env.Spec.Image.Tag = "1.2.3"

	if got := desiredImage(env); got != "example/app:1.2.3" {
		t.Fatalf("unexpected image format: %s", got)
	}
	if got := desiredPullPolicy(env); got != corev1.PullIfNotPresent {
		t.Fatalf("default pull policy should be IfNotPresent, got %s", got)
	}

	env.Spec.Image.PullPolicy = corev1.PullAlways
	if got := desiredPullPolicy(env); got != corev1.PullAlways {
		t.Fatalf("expected explicit pull policy to be respected, got %s", got)
	}
}

func conditionStatus(conditions []metav1.Condition, condType string) metav1.ConditionStatus {
	for _, c := range conditions {
		if c.Type == condType {
			return c.Status
		}
	}
	return metav1.ConditionUnknown
}
