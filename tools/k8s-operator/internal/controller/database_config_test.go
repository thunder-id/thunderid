// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

type DatabaseConfigTestSuite struct {
	suite.Suite
}

func TestDatabaseConfigTestSuite(t *testing.T) {
	suite.Run(t, new(DatabaseConfigTestSuite))
}

func (suite *DatabaseConfigTestSuite) TestUsingPostgres() {
	pg := appsv1alpha1.DatabaseBackendSpec{Type: "postgres"}
	sqlite := appsv1alpha1.DatabaseBackendSpec{Type: "sqlite"}

	cases := []struct {
		name string
		db   *appsv1alpha1.DatabaseSpec
		want bool
	}{
		{"nil database", nil, false},
		{"unset (all default sqlite)", &appsv1alpha1.DatabaseSpec{}, false},
		{"all four postgres", &appsv1alpha1.DatabaseSpec{Config: pg, RuntimeTransient: pg, Entity: pg, RuntimePersistent: pg}, true},
		{"mixed sqlite and postgres", &appsv1alpha1.DatabaseSpec{Config: pg, RuntimeTransient: sqlite, Entity: pg, RuntimePersistent: pg}, false},
		{"three of four postgres", &appsv1alpha1.DatabaseSpec{Config: pg, RuntimeTransient: pg, Entity: pg}, false},
	}
	for _, c := range cases {
		suite.Run(c.name, func() {
			instance := &appsv1alpha1.ThunderIDInstance{Spec: appsv1alpha1.ThunderIDInstanceSpec{
				Config: appsv1alpha1.AppConfigSpec{Database: c.db},
			}}
			suite.Equal(c.want, usingPostgres(instance))
		})
	}
}

func (suite *DatabaseConfigTestSuite) TestScopeJSONName() {
	cases := map[string]string{
		"runtime_transient":  "runtimeTransient",
		"runtime_persistent": "runtimePersistent",
		"config":             "config",
		"entity":             "entity",
	}
	for in, want := range cases {
		suite.Equal(want, scopeJSONName(in))
	}
}

func (suite *DatabaseConfigTestSuite) TestRenderDatabaseScopeSQLiteDefaults() {
	got, err := renderDatabaseScope("config", appsv1alpha1.DatabaseBackendSpec{})
	suite.Require().NoError(err)
	suite.Contains(got, `type: "sqlite"`)
	suite.Contains(got, `path: "repository/database/configdb.db"`)
	suite.Contains(got, "max_open_conns: 500")
}

func (suite *DatabaseConfigTestSuite) TestRenderDatabaseScopeSQLiteOverrides() {
	got, err := renderDatabaseScope("entity", appsv1alpha1.DatabaseBackendSpec{
		SQLite: &appsv1alpha1.SQLiteBackendSpec{
			Path: "/custom/path.db", Options: "_journal_mode=OFF", MaxOpenConns: 10, MaxIdleConns: 5, ConnMaxLifetime: 60,
		},
	})
	suite.Require().NoError(err)
	suite.Contains(got, `path: "/custom/path.db"`)
	suite.Contains(got, `options: "_journal_mode=OFF"`)
	suite.Contains(got, "max_open_conns: 10")
	suite.Contains(got, "max_idle_conns: 5")
	suite.Contains(got, "conn_max_lifetime: 60")
}

func (suite *DatabaseConfigTestSuite) TestRenderDatabaseScopeSQLiteDefaultOptionsWhenUnset() {
	got, err := renderDatabaseScope("entity", appsv1alpha1.DatabaseBackendSpec{
		SQLite: &appsv1alpha1.SQLiteBackendSpec{Path: "/custom/path.db"},
	})
	suite.Require().NoError(err)
	// Options was not overridden, so it must still fall back to the default.
	suite.Contains(got, "_journal_mode=WAL")
}

