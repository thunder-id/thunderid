// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// latestVersion names the newest captured version.
const latestVersion = "latest"

// VersionServiceInterface captures this deployment's configuration as versions and applies them to
// the gateways it administers.
type VersionServiceInterface interface {
	// Capture records the configuration as it is now as a new version.
	Capture(ctx context.Context, req CaptureRequest) (*Version, *tidcommon.ServiceError)
	ListVersions(ctx context.Context) ([]Version, *tidcommon.ServiceError)
	GetVersion(ctx context.Context, ref string) (*Version, *tidcommon.ServiceError)
	// GetApplied reports what a gateway holds. A gateway nothing was applied to holds no version.
	GetApplied(ctx context.Context, gatewayID string) (*AppliedVersion, *tidcommon.ServiceError)
	// Diff reports how a version differs from what a gateway holds, without changing anything.
	Diff(ctx context.Context, gatewayID, ref string) (*Diff, *tidcommon.ServiceError)
	// Apply writes a version to a gateway: what the version adds or changes, and the removal of what
	// the gateway holds that the version no longer has.
	Apply(ctx context.Context, gatewayID string, req ApplyRequest) (*ApplyResult, *tidcommon.ServiceError)
	// Revert returns a gateway to the version it held before the one it holds now.
	Revert(ctx context.Context, gatewayID string, req RevertRequest) (*ApplyResult, *tidcommon.ServiceError)
	// Forget drops what a removed gateway held, so a gateway registered later under the same id does
	// not inherit it.
	Forget(ctx context.Context, gatewayID string)
	// ForwardStore passes a call to a gateway's variable and secret store, presenting its key.
	ForwardStore(ctx context.Context, gatewayID string, call StoreCall) (*StoreAnswer, *tidcommon.ServiceError)
}

type versionService struct {
	gateways storeInterface
	versions versionStoreInterface
	exporter export.ExportServiceInterface
	client   gatewayClientInterface
	logger   *log.Logger
}

func newVersionService(gateways storeInterface, versions versionStoreInterface,
	exporter export.ExportServiceInterface, client gatewayClientInterface) VersionServiceInterface {
	return &versionService{
		gateways: gateways,
		versions: versions,
		exporter: exporter,
		client:   client,
		logger:   log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GatewayVersionService")),
	}
}

// maxVersions is how many versions this deployment keeps.
func maxVersions() int {
	if !config.IsServerRuntimeInitialized() {
		return 1
	}
	return config.GetServerRuntime().Config.Gateway.MaxVersionCount()
}

// configurationExport asks for every resource type, users and groups included, so a gateway a
// version is applied to holds the same accounts as the deployment it was captured from. A user's
// credentials are exported as references, which the gateway fills in from its own store.
func configurationExport() *export.ExportRequest {
	all := []string{"*"}
	return &export.ExportRequest{
		Agents: all, Applications: all, Connections: all, UserTypes: all, AgentTypes: all,
		Users: all, Groups: all,
		OrganizationUnits: all, ResourceServers: all, Roles: all, Flows: all, Translations: all,
		Layouts: all, Themes: all, ServerConfigs: all, CredentialConfigurations: all,
		PresentationDefinitions: all,
		Options:                 &export.ExportOptions{IncludeDependencies: true, Format: "yaml"},
	}
}

