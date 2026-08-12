package controller

import (
	"context"
	"fmt"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/A1D4863/kubernetes-platform-operator/api/v1alpha1"
)

const (
	configMapSuffix = "-config"
	serviceSuffix   = "-svc"
)

// ApplicationEnvironmentReconciler reconciles an ApplicationEnvironment object.
type ApplicationEnvironmentReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=platform.example.com,resources=applicationenvironments,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.example.com,resources=applicationenvironments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.example.com,resources=applicationenvironments/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
func (r *ApplicationEnvironmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("applicationEnvironment", req.NamespacedName)
	logger.Info("reconciliation started")

	env := &platformv1alpha1.ApplicationEnvironment{}
	if err := r.Get(ctx, req.NamespacedName, env); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("resource no longer exists")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if err := r.reconcileConfigMap(ctx, env); err != nil {
		r.Recorder.Event(env, corev1.EventTypeWarning, "ReconcileFailed", fmt.Sprintf("failed to reconcile configmap: %v", err))
		return r.failStatus(ctx, env, "ConfigMapError", err)
	}

	if err := r.reconcileService(ctx, env); err != nil {
		r.Recorder.Event(env, corev1.EventTypeWarning, "ReconcileFailed", fmt.Sprintf("failed to reconcile service: %v", err))
		return r.failStatus(ctx, env, "ServiceError", err)
	}

	if err := r.reconcileDeployment(ctx, env); err != nil {
		r.Recorder.Event(env, corev1.EventTypeWarning, "ReconcileFailed", fmt.Sprintf("failed to reconcile deployment: %v", err))
		return r.failStatus(ctx, env, "DeploymentError", err)
	}

	if err := r.reconcileHPA(ctx, env); err != nil {
		r.Recorder.Event(env, corev1.EventTypeWarning, "ReconcileFailed", fmt.Sprintf("failed to reconcile autoscaling: %v", err))
		return r.failStatus(ctx, env, "AutoscalingError", err)
	}

	if err := r.updateStatus(ctx, env); err != nil {
		return ctrl.Result{}, err
	}

	r.Recorder.Event(env, corev1.EventTypeNormal, "Reconciled", "Successfully reconciled ApplicationEnvironment")
	logger.Info("reconciliation completed")
	return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
}

func (r *ApplicationEnvironmentReconciler) reconcileConfigMap(ctx context.Context, env *platformv1alpha1.ApplicationEnvironment) error {
	logger := log.FromContext(ctx)
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: env.Name + configMapSuffix, Namespace: env.Namespace}}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if err := controllerutil.SetControllerReference(env, cm, r.Scheme); err != nil {
			return err
		}
		cm.Data = make(map[string]string, len(env.Spec.Config))
		for _, key := range stableMapKeys(env.Spec.Config) {
			cm.Data[key] = env.Spec.Config[key]
		}
		return nil
	})
	if err == nil {
		logger.Info("configmap reconciled", "name", cm.Name, "result", result)
	}
	return err
}

func (r *ApplicationEnvironmentReconciler) reconcileService(ctx context.Context, env *platformv1alpha1.ApplicationEnvironment) error {
	logger := log.FromContext(ctx)
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: env.Name + serviceSuffix, Namespace: env.Namespace}}
	labels := managedLabels(env)

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		if err := controllerutil.SetControllerReference(env, svc, r.Scheme); err != nil {
			return err
		}
		svc.Spec.Selector = labels
		svc.Spec.Type = desiredServiceType(env)
		svc.Spec.Ports = []corev1.ServicePort{{
			Name:       "http",
			Port:       desiredServicePort(env),
			TargetPort: intstr.FromInt32(desiredServicePort(env)),
			Protocol:   corev1.ProtocolTCP,
		}}
		return nil
	})
	if err == nil {
		logger.Info("service reconciled", "name", svc.Name, "result", result)
	}
	return err
}

