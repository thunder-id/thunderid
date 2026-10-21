// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

const (
	databaseTypePostgres = "postgres"

	// databaseSecretKeyPassword is the fixed local filename each scope's password Secret key is
	// projected to inside its own Volume (see databaseVolumeName/databasePasswordMountPath) — the
	// arbitrary key name a scope's passwordRef.key points at in the source Secret is remapped to this
	// one name via the Volume's Items, so every scope's VolumeMount SubPath stays the same
	// regardless of what the user called the key. Also doubles as the email password Secret's
	// fixed key name (EmailSpec.SecretRef, unrelated to passwordRef, still always uses this literal key).
	databaseSecretKeyPassword = "password"

	sqliteConnOptionsDefault = "_journal_mode=WAL&_busy_timeout=5000&_pragma=foreign_keys(1)"

	// The four database scopes ThunderID's own deployment.yaml "database:" section has - see
	// DatabaseSpec's doc comment for why these four and not one shared connection.
	databaseScopeConfig            = "config"
	databaseScopeRuntimeTransient  = "runtime_transient"
	databaseScopeEntity            = "entity"
	databaseScopeRuntimePersistent = "runtime_persistent"
)

// databasePasswordMountPath is where scope's password (whatever Secret+key its passwordRef points at)
// is mounted into the container — one path per scope, since two scopes' passwordRef can point at
// different Secrets/keys entirely and so need to coexist as distinct files. Every postgres
// scope's rendered block references its own copy via a file:// URI, the same pattern
// deployment-config.yaml already uses for direct_auth_secret and crypto.key.
func databasePasswordMountPath(scope string) string {
	return "config/secrets/db_password_" + scope
}

// databaseVolumeName is the Pod Volume name backing scope's password mount — one Volume per
// scope (not deduplicated by Secret name, even when two scopes share a Secret): each needs its
// own Items mapping to select its own key, so sharing a Volume would gain nothing. Volume names
// are RFC 1123 DNS labels (lowercase alphanumeric and '-' only), so scope's underscores (e.g.
// "runtime_transient") are swapped for hyphens — the same characters databaseScope* already use
// otherwise, just not valid here verbatim.
func databaseVolumeName(scope string) string {
	return "database-" + strings.ReplaceAll(scope, "_", "-")
}

// databaseScopeSecret pairs one postgres scope with the Secret+key its password comes from —
// collectDatabaseScopeSecrets' per-scope result, consumed by resolveDatabaseConfig (to validate
// and fetch) and by reconcileDeployment/createBootstrapJob (to build that scope's Volume/Mount).
type databaseScopeSecret struct {
	scope string
	ref   appsv1alpha1.SecretKeyRef
}

// databaseScopes returns db's four scopes paired with their JSON-facing names, in the fixed
// order ThunderID's own deployment.yaml "database:" section uses them — the single place that
// order is defined, so every caller iterating scopes (resolveDatabaseConfig,
// collectDatabaseScopeSecrets) agrees.
func databaseScopes(db *appsv1alpha1.DatabaseSpec) []struct {
	name string
	spec appsv1alpha1.DatabaseBackendSpec
} {
	return []struct {
		name string
		spec appsv1alpha1.DatabaseBackendSpec
	}{
		{databaseScopeConfig, db.Config},
		{databaseScopeRuntimeTransient, db.RuntimeTransient},
		{databaseScopeEntity, db.Entity},
		{databaseScopeRuntimePersistent, db.RuntimePersistent},
	}
}

// collectDatabaseScopeSecrets returns every postgres scope's Secret+key reference — nil when
// spec.config.database is unset or has no postgres scope. Used directly by
// reconcileDeployment/createBootstrapJob to build each scope's Volume/VolumeMount;
// resolveDatabaseConfig uses it too, then additionally fetches and validates each Secret.
func collectDatabaseScopeSecrets(instance *appsv1alpha1.ThunderIDInstance) []databaseScopeSecret {
	db := instance.Spec.Config.Database
	if db == nil {
		return nil
	}
	var out []databaseScopeSecret
	for _, s := range databaseScopes(db) {
		if s.spec.Type == databaseTypePostgres && s.spec.Postgres != nil && s.spec.Postgres.PasswordRef != nil {
			out = append(out, databaseScopeSecret{scope: s.name, ref: *s.spec.Postgres.PasswordRef})
		}
	}
	return out
}

// databaseScopeDefaultPath is the sqlite file each scope falls back to when its own
// spec.config.database.<scope>.sqlite.path is unset — the image's own convention, and what
// deployment-config.yaml's static default section (staticSQLiteDatabaseSection) already uses.
var databaseScopeDefaultPath = map[string]string{
	databaseScopeConfig:            "repository/database/configdb.db",
	databaseScopeRuntimeTransient:  "repository/database/runtime_transient.db",
	databaseScopeEntity:            "repository/database/entitydb.db",
	databaseScopeRuntimePersistent: "repository/database/runtime_persistent.db",
}