func (s *versionService) Capture(ctx context.Context, req CaptureRequest) (*Version, *tidcommon.ServiceError) {
	exported, svcErr := s.exporter.ExportResources(ctx, configurationExport())
	if svcErr != nil {
		return nil, svcErr
	}
	// A resource the export could not write, such as a user without a username, is left out of the
	// version as the export leaves it out of its own answer, and is named in the capture's answer so
	// the administrator sees what the version is missing.
	var skipped []export.ExportError
	if exported.Summary != nil {
		skipped = exported.Summary.Errors
	}
	for _, failed := range skipped {
		s.logger.Warn(ctx, "A resource the export could not write is left out of the version",
			log.String("resourceType", failed.ResourceType), log.String("resourceId", failed.ResourceID),
			log.String("error", failed.Error))
	}

	documents := make([]string, 0, len(exported.Files))
	for _, file := range exported.Files {
		documents = append(documents, strings.TrimSpace(file.Content))
	}

	sealed := ""
	if exported.EnvFile != nil {
		variables := parseEnvFile(exported.EnvFile.Content)
		if len(variables) > 0 {
			var err *tidcommon.ServiceError
			if sealed, err = s.seal(ctx, variables); err != nil {
				return nil, err
			}
		}
	}

	version, err := s.versions.Add(ctx, joinBundle(documents), sealed, strings.TrimSpace(req.Note))
	if err != nil {
		s.logger.Error(ctx, "Failed to capture a version", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if err := s.versions.Prune(ctx, version.Seq-maxVersions()); err != nil {
		// The version is captured; an older one surviving a little longer is not worth failing for.
		s.logger.Warn(ctx, "Failed to remove old versions", log.Error(err))
	}
	s.logger.Info(ctx, "Captured a configuration version", log.Int("version", version.Seq),
		log.Int("skipped", len(skipped)))
	version.Skipped = skipped
	return version, nil
}

func (s *versionService) ListVersions(ctx context.Context) ([]Version, *tidcommon.ServiceError) {
	versions, err := s.versions.List(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to list the versions", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return versions, nil
}

func (s *versionService) GetVersion(ctx context.Context, ref string) (*Version, *tidcommon.ServiceError) {
	return s.resolve(ctx, ref)
}

// resolve reads the version a reference names: a number, or "latest" when empty.
func (s *versionService) resolve(ctx context.Context, ref string) (*Version, *tidcommon.ServiceError) {
	ref = strings.TrimSpace(ref)
	var seq int
	if ref == "" || ref == latestVersion {
		latest, err := s.versions.Latest(ctx)
		if err != nil {
			s.logger.Error(ctx, "Failed to read the latest version", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if latest == 0 {
			return nil, &ErrorNoVersions
		}
		seq = latest
	} else {
		number, err := strconv.Atoi(ref)
		if err != nil || number <= 0 {
			return nil, &ErrorInvalidVersion
		}
		seq = number
	}

	version, err := s.versions.Get(ctx, seq)
	if err != nil {
		s.logger.Error(ctx, "Failed to read the version", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if version == nil {
		return nil, &ErrorVersionNotFound
	}
	return version, nil
}

func (s *versionService) GetApplied(ctx context.Context,
	gatewayID string) (*AppliedVersion, *tidcommon.ServiceError) {
	if _, svcErr := s.gateway(ctx, gatewayID); svcErr != nil {
		return nil, svcErr
	}
	applied, err := s.versions.GetApplied(ctx, gatewayID)
	if err != nil {
		s.logger.Error(ctx, "Failed to read what the gateway holds", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if applied == nil {
		return &AppliedVersion{GatewayID: gatewayID}, nil
	}
	return applied, nil
}

func (s *versionService) Diff(ctx context.Context, gatewayID, ref string) (*Diff, *tidcommon.ServiceError) {
	if _, svcErr := s.gateway(ctx, gatewayID); svcErr != nil {
		return nil, svcErr
	}
	target, svcErr := s.resolve(ctx, ref)
	if svcErr != nil {
		return nil, svcErr
	}
	_, diff, svcErr := s.plan(ctx, gatewayID, target)
	return diff, svcErr
}

func (s *versionService) Apply(ctx context.Context, gatewayID string,
	req ApplyRequest) (*ApplyResult, *tidcommon.ServiceError) {
	gw, svcErr := s.gateway(ctx, gatewayID)
	if svcErr != nil {
		return nil, svcErr
	}
	target, svcErr := s.resolve(ctx, req.Version)
	if svcErr != nil {
		return nil, svcErr
	}
	return s.write(ctx, gw, target, req.DryRun, false)
}

func (s *versionService) Revert(ctx context.Context, gatewayID string,
	req RevertRequest) (*ApplyResult, *tidcommon.ServiceError) {
	gw, svcErr := s.gateway(ctx, gatewayID)
	if svcErr != nil {
		return nil, svcErr
	}
	applied, err := s.versions.GetApplied(ctx, gatewayID)
	if err != nil {
		s.logger.Error(ctx, "Failed to read what the gateway holds", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if applied == nil || applied.PreviousVersion == 0 {
		return nil, &ErrorNothingToRevert
	}
	target, svcErr := s.resolve(ctx, strconv.Itoa(applied.PreviousVersion))
	if svcErr != nil {
		return nil, svcErr
	}
	return s.write(ctx, gw, target, req.DryRun, true)
}

func (s *versionService) Forget(ctx context.Context, gatewayID string) {
	if err := s.versions.DeleteApplied(ctx, gatewayID); err != nil {
		s.logger.Warn(ctx, "Failed to forget what a removed gateway held", log.Error(err))
	}
}

// write applies a version to a gateway and, unless it is a dry run, records that the gateway holds it.
//
// After an apply, the version the gateway held becomes the one a revert returns to. After a revert,
// the version reverted from takes that place, so a revert can itself be undone.
func (s *versionService) write(ctx context.Context, gw *Gateway, target *Version,
	dryRun, isRevert bool) (*ApplyResult, *tidcommon.ServiceError) {
	applied, diff, svcErr := s.plan(ctx, gw.ID, target)
	if svcErr != nil {
		return nil, svcErr
	}

	key, svcErr := s.reveal(ctx, gw.Key)
	if svcErr != nil {
		return nil, svcErr
	}
	// A version names values the gateway holds rather than carrying them, so one the gateway lacks
	// would leave a resource unusable once applied. A dry run reports them; an apply is refused.
	missing, svcErr := s.missingValues(ctx, gw, key, target)
	if svcErr != nil {
		return nil, svcErr
	}
	if !missing.empty() && !dryRun {
		return nil, &ErrorMissingValues
	}
	variables := map[string]interface{}{}
	if target.variables != "" {
		opened, svcErr := s.revealVariables(ctx, target.variables)
		if svcErr != nil {
			return nil, svcErr
		}
		variables = opened
	}

	request := gatewayImportRequest{
		Content:   target.Resources,
		Variables: variables,
		DryRun:    dryRun,
		Options:   gatewayImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
		Deletions: deletionsOf(diff.Changes),
	}
	answer, err := s.client.Import(ctx, gw, key, request)
	if err != nil {
		s.logger.Error(ctx, "Failed to apply a version to a gateway", log.String("gatewayId", gw.ID),
			log.Int("version", target.Seq), log.Error(err))
		return nil, &ErrorGatewayUnreachable
	}

	result := &ApplyResult{GatewayID: gw.ID, DryRun: dryRun, Diff: *diff, Import: answer}
	if !missing.empty() {
		result.Missing = missing
	}
	// The gateway completed the import, so it holds this version, even where it refused a resource:
	// the import's own results say which. Every apply sends the version whole, so a refused resource
	// is sent again by the next one; only the deletions are worked out from what the gateway held.
	if dryRun {
		return result, nil
	}

	held, previous := 0, 0
	if applied != nil {
		held, previous = applied.AppliedVersion, applied.PreviousVersion
	}
	// Re-applying the version a gateway already holds leaves its revert target where it was.
	if isRevert || held != target.Seq {
		previous = held
	}
	if err := s.versions.SetApplied(ctx, gw.ID, target.Seq, previous, held); err != nil {
		if errors.Is(err, errVersionRemoved) {
			s.logger.Warn(ctx, "A version was removed while it was applied to a gateway",
				log.String("gatewayId", gw.ID), log.Int("version", target.Seq))
			return nil, &ErrorVersionRemoved
		}
		if errors.Is(err, errAppliedChanged) {
			s.logger.Warn(ctx, "Another apply to the gateway finished first", log.String("gatewayId", gw.ID),
				log.Int("version", target.Seq))
			return nil, &ErrorAppliedChanged
		}
		s.logger.Error(ctx, "Failed to record what the gateway holds", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	result.Recorded = true
	s.logger.Info(ctx, "Applied a configuration version to a gateway", log.String("gatewayId", gw.ID),
		log.Int("version", target.Seq))
	return result, nil
}

// missingValues returns the variables and secrets a version refers to that a gateway does not hold.
func (s *versionService) missingValues(ctx context.Context, gw *Gateway, key string,
	target *Version) (*MissingValues, *tidcommon.ServiceError) {
	variables, secrets := referencesIn(target.Resources)
	missing := &MissingValues{}
	for _, wanted := range []struct {
		collection string
		names      []string
		into       *[]string
	}{
		{collectionVariables, variables, &missing.Variables},
		{collectionSecrets, secrets, &missing.Secrets},
	} {
		if len(wanted.names) == 0 {
			continue
		}
		held, err := s.client.Held(ctx, gw, key, wanted.collection, wanted.names)
		if err != nil {
			s.logger.Error(ctx, "Failed to read which values a gateway holds", log.String("gatewayId", gw.ID),
				log.Error(err))
			return nil, &ErrorGatewayUnreachable
		}
		for _, name := range wanted.names {
			if !held[name] {
				*wanted.into = append(*wanted.into, name)
			}
		}
	}
	return missing, nil
}

func (s *versionService) ForwardStore(ctx context.Context, gatewayID string,
	call StoreCall) (*StoreAnswer, *tidcommon.ServiceError) {
	gw, svcErr := s.gateway(ctx, gatewayID)
	if svcErr != nil {
		return nil, svcErr
	}
	key, svcErr := s.reveal(ctx, gw.Key)
	if svcErr != nil {
		return nil, svcErr
	}
	answer, err := s.client.Forward(ctx, gw, key, call)
	if err != nil {
		s.logger.Error(ctx, "Failed to reach a gateway's store", log.String("gatewayId", gw.ID), log.Error(err))
		return nil, &ErrorGatewayUnreachable
	}
	return answer, nil
}

// plan diffs a version against what a gateway holds.
func (s *versionService) plan(ctx context.Context, gatewayID string,
	target *Version) (*AppliedVersion, *Diff, *tidcommon.ServiceError) {
	applied, err := s.versions.GetApplied(ctx, gatewayID)
	if err != nil {
		s.logger.Error(ctx, "Failed to read what the gateway holds", log.Error(err))
		return nil, nil, &tidcommon.InternalServerError
	}

	var held []bundleResource
	diff := &Diff{ToVersion: target.Seq}
	if applied != nil && applied.AppliedVersion > 0 {
		heldVersion, err := s.versions.Get(ctx, applied.AppliedVersion)
		if err != nil {
			s.logger.Error(ctx, "Failed to read the version the gateway holds", log.Error(err))
			return nil, nil, &tidcommon.InternalServerError
		}
		// A version a gateway holds is never pruned, and only a version still kept is recorded, so it
		// is always there. Were it not, the deletions could not be worked out, and applying without
		// them would leave behind what the new version dropped.
		if heldVersion == nil {
			s.logger.Error(ctx, "The version a gateway holds is not kept", log.String("gatewayId", gatewayID),
				log.Int("version", applied.AppliedVersion))
			return nil, nil, &tidcommon.InternalServerError
		}
		held = parseBundle(heldVersion.Resources)
		diff.FromVersion = heldVersion.Seq
	}
	diff.Changes = diffBundles(held, parseBundle(target.Resources))
	diff.Summary = summarize(diff.Changes)
	return applied, diff, nil
}

// gateway reads a gateway with its stored key.
func (s *versionService) gateway(ctx context.Context, id string) (*Gateway, *tidcommon.ServiceError) {
	if strings.TrimSpace(id) == "" {
		return nil, &ErrorInvalidGatewayID
	}
	gw, err := s.gateways.GetByID(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "Failed to read the gateway", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if gw == nil {
		return nil, &ErrorGatewayNotFound
	}
	return gw, nil
}

// deletionsOf turns removed resources into the deletions an import takes. A resource without an id
// cannot be named to the gateway, so it is left in place.
func deletionsOf(changes []Change) []gatewayDeletion {
	var deletions []gatewayDeletion
	for _, change := range changes {
		if change.Change == ChangeDeleted && change.ID != "" {
			deletions = append(deletions, gatewayDeletion{ResourceType: change.ResourceType, ID: change.ID})
		}
	}
	return deletions
}

// parseEnvFile reads the KEY=value lines an export writes beside its documents.
func parseEnvFile(content string) map[string]string {
	variables := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		variables[name] = value
	}
	return variables
}

// seal encrypts a version's values as one secret. They can include credentials, so they are held the
// way any stored credential is.
func (s *versionService) seal(ctx context.Context,
	variables map[string]string) (string, *tidcommon.ServiceError) {
	encoded, err := json.Marshal(variables)
	if err != nil {
		s.logger.Error(ctx, "Failed to encode a version's values", log.Error(err))
		return "", &tidcommon.InternalServerError
	}
	property, err := cmodels.NewProperty("variables", string(encoded), true)
	if err == nil {
		var stored string
		if stored, err = cmodels.SerializePropertiesToJSONArray([]cmodels.Property{*property}); err == nil {
			return stored, nil
		}
	}
	s.logger.Error(ctx, "Failed to protect a version's values", log.Error(err))
	return "", &tidcommon.InternalServerError
}

// revealVariables opens a version's sealed values.
func (s *versionService) revealVariables(ctx context.Context,
	sealed string) (map[string]interface{}, *tidcommon.ServiceError) {
	opened, svcErr := s.reveal(ctx, sealed)
	if svcErr != nil {
		return nil, svcErr
	}
	stored := map[string]string{}
	if err := json.Unmarshal([]byte(opened), &stored); err != nil {
		s.logger.Error(ctx, "Failed to read a version's values", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return importVariables(stored), nil
}

// importVariables turns a version's values into what an import resolves its placeholders with.
//
// An export writes a list as JSON text in its .env, since a .env holds strings, and the document
// ranges over it. An import needs the list itself to range over, so a value that is a JSON array is
// given as one.
func importVariables(stored map[string]string) map[string]interface{} {
	variables := make(map[string]interface{}, len(stored))
	for name, value := range stored {
		var list []interface{}
		if strings.HasPrefix(strings.TrimSpace(value), "[") && json.Unmarshal([]byte(value), &list) == nil {
			variables[name] = list
			continue
		}
		variables[name] = value
	}
	return variables
}

// reveal opens a sealed secret.
//
// A key registered through the API is sealed. One declared in a file is held in memory as the file
// wrote it, since nothing about a declared gateway is stored, so a value that is not sealed is taken
// as it is.
func (s *versionService) reveal(ctx context.Context, stored string) (string, *tidcommon.ServiceError) {
	properties, err := cmodels.DeserializePropertiesFromJSON(stored)
	if err != nil || len(properties) == 0 {
		return stored, nil
	}
	value, err := properties[0].GetValue()
	if err != nil {
		s.logger.Error(ctx, "Failed to open a sealed value", log.Error(err))
		return "", &tidcommon.InternalServerError
	}
	return value, nil
}