func (r *ApplicationEnvironmentReconciler) reconcileDeployment(ctx context.Context, env *platformv1alpha1.ApplicationEnvironment) error {
	logger := log.FromContext(ctx)
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: env.Name, Namespace: env.Namespace}}
	replicas := desiredReplicas(env)
	labels := map[string]string{
		"app.kubernetes.io/name":       env.Name,
		"app.kubernetes.io/managed-by": "kubernetes-platform-operator",
		"platform.example.com/owner":   env.Name,
		"platform.example.com/env":     env.Spec.Environment,
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		if err := controllerutil.SetControllerReference(env, dep, r.Scheme); err != nil {
			return err
		}

		dep.Spec.Replicas = &replicas
		dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		dep.Spec.Template.ObjectMeta.Labels = labels
		dep.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:            "app",
			Image:           desiredImage(env),
			ImagePullPolicy: desiredPullPolicy(env),
			Ports: []corev1.ContainerPort{{
				ContainerPort: desiredServicePort(env),
				Name:          "http",
			}},
			EnvFrom: []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: env.Name + configMapSuffix}},
			}},
			Resources: env.Spec.Resources,
		}}
		dep.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsNonRoot: ptrBool(true),
		}
		dep.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptrBool(false),
			ReadOnlyRootFilesystem:   ptrBool(true),
		}
		return nil
	})
	if err == nil {
		logger.Info("deployment reconciled", "name", dep.Name, "result", result)
	}
	return err
}

func (r *ApplicationEnvironmentReconciler) reconcileHPA(ctx context.Context, env *platformv1alpha1.ApplicationEnvironment) error {
	logger := log.FromContext(ctx)
	hpaName := env.Name
	if env.Spec.Autoscaling == nil || !env.Spec.Autoscaling.Enabled {
		existing := &autoscalingv2.HorizontalPodAutoscaler{}
		err := r.Get(ctx, types.NamespacedName{Name: hpaName, Namespace: env.Namespace}, existing)
		if err == nil {
			delErr := r.Delete(ctx, existing)
			if delErr == nil {
				logger.Info("autoscaling disabled, removed hpa", "name", existing.Name)
			}
			return delErr
		}
		if apierrors.IsNotFound(err) {
			logger.Info("autoscaling disabled, no hpa present")
			return nil
		}
		return err
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: hpaName, Namespace: env.Namespace}}
	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, hpa, func() error {
		if err := controllerutil.SetControllerReference(env, hpa, r.Scheme); err != nil {
			return err
		}

		minReplicas := desiredHPAMinReplicas(env)
		hpa.Spec.ScaleTargetRef = autoscalingv2.CrossVersionObjectReference{APIVersion: "apps/v1", Kind: "Deployment", Name: env.Name}
		hpa.Spec.MinReplicas = &minReplicas
		hpa.Spec.MaxReplicas = desiredHPAMaxReplicas(env)
		hpa.Spec.Metrics = []autoscalingv2.MetricSpec{{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: desiredCPUUtilization(env),
				},
			},
		}}
		hpa.Spec.Behavior = env.Spec.Autoscaling.Behavior
		return nil
	})
	if err == nil {
		logger.Info("hpa reconciled", "name", hpa.Name, "result", result)
	}
	return err
}

func (r *ApplicationEnvironmentReconciler) failStatus(ctx context.Context, env *platformv1alpha1.ApplicationEnvironment, reason string, recErr error) (ctrl.Result, error) {
	message := recErr.Error()
	setCondition(&env.Status.Conditions, metav1.Condition{
		Type:               platformv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: env.Generation,
		LastTransitionTime: metav1.Now(),
	})
	setCondition(&env.Status.Conditions, metav1.Condition{
		Type:               platformv1alpha1.ConditionTypeProgressing,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            "Reconciliation failed",
		ObservedGeneration: env.Generation,
		LastTransitionTime: metav1.Now(),
	})
	setCondition(&env.Status.Conditions, metav1.Condition{
		Type:               platformv1alpha1.ConditionTypeDegraded,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: env.Generation,
		LastTransitionTime: metav1.Now(),
	})
	env.Status.ObservedGeneration = env.Generation
	now := metav1.Now()
	env.Status.LastReconciledTime = &now
	if err := r.Status().Update(ctx, env); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, recErr
}

