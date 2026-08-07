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

// Package service holds the environment-management orchestration: capturing config into versions,
// diffing versions, applying a version to a data plane via the import API (create/update/delete), and
// promoting or reverting between versions.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/envmgr/auth"
	"github.com/thunder-id/thunderid/internal/envmgr/bundle"
	"github.com/thunder-id/thunderid/internal/envmgr/diff"
	"github.com/thunder-id/thunderid/internal/envmgr/model"
	"github.com/thunder-id/thunderid/internal/envmgr/store"
	"github.com/thunder-id/thunderid/internal/envmgr/thunder"
)

// ThunderClient is the subset of the ThunderID API the service depends on. It is an interface so tests
// can substitute a fake.
type ThunderClient interface {
	Export(ctx context.Context) (thunder.ExportResult, error)
	SecretKeys(ctx context.Context) ([]string, error)
	EnvironmentVariables(ctx context.Context) (map[string]string, error)
	Import(ctx context.Context, req thunder.ImportRequest) (*thunder.ImportResponse, error)
}

// ClientFactory builds a ThunderClient for a base URL.
type ClientFactory func(baseURL string, creds thunder.Credentials, insecure bool) ThunderClient

// callerCredentials presents the token of whoever is driving this request.
//
// The only server this service calls over HTTP is the control plane it runs inside, and always while
// serving a request, so forwarding the caller's own token is both sufficient and correct: the capture
// reads exactly what that caller is allowed to read. Data planes are reached over the channel they
// dial out on, and need no credential at all.
func callerCredentials(ctx context.Context) thunder.Credentials {
	return thunder.Credentials{Token: auth.CallerTokenFromContext(ctx)}
}

// Service is the environment-management application service.
type Service struct {
	store     *store.Store
	newClient ClientFactory
	now       func() time.Time
	// hasher hashes a credential that is only ever verified. It is nil until the server installs one.
	hasher SecretHasher
	// dataPlanes reaches the data planes connected to this control plane.
	dataPlanes DataPlanes
	// tokens issues the credential a data plane connects with.
	tokens DataPlaneTokenIssuer
	// localCP is the control plane hosting this service, when it hosts one. It is how a promote reaches
	// a tenant other than the caller's own.
	localCP LocalControlPlane
}

// LocalControlPlane writes configuration into a named tenant of the control plane this service runs in.
//
// It exists because a tenant is resolved from the caller's token: a promotion into another tenant has
// no token for that tenant and cannot get one, so it is carried out in process against the tenant the
// environment names.
type LocalControlPlane interface {
	// Hosts reports whether a base URL is this very server, so a genuinely remote control plane is
	// still reached over HTTP rather than written to locally by mistake.
	Hosts(baseURL string) bool
	Import(ctx context.Context, deploymentID string, req thunder.ImportRequest) (*thunder.ImportResponse, error)
}

// SetLocalControlPlane installs the control plane this service runs in. It is separate from New because
// the control plane's own services are built after this one.
func (s *Service) SetLocalControlPlane(cp LocalControlPlane) {
	s.localCP = cp
}

// New builds a Service.
func New(st *store.Store, factory ClientFactory) *Service {
	return &Service{store: st, newClient: factory, now: time.Now}
}

// Errors surfaced to the HTTP layer.
var (
	ErrNotFound     = store.ErrNotFound
	ErrValidation   = errors.New("invalid request")
	ErrNoSource     = errors.New("environment has no control-plane source configured")
	ErrPromotionFed = errors.New(
		"this environment receives its configuration by promotion, so there is nothing to capture")
	ErrNoVersions        = errors.New("environment has no versions")
	ErrNothingApplied    = errors.New("environment has nothing applied yet")
	ErrNoPreviousVersion = errors.New("environment has no previous version to revert to")
	ErrBadRef            = errors.New("invalid version reference")
	ErrNoPromotionPath   = errors.New("no promotion path exists between these environments")
	// ErrNoVersionSource is ErrNoSource under the name the capture path reads better with.
	ErrNoVersionSource = ErrNoSource
)

// ---- environments ----

// CreateEnvironmentInput is the input to CreateEnvironment.
type CreateEnvironmentInput struct {
	Name       string
	Rank       *int
	PromotesTo []string
	Target     model.Target
	Source     *model.Source
}

