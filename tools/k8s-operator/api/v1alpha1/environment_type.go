// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EnvironmentValuesSpec defines the desired state of EnvironmentValues.
type EnvironmentValuesSpec struct {
	// Env holds this environment's key-value pairs, mirrored into the generated Secret's Data.
	// Plaintext only until the next reconcile: the controller overwrites each key's value here
	// with a masked marker ("thunderid-masked:<up to 3 leading chars>**********") once it's
	// synced to the Secret, so only a short prefix (never the rest of the raw value, and never
	// its real length either) is visible via `kubectl get/describe environmentvalues` past that
	// point — see the controller. To change a key's value later, re-apply with new plaintext for
	// just that key; kubectl apply's 3-way merge leaves already-masked sibling keys untouched.
	// +optional
	Env map[string]string `json:"env,omitempty"`
}

// EnvironmentValuesStatus defines the observed state of EnvironmentValues.
type EnvironmentValuesStatus struct {
	// +optional
	Phase string `json:"phase,omitempty"`

	// +optional
	Synced bool `json:"synced,omitempty"`

	// SecretName is always the EnvironmentValues's own name — the generated Secret is 1:1 with it.
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// Keys lists the currently-synced key names only (never values) — for a developer checking
	// "did my key actually get picked up" without needing to know this becomes a Secret.
	// +optional
	Keys []string `json:"keys,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Secret",type="string",JSONPath=".status.secretName"
// +kubebuilder:printcolumn:name="Synced",type="boolean",JSONPath=".status.synced"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"

// EnvironmentValues mirrors spec.env into a Secret of the same name, then masks each synced key's
// value in place (see EnvironmentValuesSpec.Env) — see environment_controller.go. The object is never
// deleted: it stays gettable forever, just with masked values, so a developer who doesn't know
// this becomes a Secret never hits a surprise NotFound. The Secret it produces is deliberately
// not owned by this object (no controller
// reference) — nothing here ever deletes this object, but keeping them decoupled means a future
// change to one can't accidentally cascade-delete the other. Do NOT put this CRD under GitOps
// (Flux) management: a continuously reconciling GitOps tool enforces live-must-match-Git and
// will fight the masking forever (reverting it back to plaintext every reconcile) — see the
// GitOps (Flux) section of README.md.
//
// Labeling this object with InstanceRefLabel (thunderid.io/instance: <name>, same label a
// ThunderIDResource uses) makes that ThunderIDInstance auto-discover this EnvironmentValues's Secret and mount
// it whole via envFrom — see findEnvironment in env_secret.go. There is no field on
// ThunderIDInstanceSpec naming it the other way around: the reference lives here, on the
// EnvironmentValues, not on the instance. The label is optional — an unlabeled EnvironmentValues still syncs
// its Secret exactly the same, it's simply not wired into any instance's envFrom. Exactly one
// EnvironmentValues should carry a given instance's label;
// more than one is a config error the instance surfaces as an InvalidSpec event rather than
// picking one arbitrarily. A hand-written Secret with no matching EnvironmentValues object can still be
// discovered the same way: create a thin EnvironmentValues with that Secret's name, the label, and no
// spec.env (or spec.env omitted) — its reconcile then adopts the existing Secret without touching
// its contents (see upsertSecret's "adopted" path in environment_controller.go). The Secret must
// also carry EnvironmentValuesAdoptLabel (thunderid.io/environment-values: <this name>) or
// adoption is refused — matching by Secret name alone would let anyone who can create an
// EnvironmentValues overwrite keys in any same-named Secret in the namespace, including ones they
// have no RBAC permission to edit directly.
type EnvironmentValues struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec EnvironmentValuesSpec `json:"spec,omitempty"`

	Status EnvironmentValuesStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EnvironmentValuesList contains a list of EnvironmentValues.
type EnvironmentValuesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EnvironmentValues `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &EnvironmentValues{}, &EnvironmentValuesList{})
		return nil
	})
}
