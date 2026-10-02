// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// SQLiteBackendSpec configures one database scope's SQLite connection — mirrors ThunderID's own
// database.<scope>.sqlite: block field-for-field. Any field left unset falls back to the same
// default the scope uses when spec.config.database is omitted entirely.
type SQLiteBackendSpec struct {
	// +optional
	Path string `json:"path,omitempty"`
	// +optional
	Options string `json:"options,omitempty"`
	// +optional
	MaxOpenConns int32 `json:"maxOpenConns,omitempty"`
	// +optional
	MaxIdleConns int32 `json:"maxIdleConns,omitempty"`
	// +optional
	ConnMaxLifetime int32 `json:"connMaxLifetime,omitempty"`
}

// PostgresBackendSpec configures one database scope's PostgreSQL connection — mirrors
// ThunderID's own database.<scope>.postgres: block, except password: PasswordRef backs it, since
// ThunderID's own password field is always a file:// reference, never inline.
type PostgresBackendSpec struct {
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// +optional
	Port string `json:"port,omitempty"`
	// +optional
	Username string `json:"username,omitempty"`
	// +optional
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Enum=require;verify-full;verify-ca;disable
	// +optional
	SSLMode string `json:"sslmode,omitempty"`
	// +optional
	MaxOpenConns int32 `json:"maxOpenConns,omitempty"`
	// +optional
	MaxIdleConns int32 `json:"maxIdleConns,omitempty"`
	// +optional
	ConnMaxLifetime int32 `json:"connMaxLifetime,omitempty"`

	// PasswordRef names the Secret+key holding this scope's PostgreSQL password. Required (this scope's
	// DatabaseBackendSpec.Type is "postgres" whenever Postgres itself is set). K8s-only glue, not
	// a ThunderID field. Scopes are independent: point two scopes' PasswordRef at the same Secret with
	// different Key values to give each its own password out of one Secret object, or at the same
	// Secret+Key to share one password across them — the operator mounts exactly what each scope's
	// PasswordRef says, nothing more.
	// +optional
	PasswordRef *SecretKeyRef `json:"passwordRef,omitempty"`
}

// SecretKeyRef names one key within one Secret in the ThunderIDInstance's own namespace — the
// k8s-only glue for a single credential value. Unlike corev1.SecretKeySelector this has no
// "optional" flag: a SecretKeyRef is only ever set when the value it points at is required, so a
// missing Secret or key is always a config error, surfaced the same way any other one is.
type SecretKeyRef struct {
	// Name of the Secret, in the same namespace as the ThunderIDInstance.
	Name string `json:"name"`
	// Key within the Secret holding the value.
	Key string `json:"key"`
}

// DatabaseBackendSpec is one database scope. ThunderID has four (config, runtime_transient,
// entity, runtime_persistent), each independently typed — see DatabaseSpec.
type DatabaseBackendSpec struct {
	// +kubebuilder:validation:Enum=sqlite;postgres
	// +optional
	Type string `json:"type,omitempty"`

	// Set when type is "sqlite" (or unset).
	// +optional
	SQLite *SQLiteBackendSpec `json:"sqlite,omitempty"`

	// Set when type is "postgres".
	// +optional
	Postgres *PostgresBackendSpec `json:"postgres,omitempty"`
}

// DatabaseSpec mirrors ThunderID's own deployment.yaml "database:" section 1:1 — four
// independent scopes, each with its own backend, rather than one connection reused everywhere
// (config+entity always share a database in practice, and so do runtime_transient+
// runtime_persistent, but ThunderID's own schema doesn't encode that pairing, so neither does
// this). When spec.config.database is omitted entirely, deployment-config.yaml's static sqlite
// section stands untouched (see staticSQLiteDatabaseSection in database_config.go) — this type
// only comes into play once at least one scope is set.
type DatabaseSpec struct {
	// +optional
	Config DatabaseBackendSpec `json:"config,omitempty"`
	// +optional
	RuntimeTransient DatabaseBackendSpec `json:"runtimeTransient,omitempty"`
	// +optional
	Entity DatabaseBackendSpec `json:"entity,omitempty"`
	// +optional
	RuntimePersistent DatabaseBackendSpec `json:"runtimePersistent,omitempty"`
}

// TLSConfigSpec mirrors ThunderID's own deployment.yaml "tls:" section — only min_version is
// user-configurable; cert_file/key_file are fixed, operator-managed paths.
type TLSConfigSpec struct {
	// MinVersion is the minimum TLS version ThunderID's own HTTPS listener accepts. Unrelated to
	// spec.tlsSecret, which is the Ingress's TLS termination — this is the app's own listener.
	// +kubebuilder:validation:Enum="1.2";"1.3"
	// +kubebuilder:default="1.3"
	// +optional
	MinVersion string `json:"minVersion,omitempty"`
}