// staticSQLiteDatabaseSection is the literal "database:" body deployment-config.yaml carries by
// default (sqlite, the image's own built-in behavior) — the template owns this content directly,
// not the operator. resolveDatabaseConfig only comes into play once spec.config.database is set,
// where it needs this exact text as the search target to swap out wholesale for the rendered
// scopes. Must stay byte-identical to deployment-config.yaml's database: body (minus the
// "database:" header line, which stays put either way).
const staticSQLiteDatabaseSection = `  config:
    type: "sqlite"
    sqlite:
      path: "repository/database/configdb.db"
      options: "_journal_mode=WAL&_busy_timeout=5000&_pragma=foreign_keys(1)"
      max_open_conns: 500
      max_idle_conns: 100
      conn_max_lifetime: 3600
  runtime_transient:
    type: "sqlite"
    sqlite:
      path: "repository/database/runtime_transient.db"
      options: "_journal_mode=WAL&_busy_timeout=5000&_pragma=foreign_keys(1)"
      max_open_conns: 500
      max_idle_conns: 100
      conn_max_lifetime: 3600
  entity:
    type: "sqlite"
    sqlite:
      path: "repository/database/entitydb.db"
      options: "_journal_mode=WAL&_busy_timeout=5000&_pragma=foreign_keys(1)"
      max_open_conns: 500
      max_idle_conns: 100
      conn_max_lifetime: 3600
  runtime_persistent:
    type: "sqlite"
    sqlite:
      path: "repository/database/runtime_persistent.db"
      options: "_journal_mode=WAL&_busy_timeout=5000&_pragma=foreign_keys(1)"
      max_open_conns: 500
      max_idle_conns: 100
      conn_max_lifetime: 3600`

// usingPostgres reports whether instance is configured with every database scope pointed at
// PostgreSQL — meaning no local sqlite files exist anywhere, so spec.dataVolumeSize becomes
// unnecessary. A partial/mixed configuration (or spec.config.database unset entirely, which means
// all-sqlite) still needs it, since at least one scope's sqlite file needs somewhere to persist.
func usingPostgres(instance *appsv1alpha1.ThunderIDInstance) bool {
	db := instance.Spec.Config.Database
	if db == nil {
		return false
	}
	return db.Config.Type == databaseTypePostgres &&
		db.RuntimeTransient.Type == databaseTypePostgres &&
		db.Entity.Type == databaseTypePostgres &&
		db.RuntimePersistent.Type == databaseTypePostgres
}

// resolveDatabaseConfig returns "" when spec.config.database is unset (deployment-config.yaml's
// static sqlite section already correct, nothing to substitute), or the full "database:" body —
// all four scopes, mirroring ThunderID's own deployment.yaml shape 1:1 — to swap in for
// staticSQLiteDatabaseSection. scopeSecrets is every postgres scope's Secret+key reference,
// already validated to exist and hold a non-empty value — reconcileDeployment/createBootstrapJob
// resolve the identical list themselves (via collectDatabaseScopeSecrets) to build the actual
// Volumes/Mounts, since spec is already in hand there and re-deriving it is cheaper than
// threading this return value through both.
func (r *ThunderIDInstanceReconciler) resolveDatabaseConfig(
	ctx context.Context, instance *appsv1alpha1.ThunderIDInstance) (block string, scopeSecrets []databaseScopeSecret, err error) {
	db := instance.Spec.Config.Database
	if db == nil {
		return "", nil, nil
	}

	scopeSecrets = collectDatabaseScopeSecrets(instance)
	fetched := make(map[string]*corev1.Secret, len(scopeSecrets))
	for _, ss := range scopeSecrets {
		if ss.ref.Name == "" || ss.ref.Key == "" {
			return "", nil, fmt.Errorf("spec.config.database.%s has type postgres but postgres.passwordRef.name/postgres.passwordRef.key is not set", scopeJSONName(ss.scope))
		}
		secret, ok := fetched[ss.ref.Name]
		if !ok {
			secret = &corev1.Secret{}
			if err := r.Get(ctx, types.NamespacedName{
				Name: ss.ref.Name, Namespace: instance.Namespace,
			}, secret); err != nil {
				return "", nil, fmt.Errorf("spec.config.database.%s.postgres.passwordRef %q: %w", scopeJSONName(ss.scope), ss.ref.Name, err)
			}
			fetched[ss.ref.Name] = secret
		}
		if string(secret.Data[ss.ref.Key]) == "" {
			return "", nil, fmt.Errorf("spec.config.database.%s.postgres.passwordRef: Secret %q is missing required key %q",
				scopeJSONName(ss.scope), ss.ref.Name, ss.ref.Key)
		}
	}
	// Every postgres scope needs its own passwordRef even when Type is postgres but Postgres/PasswordRef
	// itself is nil (collectDatabaseScopeSecrets silently skips that case, since it doesn't know
	// a scope's Type without also checking it) — catch that here explicitly.
	for _, s := range databaseScopes(db) {
		if s.spec.Type == databaseTypePostgres && (s.spec.Postgres == nil || s.spec.Postgres.PasswordRef == nil) {
			return "", nil, fmt.Errorf("spec.config.database.%s has type postgres but postgres.passwordRef is not set", scopeJSONName(s.name))
		}
	}

	// No "database:\n" header here — staticSQLiteDatabaseSection (the search target in
	// reconcileConfigMap) excludes the header line too, since the template keeps it fixed and
	// only the body underneath gets swapped.
	var sb strings.Builder
	for _, s := range databaseScopes(db) {
		scopeBlock, err := renderDatabaseScope(s.name, s.spec)
		if err != nil {
			return "", nil, fmt.Errorf("spec.config.database.%s: %w", scopeJSONName(s.name), err)
		}
		fmt.Fprintf(&sb, "  %s:\n%s\n", s.name, scopeBlock)
	}

	return strings.TrimRight(sb.String(), "\n"), scopeSecrets, nil
}

