package v1alpha1

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionTypeReady       = "Ready"
	ConditionTypeProgressing = "Progressing"
	ConditionTypeDegraded    = "Degraded"
)

type ApplicationEnvironmentImageSpec struct {
	Repository string            `json:"repository"`
	Tag        string            `json:"tag"`
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

type ApplicationEnvironmentServiceSpec struct {
	Type corev1.ServiceType `json:"type,omitempty"`
	Port int32              `json:"port,omitempty"`
}

type ApplicationEnvironmentAutoscalingSpec struct {
	Enabled                        bool                                           `json:"enabled,omitempty"`
	MinReplicas                    *int32                                         `json:"minReplicas,omitempty"`
	MaxReplicas                    int32                                          `json:"maxReplicas,omitempty"`
	TargetCPUUtilizationPercentage *int32                                         `json:"targetCPUUtilizationPercentage,omitempty"`
	Behavior                       *autoscalingv2.HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`
}

// ApplicationEnvironmentSpec captures the desired app environment state managed by the operator.
type ApplicationEnvironmentSpec struct {
	Environment string `json:"environment"`

	Replicas *int32 `json:"replicas,omitempty"`

	Image ApplicationEnvironmentImageSpec `json:"image"`

	Service ApplicationEnvironmentServiceSpec `json:"service,omitempty"`

	Config map[string]string `json:"config,omitempty"`

	Autoscaling *ApplicationEnvironmentAutoscalingSpec `json:"autoscaling,omitempty"`

	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// ApplicationEnvironmentStatus captures the observed state of the managed environment.
type ApplicationEnvironmentStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	ServiceName string `json:"serviceName,omitempty"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`

	LastReconciledTime *metav1.Time `json:"lastReconciledTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=appenv
// +kubebuilder:printcolumn:name="Environment",type=string,JSONPath=`.spec.environment`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image.repository`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type==\"Ready\")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="self.spec.image.repository.size() > 0",message="image.repository is required"
// +kubebuilder:validation:XValidation:rule="self.spec.image.tag.size() > 0",message="image.tag is required"
type ApplicationEnvironment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationEnvironmentSpec   `json:"spec,omitempty"`
	Status ApplicationEnvironmentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ApplicationEnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApplicationEnvironment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApplicationEnvironment{}, &ApplicationEnvironmentList{})
}