// CreateEnvironment registers a new environment.
func (s *Service) CreateEnvironment(ctx context.Context,
	in CreateEnvironmentInput) (CreateEnvironmentResult, error) {
	if strings.TrimSpace(in.Name) == "" {
		return CreateEnvironmentResult{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(in.Target.DataPlaneID) == "" {
		return CreateEnvironmentResult{}, fmt.Errorf("%w: target.dataPlaneId is required", ErrValidation)
	}
	rank := s.store.NextRank()
	if in.Rank != nil {
		rank = *in.Rank
	}
	now := s.now().UTC()
	env := model.Environment{
		ID:         newID("env"),
		Name:       in.Name,
		Rank:       rank,
		PromotesTo: in.PromotesTo,
		Target:     in.Target,
		Source:     in.Source,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := validateGraph(append(s.store.ListEnvironments(), env)); err != nil {
		return CreateEnvironmentResult{}, err
	}
	if err := s.store.SaveEnvironment(env); err != nil {
		return CreateEnvironmentResult{}, err
	}

	// The data plane needs a credential to connect with, and this is the only moment it is readable.
	// Issuing it here rather than asking for one means there is nothing for an operator to invent, and
	// nothing to configure on this side at all.
	token, err := s.issueDataPlaneToken(ctx, env)
	if err != nil {
		return CreateEnvironmentResult{}, err
	}
	return CreateEnvironmentResult{Environment: env, DataPlaneToken: token}, nil
}

// CreateEnvironmentResult is a registered environment and, once, the token its data plane connects
// with. The token is not stored in readable form and is never returned again.
type CreateEnvironmentResult struct {
	model.Environment
	// DataPlaneToken is empty when this control plane issues no tokens, which is the case for a
	// deployment using a single shared one configured on both sides.
	DataPlaneToken string `json:"dataPlaneToken,omitempty"`
}

// issueDataPlaneToken mints the credential the environment's data plane presents when it connects.
func (s *Service) issueDataPlaneToken(ctx context.Context, env model.Environment) (string, error) {
	if s.tokens == nil {
		return "", nil
	}
	deploymentID := ""
	if env.Source != nil {
		deploymentID = env.Source.DeploymentID
	}
	token, err := s.tokens.Issue(ctx, env.Target.DataPlaneID, deploymentID)
	if err != nil {
		return "", fmt.Errorf("failed to issue a token for %s: %w", env.Target.DataPlaneID, err)
	}
	return token, nil
}

// RegenerateDataPlaneToken issues a new token for an environment's data plane and returns it once.
//
// The previous token stops working immediately, so that data plane drops until the new one is in
// place. That is the honest behaviour for a rotation: a credential that still worked afterwards would
// not have been rotated.
func (s *Service) RegenerateDataPlaneToken(ctx context.Context, envID string) (string, error) {
	env, err := s.store.GetEnvironment(envID)
	if err != nil {
		return "", err
	}
	if s.tokens == nil {
		return "", fmt.Errorf("this server issues no data plane tokens")
	}
	return s.issueDataPlaneToken(ctx, env)
}

// UpdateEnvironmentEdges replaces an environment's outgoing promotion edges. Edges are set after the
// fact because the environments they point at generally do not exist yet when the first one is
// registered.
func (s *Service) UpdateEnvironmentEdges(envID string, promotesTo []string) (model.Environment, error) {
	env, err := s.store.GetEnvironment(envID)
	if err != nil {
		return model.Environment{}, err
	}
	env.PromotesTo = promotesTo
	env.UpdatedAt = s.now().UTC()

	envs := s.store.ListEnvironments()
	for i := range envs {
		if envs[i].ID == envID {
			envs[i] = env
		}
	}
	if err := validateGraph(envs); err != nil {
		return model.Environment{}, err
	}
	if err := s.store.SaveEnvironment(env); err != nil {
		return model.Environment{}, err
	}
	return env, nil
}

// GetEnvironment returns an environment.
func (s *Service) GetEnvironment(id string) (model.Environment, error) {
	return s.store.GetEnvironment(id)
}

// ListEnvironments returns all environments ordered by rank.
func (s *Service) ListEnvironments() []model.Environment {
	return s.store.ListEnvironments()
}

// EnvironmentSummary is an environment plus the version state the promotion view needs, so the chain
// can be rendered without a request per environment.
type EnvironmentSummary struct {
	model.Environment
	LatestSeq int `json:"latestSeq"`
	// PromotesToResolved are the outgoing edges actually in effect, with the rank fallback applied,
	// so a caller does not have to reproduce that rule.
	PromotesToResolved []string `json:"promotesToResolved"`
	// PromotedFrom are the incoming edges: the environments that can promote into this one.
	PromotedFrom []string `json:"promotedFrom"`
	// HasPendingChanges reports whether the latest version differs from what is applied.
	HasPendingChanges bool `json:"hasPendingChanges"`
	// DataPlane reports whether this environment's data plane is connected. Nothing can be applied or
	// promoted to one that is not, so it is shown alongside the chain rather than discovered by
	// starting a promotion that cannot finish.
	DataPlane model.DataPlaneStatus `json:"dataPlane"`
}

// ListEnvironmentSummaries returns every environment in promotion order (each one before those it
// promotes into), annotated with its version state and its edges in the promotion graph.
func (s *Service) ListEnvironmentSummaries() ([]EnvironmentSummary, error) {
	envs := s.store.ListEnvironments()
	ordered := topologicalOrder(envs)
	adjacency := buildAdjacency(envs)

	incoming := make(map[string][]string, len(envs))
	for _, env := range ordered {
		for _, target := range adjacency[env.ID] {
			incoming[target] = append(incoming[target], env.ID)
		}
	}

	out := make([]EnvironmentSummary, 0, len(ordered))
	for _, env := range ordered {
		latest, err := s.store.LatestSeq(env.ID)
		if err != nil {
			return nil, err
		}
		resolved := adjacency[env.ID]
		if resolved == nil {
			resolved = []string{}
		}
		from := incoming[env.ID]
		if from == nil {
			from = []string{}
		}
		out = append(out, EnvironmentSummary{
			Environment:        env,
			LatestSeq:          latest,
			PromotesToResolved: resolved,
			PromotedFrom:       from,
			HasPendingChanges:  latest > 0 && latest != env.AppliedSeq,
			DataPlane:          s.DataPlaneStatus(env),
		})
	}
	return out, nil
}

// DeleteEnvironment removes an environment and its versions.
func (s *Service) DeleteEnvironment(id string) error {
	return s.store.DeleteEnvironment(id)
}

// ---- versions ----

// CaptureVersion pulls the current config from an environment's control-plane source (export plus
// revealed secret values) and stores it as a new version.
func (s *Service) CaptureVersion(ctx context.Context, envID, note string) (model.Version, error) {
	env, err := s.store.GetEnvironment(envID)
	if err != nil {
		return model.Version{}, err
	}
	if env.Source == nil {
		return model.Version{}, ErrNoVersionSource
	}
	// An environment fed by promotion has no configuration of its own to read: its control plane tenant
	// holds what the promotion put there. Capturing would pull the caller's own tenant into this
	// environment's history and present it as this environment's state.
	if isPromotionFed(s.store.ListEnvironments(), envID) {
		return model.Version{}, ErrPromotionFed
	}
	client := s.newClient(env.Source.BaseURL, callerCredentials(ctx), env.Source.InsecureSkipVerify)
	exported, err := client.Export(ctx)
	if err != nil {
		return model.Version{}, fmt.Errorf("export failed: %w", err)
	}
	// The default resource server's identifier is this deployment's audience. Captured verbatim it
	// would travel to every environment the bundle is promoted to, so each of them would name the
	// audience of the environment it was captured from. Templated here, resolved per target on apply.
	exported.Resources = bundle.TemplateDeploymentURL(exported.Resources)

	secretKeys, err := client.SecretKeys(ctx)
	if err != nil {
		return model.Version{}, fmt.Errorf("listing secrets failed: %w", err)
	}
	// The control plane only lists the secrets it still holds, and it forwards a credential to the data
	// plane rather than keeping it. The bundle is the authority on which placeholders are credentials.
	secretKeys = mergeKeys(secretKeys, bundle.SecretVariables(exported.Resources))
	configured, err := client.EnvironmentVariables(ctx)
	if err != nil {
		return model.Version{}, fmt.Errorf("reading environment variables failed: %w", err)
	}

	// The export's own .env is the starting point, then the control plane's configured environment
	// variables override it: those are what the operator set for this deployment, whereas the export
	// only reflects whatever the resources happened to contain.
	vars := bundle.ParseEnv(exported.EnvFile)
	for k, v := range configured {
		vars[k] = v
	}
	// A secret's value is never stored here; the placeholder is sent on apply instead.
	for _, key := range secretKeys {
		delete(vars, key)
	}

	return s.store.AddVersion(model.Version{
		EnvID:      envID,
		Origin:     model.OriginCaptured,
		Note:       note,
		CreatedAt:  s.now().UTC(),
		Resources:  exported.Resources,
		Variables:  vars,
		SecretKeys: secretKeys,
	})
}

// UploadVersion stores a caller-supplied bundle as a new version.
func (s *Service) UploadVersion(envID, resources string, variables map[string]string, note string) (
	model.Version, error) {
	if _, err := s.store.GetEnvironment(envID); err != nil {
		return model.Version{}, err
	}
	if variables == nil {
		variables = map[string]string{}
	}
	return s.store.AddVersion(model.Version{
		EnvID:     envID,
		Origin:    model.OriginUploaded,
		Note:      note,
		CreatedAt: s.now().UTC(),
		Resources: resources,
		Variables: variables,
	})
}

// GetVersion returns a full version.
func (s *Service) GetVersion(envID string, seq int) (model.Version, error) {
	return s.store.GetVersion(envID, seq)
}

// ListVersions returns version metadata newest first.
func (s *Service) ListVersions(envID string) ([]model.Version, error) {
	if _, err := s.store.GetEnvironment(envID); err != nil {
		return nil, err
	}
	return s.store.ListVersions(envID)
}

// Diff computes the difference between two version references (a sequence number, "latest" or
// "applied") within one environment.
func (s *Service) Diff(envID, fromRef, toRef string) (diff.Diff, error) {
	env, err := s.store.GetEnvironment(envID)
	if err != nil {
		return diff.Diff{}, err
	}
	from, err := s.resolveResources(env, fromRef)
	if err != nil {
		return diff.Diff{}, err
	}
	to, err := s.resolveResources(env, toRef)
	if err != nil {
		return diff.Diff{}, err
	}
	// Held back resources are filtered from both sides, so a preview of an apply shows what the apply
	// would actually do rather than listing changes it is going to skip.
	return diff.Compute(withoutExcluded(from, env.Excluded), withoutExcluded(to, env.Excluded)), nil
}

// ---- apply ----

// ApplyResult reports what an apply did (or would do, for a dry run).
type ApplyResult struct {
	TargetSeq int       `json:"targetSeq"`
	Diff      diff.Diff `json:"diff"`
	DryRun    bool      `json:"dryRun"`
	// MissingVariables are placeholders that resolved to nothing. The import still reports success for
	// them, because an absent value simply renders as empty, so they are reported here to explain why an
	// applied resource can come out with a field such as its redirect URIs stripped.
	MissingVariables []string                `json:"missingVariables,omitempty"`
	Import           *thunder.ImportResponse `json:"import,omitempty"`
}

// resolveVariables returns the values an apply would use: the version's captured snapshot overlaid
// with whatever the control plane currently holds.
//
// The overlay happens at apply time rather than only at capture time on purpose. A variable such as a
// redirect URL is a property of the environment, not of the configuration version, so editing it in
// the control plane has to take effect on the next apply without forcing a re-capture. When the
// control plane cannot be reached the snapshot is used unchanged, so an apply is still possible.
func (s *Service) resolveVariables(ctx context.Context, env model.Environment,
	version model.Version) map[string]string {
	values := map[string]string{}
	// The deployment's own URL, which the captured bundle refers to in place of the audience it was
	// captured with. It sits underneath the configured variables so an operator can still override it.
	if url := strings.TrimSpace(env.Target.BaseURL); url != "" {
		values[bundle.DeploymentURLVariable] = strings.TrimRight(url, "/")
	}
	for k, v := range version.Variables {
		values[k] = v
	}
	if env.Source == nil {
		return values
	}

	client := s.newClient(env.Source.BaseURL, callerCredentials(ctx),
		env.Source.InsecureSkipVerify)
	live, err := client.EnvironmentVariables(ctx)
	if err != nil {
		return values
	}
	for k, v := range live {
		values[k] = v
	}
	return values
}

// VariableStatus reports how an environment's next apply would resolve its placeholders.
type VariableStatus struct {
	EnvID string `json:"envId"`
	Seq   int    `json:"seq"`
	// Required is every placeholder the version references.
	Required []string `json:"required"`
	// Missing is the subset with no value configured and no backing secret.
	Missing []string `json:"missing"`
	// SecretBacked placeholders are supplied by the data plane, so they need no value here.
	SecretBacked []string `json:"secretBacked"`
	// MissingSecrets are the secret backed placeholders the data plane's secret service does not hold.
	// Applying with these unresolved leaves a credential that rejects every attempt, so they are
	// reported before the apply rather than diagnosed after a login fails.
	MissingSecrets []string `json:"missingSecrets"`
	// SecretsChecked reports whether the secret service could be consulted at all. When false,
	// MissingSecrets is not a judgement: nothing is known either way.
	SecretsChecked bool `json:"secretsChecked"`
}

// missingSecrets reports which of the secret backed placeholders the data plane's secret service does
// not hold. The second return value is false when the service is not configured or cannot be reached,
// so a caller can tell "nothing missing" apart from "nothing known".
func (s *Service) missingSecrets(ctx context.Context, env model.Environment,
	secretKeys []string) ([]string, bool) {
	if len(secretKeys) == 0 {
		return nil, false
	}
	plane, err := s.dataPlaneFor(env)
	if err != nil {
		return nil, false
	}
	names, err := plane.SecretNames(ctx)
	if err != nil {
		return nil, false
	}

	held := make(map[string]bool, len(names))
	for _, name := range names {
		held[name] = true
	}
	missing := []string{}
	for _, key := range secretKeys {
		if !held[key] {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing, true
}

// CheckVariables reports which placeholders a version would fail to resolve, so a caller can fix them
// before applying rather than discovering a silently emptied field afterwards.
func (s *Service) CheckVariables(ctx context.Context, envID, versionRef string) (VariableStatus, error) {
	env, err := s.store.GetEnvironment(envID)
	if err != nil {
		return VariableStatus{}, err
	}
	seq, err := s.resolveSeq(env, defaultRef(versionRef, "latest"))
	if err != nil {
		return VariableStatus{}, err
	}
	if seq == 0 {
		return VariableStatus{}, ErrNoVersions
	}
	version, err := s.store.GetVersion(envID, seq)
	if err != nil {
		return VariableStatus{}, err
	}

	scalars, arrays := bundle.RequiredVariables(version.Resources)
	required := append(append([]string{}, scalars...), arrays...)
	sort.Strings(required)

	secretBacked := secretKeysOf(version)
	missing := bundle.MissingVariables(version.Resources, s.resolveVariables(ctx, env, version), secretBacked)
	if missing == nil {
		missing = []string{}
	}
	missingSecrets, checked := s.missingSecrets(ctx, env, secretBacked)
	if missingSecrets == nil {
		missingSecrets = []string{}
	}
	return VariableStatus{
		EnvID: envID, Seq: seq, Required: required, Missing: missing, SecretBacked: secretBacked,
		MissingSecrets: missingSecrets, SecretsChecked: checked,
	}, nil
}

// Apply applies a version (default latest) to the environment's data-plane target. It diffs the
// target version against what is currently applied and drives the import API with the full target
// bundle (idempotent upsert) plus deletions for resources the diff shows were removed.
func (s *Service) Apply(ctx context.Context, envID, versionRef string, dryRun bool) (ApplyResult, error) {
	env, err := s.store.GetEnvironment(envID)
	if err != nil {
		return ApplyResult{}, err
	}
	targetSeq, err := s.resolveSeq(env, defaultRef(versionRef, "latest"))
	if err != nil {
		return ApplyResult{}, err
	}
	if targetSeq == 0 {
		return ApplyResult{}, ErrNoVersions
	}
	target, err := s.store.GetVersion(envID, targetSeq)
	if err != nil {
		return ApplyResult{}, err
	}

	var appliedRes []bundle.Resource
	if env.AppliedSeq > 0 {
		if applied, err := s.store.GetVersion(envID, env.AppliedSeq); err == nil {
			appliedRes = withoutExcluded(bundle.Parse(applied.Resources), env.Excluded)
		}
	}
	// A resource held back from this environment is not pushed to its data plane either, so what runs
	// there matches what was agreed rather than quietly reappearing on the next apply. Both sides of
	// the comparison are filtered: dropping it from only one would read as a deletion, and holding a
	// resource back means leaving it alone, not removing it from the data plane.
	targetRes := withoutExcluded(bundle.Parse(target.Resources), env.Excluded)
	d := diff.Compute(appliedRes, targetRes)

	values := s.resolveVariables(ctx, env, target)
	req := thunder.ImportRequest{
		Content:   bundle.Marshal(targetRes),
		Variables: bundle.BuildTemplateVariables(target.Resources, values, secretKeysOf(target)),
		DryRun:    dryRun,
		Deletions: deletionsFromDiff(d),
		// Everything here comes from the control plane, so the data plane records it as owned there and
		// refuses local edits to it. Its own resources, written by other means, stay editable.
		Options: &thunder.ImportOptions{MarkManaged: true},
	}
	plane, err := s.dataPlaneFor(env)
	if err != nil {
		return ApplyResult{}, err
	}
	resp, err := plane.Import(ctx, req)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("import failed: %w", err)
	}

	if !dryRun {
		env.AppliedSeq = targetSeq
		env.UpdatedAt = s.now().UTC()
		if err := s.store.SaveEnvironment(env); err != nil {
			return ApplyResult{}, err
		}
	}
	return ApplyResult{
		TargetSeq:        targetSeq,
		Diff:             d,
		DryRun:           dryRun,
		MissingVariables: bundle.MissingVariables(target.Resources, values, secretKeysOf(target)),
		Import:           resp,
	}, nil
}

// ApplyAllResult reports one environment's outcome from an apply across every environment.
type ApplyAllResult struct {
	EnvID   string       `json:"envId"`
	EnvName string       `json:"envName"`
	Applied *ApplyResult `json:"applied,omitempty"`
	// Error explains why this environment was skipped or failed. The others are still attempted, so a
	// single unreachable data plane does not block the rest.
	Error string `json:"error,omitempty"`
}

// ApplyAll applies each environment's latest version to its data plane.
//
// This exists for the case where a value the configuration references changes rather than the
// configuration itself, for example a redirect URL edited on the control plane. The stored versions are
// untouched, but every data plane holding the old value needs the new one, and re-applying is what
// pushes it. Environments with no version are skipped.
func (s *Service) ApplyAll(ctx context.Context, dryRun bool) []ApplyAllResult {
	envs := s.store.ListEnvironments()
	results := make([]ApplyAllResult, 0, len(envs))

	for _, env := range envs {
		outcome := ApplyAllResult{EnvID: env.ID, EnvName: env.Name}

		latest, err := s.store.LatestSeq(env.ID)
		if err != nil {
			outcome.Error = err.Error()
			results = append(results, outcome)
			continue
		}
		if latest == 0 {
			outcome.Error = ErrNoVersions.Error()
			results = append(results, outcome)
			continue
		}

		applied, err := s.Apply(ctx, env.ID, strconv.Itoa(latest), dryRun)
		if err != nil {
			outcome.Error = err.Error()
		} else {
			outcome.Applied = &applied
		}
		results = append(results, outcome)
	}
	return results
}

// ---- promote ----

// PromoteInput is the input to Promote.
type PromoteInput struct {
	FromEnvID  string
	ToEnvID    string
	VersionRef string   // version in the source environment; default "latest"
	Selection  []string // resource keys to promote, honoured only when SelectionProvided
	// SelectionProvided distinguishes "the user chose these" from "the user expressed no preference".
	// Without it an empty selection could not mean "hold everything back", because it would be
	// indistinguishable from a caller that simply did not send the field.
	SelectionProvided bool
	Apply             bool
	DryRun            bool
	Note              string
}

// PromoteResult reports a promotion.
type PromoteResult struct {
	Preview    diff.Diff     `json:"preview"`
	NewVersion model.Version `json:"newVersion"`
	// Secrets reports what happened to the target environment's credentials.
	Secrets targetSecretOutcome `json:"secrets"`
	// ControlPlane is the outcome of writing the configuration into the target's control plane tenant.
	ControlPlane *thunder.ImportResponse `json:"controlPlane,omitempty"`
	Applied      *ApplyResult            `json:"applied,omitempty"`
}

// PromotePreview returns the diff of a source environment's version against the target environment's
// current version, without writing anything. This is what a caller reviews (and selects from) before
// promoting.
func (s *Service) PromotePreview(fromEnvID, toEnvID, versionRef string) (diff.Diff, error) {
	_, _, sourceRes, targetRes, err := s.promoteInputs(fromEnvID, toEnvID, versionRef)
	if err != nil {
		return diff.Diff{}, err
	}
	return diff.Compute(targetRes, sourceRes), nil
}

// Promote copies selected changes from a source environment's version into a new version of the
// target environment, optionally applying it to the target's data plane.
func (s *Service) Promote(ctx context.Context, in PromoteInput) (PromoteResult, error) {
	source, target, sourceRes, targetRes, err := s.promoteInputs(in.FromEnvID, in.ToEnvID, in.VersionRef)
	if err != nil {
		return PromoteResult{}, err
	}
	preview := diff.Compute(targetRes, sourceRes)

	targetEnv, err := s.store.GetEnvironment(in.ToEnvID)
	if err != nil {
		return PromoteResult{}, err
	}

	// A resource held back on an earlier run stays held back unless this run deliberately selects it.
	selection := in.Selection
	if !in.SelectionProvided {
		selection = defaultSelection(preview, targetEnv.Excluded)
	} else if err := s.rememberSelection(targetEnv, preview, selection); err != nil {
		return PromoteResult{}, err
	} else if targetEnv, err = s.store.GetEnvironment(in.ToEnvID); err != nil {
		return PromoteResult{}, err
	}

	promotedRes := applySelection(targetRes, sourceRes, preview, selection)
	vars := mergePreferTarget(source.Variables, target.currentVariables)

	// No credential travels with a promotion. The destination's control plane holds none, and its data
	// plane's secret service is where they live and where an operator sets them; inventing one here
	// would reach that data plane through capture and replace a credential already in use.
	secretOutcome := targetSecretOutcome{Generated: []string{}, Reused: []string{}, Skipped: secretKeysOf(source)}

	newVersion, err := s.store.AddVersion(model.Version{
		EnvID:       in.ToEnvID,
		Origin:      model.OriginPromoted,
		ParentSeq:   target.currentSeq,
		SourceEnvID: in.FromEnvID,
		SourceSeq:   source.Seq,
		Note:        in.Note,
		CreatedAt:   s.now().UTC(),
		Resources:   bundle.Marshal(promotedRes),
		Variables:   vars,
		SecretKeys:  secretKeysOf(source),
	})
	if err != nil {
		return PromoteResult{}, err
	}

	// The promoted configuration is written into the target's own control plane tenant, so switching to
	// that tenant shows the promoted state. Redirect URLs are deliberately not required here: they are
	// asked for when that tenant is applied to its data plane.
	controlPlane, err := s.importIntoTargetControlPlane(ctx, targetEnv, promotedRes, vars,
		secretKeysOf(source), deletionsFromDiff(diff.Compute(targetRes, promotedRes)))
	if err != nil {
		return PromoteResult{}, err
	}

	if controlPlane != nil {
		s.recordControlPlaneSeq(targetEnv, newVersion.Seq)
	}

	result := PromoteResult{
		Preview: preview, NewVersion: stripPayload(newVersion),
		Secrets: secretOutcome, ControlPlane: controlPlane,
	}
	if in.Apply {
		applied, err := s.Apply(ctx, in.ToEnvID, strconv.Itoa(newVersion.Seq), in.DryRun)
		if err != nil {
			return result, err
		}
		result.Applied = &applied
	}
	return result, nil
}

// ApplyToControlPlane writes a version into the environment's own control plane tenant, leaving the
// data plane untouched.
//
// Promote and revert already do this as part of their work. This is the same write on its own, for
// putting a tenant back in step after one of those failed part way, or for pushing a version to the
// control plane without touching what is serving traffic.
func (s *Service) ApplyToControlPlane(ctx context.Context, envID,
	versionRef string) (*thunder.ImportResponse, error) {
	env, err := s.store.GetEnvironment(envID)
	if err != nil {
		return nil, err
	}
	seq, err := s.resolveSeq(env, defaultRef(versionRef, "latest"))
	if err != nil {
		return nil, err
	}
	if seq == 0 {
		return nil, ErrNoVersions
	}
	version, err := s.store.GetVersion(envID, seq)
	if err != nil {
		return nil, err
	}

	// Credentials are left as the tenant holds them, for the same reason a revert does: they are hashes
	// this service cannot reproduce, and rewriting them would lock out every client that has one.
	resp, err := s.importIntoTargetControlPlane(ctx, env, bundle.Parse(version.Resources),
		version.Variables, secretKeysOf(version), s.controlPlaneDeletions(env, version))
	if err != nil {
		return nil, err
	}
	if resp != nil {
		s.recordControlPlaneSeq(env, seq)
	}
	return resp, nil
}

// controlPlaneDeletions lists what the tenant holds that a version does not describe.
//
// The baseline is what was last written to that tenant, not the newest version. Those are the same
// until a version is written to the control plane on its own, and then they are not: the tenant holds
// what was written, while the newest version is whatever was captured before that. Comparing against
// the newest one would compute removals for resources the tenant no longer has and miss the ones it
// does.
//
// With nothing written by this service yet, the newest version stands in: a capture is a reading of
// the tenant, so it is the best available account of what is there.
func (s *Service) controlPlaneDeletions(env model.Environment,
	version model.Version) []thunder.ResourceDeletion {
	baseSeq, err := s.controlPlaneBaseline(env)
	if err != nil || baseSeq == 0 || baseSeq == version.Seq {
		return nil
	}
	base, err := s.store.GetVersion(env.ID, baseSeq)
	if err != nil {
		return nil
	}
	return deletionsFromDiff(diff.Compute(bundle.Parse(base.Resources), bundle.Parse(version.Resources)))
}

// controlPlaneBaseline is the version the control plane tenant currently reflects. It is what a write
// into that tenant compares against, so the removals it computes match what is actually there.
func (s *Service) controlPlaneBaseline(env model.Environment) (int, error) {
	if env.ControlPlaneSeq > 0 {
		return env.ControlPlaneSeq, nil
	}
	return s.store.LatestSeq(env.ID)
}

// promotionBaseline is the version an environment is taken to be at when promoting.
//
// It is what has been applied to its data plane, not what has been captured. A capture is a draft: it
// records the control plane as it stands, but nothing is committed to until it is applied, and
// several captures can pile up without the environment moving. Promoting the newest capture would
// push work that was never adopted, and comparing against it would describe a destination state that
// nothing is running.
//
// An environment with nothing applied yet falls back to its newest version, so a freshly registered
// one can still promote.
func (s *Service) promotionBaseline(env model.Environment) (int, error) {
	if env.AppliedSeq > 0 {
		return env.AppliedSeq, nil
	}
	return s.store.LatestSeq(env.ID)
}

// recordControlPlaneSeq remembers which version a tenant was last written with, so the next write
// compares against what is actually there.
func (s *Service) recordControlPlaneSeq(env model.Environment, seq int) {
	if seq <= 0 || env.ControlPlaneSeq == seq {
		return
	}
	env.ControlPlaneSeq = seq
	env.UpdatedAt = s.now().UTC()
	// A failure here costs accuracy on the next comparison, not this write, so it is not worth
	// failing the operation that just succeeded.
	_ = s.store.SaveEnvironment(env)
}

// ---- revert ----

// RevertInput is the input to Revert. ToRef accepts a version number or "previous", which targets the
// version immediately before the current head.
type RevertInput struct {
	EnvID  string
	ToRef  string
	Apply  bool
	DryRun bool
	Note   string
}

// RevertResult reports a revert.
type RevertResult struct {
	Preview    diff.Diff     `json:"preview"`
	NewVersion model.Version `json:"newVersion"`
	Applied    *ApplyResult  `json:"applied,omitempty"`
	// ControlPlane is the outcome of restoring the environment's own control plane tenant, which goes
	// back with the data plane so the two describe the same state.
	ControlPlane *thunder.ImportResponse `json:"controlPlane,omitempty"`
}

// Revert creates a new version that restores the content of an earlier version, optionally applying
// it. History is preserved: reverting adds a new head rather than deleting versions.
func (s *Service) Revert(ctx context.Context, in RevertInput) (RevertResult, error) {
	env, err := s.store.GetEnvironment(in.EnvID)
	if err != nil {
		return RevertResult{}, err
	}
	toSeq, err := s.resolveSeq(env, in.ToRef)
	if err != nil {
		return RevertResult{}, err
	}
	if toSeq == 0 {
		return RevertResult{}, ErrNoVersions
	}
	target, err := s.store.GetVersion(in.EnvID, toSeq)
	if err != nil {
		return RevertResult{}, err
	}
	latestSeq, err := s.store.LatestSeq(in.EnvID)
	if err != nil {
		return RevertResult{}, err
	}
	var currentRes []bundle.Resource
	if latestSeq > 0 {
		if current, err := s.store.GetVersion(in.EnvID, latestSeq); err == nil {
			currentRes = bundle.Parse(current.Resources)
		}
	}
	preview := diff.Compute(currentRes, bundle.Parse(target.Resources))

	newVersion, err := s.store.AddVersion(model.Version{
		EnvID:      in.EnvID,
		Origin:     model.OriginReverted,
		ParentSeq:  latestSeq,
		SourceSeq:  toSeq,
		Note:       in.Note,
		CreatedAt:  s.now().UTC(),
		Resources:  target.Resources,
		Variables:  target.Variables,
		SecretKeys: secretKeysOf(target),
	})
	if err != nil {
		return RevertResult{}, err
	}
	result := RevertResult{Preview: preview, NewVersion: stripPayload(newVersion)}

	// A revert restores the environment as a whole, so its control plane goes back with it. Leaving it
	// on the newer configuration would show a state that neither the operator asked for nor the data
	// plane is running.
	controlPlane, err := s.importIntoTargetControlPlane(ctx, env, bundle.Parse(target.Resources),
		target.Variables, secretKeysOf(target), deletionsFromDiff(preview))
	if err != nil {
		return result, err
	}
	result.ControlPlane = controlPlane
	if controlPlane != nil {
		s.recordControlPlaneSeq(env, newVersion.Seq)
	}

	if in.Apply {
		applied, err := s.Apply(ctx, in.EnvID, strconv.Itoa(newVersion.Seq), in.DryRun)
		if err != nil {
			return result, err
		}
		result.Applied = &applied
	}
	return result, nil
}

// ---- helpers ----

// promoteContext carries the resolved current state of the target environment.
type promoteContext struct {
	currentSeq       int
	currentVariables map[string]string
}

// promoteInputs resolves the source version and the target environment's current version.
func (s *Service) promoteInputs(fromEnvID, toEnvID, versionRef string) (
	model.Version, promoteContext, []bundle.Resource, []bundle.Resource, error) {
	fromEnv, err := s.store.GetEnvironment(fromEnvID)
	if err != nil {
		return model.Version{}, promoteContext{}, nil, nil, err
	}
	toEnv, err := s.store.GetEnvironment(toEnvID)
	if err != nil {
		return model.Version{}, promoteContext{}, nil, nil, err
	}
	// Movement must follow an edge of the promotion graph. The reverse direction is allowed too, which
	// is what a demotion is: pushing a version back down the same edge it came up.
	envs := s.store.ListEnvironments()
	if !canPromote(envs, fromEnvID, toEnvID) && !canPromote(envs, toEnvID, fromEnvID) {
		return model.Version{}, promoteContext{}, nil, nil, ErrNoPromotionPath
	}

	// A promotion moves what the source is running, not what has been captured from it. A caller naming
	// a version explicitly still gets that one.
	sourceSeq, err := s.resolveSeq(fromEnv, defaultRef(versionRef, "promotable"))
	if err != nil {
		return model.Version{}, promoteContext{}, nil, nil, err
	}
	if sourceSeq == 0 {
		return model.Version{}, promoteContext{}, nil, nil, ErrNoVersions
	}
	source, err := s.store.GetVersion(fromEnvID, sourceSeq)
	if err != nil {
		return model.Version{}, promoteContext{}, nil, nil, err
	}

	// The destination is likewise taken to be at what it is running, so the comparison is between two
	// states that exist rather than between two drafts.
	tctx := promoteContext{currentVariables: map[string]string{}}
	var targetRes []bundle.Resource
	if targetSeq, err := s.promotionBaseline(toEnv); err == nil && targetSeq > 0 {
		if targetVersion, err := s.store.GetVersion(toEnvID, targetSeq); err == nil {
			tctx.currentSeq = targetSeq
			tctx.currentVariables = targetVersion.Variables
			targetRes = bundle.Parse(targetVersion.Resources)
		}
	}
	return source, tctx, bundle.Parse(source.Resources), targetRes, nil
}

// resolveResources resolves a version reference to its parsed resources.
func (s *Service) resolveResources(env model.Environment, ref string) ([]bundle.Resource, error) {
	seq, err := s.resolveSeq(env, ref)
	if err != nil {
		return nil, err
	}
	if seq == 0 {
		return nil, nil
	}
	v, err := s.store.GetVersion(env.ID, seq)
	if err != nil {
		return nil, err
	}
	return bundle.Parse(v.Resources), nil
}

// resolveSeq resolves a reference ("latest", "previous", "applied", or a number) to a version
// sequence. It returns 0 when there is nothing to resolve to (no versions / nothing applied).
func (s *Service) resolveSeq(env model.Environment, ref string) (int, error) {
	switch ref {
	case "", "latest":
		return s.store.LatestSeq(env.ID)
	case "previous":
		// The version immediately before the current head, which is what a one-click revert targets.
		versions, err := s.store.ListVersions(env.ID)
		if err != nil {
			return 0, err
		}
		if len(versions) < 2 {
			return 0, ErrNoPreviousVersion
		}
		return versions[1].Seq, nil
	case "applied":
		return env.AppliedSeq, nil
	case "control-plane":
		return s.controlPlaneBaseline(env)
	case "promotable":
		return s.promotionBaseline(env)
	default:
		seq, err := strconv.Atoi(ref)
		if err != nil || seq <= 0 {
			return 0, ErrBadRef
		}
		return seq, nil
	}
}

func defaultRef(ref, fallback string) string {
	if strings.TrimSpace(ref) == "" {
		return fallback
	}
	return ref
}

// deletionsFromDiff turns deleted resources into import deletion requests. Resources without an id
// (translations and server configuration, which are keyed by language and section) are still sent so
// the import API reports an explicit outcome rather than the prune silently skipping them.
func deletionsFromDiff(d diff.Diff) []thunder.ResourceDeletion {
	var out []thunder.ResourceDeletion
	for _, c := range d.Changes {
		if c.Change != diff.Deleted {
			continue
		}
		out = append(out, thunder.ResourceDeletion{ResourceType: c.Type, ID: c.ID, Category: c.Category})
	}
	return out
}

// applySelection builds the promoted resource set: start from the target's current resources and
// apply the selected changes (add/update sets the source resource, delete removes it). An empty
// selection applies every change. Ordering follows the source bundle, then any target-only resources.
func applySelection(targetRes, sourceRes []bundle.Resource, d diff.Diff, selection []string) []bundle.Resource {
	// An empty selection means nothing was chosen, not everything. The caller resolves what "no
	// preference" means before getting here, so that an environment with every change held back
	// promotes nothing rather than promoting the lot.
	sel := map[string]bool{}
	for _, k := range selection {
		sel[k] = true
	}
	sourceIdx := bundle.Index(sourceRes)
	result := map[string]bundle.Resource{}
	for _, r := range targetRes {
		result[r.Key()] = r
	}
	for _, c := range d.Changes {
		if c.Change == diff.Unchanged {
			continue
		}
		if !sel[c.Key] {
			continue
		}
		switch c.Change {
		case diff.Added, diff.Updated:
			result[c.Key] = sourceIdx[c.Key]
		case diff.Deleted:
			delete(result, c.Key)
		}
	}

	var out []bundle.Resource
	emitted := map[string]bool{}
	for _, r := range sourceRes {
		if v, ok := result[r.Key()]; ok && !emitted[r.Key()] {
			out = append(out, v)
			emitted[r.Key()] = true
		}
	}
	for _, r := range targetRes {
		if v, ok := result[r.Key()]; ok && !emitted[r.Key()] {
			out = append(out, v)
			emitted[r.Key()] = true
		}
	}
	return out
}

// mergePreferTarget returns source overlaid with target, so target's environment-specific values win
// while new placeholders introduced by promoted resources are filled from the source.
func mergePreferTarget(source, target map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range source {
		out[k] = v
	}
	for k, v := range target {
		out[k] = v
	}
	return out
}

func stripPayload(v model.Version) model.Version {
	v.Resources = ""
	v.Variables = nil
	return v
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "-0000000000000000"
	}
	return prefix + "-" + hex.EncodeToString(buf)
}

// mergeKeys unions two key lists into a sorted list with no duplicates.
func mergeKeys(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, list := range [][]string{a, b} {
		for _, key := range list {
			if key != "" && !seen[key] {
				seen[key] = true
				out = append(out, key)
			}
		}
	}
	sort.Strings(out)
	return out
}

// secretKeysOf returns the secret backed placeholders of a version.
//
// The stored list is only a record of what the control plane knew at capture time, so it is merged
// with what the bundle itself says. That keeps a version captured before a credential was classified
// as a secret from reporting it forever as a variable with no value.
func secretKeysOf(version model.Version) []string {
	return mergeKeys(version.SecretKeys, bundle.SecretVariables(version.Resources))
}
