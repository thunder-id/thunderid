/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package store persists environments and their version history to the environment database.
// Version history is bounded: each environment retains its current version plus up to KeepPrevious
// older versions (and always the currently-applied version, even if it falls outside that window).
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/thunder-id/thunderid/internal/envmgr/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// KeepPrevious is how many previous versions to retain in addition to the current one.
const KeepPrevious = 3

// ErrNotFound is returned when an environment or version does not exist.
var ErrNotFound = errors.New("not found")

// Store holds one deployment's environments and their captured versions.
//
// The deployment is the organization, not one of its environments: a deployment id names an
// environment as "<org>:<env>", and promotion compares one environment against another, so the
// whole chain an organization promotes through belongs to a single store.
//
// Rows are the shared state every Control Plane replica reads and writes, so nothing is cached here.
type Store struct {
	deploymentID string
}

// New returns a store scoped to a deployment.
func New(deploymentID string) (*Store, error) {
	if deploymentID == "" {
		return nil, errors.New("a deployment id is required to open an environment store")
	}
	return &Store{deploymentID: deploymentID}, nil
}

// client resolves the environment datasource. It is resolved per call rather than held, so building
// a store for a deployment does not open a connection before anything asks it for one.
func (s *Store) client() (provider.DBClientInterface, error) {
	dbClient, err := provider.GetDBProvider().GetEnvironmentDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	return dbClient, nil
}

// Deployments lists every deployment that has at least one environment.
//
// It reaches across deployments on purpose, and is the only thing here that does: seeding a new
// tenant means finding which organization's chain already manages the tenant being copied from,
// which cannot be answered from inside one store.
func Deployments(ctx context.Context) ([]string, error) {
	dbClient, err := provider.GetDBProvider().GetEnvironmentDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryListDeployments)
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	ids := make([]string, 0, len(results))
	for _, row := range results {
		if id := string(toBytes(row["deployment_id"])); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// ---- environments ----

// SaveEnvironment inserts or updates an environment.
func (s *Store) SaveEnvironment(ctx context.Context, env model.Environment) error {
	raw, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("failed to encode environment: %w", err)
	}

	dbClient, err := s.client()
	if err != nil {
		return err
	}

	if _, err := dbClient.QueryContext(ctx, querySaveEnvironment,
		s.deploymentID, env.ID, string(raw)); err != nil {
		return fmt.Errorf("failed to save environment: %w", err)
	}
	return nil
}

// GetEnvironment returns an environment by id.
func (s *Store) GetEnvironment(ctx context.Context, id string) (model.Environment, error) {
	dbClient, err := s.client()
	if err != nil {
		return model.Environment{}, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetEnvironment, s.deploymentID, id)
	if err != nil {
		return model.Environment{}, fmt.Errorf("failed to read environment: %w", err)
	}
	if len(results) == 0 {
		return model.Environment{}, ErrNotFound
	}
	return decodeEnvironment(results[0]["data"])
}