func (suite *DatabaseConfigTestSuite) TestRenderDatabaseScopePostgresRequiresFields() {
	_, err := renderDatabaseScope("config", appsv1alpha1.DatabaseBackendSpec{
		Type:     "postgres",
		Postgres: &appsv1alpha1.PostgresBackendSpec{},
	})
	suite.Require().Error(err)
	for _, field := range []string{"postgres.hostname", "postgres.username", "postgres.name"} {
		suite.Contains(err.Error(), field)
	}
}

func (suite *DatabaseConfigTestSuite) TestRenderDatabaseScopePostgresMissingBlock() {
	_, err := renderDatabaseScope("config", appsv1alpha1.DatabaseBackendSpec{Type: "postgres"})
	suite.Error(err)
}

func (suite *DatabaseConfigTestSuite) TestRenderDatabaseScopePostgresDefaults() {
	got, err := renderDatabaseScope("config", appsv1alpha1.DatabaseBackendSpec{
		Type: "postgres",
		Postgres: &appsv1alpha1.PostgresBackendSpec{
			Hostname: "db.example.com",
			Username: "thunderid",
			Name:     "thunderid_config",
		},
	})
	suite.Require().NoError(err)
	suite.Contains(got, `hostname: "db.example.com"`)
	suite.Contains(got, `port: 5432`)
	suite.Contains(got, `username: "thunderid"`)
	suite.Contains(got, `password: "file://`+databasePasswordMountPath("config")+`"`)
	suite.Contains(got, `sslmode: "require"`)
}

// A hostname/username/name/sslmode containing a double quote must not break out of its YAML
// double-quoted scalar - %q escapes it instead of %s splicing it in raw.
func (suite *DatabaseConfigTestSuite) TestRenderDatabaseScopePostgresEscapesQuotesInValues() {
	got, err := renderDatabaseScope("config", appsv1alpha1.DatabaseBackendSpec{
		Type: "postgres",
		Postgres: &appsv1alpha1.PostgresBackendSpec{
			Hostname: `db".example.com`,
			Username: "thunderid",
			Name:     "thunderid_config",
		},
	})
	suite.Require().NoError(err)
	suite.Contains(got, `hostname: "db\".example.com"`)
}

func (suite *DatabaseConfigTestSuite) TestRenderDatabaseScopePostgresRejectsNonNumericPort() {
	_, err := renderDatabaseScope("config", appsv1alpha1.DatabaseBackendSpec{
		Type: "postgres",
		Postgres: &appsv1alpha1.PostgresBackendSpec{
			Hostname: "db.example.com",
			Username: "thunderid",
			Name:     "thunderid_config",
			Port:     "5432; rm -rf /",
		},
	})
	suite.Require().Error(err)
	suite.Contains(err.Error(), "postgres.port")
}

// A SQLite path/options value containing a double quote must not break out of its YAML
// double-quoted scalar either.
func (suite *DatabaseConfigTestSuite) TestRenderDatabaseScopeSQLiteEscapesQuotesInValues() {
	got, err := renderDatabaseScope("config", appsv1alpha1.DatabaseBackendSpec{
		SQLite: &appsv1alpha1.SQLiteBackendSpec{Options: `_pragma=busy_timeout(5000)" extra: true`},
	})
	suite.Require().NoError(err)
	suite.Contains(got, `options: "_pragma=busy_timeout(5000)\" extra: true"`)
}

func (suite *DatabaseConfigTestSuite) TestResolveDatabaseConfigUnset() {
	r := newFakeReconciler()
	instance := &appsv1alpha1.ThunderIDInstance{}
	block, scopeSecrets, err := r.resolveDatabaseConfig(context.Background(), instance)
	suite.Require().NoError(err)
	suite.Empty(block)
	suite.Empty(scopeSecrets)
}

