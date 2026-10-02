// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ThunderIDResourceStatus defines the observed state of ThunderIDResource.
type ThunderIDResourceStatus struct {
	// +optional
	Phase string `json:"phase,omitempty"`

	// +optional
	ID string `json:"id,omitempty"`

	// +optional
	Synced bool `json:"synced,omitempty"`

	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// SyncedResourceType is the spec.resource_type value last successfully synced to a ConfigMap —
	// not necessarily the current spec.resource_type. Lets the controller detect a resource_type
	// change (and rebuild the old type's ConfigMap to remove the stale entry, not just the new
	// type's) and know which ConfigMap to clean up on deletion even if spec.resource_type has
	// since been edited to something invalid or unknown.
	// +optional
	SyncedResourceType string `json:"syncedResourceType,omitempty"`

	// SyncedInstance is the ThunderIDInstance name (thunderid.io/instance label value) last
	// successfully synced to — not necessarily the current label value. Mirrors
	// SyncedResourceType's reasoning: lets the controller detect the object being moved to a
	// different instance (and remove the stale entry from the old instance's ConfigMap, not just
	// stop writing to it) and know which instance to clean up on deletion even if the label has
	// since been changed or points at an instance that's gone.
	// +optional
	SyncedInstance string `json:"syncedInstance,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Instance",type="string",JSONPath=".metadata.labels['thunderid\\.io/instance']"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.resource_type"
// +kubebuilder:printcolumn:name="Synced",type="boolean",JSONPath=".status.synced"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"

// ThunderIDResource is the generic ThunderID resource kind — its spec is ThunderID's own
// exported bootstrap document, pasted in essentially as-is. Paste a document like:
//
//	resource_type: application
//	id: 01900000-0000-7000-8000-000000000060
//	ouId: 01900000-0000-7000-8000-000000000001
//	name: Console
//	...
//
// directly under spec (dropping the id: line — the operator generates that itself). The
// operator reads spec.resource_type to know which ConfigMap this belongs in, and forwards it to
// ThunderID along with everything else — ThunderID's own bootstrap loader needs that same field
// to identify which resource kind a given YAML block in resources.yaml represents. Every other
// key, including ones this operator has never heard of, passes through completely untouched —
// including a `{{.VAR}}` token in any field, resolved by ThunderID's own live resources.yaml
// loader at boot against whatever Secret the EnvironmentValues labeled thunderid.io/instance=<the
// owning ThunderIDInstance> provides.
type ThunderIDResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Spec runtime.RawExtension `json:"spec,omitempty"`

	Status ThunderIDResourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ThunderIDResourceList contains a list of ThunderIDResource.
type ThunderIDResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ThunderIDResource `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &ThunderIDResource{}, &ThunderIDResourceList{})
		return nil
	})
}
