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

package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/thunder-id/thunderid/internal/envmgr/model"
	"github.com/thunder-id/thunderid/internal/envmgr/store"
)

// memStore is an in-memory Store for the service tests.
//
// It mirrors what the database-backed store guarantees, including the version pruning, so these
// tests exercise the service rather than a database.
type memStore struct {
	envs     map[string]model.Environment
	versions map[string]map[int]model.Version
	jobs     map[string]store.Job
	jobSeq   int
}

func newMemStore() *memStore {
	return &memStore{
		envs:     map[string]model.Environment{},
		versions: map[string]map[int]model.Version{},
		jobs:     map[string]store.Job{},
	}
}

func (m *memStore) SaveEnvironment(_ context.Context, env model.Environment) error {
	m.envs[env.ID] = env
	return nil
}

func (m *memStore) GetEnvironment(_ context.Context, id string) (model.Environment, error) {
	env, ok := m.envs[id]
	if !ok {
		return model.Environment{}, store.ErrNotFound
	}
	return env, nil
}

func (m *memStore) ListEnvironments(_ context.Context) ([]model.Environment, error) {
	out := make([]model.Environment, 0, len(m.envs))
	for _, env := range m.envs {
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

func (m *memStore) DeleteEnvironment(_ context.Context, id string) error {
	if _, ok := m.envs[id]; !ok {
		return store.ErrNotFound
	}
	delete(m.envs, id)
	delete(m.versions, id)
	return nil
}

func (m *memStore) NextRank(_ context.Context) (int, error) {
	max := 0
	for _, env := range m.envs {
		if env.Rank > max {
			max = env.Rank
		}
	}
	return max + 1, nil
}

func (m *memStore) AddVersion(_ context.Context, v model.Version) (model.Version, error) {
	env, ok := m.envs[v.EnvID]
	if !ok {
		return model.Version{}, store.ErrNotFound
	}
	seqs := m.seqs(v.EnvID)
	next := 1
	if len(seqs) > 0 {
		next = seqs[len(seqs)-1] + 1
	}
	v.Seq = next
	if m.versions[v.EnvID] == nil {
		m.versions[v.EnvID] = map[int]model.Version{}
	}
	m.versions[v.EnvID][v.Seq] = v

	keep := map[int]bool{}
	all := m.seqs(v.EnvID)
	for i := len(all) - 1; i >= 0 && len(keep) < store.KeepPrevious+1; i-- {
		keep[all[i]] = true
	}
	if env.AppliedSeq > 0 {
		keep[env.AppliedSeq] = true
	}
	for _, seq := range all {
		if !keep[seq] {
			delete(m.versions[v.EnvID], seq)
		}
	}
	return v, nil
}

func (m *memStore) GetVersion(_ context.Context, envID string, seq int) (model.Version, error) {
	v, ok := m.versions[envID][seq]
	if !ok {
		return model.Version{}, store.ErrNotFound
	}
	return v, nil
}

func (m *memStore) ListVersions(_ context.Context, envID string) ([]model.Version, error) {
	seqs := m.seqs(envID)
	out := make([]model.Version, 0, len(seqs))
	for i := len(seqs) - 1; i >= 0; i-- {
		v := m.versions[envID][seqs[i]]
		v.Resources = ""
		v.Variables = nil
		out = append(out, v)
	}
	return out, nil
}

func (m *memStore) LatestSeq(_ context.Context, envID string) (int, error) {
	seqs := m.seqs(envID)
	if len(seqs) == 0 {
		return 0, nil
	}
	return seqs[len(seqs)-1], nil
}

// seqs returns an environment's stored sequences in ascending order.
func (m *memStore) seqs(envID string) []int {
	seqs := make([]int, 0, len(m.versions[envID]))
	for seq := range m.versions[envID] {
		seqs = append(seqs, seq)
	}
	sort.Ints(seqs)
	return seqs
}

// ---- queued work ----

func (m *memStore) EnqueueJob(_ context.Context, job store.Job) (store.Job, error) {
	if job.ID == "" {
		m.jobSeq++
		job.ID = fmt.Sprintf("job-%d", m.jobSeq)
	}
	job.Status = store.JobPending
	if m.jobs == nil {
		m.jobs = map[string]store.Job{}
	}
	m.jobs[job.ID] = job
	return job, nil
}

func (m *memStore) GetJob(_ context.Context, id string) (store.Job, error) {
	job, ok := m.jobs[id]
	if !ok {
		return store.Job{}, store.ErrNotFound
	}
	return job, nil
}

func (m *memStore) ClaimNextJob(_ context.Context, dataPlaneID, claimedBy string) (store.Job, bool, error) {
	for id, job := range m.jobs {
		if job.DataPlaneID != dataPlaneID || job.Status != store.JobPending {
			continue
		}
		job.Status = store.JobClaimed
		job.Attempts++
		m.jobs[id] = job
		return job, true, nil
	}
	return store.Job{}, false, nil
}

func (m *memStore) CompleteJob(_ context.Context, _, id, result, failure string) error {
	job, ok := m.jobs[id]
	if !ok {
		return store.ErrNotFound
	}
	job.Status = store.JobDone
	if failure != "" {
		job.Status = store.JobFailed
	}
	job.Result, job.Error = result, failure
	m.jobs[id] = job
	return nil
}

func (m *memStore) ReleaseJob(_ context.Context, _, id string) error {
	if job, ok := m.jobs[id]; ok {
		job.Status = store.JobPending
		m.jobs[id] = job
	}
	return nil
}

// fakeSealer stands in for the server's encryption. It reverses the bytes, which is enough to prove
// a queued credential is transformed on the way in and restored on the way out.
type fakeSealer struct{}

func (fakeSealer) Seal(_ context.Context, plaintext []byte) ([]byte, error) {
	return reversed(plaintext), nil
}

func (fakeSealer) Open(_ context.Context, sealed []byte) ([]byte, error) {
	return reversed(sealed), nil
}

func reversed(in []byte) []byte {
	out := make([]byte, len(in))
	for i, b := range in {
		out[len(in)-1-i] = b
	}
	return out
}
