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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/envmgr/bundle"
	"github.com/thunder-id/thunderid/internal/envmgr/model"
	"github.com/thunder-id/thunderid/internal/envmgr/store"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// Secret kinds, matching the data plane's secret service.
const (
	// KindHash is a credential that is only ever checked against something a caller presents, so only a
	// one-way hash is stored and the original is never recoverable.
	KindHash = "hash"
	// KindValue is a credential the data plane has to replay to a third party, so it is stored as is.
	KindValue = "value"
)

// hashedFields are the credential fields whose value is only ever verified. Everything else, such as a
// gateway API key, is replayed to a third party and cannot be hashed.
var hashedFields = map[string]bool{
	"password":     true,
	"clientsecret": true,
	"flowsecret":   true,
}

// HashedSecret is a hashed credential in the form the secret service stores it.
type HashedSecret struct {
	Hash        string
	Algorithm   string
	Salt        string
	Iterations  int
	KeySize     int
	Memory      int
	Parallelism int
}

// SecretHasher hashes a credential. It is supplied by the server so a secret set here is hashed
// exactly the way the server hashes one it captures itself; without it, only replayable credentials
// can be written.
type SecretHasher func(value string) (HashedSecret, error)

// SetSecretHasher installs the hasher. It is separate from New because the hashing configuration
// belongs to the server, which builds the service.
func (s *Service) SetSecretHasher(h SecretHasher) {
	s.hasher = h
}

