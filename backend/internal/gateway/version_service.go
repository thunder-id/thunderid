// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// maxOriginLength is the longest label an apply may give for where its version came from, since each
// entry of a master it writes records it.
const maxOriginLength = 255

// latestVersion names the newest captured version.
const latestVersion = "latest"

// minHashPrefix is the fewest characters of a version's hash that name it, as many as are shown.
const minHashPrefix = 7

// hashPrefix is what a version's hash, or a prefix of it, looks like.
var hashPrefix = regexp.MustCompile(`^[0-9a-f]{` + strconv.Itoa(minHashPrefix) + `,64}$`)

// currentVersion names the configuration as it stands, read but not captured. Only a read of one
// version takes it: an apply, a revert and a diff need a version that was kept.
const currentVersion = "current"

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
	// Promote applies the version one gateway runs to another, as a console promotes one stage to the
	// next.
	Promote(ctx context.Context, req PromoteRequest) (*ApplyResult, *tidcommon.ServiceError)
	// Revert returns a gateway to the version it held before the one it holds now.
	Revert(ctx context.Context, gatewayID string, req RevertRequest) (*ApplyResult, *tidcommon.ServiceError)
	// Forget drops what a removed gateway held, so a gateway registered later under the same id does
	// not inherit it.
	Forget(ctx context.Context, gatewayID string)
	// GetMaster reports what a gateway holds, resource by resource, as the gateway last accepted each.
	GetMaster(ctx context.Context, gatewayID string) (*Master, *tidcommon.ServiceError)
	// ListMasters reports a gateway's masters, newest first, without their entries.
	ListMasters(ctx context.Context, gatewayID string) ([]Master, *tidcommon.ServiceError)
	// GetResourceHistory reports one resource's entry across a gateway's masters, newest first.
	GetResourceHistory(ctx context.Context, gatewayID, key string) (*ResourceHistory, *tidcommon.ServiceError)
}

type versionService struct {
	gateways storeInterface
	versions versionStoreInterface
	exporter export.ExportServiceInterface
	client   gatewayClientInterface
	// secretFields names, by resource type, the fields a secret reference may stand in.
	secretFields map[string][]string
	names        actorNamer
	// autoApply makes a capture of the current version apply it to the default gateway.
	autoApply bool
	logger    *log.Logger
}

func newVersionService(gateways storeInterface, versions versionStoreInterface,
	exporter export.ExportServiceInterface, client gatewayClientInterface,
	secretFields map[string][]string, names actorNamer) *versionService {
	return &versionService{
		gateways:     gateways,
		versions:     versions,
		exporter:     exporter,
		client:       client,
		secretFields: secretFields,
		names:        names,
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GatewayVersionService")),
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
		PresentationDefinitions: all, AuthorizationDetailTypes: all, NotificationTemplates: all,
		Options: &export.ExportOptions{IncludeDependencies: true, Format: "yaml"},
	}
}

// Capture captures the configuration as a version and answers with it, its user named.
func (s *versionService) Capture(ctx context.Context, req CaptureRequest) (*Version, *tidcommon.ServiceError) {
	version, svcErr := s.capture(ctx, req)
	return s.named(ctx, version), svcErr
}

