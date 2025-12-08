/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CleanupPolicySpec defines the desired state of CleanupPolicy.
type CleanupPolicySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	
	// Foo is an example field of CleanupPolicy. Edit cleanuppolicy_types.go to remove/update
	// Foo string `json:"foo,omitempty"`

	// Schedule in Cron format for cleanup execution (default: daily at 6 AM)
	// +kubebuilder:default="0 6 * * *"

	Schedule string `json:"schedule,omitempty"`

	// DryRun mode - if true, only log actions without actual deletion
	// +kubebuilder:default=false
	DryRun bool `json:"dryRun,omitempty"`

	// RequireApproval - if true, mark resources for deletion but wait for manual approval
	// +kubebuilder:default=false
	RequireApproval bool `json:"requireApproval,omitempty"`

	// Namespaces to include (empty means all namespaces)
	// +optional
	IncludeNamespaces []string `json:"includeNamespaces,omitempty"`

	// Namespaces to exclude from cleanup
	// +kubebuilder:default={"kube-system","kube-public","kube-node-lease"}
	ExcludeNamespaces []string `json:"excludeNamespaces,omitempty"`

	// Pod cleanup policies
	// +optional
	PodPolicies *PodCleanupPolicy `json:"podPolicies,omitempty"`

	// PersistentVolume cleanup policies
	// +optinal
	PVPolicies *PVCleanupPolicy `json:"pvPolicies,omitempty"`
}

// PodCleanupPolicy defines cleanup rules for Pods
type PodCleanupPolicy struct {
	// Enable pod cleanup
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Clean up failed pods (CrashLoopBackOff etc.)
	// +optional
	FailedPods *FailedPodPolicy `json:"failedPods,omitempty"`

	// Clean up idle pods (low resource usage)
	// +optional
	IdlePods *IdlePodPolicy `json:"idlePods,omitempty"`
}

// FailedPodPolicy defines rules for failed pod cleanup
type FailedPodPolicy struct {
	// Enable failed pod cleanup
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Status to clean up
	// +kubebuilder:default={"CrashLoopBackOff","ImagePullBackOff","Pending","Error","Evicted","Failed"}
	States []string `json:"states,omitempty"`

	// Minimum age before cleanup (duration format: 1h, 24h, 7d)
	// +kubebuilder:default="3h"
	MinAge string `json:"minAge,omitempty"`
}

type IdlePodPolicy struct {
	// Enable idle pod cleanup
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// CPU Usage threshold percentage (e.g. 80 means < 80%)
	// +kubebuilder:default=80
	CPUThresholdPercent int `json:"cpuThresholdPercent,omitempty"`

	// Memory Usage threshold percentage
	// +kubebuilder:default=80
	MemoryThresholdPercent int `json:"memoryThresholdPercent,omitempty"`

	// Duration pod must be idle before cleanup (e.g. 14d)
	// +kubebuilder:default="14d"
	IdleDuration string `json:"idleDuration,omitempty"`
}

// PVCleanupPolicy defines cleanup rules for PersistentVolumes
type PVCleanupPolicy struct {
	// Enable PV cleanup
	// +kubebuilder:default=true
	Enabled bool `json:"true,omitempty"`

	// Minimum age for unused PVs (e.g. "14d")
	// +kubebuilder:default="14d"
	MinAge string `json:"minAge,omitempty"`

	// Only clean PVs in these states
	// +kubebuilder:deafault={"Released","Available"}
	States []string `json:"states,omitempty"`
}

// CleanupPolicyStatus defines the observed state of CleanupPolicy
type CleanupPolicyStatus struct {
	// Last execution time
	LastExecutionTime *metav1.Time `json:"lastExecutionTime,omitempty"`

	// Next scheduled execution time
	NextExecutionTime *metav1.Time `json:"nextExecutionTime,omitempty"`

	// Total resources cleaned up
	CleanedUp int `json:"cleanedUp,omitempty"`

	// Conditions represet the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Resources pending approval
	// +optional
	PendingApproval []PendingResource `json:"pendingApproval,omitempty"`
}

// PendingResource represents a resource waiting for approval
type PendingResource struct {
	// Resource type (Pod, PV, etc.)
	Kind string `json:"kind"`

	// Resource namespace
	Namespace string `json:"namespace,omitempty"`

	// Resource name
	Name string `json:"name"`

	// Reason for cleanup
	Reason string `json:"reason"`

	// Time marked for cleanup
	MarkedAt metav1.Time `json:"markedAt"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="DryRun",type="boolean",JSONPath=".spec.dryRun"
//+kubebuilder:printcolumn:name="Last Execution",type="date",JSONPath=".status.lastExecutionTime"
//+kubebuilder:printcolumn:name="Cleaned Up",type="integer",JSONPath=".status.cleanedUp"

// CleanupPolicy is the Schema for the cleanuppolicies API.
type CleanupPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CleanupPolicySpec   `json:"spec,omitempty"`
	Status CleanupPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CleanupPolicyList contains a list of CleanupPolicy.
type CleanupPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CleanupPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CleanupPolicy{}, &CleanupPolicyList{})
}