func (r *ApplicationEnvironmentReconciler) updateStatus(ctx context.Context, env *platformv1alpha1.ApplicationEnvironment) error {
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: env.Name, Namespace: env.Namespace}, dep); err != nil {
		return err
	}

	env.Status.ObservedGeneration = env.Generation
	env.Status.ReadyReplicas = dep.Status.ReadyReplicas
	env.Status.ServiceName = env.Name + serviceSuffix
	now := metav1.Now()
	env.Status.LastReconciledTime = &now

	desired := desiredReplicas(env)
	condition := metav1.Condition{
		Type:               platformv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             "Deploying",
		Message:            "Deployment is progressing",
		ObservedGeneration: env.Generation,
		LastTransitionTime: metav1.Now(),
	}
	if dep.Status.ReadyReplicas == desired {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "Ready"
		condition.Message = "Deployment is ready"
	}
	setCondition(&env.Status.Conditions, condition)
	setCondition(&env.Status.Conditions, metav1.Condition{
		Type:               platformv1alpha1.ConditionTypeProgressing,
		Status:             metav1.ConditionFalse,
		Reason:             "Stable",
		Message:            "Reconciliation converged",
		ObservedGeneration: env.Generation,
		LastTransitionTime: metav1.Now(),
	})
	if dep.Status.ReadyReplicas != desired {
		setCondition(&env.Status.Conditions, metav1.Condition{
			Type:               platformv1alpha1.ConditionTypeProgressing,
			Status:             metav1.ConditionTrue,
			Reason:             "Deploying",
			Message:            "Waiting for desired replicas to become ready",
			ObservedGeneration: env.Generation,
			LastTransitionTime: metav1.Now(),
		})
	}
	setCondition(&env.Status.Conditions, metav1.Condition{
		Type:               platformv1alpha1.ConditionTypeDegraded,
		Status:             metav1.ConditionFalse,
		Reason:             "Healthy",
		Message:            "No reconciliation errors detected",
		ObservedGeneration: env.Generation,
		LastTransitionTime: metav1.Now(),
	})
	return r.Status().Update(ctx, env)
}

func (r *ApplicationEnvironmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.ApplicationEnvironment{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Complete(r)
}

func desiredReplicas(env *platformv1alpha1.ApplicationEnvironment) int32 {
	if env.Spec.Replicas == nil || *env.Spec.Replicas < 1 {
		return 1
	}
	return *env.Spec.Replicas
}

func desiredImage(env *platformv1alpha1.ApplicationEnvironment) string {
	return fmt.Sprintf("%s:%s", env.Spec.Image.Repository, env.Spec.Image.Tag)

}

func desiredPullPolicy(env *platformv1alpha1.ApplicationEnvironment) corev1.PullPolicy {
	if env.Spec.Image.PullPolicy == "" {
		return corev1.PullIfNotPresent
	}
	return env.Spec.Image.PullPolicy
}

func desiredServicePort(env *platformv1alpha1.ApplicationEnvironment) int32 {
	if env.Spec.Service.Port < 1 || env.Spec.Service.Port > 65535 {
		return 8080
	}
	return env.Spec.Service.Port
}

func desiredServiceType(env *platformv1alpha1.ApplicationEnvironment) corev1.ServiceType {
	if env.Spec.Service.Type == "" {
		return corev1.ServiceTypeClusterIP
	}
	return env.Spec.Service.Type
}

func desiredHPAMinReplicas(env *platformv1alpha1.ApplicationEnvironment) int32 {
	if env.Spec.Autoscaling == nil || env.Spec.Autoscaling.MinReplicas == nil || *env.Spec.Autoscaling.MinReplicas < 1 {
		return 1
	}
	return *env.Spec.Autoscaling.MinReplicas
}

func desiredHPAMaxReplicas(env *platformv1alpha1.ApplicationEnvironment) int32 {
	if env.Spec.Autoscaling == nil || env.Spec.Autoscaling.MaxReplicas < 1 {
		return 5
	}
	min := desiredHPAMinReplicas(env)
	if env.Spec.Autoscaling.MaxReplicas < min {
		return min
	}
	return env.Spec.Autoscaling.MaxReplicas
}

func desiredCPUUtilization(env *platformv1alpha1.ApplicationEnvironment) *int32 {
	defaultCPU := int32(80)
	if env.Spec.Autoscaling == nil || env.Spec.Autoscaling.TargetCPUUtilizationPercentage == nil {
		return &defaultCPU
	}
	cpu := *env.Spec.Autoscaling.TargetCPUUtilizationPercentage
	if cpu < 1 || cpu > 100 {
		return &defaultCPU
	}
	return &cpu
}

func stableMapKeys(input map[string]string) []string {
	keys := make([]string, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func managedLabels(env *platformv1alpha1.ApplicationEnvironment) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       env.Name,
		"app.kubernetes.io/managed-by": "kubernetes-platform-operator",
		"platform.example.com/owner":   env.Name,
		"platform.example.com/env":     env.Spec.Environment,
	}
}

func setCondition(conditions *[]metav1.Condition, condition metav1.Condition) {
	apimeta.SetStatusCondition(conditions, condition)
}

func ptrBool(v bool) *bool {
	return &v
}