func (suite *DatabaseConfigTestSuite) TestResolveDatabaseConfigPostgresMissingPasswordRef() {
	r := newFakeReconciler()
	instance := &appsv1alpha1.ThunderIDInstance{Spec: appsv1alpha1.ThunderIDInstanceSpec{
		Config: appsv1alpha1.AppConfigSpec{Database: &appsv1alpha1.DatabaseSpec{
			Config: appsv1alpha1.DatabaseBackendSpec{Type: "postgres", Postgres: &appsv1alpha1.PostgresBackendSpec{
				Hostname: "h", Username: "u", Name: "n",
			}},
		}},
	}}
	_, _, err := r.resolveDatabaseConfig(context.Background(), instance)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "passwordRef")
	suite.Contains(err.Error(), "spec.config.database.config")
}

func (suite *DatabaseConfigTestSuite) TestResolveDatabaseConfigPostgresMissingPasswordRefKey() {
	r := newFakeReconciler()
	instance := &appsv1alpha1.ThunderIDInstance{Spec: appsv1alpha1.ThunderIDInstanceSpec{
		Config: appsv1alpha1.AppConfigSpec{Database: &appsv1alpha1.DatabaseSpec{
			Config: appsv1alpha1.DatabaseBackendSpec{Type: "postgres", Postgres: &appsv1alpha1.PostgresBackendSpec{
				Hostname: "h", Username: "u", Name: "n",
				PasswordRef: &appsv1alpha1.SecretKeyRef{Name: "db-secret"},
			}},
		}},
	}}
	_, _, err := r.resolveDatabaseConfig(context.Background(), instance)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "passwordRef")
}

func (suite *DatabaseConfigTestSuite) TestResolveDatabaseConfigPostgresSecretNotFound() {
	r := newFakeReconciler() // no Secret seeded at all
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			Config: appsv1alpha1.AppConfigSpec{Database: &appsv1alpha1.DatabaseSpec{
				Config: appsv1alpha1.DatabaseBackendSpec{Type: "postgres", Postgres: &appsv1alpha1.PostgresBackendSpec{
					Hostname: "h", Username: "u", Name: "n",
					PasswordRef: &appsv1alpha1.SecretKeyRef{Name: "does-not-exist", Key: "password"},
				}},
			}},
		},
	}
	_, _, err := r.resolveDatabaseConfig(context.Background(), instance)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "does-not-exist")
}

// TestResolveDatabaseConfigPostgresTypeWithNilPostgresBlock: type: postgres with no Postgres
// block set at all — PasswordRef now lives inside PostgresBackendSpec, so a nil Postgres block and a
// missing passwordRef are the same condition; the safety-net passwordRef check catches this before ever
// reaching renderDatabaseScope (whose own distinct "type is postgres but postgres is not set"
// error is exercised directly by TestRenderDatabaseScopePostgresMissingBlock instead).
func (suite *DatabaseConfigTestSuite) TestResolveDatabaseConfigPostgresTypeWithNilPostgresBlock() {
	r := newFakeReconciler()
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			Config: appsv1alpha1.AppConfigSpec{Database: &appsv1alpha1.DatabaseSpec{
				Config: appsv1alpha1.DatabaseBackendSpec{Type: "postgres"},
			}},
		},
	}
	_, _, err := r.resolveDatabaseConfig(context.Background(), instance)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "spec.config.database.config")
	suite.Contains(err.Error(), "passwordRef")
}

func (suite *DatabaseConfigTestSuite) TestResolveDatabaseConfigPostgresMissingPasswordKey() {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "db-secret", Namespace: "default"},
		Data:       map[string][]byte{"not-password": []byte("x")},
	}
	r := newFakeReconciler(secret)
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			Config: appsv1alpha1.AppConfigSpec{Database: &appsv1alpha1.DatabaseSpec{
				Config: appsv1alpha1.DatabaseBackendSpec{Type: "postgres", Postgres: &appsv1alpha1.PostgresBackendSpec{
					Hostname: "h", Username: "u", Name: "n",
					PasswordRef: &appsv1alpha1.SecretKeyRef{Name: "db-secret", Key: "password"},
				}},
			}},
		},
	}
	_, _, err := r.resolveDatabaseConfig(context.Background(), instance)
	suite.Require().Error(err)
	suite.Contains(err.Error(), `missing required key "password"`)
}