// scopeJSONName maps a rendered scope's snake_case name (ThunderID's own convention) to the CRD's
// camelCase field name, for error messages.
func scopeJSONName(scope string) string {
	switch scope {
	case databaseScopeRuntimeTransient:
		return "runtimeTransient"
	case databaseScopeRuntimePersistent:
		return "runtimePersistent"
	default:
		return scope
	}
}

// renderDatabaseScope returns the 4-space-indented "type:"/"sqlite:"/"postgres:" body for one
// database scope.
func renderDatabaseScope(scopeName string, spec appsv1alpha1.DatabaseBackendSpec) (string, error) {
	if spec.Type == databaseTypePostgres {
		pg := spec.Postgres
		if pg == nil {
			return "", fmt.Errorf("type is %q but postgres is not set", databaseTypePostgres)
		}
		var missing []string
		if pg.Hostname == "" {
			missing = append(missing, "postgres.hostname")
		}
		if pg.Username == "" {
			missing = append(missing, "postgres.username")
		}
		if pg.Name == "" {
			missing = append(missing, "postgres.name")
		}
		if len(missing) > 0 {
			return "", fmt.Errorf("missing required field(s): %s", strings.Join(missing, ", "))
		}

		port := pg.Port
		if port == "" {
			port = "5432"
		}
		if _, err := strconv.Atoi(port); err != nil {
			return "", fmt.Errorf("postgres.port %q is not a number", port)
		}
		sslmode := pg.SSLMode
		if sslmode == "" {
			sslmode = "require"
		}
		maxOpen := pg.MaxOpenConns
		if maxOpen == 0 {
			maxOpen = 500
		}
		maxIdle := pg.MaxIdleConns
		if maxIdle == 0 {
			maxIdle = 100
		}
		connLifetime := pg.ConnMaxLifetime
		if connLifetime == 0 {
			connLifetime = 3600
		}

		return fmt.Sprintf(`    type: "postgres"
    postgres:
      hostname: %q
      port: %s
      username: %q
      password: "file://%s"
      name: %q
      sslmode: %q
      max_open_conns: %d
      max_idle_conns: %d
      conn_max_lifetime: %d`, pg.Hostname, port, pg.Username, databasePasswordMountPath(scopeName), pg.Name, sslmode,
			maxOpen, maxIdle, connLifetime), nil
	}

	// sqlite — default when Type is empty or "sqlite".
	path := databaseScopeDefaultPath[scopeName]
	options := sqliteConnOptionsDefault
	maxOpen, maxIdle, connLifetime := int32(500), int32(100), int32(3600)
	if spec.SQLite != nil {
		if spec.SQLite.Path != "" {
			path = spec.SQLite.Path
		}
		if spec.SQLite.Options != "" {
			options = spec.SQLite.Options
		}
		if spec.SQLite.MaxOpenConns != 0 {
			maxOpen = spec.SQLite.MaxOpenConns
		}
		if spec.SQLite.MaxIdleConns != 0 {
			maxIdle = spec.SQLite.MaxIdleConns
		}
		if spec.SQLite.ConnMaxLifetime != 0 {
			connLifetime = spec.SQLite.ConnMaxLifetime
		}
	}

	return fmt.Sprintf(`    type: "sqlite"
    sqlite:
      path: %q
      options: %q
      max_open_conns: %d
      max_idle_conns: %d
      conn_max_lifetime: %d`, path, options, maxOpen, maxIdle, connLifetime), nil
}