// JWTConfigSpec mirrors ThunderID's own deployment.yaml "jwt:" section.
type JWTConfigSpec struct {
	// PreferredKeyID selects which of ThunderID's two built-in signing keys ("default-key", RSA,
	// or "ecdsa-key", ECDSA) it signs new tokens with. Both remain valid for verifying
	// already-issued tokens regardless of this setting.
	// +kubebuilder:validation:Enum=default-key;ecdsa-key
	// +kubebuilder:default="default-key"
	// +optional
	PreferredKeyID string `json:"preferredKeyId,omitempty"`
}

// PasskeyConfigSpec mirrors ThunderID's own deployment.yaml "passkey:" section.
type PasskeyConfigSpec struct {
	// ExtraAllowedOrigins adds WebAuthn/passkey origins beyond the instance's own public URL
	// (spec.domain, or https://localhost:<port> when unset), which the operator always includes
	// automatically — e.g. a separate frontend origin that also needs to register/verify passkeys
	// against this instance.
	// +optional
	ExtraAllowedOrigins []string `json:"extraAllowedOrigins,omitempty"`
}

// SMTPConfigSpec mirrors ThunderID's own deployment.yaml "email.smtp:" section, except password:
// SecretRef backs it, since ThunderID's own password field is always a file:// reference.
type SMTPConfigSpec struct {
	// +kubebuilder:validation:Required
	Host string `json:"host"`

	// +kubebuilder:validation:Required
	Port int32 `json:"port"`

	// Username authenticates to the SMTP server. Required when EnableAuthentication is true,
	// ignored otherwise.
	// +optional
	Username string `json:"username,omitempty"`

	// +kubebuilder:validation:Required
	FromAddress string `json:"fromAddress"`

	// EnableStartTLS upgrades the SMTP connection to TLS via STARTTLS.
	// +optional
	EnableStartTLS bool `json:"enableStartTLS,omitempty"`

	// EnableAuthentication authenticates to the SMTP server using Username/SecretRef.
	// +optional
	EnableAuthentication bool `json:"enableAuthentication,omitempty"`

	// SecretRef names the Secret+key holding the SMTP password. Required when
	// EnableAuthentication is true, ignored otherwise. K8s-only glue, not a ThunderID field. Same
	// Secret object a spec.config.database scope's postgres.passwordRef uses is fine too — just a
	// different Key.
	// +optional
	SecretRef *SecretKeyRef `json:"secretRef,omitempty"`
}

// EmailSpec configures the SMTP client ThunderID uses for outbound email (registration
// confirmation, password reset, etc). Unset (the default) means no email section is written into
// deployment.yaml at all, matching the image's own out-of-the-box behavior.
type EmailSpec struct {
	// +kubebuilder:validation:Required
	SMTP SMTPConfigSpec `json:"smtp"`
}

// AutoScalingSpec configures a HorizontalPodAutoscaler for the Deployment - see reconcileHPA.
type AutoScalingSpec struct {
	MinReplicas                       int32 `json:"minReplicas"`
	MaxReplicas                       int32 `json:"maxReplicas"`
	TargetCPUUtilizationPercentage    int32 `json:"targetCPUUtilizationPercentage,omitempty"`
	TargetMemoryUtilizationPercentage int32 `json:"targetMemoryUtilizationPercentage,omitempty"`
}

// AppConfigSpec groups every field that maps directly into ThunderID's own deployment.yaml —
// as opposed to ThunderIDInstanceSpec's other fields (image, replicas, resources, autoScaling,
// domain, tlsSecret, securitySecret, resourcesConfigMap, dataVolumeSize), which are Kubernetes/
// operator-level concerns deployment.yaml has no notion of at all.
type AppConfigSpec struct {
	// Database mirrors ThunderID's own "database:" section — see DatabaseSpec. Unset means the
	// image's built-in sqlite default stands untouched. When unset, or set but not every scope
	// resolves to postgres, spec.dataVolumeSize must be set (at least one local sqlite file needs
	// somewhere to persist).
	// +optional
	Database *DatabaseSpec `json:"database,omitempty"`

	// Port is the port ThunderID listens on inside the pod — also used for the container port,
	// all three probes, the Service, and the Ingress backend, so it only needs setting once here.
	// Dual-purpose (Kubernetes wiring too), so it stays flat here rather than nested under a
	// mirrored "server:" block like the rest of this struct.
	// +kubebuilder:default=8090
	// +optional
	Port int32 `json:"port,omitempty"`

	// TLS mirrors ThunderID's own "tls:" section.
	// +optional
	TLS TLSConfigSpec `json:"tls,omitempty"`

	// JWT mirrors ThunderID's own "jwt:" section.
	// +optional
	JWT JWTConfigSpec `json:"jwt,omitempty"`

	// Passkey mirrors ThunderID's own "passkey:" section.
	// +optional
	Passkey PasskeyConfigSpec `json:"passkey,omitempty"`

	// Email mirrors ThunderID's own "email:" section. Unset means no email section in
	// deployment.yaml at all — see EmailSpec.
	// +optional
	Email *EmailSpec `json:"email,omitempty"`
}