// capture captures the configuration as a version. Its user is left unnamed: the capture after each
// change discards the answer, so a name read for it would be an issuer call for nothing.
func (s *versionService) capture(ctx context.Context, req CaptureRequest) (*Version, *tidcommon.ServiceError) {
	exported, svcErr := s.exporter.ExportResources(ctx, configurationExport())
	if svcErr != nil {
		return nil, svcErr
	}
	// A resource the export could not write, such as a user without a username, is left out of the
	// version as the export leaves it out of its own answer, and is named in the capture's answer so
	// the administrator sees what the version is missing.
	var skipped []declarativeresource.ExportError
	if exported.Summary != nil {
		skipped = exported.Summary.Errors
	}
	for _, failed := range skipped {
		s.logger.Warn(ctx, "A resource the export could not write is left out of the version",
			log.String("resourceType", failed.ResourceType), log.MaskedString("resourceID", failed.ResourceID),
			log.String("error", failed.Error))
	}

	documents := documentsOf(exported)

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

	content := joinBundle(documents)
	// A secret standing where no credential is kept would be refused by every gateway the version is
	// applied to, so the configuration is refused here, where the field can still be put right.
	if misplaced := misplacedSecretsIn(content, s.secretFields); len(misplaced) > 0 {
		s.logger.Warn(ctx, "The configuration refers to a secret from a field that holds no credential",
			log.String("resourceType", misplaced[0].ResourceType), log.String("field", misplaced[0].Field),
			log.Int("misplaced", len(misplaced)))
		return nil, misplacedSecretError(misplaced)
	}
	hash := versionHash(content)
	if kept, svcErr := s.versionWithHash(ctx, hash); svcErr != nil || kept != nil {
		return kept, svcErr
	}

	captured := Version{Hash: hash, Name: strings.TrimSpace(req.Name), Note: strings.TrimSpace(req.Note),
		Resources: content, variables: sealed, Current: req.Current, CreatedBy: actorOf(ctx)}
	if req.Current {
		// The current version is kept up to date in place until a gateway holds it, so a console that
		// captures on every change leaves one version for them rather than one each.
		replaced, err := s.versions.ReplaceCurrent(ctx, captured)
		if err != nil {
			// A capture of the same configuration at the same moment collides on its hash; the one
			// refused answers with the version the other kept.
			if kept, svcErr := s.versionWithHash(ctx, hash); svcErr == nil && kept != nil {
				return kept, nil
			}
			s.logger.Error(ctx, "Failed to replace the current version", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if replaced != nil {
			s.logger.Info(ctx, "Replaced the current configuration version", log.String("version", replaced.Hash),
				log.Int("skipped", len(skipped)))
			replaced.Skipped = skipped
			return replaced, nil
		}
	}

	version, err := s.versions.Add(ctx, captured)
	if err != nil {
		// Two captures of the same configuration at once collide on its hash; the one refused answers
		// with the version the other kept.
		if kept, svcErr := s.versionWithHash(ctx, hash); svcErr == nil && kept != nil {
			return kept, nil
		}
		s.logger.Error(ctx, "Failed to capture a version", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if err := s.versions.Prune(ctx, version.Seq-maxVersions()); err != nil {
		// The version is captured; an older one surviving a little longer is not worth failing for.
		s.logger.Warn(ctx, "Failed to remove old versions", log.Error(err))
	}
	s.logger.Info(ctx, "Captured a configuration version", log.String("version", version.Hash),
		log.Int("skipped", len(skipped)))
	version.Skipped = skipped
	return version, nil
}

// named puts the name to the user a version records, through the trusted issuer's user lookup, and
// leaves the user out when no name is known.
func (s *versionService) named(ctx context.Context, version *Version) *Version {
	if version != nil {
		s.names.fill(ctx, version.CreatedBy)
		version.CreatedBy = shown(version.CreatedBy)
	}
	return version
}

// versionHash names a version by the configuration it captured, so the same configuration is always
// the same version.
func versionHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// versionWithHash reads the version a hash already names, marked unchanged, or nil when none does.
func (s *versionService) versionWithHash(ctx context.Context, hash string) (*Version, *tidcommon.ServiceError) {
	found, err := s.versions.Find(ctx, hash)
	if err != nil {
		s.logger.Error(ctx, "Failed to look for a version", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if len(found) == 0 {
		return nil, nil
	}
	version, svcErr := s.bySeq(ctx, found[0])
	if svcErr != nil {
		return nil, svcErr
	}
	version.Unchanged = true
	return version, nil
}

func (s *versionService) ListVersions(ctx context.Context) ([]Version, *tidcommon.ServiceError) {
	versions, err := s.versions.List(ctx)
	if err != nil {
		s.logger.Error(ctx, "Failed to list the versions", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	// Only the newest version can be the current one; an older one captured as current was kept once
	// a gateway held it, and is a version like any other now.
	for i := 1; i < len(versions); i++ {
		versions[i].Current = false
	}
	actors := make([]*Actor, 0, len(versions))
	for i := range versions {
		actors = append(actors, versions[i].CreatedBy)
	}
	s.names.fill(ctx, actors...)
	for i := range versions {
		versions[i].CreatedBy = shown(versions[i].CreatedBy)
	}
	return versions, nil
}

func (s *versionService) GetVersion(ctx context.Context, ref string) (*Version, *tidcommon.ServiceError) {
	if strings.TrimSpace(ref) == currentVersion {
		return s.current(ctx)
	}
	version, svcErr := s.resolve(ctx, ref)
	if svcErr != nil {
		return nil, svcErr
	}
	read := *version
	read.References = referencesOf(version.Resources)
	return s.named(ctx, &read), nil
}

// current reads the configuration as it stands, the way a capture reads it, without keeping it. Its
// references say what each value is for as soon as a resource is written, before any capture, and its
// hash is the one a capture would give it.
func (s *versionService) current(ctx context.Context) (*Version, *tidcommon.ServiceError) {
	exported, svcErr := s.exporter.ExportResources(ctx, configurationExport())
	if svcErr != nil {
		return nil, svcErr
	}
	content := joinBundle(documentsOf(exported))
	version := &Version{Hash: versionHash(content), CreatedAt: time.Now().UTC(), Resources: content,
		References: referencesOf(content)}
	if exported.Summary != nil {
		version.Skipped = exported.Summary.Errors
	}
	return version, nil
}

// documentsOf returns each file an export wrote as one document.
func documentsOf(exported *export.ExportResponse) []string {
	documents := make([]string, 0, len(exported.Files))
	for _, file := range exported.Files {
		documents = append(documents, strings.TrimSpace(file.Content))
	}
	return documents
}

// resolve reads the version a reference names: its hash, a prefix of at least minHashPrefix of its
// characters, or "latest" when empty.
func (s *versionService) resolve(ctx context.Context, ref string) (*Version, *tidcommon.ServiceError) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if ref == "" || ref == latestVersion {
		latest, err := s.versions.Latest(ctx)
		if err != nil {
			s.logger.Error(ctx, "Failed to read the latest version", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if latest == 0 {
			return nil, &ErrorNoVersions
		}
		return s.bySeq(ctx, latest)
	}
	if !hashPrefix.MatchString(ref) {
		return nil, &ErrorInvalidVersion
	}
	found, err := s.versions.Find(ctx, ref)
	if err != nil {
		s.logger.Error(ctx, "Failed to find the version", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	switch len(found) {
	case 0:
		return nil, &ErrorVersionNotFound
	case 1:
		return s.bySeq(ctx, found[0])
	default:
		return nil, &ErrorAmbiguousVersion
	}
}

// bySeq reads the version at a place in the deployment's order.
func (s *versionService) bySeq(ctx context.Context, seq int) (*Version, *tidcommon.ServiceError) {
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
	s.names.fill(ctx, applied.AppliedBy)
	applied.AppliedBy = shown(applied.AppliedBy)
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
	if svcErr != nil {
		return nil, svcErr
	}
	held, svcErr := s.heldBack(ctx, gatewayID, nil)
	if svcErr != nil {
		return nil, svcErr
	}
	if _, svcErr := holdBack(diff, nil, nil, held); svcErr != nil {
		return nil, svcErr
	}
	return diff, nil
}

func (s *versionService) Apply(ctx context.Context, gatewayID string,
	req ApplyRequest) (*ApplyResult, *tidcommon.ServiceError) {
	gw, svcErr := s.gateway(ctx, gatewayID)
	if svcErr != nil {
		return nil, svcErr
	}
	origin := strings.TrimSpace(req.Origin)
	if utf8.RuneCountInString(origin) > maxOriginLength {
		return nil, &ErrorOriginTooLong
	}
	target, svcErr := s.resolve(ctx, req.Version)
	if svcErr != nil {
		return nil, svcErr
	}
	// An apply that asks for "latest" with nothing chosen and nothing confirmed is the one a burst of
	// changes repeats, so it is the one folded into the apply already running. A version named by its
	// hash, a selection, or a confirmation is a choice of its own, which a later run could not carry
	// out as asked. What the queued run would refuse for its removals is refused now instead.
	coalesce := !req.DryRun && req.Selection == nil && len(req.LeaveOut) == 0 && !req.ConfirmRemovals &&
		asksForLatest(req.Version)
	return s.write(ctx, gw, target, applyOptions{dryRun: req.DryRun, selection: req.Selection,
		leaveOut:        req.LeaveOut,
		confirmRemovals: req.ConfirmRemovals, coalesce: coalesce, origin: origin})
}

// asksForLatest reports whether a reference asks for the latest version rather than naming one.
func asksForLatest(ref string) bool {
	ref = strings.ToLower(strings.TrimSpace(ref))
	return ref == "" || ref == latestVersion
}

// refuseUnconfirmedRemovalsOf runs the removal check an apply would, against what the gateway holds
// now, before the apply is queued behind another.
func (s *versionService) refuseUnconfirmedRemovalsOf(ctx context.Context, gw *Gateway,
	target *Version) *tidcommon.ServiceError {
	_, diff, svcErr := s.plan(ctx, gw.ID, target)
	if svcErr != nil {
		return svcErr
	}
	// What the gateway holds back is not sent, so it is not counted, as the apply itself counts.
	selection, svcErr := s.holdBackOn(ctx, gw.ID, diff, nil, nil)
	if svcErr != nil {
		return svcErr
	}
	sent, _ := chosen(diff, target, selection)
	return s.refuseUnconfirmedRemovals(ctx, gw, len(sent.deletions), applyOptions{})
}

// Promote applies the version the source is recorded as running to the target, as far as the source
// holds it: an addition or update is sent only when the source holds it deployed as the version has
// it, and what the target holds back stays out. The target's master records the source as the origin
// of each resource the promotion writes.
func (s *versionService) Promote(ctx context.Context, req PromoteRequest) (*ApplyResult, *tidcommon.ServiceError) {
	if req.SourceGatewayID == req.TargetGatewayID {
		return nil, &ErrorPromoteToItself
	}
	source, svcErr := s.GetApplied(ctx, req.SourceGatewayID)
	if svcErr != nil {
		return nil, svcErr
	}
	if source.Applied == "" {
		return nil, &ErrorSourceRunsNothing
	}
	master, svcErr := s.GetMaster(ctx, req.SourceGatewayID)
	if svcErr != nil {
		return nil, svcErr
	}
	gw, svcErr := s.gateway(ctx, req.TargetGatewayID)
	if svcErr != nil {
		return nil, svcErr
	}
	version, svcErr := s.resolve(ctx, source.Applied)
	if svcErr != nil {
		return nil, svcErr
	}
	// The selection is read from the diff the apply itself builds, under its lease, so what the
	// target holds is read once and the selection cannot come from an earlier read than the apply's.
	return s.write(ctx, gw, version, applyOptions{dryRun: req.DryRun, confirmRemovals: req.ConfirmRemovals,
		origin: req.SourceGatewayID,
		choose: func(diff *Diff) []string { return promoteSelection(master, version, diff) }})
}

// promoteSelection picks the target's changes a promotion sends: a removal, or an addition or update
// the source's master holds deployed with the version's content. A resource an apply to the source left
// out was recorded with the version all the same while the source kept an earlier document, so it is
// not promoted. What the target holds back stays out: selecting it would send it and stop holding it
// back.
func promoteSelection(source *Master, version *Version, target *Diff) []string {
	inVersion := map[string]string{}
	for _, resource := range parseBundle(version.Resources) {
		inVersion[resource.key()] = contentHash(resource.Content)
	}
	deployed := map[string]bool{}
	for _, entry := range source.Entries {
		if entry.State == EntryDeployed && entry.ContentHash == inVersion[entry.Key] {
			deployed[entry.Key] = true
		}
	}
	selection := []string{}
	for _, change := range target.Changes {
		if change.Change == ChangeUnchanged || change.HeldBack {
			continue
		}
		if (change.Change == ChangeAdded || change.Change == ChangeUpdated) && !deployed[change.Key] {
			continue
		}
		selection = append(selection, change.Key)
	}
	return selection
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
	target, svcErr := s.bySeq(ctx, applied.PreviousVersion)
	if svcErr != nil {
		return nil, svcErr
	}
	options := applyOptions{dryRun: req.DryRun, isRevert: true, confirmRemovals: req.ConfirmRemovals}
	// A revert returns the gateway to the master it held before the version it holds now, which is
	// the earlier version as far as the gateway held it. Only a gateway recorded before masters were,
	// or whose earlier master is no longer kept, is returned to the earlier version whole.
	if applied.PreviousMasterSeq > 0 {
		earlier, err := s.versions.GetMasterAt(ctx, gatewayID, applied.PreviousMasterSeq)
		if err != nil {
			s.logger.Error(ctx, "Failed to read the master to revert to", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if earlier != nil {
			reverted := *target
			reverted.Resources = earlier.Resources
			options.whole, options.revertTo = target, earlier
			target = &reverted
		}
	}
	return s.write(ctx, gw, target, options)
}

func (s *versionService) Forget(ctx context.Context, gatewayID string) {
	if err := s.versions.DeleteApplied(ctx, gatewayID); err != nil {
		s.logger.Warn(ctx, "Failed to forget what a removed gateway held", log.Error(err))
	}
	if err := s.versions.DeleteExcluded(ctx, gatewayID); err != nil {
		s.logger.Warn(ctx, "Failed to forget what a removed gateway left alone", log.Error(err))
	}
	if err := s.versions.DeleteLease(ctx, gatewayID); err != nil {
		s.logger.Warn(ctx, "Failed to forget a removed gateway's apply lease", log.Error(err))
	}
	if err := s.versions.DeleteMasters(ctx, gatewayID); err != nil {
		s.logger.Warn(ctx, "Failed to forget a removed gateway's masters", log.Error(err))
	}
}

// applyOptions is how an apply or revert is to run.
type applyOptions struct {
	dryRun, isRevert bool
	// confirmRemovals lets the apply remove more than the deployment's threshold of resources.
	confirmRemovals bool
	// coalesce folds the apply into one already running, to be sent again once it finishes.
	coalesce  bool
	selection *[]string
	// leaveOut names changes to hold back from this apply and the ones after it.
	leaveOut []string
	// choose, when given, picks the selection from the diff once the apply has built it and marked what
	// the gateway holds back, in place of a selection made before the apply.
	choose func(diff *Diff) []string
	// origin is the label the master records for where the version came from.
	origin string
	// revertTo is the earlier master a revert returns the gateway to, and whole the version recorded
	// with it; the master's documents are what is sent, and each keeps where it came from.
	revertTo *Master
	whole    *Version
}

// write applies a version to a gateway and, unless it is a dry run, records that the gateway holds it.
//
// Only what the gateway's master says it lacks or holds differently is sent, narrowed to the
// selection when there is one; the rest stays on offer. The master is then updated from the gateway's
// answer, resource by resource.
//
// After an apply, the version the gateway held becomes the one a revert returns to. After a revert,
// the gateway has no revert target until a later apply.
func (s *versionService) write(ctx context.Context, gw *Gateway, target *Version,
	options applyOptions) (*ApplyResult, *tidcommon.ServiceError) {
	dryRun, isRevert := options.dryRun, options.isRevert
	// Settled before anything is sent, the version is given back its place as the current one on
	// every way out on which the gateway received nothing: a value it lacks, a store it could not
	// answer for, or an import that never reached it.
	delivered := false
	token := ""
	var prepared time.Time
	if !dryRun {
		release, held, claimedAt, queued, svcErr := s.lease(ctx, gw.ID, options.coalesce,
			func() *tidcommon.ServiceError { return s.refuseUnconfirmedRemovalsOf(ctx, gw, target) })
		if svcErr != nil {
			return nil, svcErr
		}
		prepared = claimedAt
		if queued {
			return &ApplyResult{GatewayID: gw.ID, Queued: true, QueuedVersion: target.Hash}, nil
		}
		token = held
		defer release()
		if svcErr := s.settle(ctx, target); svcErr != nil {
			return nil, svcErr
		}
		defer func() {
			if !delivered {
				// On a context of its own: the request's may be gone by now, and the version would
				// otherwise stay settled with nothing holding it.
				restore, cancel := context.WithTimeout(context.WithoutCancel(ctx), storeTimeout)
				defer cancel()
				s.unsettle(restore, target)
			}
		}()
	}
	applied, diff, svcErr := s.plan(ctx, gw.ID, target)
	if svcErr != nil {
		return nil, svcErr
	}
	key, svcErr := s.reveal(ctx, gw.Key)
	if svcErr != nil {
		return nil, svcErr
	}
	held, changed, svcErr := s.heldValues(ctx, gw, key, target, applied)
	if svcErr != nil {
		return nil, svcErr
	}
	// A resource whose value was set again since the last apply is sent again, as it differs on the
	// gateway even though no document does.
	diff.Changes = markValueChanged(diff.Changes, changed)
	diff.Summary = summarize(diff.Changes)
	selection, svcErr := s.selectionFor(ctx, gw, diff, options)
	if svcErr != nil {
		return nil, svcErr
	}
	sent, ok := chosen(diff, target, selection)
	if !ok {
		return nil, &ErrorInvalidSelection
	}
	values := valuesToRecord(held, target, changed, diff, applied)
	previous, svcErr := s.masterBefore(ctx, gw.ID, dryRun)
	if svcErr == nil {
		svcErr = s.refuseUnconfirmedRemovals(ctx, gw, len(sent.deletions), options)
	}
	if svcErr != nil {
		return nil, svcErr
	}
	missing, svcErr := s.missingValues(ctx, gw, held, sent.content, dryRun)
	if svcErr != nil {
		return nil, svcErr
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
		Content:   sent.content,
		Variables: variables,
		DryRun:    dryRun,
		Options:   gatewayImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
		Deletions: sent.deletions,
	}
	if !dryRun {
		if svcErr := s.mayStillSend(ctx, gw, target, token, prepared); svcErr != nil {
			return nil, svcErr
		}
	}
	// With nothing to send, the gateway is not called: its master already holds the version. The
	// version is recorded as applied all the same, so a revert returns to what was held before.
	answer, err := json.RawMessage(`{"results":[]}`), error(nil)
	if request.Content != "" || len(request.Deletions) > 0 {
		answer, err = s.client.Import(ctx, gw, key, request)
	}
	// Once the gateway may have written any of it, the version is kept as it is.
	delivered = mayHaveWritten(err)
	if err != nil {
		s.logger.Error(ctx, "Failed to apply a version to a gateway", log.String("gatewayID", gw.ID),
			log.Int("version", target.Seq), log.Error(err))
		return nil, unreachable(err)
	}

	result := applyResultOf(gw, dryRun, diff, answer, missing, changed)
	if dryRun {
		return result, nil
	}
	// The version is recorded whether or not the gateway now holds all of it. One it holds in part, as
	// a selection or a refusal left it, is recorded as partly applied, with the master the apply left,
	// and the diff goes on offering the rest. An apply that sent nothing leaves a recorded master as it
	// is; a gateway with none is given its first, so it is not later taken to hold the whole version it
	// is recorded as applied.
	whole := target
	if options.whole != nil {
		whole = options.whole
	}
	var write *masterWrite
	if request.Content != "" || len(request.Deletions) > 0 || previous.Seq == 0 {
		write = &masterWrite{sent: parseBundle(sent.content), deletions: request.Deletions, answer: answer,
			source: target.Hash, origin: options.origin, by: actorOf(ctx), at: time.Now().UTC(),
			earlier: options.revertTo}
	}
	var state appliedState
	master, svcErr := s.recordOutcome(ctx, previous, write, token,
		func(txCtx context.Context, master *Master) *tidcommon.ServiceError {
			state = levelWith(master, whole)
			if svcErr := s.record(txCtx, gw, target, applied, isRevert, values, token, state); svcErr != nil {
				return svcErr
			}
			if isRevert {
				return nil
			}
			return s.rememberHeldBack(txCtx, gw.ID, diff, options.leaveOut)
		})
	if svcErr != nil {
		return nil, svcErr
	}
	result.Master = master.Hash
	result.Recorded = true
	result.Complete = state.complete
	s.logger.Info(ctx, "Applied a configuration version to a gateway", log.String("gatewayID", gw.ID),
		log.Int("version", target.Seq), log.Bool("complete", state.complete), log.Int("held", state.held),
		log.Int("total", state.total))
	return result, nil
}

// recordOutcome writes what an apply left: the master, from the gateway's own account of what it holds
// resource by resource, when write says what the apply sent, and the version recorded as applied, by
// recordApplied from that master. Both are written in one transaction and only while the apply's token
// holds the gateway's lease, so an apply another node took over records neither, and neither is kept
// without the other.
func (s *versionService) recordOutcome(ctx context.Context, previous *Master, write *masterWrite, token string,
	recordApplied func(txCtx context.Context, master *Master) *tidcommon.ServiceError) (
	*Master, *tidcommon.ServiceError) {
	master := previous
	var refused *tidcommon.ServiceError
	err := s.versions.Transact(ctx, func(txCtx context.Context) error {
		if write != nil {
			master, refused = s.recordMaster(txCtx, previous, *write, token)
		}
		if refused == nil {
			refused = recordApplied(txCtx, master)
		}
		if refused != nil {
			return errApplyNotRecorded
		}
		return nil
	})
	if refused != nil {
		return nil, refused
	}
	if err != nil {
		s.logger.Error(ctx, "Failed to record what the gateway holds", log.String("gatewayID", previous.GatewayID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if master.Seq != previous.Seq {
		s.logger.Info(ctx, "Recorded the gateway's master", log.String("gatewayID", previous.GatewayID),
			log.Int("master", master.Seq), log.Int("deployed", master.Summary.Deployed),
			log.Int("failed", master.Summary.Failed), log.Int("removed", master.Summary.Removed))
		s.pruneMasters(ctx, previous.GatewayID, master.Seq)
	}
	return master, nil
}

// levelWith says how far a master holds a version: how many of the version's resources it holds
// deployed as the version has them, of how many, and whether it holds the version whole, nothing
// missing and nothing more.
func levelWith(master *Master, version *Version) appliedState {
	state := appliedState{masterSeq: master.Seq, master: master.Hash}
	entries := make(map[string]MasterEntry, len(master.Entries))
	for _, entry := range master.Entries {
		entries[entry.Key] = entry
	}
	inVersion := map[string]bool{}
	for _, resource := range parseBundle(version.Resources) {
		inVersion[resource.key()] = true
		state.total++
		entry, ok := entries[resource.key()]
		if ok && entry.State == EntryDeployed && entry.ContentHash == contentHash(resource.Content) {
			state.held++
		}
	}
	extra := false
	for _, resource := range parseBundle(master.Resources) {
		if !inVersion[resource.key()] {
			extra = true
			break
		}
	}
	state.complete = state.held == state.total && !extra
	return state
}

// masterBefore reads the master a gateway holds as an apply is about to send its import, under the
// apply's lease, so what the master is updated from is what the gateway held when the import went
// out. A dry run writes no master and reads none.
func (s *versionService) masterBefore(ctx context.Context, gatewayID string,
	dryRun bool) (*Master, *tidcommon.ServiceError) {
	if dryRun {
		return nil, nil
	}
	return s.master(ctx, gatewayID)
}

// mayStillSend reports whether an apply may send its import now. A lease held longer than an apply
// can run is taken over. An apply sends its import only within prepareBudget of its claim, so the
// import, bounded by importTimeout, ends before the lease could be taken over; and one whose lease
// was taken over meanwhile, on a node that stalled, sends nothing. So two applies never write the
// gateway at once.
func (s *versionService) mayStillSend(ctx context.Context, gw *Gateway, target *Version, token string,
	prepared time.Time) *tidcommon.ServiceError {
	if time.Since(prepared) > prepareBudget {
		s.logger.Warn(ctx, "An apply took too long to prepare and sends nothing", log.String("gatewayID", gw.ID),
			log.Int("version", target.Seq))
		return &ErrorApplyTookTooLong
	}
	holds, err := s.versions.HoldsLease(ctx, gw.ID, token)
	if err != nil {
		s.logger.Error(ctx, "Failed to check the gateway's apply lease", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if !holds {
		s.logger.Warn(ctx, "Another apply took over the gateway before the import was sent",
			log.String("gatewayID", gw.ID), log.Int("version", target.Seq))
		return &ErrorAppliedChanged
	}
	return nil
}

// record keeps what the gateway now holds: the version applied, and as what a revert returns to, the
// version it held before. Re-applying the version a gateway already holds leaves its revert target
// where it was.
func (s *versionService) record(ctx context.Context, gw *Gateway, target *Version, applied *AppliedVersion,
	isRevert bool, values, token string, state appliedState) *tidcommon.ServiceError {
	held, previous := 0, 0
	if applied != nil {
		held, previous = applied.AppliedVersion, applied.PreviousVersion
		state.previousMasterSeq = applied.PreviousMasterSeq
	}
	// Re-applying the version a gateway already holds leaves its revert target where it was, so
	// applying the rest of a partly applied version still reverts to what was held before it. A revert
	// leaves none: reverting again would return to the version just reverted from, which is not an
	// earlier one, so the gateway goes back one step only until a later apply. The master a revert
	// returns to moves with the version.
	switch {
	case isRevert:
		previous, state.previousMasterSeq = 0, 0
	case held != target.Seq:
		previous = held
		state.previousMasterSeq = 0
		if applied != nil {
			state.previousMasterSeq = applied.MasterSeq
		}
	}
	err := s.versions.SetApplied(ctx, gw.ID, target.Seq, target.Hash, previous, held, actorOf(ctx), values, token,
		state)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errVersionRemoved):
		s.logger.Warn(ctx, "A version was removed or replaced while it was applied to a gateway",
			log.String("gatewayID", gw.ID), log.Int("version", target.Seq))
		return &ErrorVersionRemoved
	case errors.Is(err, errAppliedChanged):
		s.logger.Warn(ctx, "Another apply to the gateway finished first", log.String("gatewayID", gw.ID),
			log.Int("version", target.Seq))
		return &ErrorAppliedChanged
	default:
		s.logger.Error(ctx, "Failed to record what the gateway holds", log.Error(err))
		return &tidcommon.InternalServerError
	}
}

// refuseUnconfirmedRemovals refuses an apply that would remove more resources than the deployment's
// threshold unless the caller confirmed it. A rollback, or a version many resources were deleted
// from, removes a great deal in one go; past the threshold it goes only once the caller has seen it,
// in a dry run, and said so.
func (s *versionService) refuseUnconfirmedRemovals(ctx context.Context, gw *Gateway, removals int,
	options applyOptions) *tidcommon.ServiceError {
	if options.dryRun || options.confirmRemovals {
		return nil
	}
	limit := removalConfirmationLimit()
	if limit <= 0 || removals <= limit {
		return nil
	}
	s.logger.Info(ctx, "Refused an apply that removes more than the threshold until it is confirmed",
		log.String("gatewayID", gw.ID), log.Int("removals", removals), log.Int("threshold", limit))
	return &ErrorRemovalsNeedConfirmation
}

// applyResultOf assembles what an apply reports, leaving out what is empty.
func applyResultOf(gw *Gateway, dryRun bool, diff *Diff, answer json.RawMessage, missing *MissingValues,
	changed *ChangedValues) *ApplyResult {
	result := &ApplyResult{GatewayID: gw.ID, DryRun: dryRun, Diff: *diff, Import: answer}
	if !missing.empty() {
		result.Missing = missing
	}
	if !changed.empty() {
		result.Changed = changed
	}
	return result
}

// settle keeps the version an apply sends as it is from now on, before anything is sent, so a change
// captured while the gateway imports it adds a version rather than rewriting this one under the
// apply. A version replaced or removed since it was read is refused here, before the gateway
// receives it.
func (s *versionService) settle(ctx context.Context, target *Version) *tidcommon.ServiceError {
	settled, err := s.versions.Settle(ctx, target.Seq, target.Hash)
	if err != nil {
		s.logger.Error(ctx, "Failed to settle the version to apply", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if !settled {
		s.logger.Warn(ctx, "A version was removed or replaced before it was applied",
			log.Int("version", target.Seq))
		return &ErrorVersionRemoved
	}
	return nil
}

// heldValues asks the gateway's store about every value the version refers to: which were set on it
// again since the version it holds was applied, and when each was last set, as the store says, for
// the next apply to compare with. Only the store's own times are compared with each other, never with
// this plane's clock. A value set since the last apply takes effect with this one, outside any
// version, so it is reported by name and the resources that refer to it are sent again.
func (s *versionService) heldValues(ctx context.Context, gw *Gateway, key string, target *Version,
	applied *AppliedVersion) (*HeldValues, *ChangedValues, *tidcommon.ServiceError) {
	variables, secrets := referencesIn(target.Resources)
	held, err := s.client.HeldValues(ctx, gw, key, variables, secrets)
	if err != nil {
		s.logger.Error(ctx, "Failed to read which values a gateway holds", log.String("gatewayID", gw.ID),
			log.Error(err))
		return nil, nil, unreachable(err)
	}
	changed := &ChangedValues{}
	if applied != nil {
		changed = held.changedFrom(variables, secrets, applied.heldValues)
		if !changed.empty() {
			changed.References = referencesTo(target.Resources, &MissingValues{Variables: changed.Variables,
				Secrets: changed.Secrets})
		}
	}
	return held, changed, nil
}

// valuesToRecord is when each value the version refers to was last set, as the apply records it for
// the next one to compare with. A value set again is recorded at its new time only when every
// resource that refers to it was sent; while one is left out, its earlier time is kept, so the next
// diff offers that resource again rather than taking the gateway to run it with the new value.
func valuesToRecord(held *HeldValues, target *Version, changed *ChangedValues, diff *Diff,
	applied *AppliedVersion) string {
	variables, secrets := referencesIn(target.Resources)
	sent := map[string]bool{}
	for _, change := range diff.Changes {
		if change.Selected {
			sent[change.Key] = true
		}
	}
	unsent := &MissingValues{}
	for _, reference := range changed.References {
		if sent[reference.key()] {
			continue
		}
		switch reference.Kind {
		case ReferenceSecret:
			unsent.Secrets = append(unsent.Secrets, reference.Name)
		default:
			unsent.Variables = append(unsent.Variables, reference.Name)
		}
	}
	previous := ""
	if applied != nil {
		previous = applied.heldValues
	}
	return held.snapshotKeeping(variables, secrets, unsent, previous)
}

// missingValues reads, from what the gateway's store holds, the values the content to send refers to
// that it lacks. A version names values the gateway holds rather than carrying them, so one the
// gateway lacks would leave a resource unusable once applied: a dry run reports them, and an apply is
// refused while there are any. Only what is sent is checked: a resource left on offer needs nothing
// yet.
func (s *versionService) missingValues(ctx context.Context, gw *Gateway, held *HeldValues, content string,
	dryRun bool) (*MissingValues, *tidcommon.ServiceError) {
	variables, secrets := referencesIn(content)
	missing := held.missing(variables, secrets)
	if !missing.empty() {
		missing.References = referencesTo(content, missing)
		if !dryRun {
			s.logger.Info(ctx, "Refused an apply while the gateway lacks values", log.String("gatewayID", gw.ID),
				log.Int("variables", len(missing.Variables)), log.Int("secrets", len(missing.Secrets)))
			return nil, &ErrorMissingValues
		}
	}
	return missing, nil
}

// markValueChanged offers as updated each resource that refers to a value set on the gateway again
// since the last apply, so the gateway reads the new value, even though the document did not change.
func markValueChanged(changes []Change, changed *ChangedValues) []Change {
	affected := map[string]bool{}
	for _, reference := range changed.References {
		affected[reference.key()] = true
	}
	for i := range changes {
		if affected[changes[i].Key] && changes[i].Change == ChangeUnchanged {
			changes[i].Change = ChangeUpdated
			changes[i].ValueChanged = true
		}
	}
	return changes
}

// selectionFor marks what the gateway holds back on the diff and settles the selection the apply sends:
// the caller's, one chosen from the diff now that it is marked, or every change not held back. A
// revert sends what it was given.
func (s *versionService) selectionFor(ctx context.Context, gw *Gateway, diff *Diff,
	options applyOptions) (*[]string, *tidcommon.ServiceError) {
	if options.isRevert {
		return options.selection, nil
	}
	selection, svcErr := s.holdBackOn(ctx, gw.ID, diff, options.selection, options.leaveOut)
	if svcErr != nil {
		return nil, svcErr
	}
	if options.choose != nil {
		picked := options.choose(diff)
		selection = &picked
	}
	return selection, nil
}

// mayHaveWritten reports whether an import's outcome leaves open that the gateway wrote some of it.
func mayHaveWritten(err error) bool {
	return err == nil || !errors.Is(err, errImportNotWritten)
}

// unsettle gives a version back its place as the current one when the apply that settled it sent
// nothing to the gateway, so the next capture after a change replaces it in place again rather than
// adding a version beside one no gateway ever received. The store restores it only while it is still
// the newest and no gateway runs or reverted from it.
func (s *versionService) unsettle(ctx context.Context, target *Version) {
	restored, err := s.versions.Unsettle(ctx, target.Seq, target.Hash)
	if err != nil {
		s.logger.Error(ctx, "Failed to make the version current again after an apply wrote nothing",
			log.Int("version", target.Seq), log.Error(err))
		return
	}
	if !restored {
		s.logger.Debug(ctx, "The version stays settled after an apply wrote nothing",
			log.Int("version", target.Seq))
	}
}

// plan diffs a version against what a gateway holds, as its master records it. A resource the
// gateway refused on its last apply is offered again, as updated even when the version and the master
// do not differ on it, since the gateway does not run it as the master holds it.
func (s *versionService) plan(ctx context.Context, gatewayID string,
	target *Version) (*AppliedVersion, *Diff, *tidcommon.ServiceError) {
	applied, err := s.versions.GetApplied(ctx, gatewayID)
	if err != nil {
		s.logger.Error(ctx, "Failed to read what the gateway holds", log.Error(err))
		return nil, nil, &tidcommon.InternalServerError
	}
	master, svcErr := s.master(ctx, gatewayID)
	if svcErr != nil {
		return nil, nil, svcErr
	}
	diff := &Diff{FromMaster: master.Hash, ToVersion: target.Hash}
	diff.Changes = markFailed(diffBundles(parseBundle(master.Resources), parseBundle(target.Resources)), master)
	diff.Summary = summarize(diff.Changes)
	// A version captured before captures refused one may still refer to a secret from a field that
	// holds no credential; the diff says so, since the gateway will refuse that resource.
	diff.MisplacedSecrets = misplacedSecretsIn(target.Resources, s.secretFields)
	return applied, diff, nil
}

// markFailed flags the changes whose resource the gateway refused on its last apply, and offers one
// the version does not differ on as updated, so it is sent again.
func markFailed(changes []Change, master *Master) []Change {
	failed := map[string]bool{}
	for _, entry := range master.Entries {
		if entry.State == EntryFailed {
			failed[entry.Key] = true
		}
	}
	for i := range changes {
		if !failed[changes[i].Key] {
			continue
		}
		changes[i].Failed = true
		if changes[i].Change == ChangeUnchanged {
			changes[i].Change = ChangeUpdated
		}
	}
	return changes
}

// gateway reads a gateway with its stored key.
func (s *versionService) gateway(ctx context.Context, id string) (*Gateway, *tidcommon.ServiceError) {
	return findGateway(ctx, s.gateways, s.logger, id)
}

// findGateway reads a gateway with its stored key.
func findGateway(ctx context.Context, gateways storeInterface, logger *log.Logger,
	id string) (*Gateway, *tidcommon.ServiceError) {
	if strings.TrimSpace(id) == "" {
		return nil, &ErrorInvalidGatewayID
	}
	gw, err := gateways.GetByID(ctx, id)
	if err != nil {
		logger.Error(ctx, "Failed to read the gateway", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if gw == nil {
		return nil, &ErrorGatewayNotFound
	}
	return gw, nil
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

func (s *versionService) reveal(ctx context.Context, stored string) (string, *tidcommon.ServiceError) {
	return reveal(ctx, s.logger, stored)
}

// reveal opens a sealed secret.
//
// A key registered through the API is sealed. One declared in a file is held in memory as the file
// wrote it, since nothing about a declared gateway is stored, so a value that is not sealed is taken
// as it is.
func reveal(ctx context.Context, logger *log.Logger, stored string) (string, *tidcommon.ServiceError) {
	properties, err := cmodels.DeserializePropertiesFromJSON(stored)
	if err != nil || len(properties) == 0 {
		return stored, nil
	}
	value, err := properties[0].GetValue()
	if err != nil {
		logger.Error(ctx, "Failed to open a sealed value", log.Error(err))
		return "", &tidcommon.InternalServerError
	}
	return value, nil
}
