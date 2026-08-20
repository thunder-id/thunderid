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

package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// Job types.
const (
	// JobTypeImport applies a configuration bundle to a Data Plane.
	JobTypeImport = "import"
	// JobTypeSecretPut stores one credential on a Data Plane.
	JobTypeSecretPut = "secret_put"
	// JobTypeSecretNames asks a Data Plane which credentials it holds. It is a read, queued for the
	// same reason the writes are: only the pod holding the connection can ask.
	JobTypeSecretNames = "secret_names"
)

// Job statuses.
const (
	JobPending = "pending"
	JobClaimed = "claimed"
	JobDone    = "done"
	JobFailed  = "failed"
)

// Job is one request for a Data Plane and, once delivered, what it answered.
//
// A Control Plane pod can only speak to the Data Planes that dialed it. Writing the request down
// before delivering it means a pod holding no link can still accept the request, and the pod that
// does hold one carries it out.
type Job struct {
	ID           string `json:"id"`
	DeploymentID string `json:"-"`
	DataPlaneID  string `json:"dataPlaneId"`
	EnvID        string `json:"envId,omitempty"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	// Payload is the request, and Encrypted reports whether it is sealed. It carries a credential for
	// a secret_put, which is why it can be.
	Payload   []byte `json:"-"`
	Encrypted bool   `json:"-"`
	// Result is what the Data Plane answered, as stored JSON. Error explains a delivery that failed.
	Result   string `json:"result,omitempty"`
	Error    string `json:"error,omitempty"`
	Attempts int    `json:"attempts"`
}

// EnqueueJob records work for this deployment's Data Plane. The stored job, with its assigned id, is
// returned: that id is what a caller collects the answer with.
func (s *Store) EnqueueJob(ctx context.Context, job Job) (Job, error) {
	if job.ID == "" {
		id, err := newJobID()
		if err != nil {
			return Job{}, err
		}
		job.ID = id
	}
	job.DeploymentID = s.deploymentID
	job.Status = JobPending

	dbClient, err := s.client()
	if err != nil {
		return Job{}, err
	}

	encrypted := "0"
	if job.Encrypted {
		encrypted = "1"
	}
	if _, err := dbClient.ExecuteContext(ctx, queryEnqueueJob,
		s.deploymentID, job.ID, job.DataPlaneID, job.EnvID, job.Type,
		string(job.Payload), encrypted, job.Status); err != nil {
		return Job{}, fmt.Errorf("failed to queue work for the data plane: %w", err)
	}
	return job, nil
}

// GetJob returns one of this deployment's jobs.
func (s *Store) GetJob(ctx context.Context, id string) (Job, error) {
	dbClient, err := s.client()
	if err != nil {
		return Job{}, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetJob, s.deploymentID, id)
	if err != nil {
		return Job{}, fmt.Errorf("failed to read the job: %w", err)
	}
	if len(results) == 0 {
		return Job{}, ErrNotFound
	}
	row := results[0]
	return Job{
		ID:           string(toBytes(row["id"])),
		DeploymentID: s.deploymentID,
		DataPlaneID:  string(toBytes(row["data_plane_id"])),
		EnvID:        string(toBytes(row["env_id"])),
		Type:         string(toBytes(row["type"])),
		Status:       string(toBytes(row["status"])),
		Result:       string(toBytes(row["result"])),
		Error:        string(toBytes(row["error"])),
		Attempts:     parseInt(row["attempts"]),
	}, nil
}

// ClaimNextJob takes the next piece of work for a Data Plane, for the pod named by claimedBy. It
// reports false when there is nothing to do, or when another pod claimed it first.
//
// It reaches across deployments, because a pod holds connections for whichever Data Planes dialed
// it rather than for one organization.
func ClaimNextJob(ctx context.Context, dataPlaneID, claimedBy string) (Job, bool, error) {
	dbClient, err := provider.GetDBProvider().GetEnvironmentDBClient()
	if err != nil {
		return Job{}, false, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryNextPendingJob, dataPlaneID)
	if err != nil {
		return Job{}, false, fmt.Errorf("failed to look for queued work: %w", err)
	}
	if len(results) == 0 {
		return Job{}, false, nil
	}

	row := results[0]
	job := Job{
		DeploymentID: string(toBytes(row["deployment_id"])),
		ID:           string(toBytes(row["id"])),
		DataPlaneID:  dataPlaneID,
		EnvID:        string(toBytes(row["env_id"])),
		Type:         string(toBytes(row["type"])),
		Payload:      toBytes(row["payload"]),
		Encrypted:    string(toBytes(row["encrypted"])) == "1",
		Status:       JobClaimed,
	}

	// The update is the claim. Another pod that got there first has already moved the row out of
	// pending, so it changes nothing here and this pod simply looks again later.
	affected, err := dbClient.ExecuteContext(ctx, queryClaimJob, job.DeploymentID, job.ID, claimedBy)
	if err != nil {
		return Job{}, false, fmt.Errorf("failed to claim queued work: %w", err)
	}
	if affected == 0 {
		return Job{}, false, nil
	}
	return job, true, nil
}

// CompleteJob records the outcome of a delivery. A non-empty failure is recorded as the reason.
func CompleteJob(ctx context.Context, deploymentID, id, result, failure string) error {
	status := JobDone
	if failure != "" {
		status = JobFailed
	}

	dbClient, err := provider.GetDBProvider().GetEnvironmentDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryCompleteJob,
		deploymentID, id, status, result, failure); err != nil {
		return fmt.Errorf("failed to record the job outcome: %w", err)
	}
	return nil
}

// ReleaseJob returns a claimed job to the queue, for a delivery that could not be attempted. A
// delivery that was attempted and failed is recorded with CompleteJob instead.
func ReleaseJob(ctx context.Context, deploymentID, id string) error {
	dbClient, err := provider.GetDBProvider().GetEnvironmentDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryReleaseJob, deploymentID, id); err != nil {
		return fmt.Errorf("failed to release the job: %w", err)
	}
	return nil
}

// newJobID returns an identifier for a job.
func newJobID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate a job id: %w", err)
	}
	return "job-" + hex.EncodeToString(b), nil
}

// ClaimNextJob takes the next piece of work for a Data Plane. It is the package function reached
// through a Store, so a caller holding one needs nothing else.
func (s *Store) ClaimNextJob(ctx context.Context, dataPlaneID, claimedBy string) (Job, bool, error) {
	return ClaimNextJob(ctx, dataPlaneID, claimedBy)
}

// CompleteJob records the outcome of a delivery.
func (s *Store) CompleteJob(ctx context.Context, deploymentID, id, result, failure string) error {
	return CompleteJob(ctx, deploymentID, id, result, failure)
}

// ReleaseJob returns a claimed job to the queue.
func (s *Store) ReleaseJob(ctx context.Context, deploymentID, id string) error {
	return ReleaseJob(ctx, deploymentID, id)
}