// ListEnvironments returns all environments ordered by rank then name.
func (s *Store) ListEnvironments(ctx context.Context) ([]model.Environment, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(ctx, queryListEnvironments, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}

	out := make([]model.Environment, 0, len(results))
	for _, row := range results {
		env, err := decodeEnvironment(row["data"])
		if err != nil {
			return nil, err
		}
		out = append(out, env)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rank != out[j].Rank {
			return out[i].Rank < out[j].Rank
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// DeleteEnvironment removes an environment and all of its versions.
func (s *Store) DeleteEnvironment(ctx context.Context, id string) error {
	if _, err := s.GetEnvironment(ctx, id); err != nil {
		return err
	}

	dbClient, err := s.client()
	if err != nil {
		return err
	}

	// Versions are removed by the foreign key, which cascades.
	if _, err := dbClient.QueryContext(ctx, queryDeleteEnvironment, s.deploymentID, id); err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}
	return nil
}

// NextRank returns a rank one above the current maximum, for placing a new environment at the top of
// the chain by default.
func (s *Store) NextRank(ctx context.Context) (int, error) {
	envs, err := s.ListEnvironments(ctx)
	if err != nil {
		return 0, err
	}
	max := 0
	for _, env := range envs {
		if env.Rank > max {
			max = env.Rank
		}
	}
	return max + 1, nil
}

// ---- versions ----

// AddVersion assigns the next sequence to v, persists it, and prunes old versions. The stored version
// (with its assigned Seq) is returned.
//
// The sequence is read and then written rather than assigned by the database. Two captures of the
// same environment at the same instant therefore collide on the primary key, and the second is
// refused: the history stays correct and the caller can capture again.
func (s *Store) AddVersion(ctx context.Context, v model.Version) (model.Version, error) {
	env, err := s.GetEnvironment(ctx, v.EnvID)
	if err != nil {
		return model.Version{}, err
	}

	seqs, err := s.versionSeqs(ctx, v.EnvID)
	if err != nil {
		return model.Version{}, err
	}
	next := 1
	if len(seqs) > 0 {
		next = seqs[len(seqs)-1] + 1
	}
	v.Seq = next

	raw, err := json.Marshal(v)
	if err != nil {
		return model.Version{}, fmt.Errorf("failed to encode version: %w", err)
	}

	dbClient, err := s.client()
	if err != nil {
		return model.Version{}, err
	}
	if _, err := dbClient.QueryContext(ctx, queryInsertVersion,
		s.deploymentID, v.EnvID, v.Seq, string(raw)); err != nil {
		return model.Version{}, fmt.Errorf("failed to store version: %w", err)
	}

	if err := s.prune(ctx, v.EnvID, env.AppliedSeq); err != nil {
		return model.Version{}, err
	}
	return v, nil
}

// GetVersion returns a full version (including resources and variables).
func (s *Store) GetVersion(ctx context.Context, envID string, seq int) (model.Version, error) {
	dbClient, err := s.client()
	if err != nil {
		return model.Version{}, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetVersion, s.deploymentID, envID, seq)
	if err != nil {
		return model.Version{}, fmt.Errorf("failed to read version: %w", err)
	}
	if len(results) == 0 {
		return model.Version{}, ErrNotFound
	}
	return decodeVersion(results[0]["data"])
}

// ListVersions returns version metadata (payload stripped) newest first.
func (s *Store) ListVersions(ctx context.Context, envID string) ([]model.Version, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(ctx, queryListVersions, s.deploymentID, envID)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}

	out := make([]model.Version, 0, len(results))
	for _, row := range results {
		v, err := decodeVersion(row["data"])
		if err != nil {
			return nil, err
		}
		v.Resources = ""
		v.Variables = nil
		out = append(out, v)
	}
	return out, nil
}

// LatestSeq returns the highest version sequence for an environment, or 0 if none exist.
func (s *Store) LatestSeq(ctx context.Context, envID string) (int, error) {
	seqs, err := s.versionSeqs(ctx, envID)
	if err != nil {
		return 0, err
	}
	if len(seqs) == 0 {
		return 0, nil
	}
	return seqs[len(seqs)-1], nil
}

// ---- internals ----

// versionSeqs returns the existing version sequences for an environment in ascending order.
func (s *Store) versionSeqs(ctx context.Context, envID string) ([]int, error) {
	dbClient, err := s.client()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(ctx, queryVersionSeqs, s.deploymentID, envID)
	if err != nil {
		return nil, fmt.Errorf("failed to list version sequences: %w", err)
	}

	seqs := make([]int, 0, len(results))
	for _, row := range results {
		seqs = append(seqs, parseInt(row["seq"]))
	}
	return seqs, nil
}

// prune keeps the newest KeepPrevious+1 versions plus the applied version, removing the rest.
func (s *Store) prune(ctx context.Context, envID string, appliedSeq int) error {
	seqs, err := s.versionSeqs(ctx, envID)
	if err != nil {
		return err
	}
	keep := map[int]bool{}
	for i := len(seqs) - 1; i >= 0 && len(keep) < KeepPrevious+1; i-- {
		keep[seqs[i]] = true
	}
	if appliedSeq > 0 {
		keep[appliedSeq] = true
	}

	dbClient, err := s.client()
	if err != nil {
		return err
	}
	for _, seq := range seqs {
		if keep[seq] {
			continue
		}
		if _, err := dbClient.QueryContext(ctx, queryDeleteVersion, s.deploymentID, envID, seq); err != nil {
			return fmt.Errorf("failed to prune version %d: %w", seq, err)
		}
	}
	return nil
}

// decodeEnvironment parses a stored environment document.
func decodeEnvironment(value interface{}) (model.Environment, error) {
	var env model.Environment
	if err := json.Unmarshal(toBytes(value), &env); err != nil {
		return model.Environment{}, fmt.Errorf("failed to parse environment: %w", err)
	}
	return env, nil
}

// decodeVersion parses a stored version document.
func decodeVersion(value interface{}) (model.Version, error) {
	var v model.Version
	if err := json.Unmarshal(toBytes(value), &v); err != nil {
		return model.Version{}, fmt.Errorf("failed to parse version: %w", err)
	}
	return v, nil
}

// toBytes coerces a text column value, which a driver may hand back as either form.
func toBytes(value interface{}) []byte {
	switch v := value.(type) {
	case []byte:
		return v
	case string:
		return []byte(v)
	default:
		return nil
	}
}

// parseInt coerces an integer column value, which a driver may widen.
func parseInt(value interface{}) int {
	switch v := value.(type) {
	case int64:
		return int(v)
	case int32:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}