func (suite *DatabaseConfigTestSuite) TestResolveDatabaseConfigAllFourScopes() {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "db-secret", Namespace: "default"},
		Data:       map[string][]byte{"password": []byte("secret")},
	}
	r := newFakeReconciler(secret)
	pgSpec := appsv1alpha1.DatabaseBackendSpec{
		Type: "postgres", Postgres: &appsv1alpha1.PostgresBackendSpec{
			Hostname: "h", Username: "u", Name: "n",
			PasswordRef: &appsv1alpha1.SecretKeyRef{Name: "db-secret", Key: "password"},
		},
	}
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			Config: appsv1alpha1.AppConfigSpec{Database: &appsv1alpha1.DatabaseSpec{
				Config: pgSpec, RuntimeTransient: pgSpec, Entity: pgSpec, RuntimePersistent: pgSpec,
			}},
		},
	}
	block, scopeSecrets, err := r.resolveDatabaseConfig(context.Background(), instance)
	suite.Require().NoError(err)
	suite.Len(scopeSecrets, 4)
	for _, ss := range scopeSecrets {
		suite.Equal("db-secret", ss.ref.Name)
		suite.Equal("password", ss.ref.Key)
	}
	for _, scope := range []string{"config:", "runtime_transient:", "entity:", "runtime_persistent:"} {
		suite.Contains(block, scope)
	}
	// No leading/trailing blank lines - reconcileConfigMap swaps this in verbatim.
	suite.False(strings.HasPrefix(block, "\n"))
	suite.False(strings.HasSuffix(block, "\n"))
}

// TestResolveDatabaseConfigScopesWithDifferentKeysOfSameSecret is the actual feature this design
// exists for: two scopes sharing one Secret object but each with its own Key, giving each scope
// its own password without requiring a second Secret.
func (suite *DatabaseConfigTestSuite) TestResolveDatabaseConfigScopesWithDifferentKeysOfSameSecret() {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-db-secret", Namespace: "default"},
		Data:       map[string][]byte{"config-password": []byte("p1"), "runtime-password": []byte("p2")},
	}
	r := newFakeReconciler(secret)
	instance := &appsv1alpha1.ThunderIDInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec: appsv1alpha1.ThunderIDInstanceSpec{
			Config: appsv1alpha1.AppConfigSpec{Database: &appsv1alpha1.DatabaseSpec{
				Config: appsv1alpha1.DatabaseBackendSpec{
					Type: "postgres", Postgres: &appsv1alpha1.PostgresBackendSpec{
						Hostname: "h", Username: "u", Name: "n",
						PasswordRef: &appsv1alpha1.SecretKeyRef{Name: "shared-db-secret", Key: "config-password"},
					},
				},
				RuntimeTransient: appsv1alpha1.DatabaseBackendSpec{
					Type: "postgres", Postgres: &appsv1alpha1.PostgresBackendSpec{
						Hostname: "h", Username: "u", Name: "n",
						PasswordRef: &appsv1alpha1.SecretKeyRef{Name: "shared-db-secret", Key: "runtime-password"},
					},
				},
			}},
		},
	}
	_, scopeSecrets, err := r.resolveDatabaseConfig(context.Background(), instance)
	suite.Require().NoError(err)
	suite.Len(scopeSecrets, 2)
	byScope := map[string]databaseScopeSecret{}
	for _, ss := range scopeSecrets {
		byScope[ss.scope] = ss
	}
	suite.Equal("shared-db-secret", byScope["config"].ref.Name)
	suite.Equal("config-password", byScope["config"].ref.Key)
	suite.Equal("shared-db-secret", byScope["runtime_transient"].ref.Name)
	suite.Equal("runtime-password", byScope["runtime_transient"].ref.Key)
}