// SecretEntry is one secret-backed placeholder of an environment, with what is known about it.
type SecretEntry struct {
	Name string `json:"name"`
	// Field is the resource field the placeholder fills, e.g. clientSecret.
	Field string `json:"field,omitempty"`
	// ResourceType and ResourceName say which resource needs it, so an operator setting a value knows
	// what it belongs to.
	ResourceType string `json:"resourceType,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
	// Kind is "hash" for a credential that is only verified and "value" for one that is replayed.
	Kind string `json:"kind"`
	// Held is whether the data plane's secret service already has it. Meaningless when Checked is false.
	Held bool `json:"held"`
	// JobID and Status are set when this entry is the result of setting a credential. The value is
	// delivered by the pod holding the data plane's connection, which may not be the one that took
	// the request, so a "pending" status means the answer is collected later by this id.
	JobID  string `json:"jobId,omitempty"`
	Status string `json:"status,omitempty"`
}

// SecretList is every secret an environment's version needs, with its status on the data plane.
type SecretList struct {
	EnvID   string        `json:"envId"`
	Seq     int           `json:"seq"`
	Secrets []SecretEntry `json:"secrets"`
	// Checked is false when the data plane's secret service could not be reached, so a missing secret
	// cannot be told apart from an unreachable service.
	Checked bool `json:"checked"`
	// CheckError is why the secret service could not be answered for. It is reported because the usual
	// cause is this environment's own credentials or endpoint, not a data plane that is down, and those
	// look identical without it.
	CheckError string `json:"checkError,omitempty"`
	// PendingJobID is set when this pod could not ask the data plane and queued the question for one
	// that can. Following it and asking again is what turns Checked true, rather than the listing
	// being wrong until someone happens to reach the right pod.
	PendingJobID string `json:"pendingJobId,omitempty"`
}

// ListSecrets reports every secret-backed placeholder of an environment's latest version and whether
// the data plane holds it.
//
// The list is derived from the configuration rather than from the secret service, so a credential that
// was never captured still appears, which is the case an operator most needs to see.
func (s *Service) ListSecrets(ctx context.Context, envID string) (SecretList, error) {
	env, err := s.store.GetEnvironment(ctx, envID)
	if err != nil {
		return SecretList{}, err
	}
	seq, err := s.resolveSeq(ctx, env, "latest")
	if err != nil {
		return SecretList{}, err
	}
	if seq == 0 {
		return SecretList{}, ErrNoVersions
	}
	version, err := s.store.GetVersion(ctx, envID, seq)
	if err != nil {
		return SecretList{}, err
	}

	entries := secretEntriesOf(version)
	held, checkErr := s.heldSecrets(ctx, env)
	if checkErr == nil {
		for i := range entries {
			entries[i].Held = held[entries[i].Name]
		}
	}
	list := SecretList{EnvID: envID, Seq: seq, Secrets: entries, Checked: checkErr == nil}
	if checkErr != nil {
		list.CheckError = checkErr.Error()
		// The question was queued for the pod that can ask it, so this is "not yet" rather than
		// "unavailable", and the caller is told what to follow.
		var pending *pendingSecretNamesError
		if errors.As(checkErr, &pending) {
			list.PendingJobID = pending.jobID
		}
	}
	return list, nil
}

// SetSecret writes a credential to the environment's data plane under a placeholder name.
//
// Nothing is kept here: the value goes straight to the data plane's secret service, the same way a
// credential captured on the control plane does.
func (s *Service) SetSecret(ctx context.Context, envID, name, value string) (SecretEntry, error) {
	if strings.TrimSpace(name) == "" {
		return SecretEntry{}, fmt.Errorf("%w: a secret name is required", ErrValidation)
	}
	if value == "" {
		return SecretEntry{}, fmt.Errorf("%w: a value is required", ErrValidation)
	}
	env, err := s.store.GetEnvironment(ctx, envID)
	if err != nil {
		return SecretEntry{}, err
	}
	entry := s.describeSecret(ctx, envID, name)

	body, err := s.secretBody(entry.Kind, value, fmt.Sprintf("Set %s", name))
	if err != nil {
		return SecretEntry{}, err
	}
	job, err := s.queueSecret(ctx, env, name, body)
	if err != nil {
		return SecretEntry{}, err
	}
	if job.Status == store.JobFailed {
		return SecretEntry{}, fmt.Errorf("failed to store the secret on %s: %s", env.Name, job.Error)
	}
	// Held reports that the data plane has it. A job still pending has not reached it yet, so this
	// says so rather than claiming a credential is in place before it is.
	entry.Held = job.Status == store.JobDone
	entry.JobID = job.ID
	entry.Status = job.Status
	return entry, nil
}

// RegenerateSecret replaces a verifiable credential with a freshly generated one and returns the new
// value, which is the only time it can be read.
//
// Only a hashed credential can be generated: a replayed one, such as a gateway API key, is issued by
// the third party and a random value would simply be wrong.
func (s *Service) RegenerateSecret(ctx context.Context, envID, name string) (SecretEntry, string, error) {
	entry := s.describeSecret(ctx, envID, name)
	if entry.Kind != KindHash {
		return SecretEntry{}, "", fmt.Errorf(
			"%w: %s is replayed to a third party, so it has to be set to the value that party issued",
			ErrValidation, name)
	}

	value, err := generateSecretValue()
	if err != nil {
		return SecretEntry{}, "", err
	}
	stored, err := s.SetSecret(ctx, envID, name, value)
	if err != nil {
		return SecretEntry{}, "", err
	}
	return stored, value, nil
}

// secretBody builds the secret service's write payload for the credential.
func (s *Service) secretBody(kind, value, description string) (map[string]interface{}, error) {
	if kind != KindHash {
		return map[string]interface{}{"kind": KindValue, "value": value, "description": description}, nil
	}
	if s.hasher == nil {
		return nil, fmt.Errorf("%w: this server cannot hash a credential, so only replayable secrets "+
			"can be set here", ErrValidation)
	}
	hashed, err := s.hasher(value)
	if err != nil {
		return nil, fmt.Errorf("failed to hash the credential: %w", err)
	}
	return map[string]interface{}{
		"kind":        KindHash,
		"value":       hashed.Hash,
		"algorithm":   hashed.Algorithm,
		"description": description,
		"parameters": map[string]interface{}{
			"salt":        hashed.Salt,
			"iterations":  hashed.Iterations,
			"keySize":     hashed.KeySize,
			"memory":      hashed.Memory,
			"parallelism": hashed.Parallelism,
		},
	}, nil
}

// describeSecret finds what the environment's configuration says about a placeholder. A name the
// configuration does not mention still gets an entry, classified from the name, so a credential can be
// set before the version that needs it is captured.
func (s *Service) describeSecret(ctx context.Context, envID, name string) SecretEntry {
	if env, err := s.store.GetEnvironment(ctx, envID); err == nil {
		if seq, err := s.resolveSeq(ctx, env, "latest"); err == nil && seq > 0 {
			if version, err := s.store.GetVersion(ctx, envID, seq); err == nil {
				for _, entry := range secretEntriesOf(version) {
					if entry.Name == name {
						return entry
					}
				}
			}
		}
	}
	return SecretEntry{Name: name, Kind: kindFromName(name)}
}

// heldSecrets reads the names the data plane's secret service holds. A non-nil error means it could not
// be reached, so "not held" is not reported as fact.
func (s *Service) heldSecrets(ctx context.Context, env model.Environment) (map[string]bool, error) {
	plane, err := s.dataPlaneFor(env)
	if err != nil {
		return s.recordedSecretNames(ctx, env, err)
	}
	names, err := plane.SecretNames(ctx)
	if err != nil {
		return s.recordedSecretNames(ctx, env, fmt.Errorf("%s: %w", env.Name, err))
	}

	// This pod could ask, so record the answer for the pods that cannot.
	env.SecretNames = names
	env.SecretNamesAt = s.now().UTC()
	if saveErr := s.store.SaveEnvironment(ctx, env); saveErr != nil {
		// Recording is an optimization for other pods; the answer in hand is still good.
		log.GetLogger().Warn(ctx, "Failed to record the credentials a data plane holds",
			log.String("environment", env.Name), log.Error(saveErr))
	}

	held := make(map[string]bool, len(names))
	for _, name := range names {
		held[name] = true
	}
	return held, nil
}

// pendingSecretNamesError reports that the question was queued rather than answered, and carries the
// job to follow. It is an error because nothing is known yet, and callers must not read that as
// "nothing held".
type pendingSecretNamesError struct {
	jobID  string
	reason error
}

func (e *pendingSecretNamesError) Error() string { return e.reason.Error() }
func (e *pendingSecretNamesError) Unwrap() error { return e.reason }

// recordedSecretNames answers from what the data plane last reported, for a pod that cannot reach it.
//
// A control plane pod holds connections only to the data planes that dialed it, so being unable to
// ask is ordinary rather than a fault. Never having been told is different, and is reported as the
// original failure so that "not held" is not presented as fact.
func (s *Service) recordedSecretNames(ctx context.Context, env model.Environment,
	reason error) (map[string]bool, error) {
	if env.SecretNamesAt.IsZero() {
		// Nothing to answer from, so ask through the queue: the pod holding the connection carries
		// it out and records the answer, and the next request reads it here.
		if job, qErr := s.queueSecretNames(ctx, env); qErr == nil {
			return nil, &pendingSecretNamesError{jobID: job.ID, reason: reason}
		}
		return nil, reason
	}
	held := make(map[string]bool, len(env.SecretNames))
	for _, name := range env.SecretNames {
		held[name] = true
	}
	return held, nil
}

// secretEntriesOf classifies every secret-backed placeholder of a version.
//
// Classification needs both halves of the resource: the field says whether the credential is only ever
// verified, and the resource type says whether it still has to be replayed. A connection's clientSecret
// is sent to the upstream provider on every request, while an application's field of the same name is
// only ever compared, so the field name alone would hash a credential that has to stay readable.
func secretEntriesOf(version model.Version) []SecretEntry {
	entries := map[string]SecretEntry{}
	for _, resource := range bundle.Parse(version.Resources) {
		for _, placeholder := range bundle.SecretPlaceholders(resource.Content) {
			entries[placeholder.Name] = SecretEntry{
				Name:         placeholder.Name,
				Field:        placeholder.Field,
				ResourceType: resource.Type,
				ResourceName: resource.Name,
				Kind:         kindOf(resource.Type, placeholder.Field),
			}
		}
	}
	// A key the control plane recorded at capture time but the bundle no longer shows still has to be
	// listed: it is a credential the data plane holds, and hiding it would make it unmanageable.
	for _, name := range version.SecretKeys {
		if _, ok := entries[name]; !ok {
			entries[name] = SecretEntry{Name: name, Kind: kindFromName(name)}
		}
	}

	out := make([]SecretEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// kindOf decides how a credential of the given resource field has to be held.
func kindOf(resourceType, field string) string {
	// A connection's credentials are handed to the upstream provider, whatever they are called.
	if strings.EqualFold(resourceType, "connection") {
		return KindValue
	}
	if hashedFields[strings.ToLower(field)] {
		return KindHash
	}
	return KindValue
}

// kindFromName classifies a placeholder the configuration does not explain, using the resource type and
// field that the exporter encodes into every generated name (TYPE_NAME_FIELD).
func kindFromName(name string) string {
	upper := strings.ToUpper(name)
	if strings.HasPrefix(upper, "CONNECTION_") {
		return KindValue
	}
	for suffix := range hashedFields {
		if strings.HasSuffix(upper, "_"+strings.ToUpper(snakeOf(suffix))) {
			return KindHash
		}
	}
	return KindValue
}

// snakeOf turns a lowercase field name into the underscore form the exporter uses: clientsecret ->
// client_secret.
func snakeOf(field string) string {
	switch field {
	case "clientsecret":
		return "client_secret"
	case "flowsecret":
		return "flow_secret"
	default:
		return field
	}
}

// targetSecretOutcome reports what a promote did about the destination's credentials.
type targetSecretOutcome struct {
	// Generated and Reused are kept for compatibility with the previous shape. A promote no longer
	// touches credentials at all, so nothing is reported in either.
	Generated []string `json:"generated"`
	Reused    []string `json:"reused"`
	// Skipped names the credentials the destination has to hold for the promoted configuration to work.
	// They are set against the destination's own data plane, which is the only place they live.
	Skipped []string `json:"skipped"`
}

// generateSecretValue produces a fresh high-entropy credential.
func generateSecretValue() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate a secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