// ThunderIDInstanceSpec defines the desired state of ThunderIDInstance
type ThunderIDInstanceSpec struct {

	// +kubebuilder:validation:Required
	Image string `json:"image"`

	Replicas int32 `json:"replicas,omitempty"`

	// Domain is the externally-reachable hostname for this instance. When set, the operator
	// creates an Ingress (Host: spec.domain, TLS via spec.tlsSecret if set) so ThunderID is
	// reachable without kubectl port-forward access — e.g. a real deployment behind a real
	// DNS record. Optional: when unset (the default, and what every local/dev workflow in
	// this repo actually uses), no Ingress is created and ThunderID is only reachable via its
	// Service (kubectl port-forward svc/<name> 8090:8090 -> https://localhost:8090).
	// +optional
	Domain string `json:"domain,omitempty"`

	// +optional
	TLSSecret string `json:"tlsSecret,omitempty"`

	// SecuritySecret names a pre-existing Secret holding ThunderID's TLS/JWT-signing/encryption
	// key material and bootstrap admin password (server.cert, server.key, signing.cert,
	// signing.key, ecdsa-signing.cert, ecdsa-signing.key, crypto.key, direct_auth_secret,
	// admin-password). When unset, the operator generates and manages this Secret itself as
	// "<name>-security".
	// +optional
	SecuritySecret string `json:"securitySecret,omitempty"`

	// +optional
	ResourcesConfigMap string `json:"resourcesConfigMap,omitempty"`

	// Environment variables mounted on the ThunderID container come from whichever EnvironmentValues
	// (or EnvironmentValues-adopted plain Secret) in this namespace carries the label
	// `thunderid.io/instance: <this instance's name>` — see EnvironmentValuesSpec's doc comment and
	// findEnvironment in env_secret.go. There is no field here naming it: the reference lives on
	// the EnvironmentValues side, the same way a ThunderIDResource finds its owning instance. Any field in any
	// ThunderIDResource's spec — a user's password, an application's OAuth clientSecret, anything — can
	// hold a `{{.KEY}}` token referencing one of that Secret's keys; ThunderID's own live
	// resources.yaml loader resolves it at boot. A `{{.VAR}}` token with no matching key fails
	// ThunderID's own boot with a clear "environment variable X is not set" error — nothing the
	// operator needs to validate ahead of time. At most one EnvironmentValues may carry a given
	// instance's label; more than one is a config error surfaced as an InvalidSpec event.

	// DataVolumeSize, when set (e.g. "1Gi"), provisions a ReadWriteOnce PersistentVolumeClaim
	// mounted at /opt/thunderid/repository/database (where the SQLite files live) and gates
	// setup.sh to run only once: the container entrypoint checks a marker file on that volume
	// and skips setup.sh on every restart after the first, instead of rerunning it (and wiping
	// the database) on every pod restart. Because the PVC is ReadWriteOnce, the Deployment
	// strategy switches to Recreate to avoid two pods mounting it at once — only meaningful with
	// replicas: 1. Either this or every Config.Database scope's Type=postgres must be set — a
	// ThunderIDInstance with neither is rejected: ephemeral, wipe-on-every-restart SQLite is not a
	// supported configuration.
	// +optional
	DataVolumeSize string `json:"dataVolumeSize,omitempty"`

	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// +optional
	AutoScaling *AutoScalingSpec `json:"autoScaling,omitempty"`

	// Config groups every field that maps directly into ThunderID's own deployment.yaml — see
	// AppConfigSpec. Kept separate from this struct's other, Kubernetes/operator-level fields.
	// +optional
	Config AppConfigSpec `json:"config,omitempty"`
}

// ThunderIDInstanceStatus defines the observed state of ThunderIDInstance.
type ThunderIDInstanceStatus struct {

	// +optional
	URL string `json:"url,omitempty"`

	// +optional
	Phase string `json:"phase,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ThunderIDInstance is the Schema for the thunderidinstances API
type ThunderIDInstance struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ThunderIDInstance
	// +required
	Spec ThunderIDInstanceSpec `json:"spec"`

	// status defines the observed state of ThunderIDInstance
	// +optional
	Status ThunderIDInstanceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ThunderIDInstanceList contains a list of ThunderIDInstance
type ThunderIDInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ThunderIDInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ThunderIDInstance{}, &ThunderIDInstanceList{})
		return nil
	})
}
