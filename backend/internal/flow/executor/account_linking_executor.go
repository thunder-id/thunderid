// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// linkingCheckpoint is what the first pass saves before verification, so the settling pass can undo
// what verification wrote and a wrong-account retry can start over from the same place.
// TODO: Replace with an engine-level checkpoint and restore around a diverted path.
type linkingCheckpoint struct {
	RuntimeData   map[string]string  `json:"runtimeData"`
	AuthUser      providers.AuthUser `json:"authUser"`
	UserInputs    map[string]string  `json:"userInputs,omitempty"`
	PromptDetails string             `json:"promptDetails,omitempty"`
}

// accountLinkingExecutor matches a federated identity that has no recorded link against existing
// accounts, sends any match to verification, and records the link once a candidate verifies.
type accountLinkingExecutor struct {
	providers.Executor
	authnProvider     providers.AuthnProviderManager
	entityTypeService entitytype.EntityTypeServiceInterface
	logger            *log.Logger
}

var _ providers.Executor = (*accountLinkingExecutor)(nil)

// newAccountLinkingExecutor creates a new instance of accountLinkingExecutor.
func newAccountLinkingExecutor(
	flowFactory core.FlowFactoryInterface,
	authnProvider providers.AuthnProviderManager,
	entityTypeService entitytype.EntityTypeServiceInterface,
) *accountLinkingExecutor {
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, ExecutorNameAccountLinking),
		log.String(log.LoggerKeyExecutorName, ExecutorNameAccountLinking))
	base := flowFactory.CreateExecutor(
		ExecutorNameAccountLinking,
		providers.ExecutorTypeUtility,
		[]providers.Input{},
		[]providers.Input{},
		&providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{
				providers.FlowTypeAuthentication,
				providers.FlowTypeRegistration,
			},
		},
	)
	return &accountLinkingExecutor{
		Executor:          base,
		authnProvider:     authnProvider,
		entityTypeService: entityTypeService,
		logger:            logger,
	}
}

// Execute runs the first pass, or settles the verification the first pass asked for. Every settling
// outcome except a wrong-account retry ends the cycle.
func (e *accountLinkingExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing account linking executor")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}

	if ctx.RuntimeData[common.RuntimeKeyLinkingVerificationRequested] != dataValueTrue {
		return e.resolveIdentity(ctx, execResp, logger)
	}

	saved, err := restoreLinkingCheckpoint(ctx.RuntimeData)
	if err != nil {
		return nil, err
	}
	if err := e.settleVerification(ctx, execResp, saved, logger); err != nil {
		return nil, err
	}
	if execResp.Status != providers.ExecUserInputRequired {
		execResp.RuntimeData[common.RuntimeKeyLinkingVerificationRequested] = ""
		execResp.RuntimeData[common.RuntimeKeyLinkingCheckpoint] = ""
	}
	return execResp, nil
}

// resolveIdentity is the first pass: an identity that resolves is already linked, otherwise the
// account-linking attributes are matched and any candidates are sent to verification.
func (e *accountLinkingExecutor) resolveIdentity(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, logger *log.Logger) (*providers.ExecutorResponse, error) {
	authenticatedID, err := e.authenticatedEntityID(ctx, execResp)
	if err != nil {
		return nil, err
	}
	if authenticatedID != "" {
		execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateExists
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	candidates, svcErr := e.authnProvider.ResolveLinkCandidates(ctx.Context, execResp.AuthUser)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to match the account linking attributes: %s", svcErr.Code)
		}
		logger.Debug(ctx.Context, "Failed to match the account linking attributes",
			log.String("errorCode", svcErr.Code))
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, entitytype.TypeCategoryUser)
		return execResp, nil
	}
	if candidates == nil || len(candidates.EntityIDs) == 0 {
		logger.Debug(ctx.Context, "No local account for this federated identity, continuing as a new user")
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	encoded, _ := json.Marshal(candidates.EntityIDs)
	execResp.RuntimeData[common.RuntimeKeyLinkingCandidateUserIDs] = string(encoded)
	execResp.RuntimeData[common.RuntimeKeyLinkingVerificationRequested] = dataValueTrue
	checkpoint := linkingCheckpoint{
		AuthUser:      ctx.AuthUser,
		UserInputs:    maps.Clone(ctx.UserInputs),
		PromptDetails: e.publishPromptDetails(ctx, execResp, candidates, logger),
	}
	if err := saveLinkingCheckpoint(ctx.RuntimeData, execResp.RuntimeData, checkpoint); err != nil {
		return nil, fmt.Errorf("failed to save the linking checkpoint: %w", err)
	}
	execResp.Status = providers.ExecUserInputRequired

	logger.Debug(ctx.Context, "Candidates matched, requesting verification",
		log.Int("candidateCount", len(candidates.EntityIDs)))
	return execResp, nil
}

// settleVerification decides the outcome of the verification the first pass asked for, against the
// candidates the checkpoint restored.
func (e *accountLinkingExecutor) settleVerification(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, saved linkingCheckpoint, logger *log.Logger) error {
	// Only REJECT is a decision. Any other action type falls through to who authenticated.
	actionType, _ := ctx.ForwardedData[common.ForwardedDataKeyActionType].(string)
	if common.ActionType(actionType) == common.ActionTypeReject {
		e.refuseCandidate(ctx, execResp, logger)
		return nil
	}

	authenticatedID, err := e.authenticatedEntityID(ctx, execResp)
	if err != nil {
		return err
	}
	if authenticatedID == "" {
		logger.Debug(ctx.Context, "Verification was offered but nobody authenticated")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrVerificationNotCompleted
		return nil
	}
	if !slices.Contains(linkingCandidateIDs(ctx.RuntimeData), authenticatedID) {
		logger.Debug(ctx.Context, "Verification was completed by an account that is not a candidate",
			log.MaskedString(log.LoggerKeyEntityID, authenticatedID))
		return e.retryVerification(ctx, execResp, saved, logger)
	}

	if err := e.writeLink(ctx, execResp, authenticatedID, logger); err != nil ||
		execResp.Status == providers.ExecFailure {
		return err
	}
	execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateExists
	execResp.RuntimeData[common.RuntimeKeyLinkingCandidateUserIDs] = ""
	// The user proved they own the account, so registration completes as that account instead of
	// provisioning failing on it as an existing user.
	if ctx.FlowType == providers.FlowTypeRegistration {
		execResp.RuntimeData[common.RuntimeKeyAllowRegistrationWithExistingUser] = dataValueTrue
	}
	execResp.Status = providers.ExecComplete
	return nil
}

// retryVerification rolls AuthUser and UserInputs back to the checkpoint and re-prompts after a
// non-candidate verified. It fails when the saved AuthUser is not a pending identity.
func (e *accountLinkingExecutor) retryVerification(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, saved linkingCheckpoint, logger *log.Logger) error {
	restorable := saved.AuthUser.IsAuthenticated()
	if restorable {
		namesEntity, err := e.namesAnEntity(ctx, saved.AuthUser)
		if err != nil {
			return err
		}
		restorable = !namesEntity
	}
	if !restorable {
		logger.Debug(ctx.Context, "The linking checkpoint holds an AuthUser that cannot be restored")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrNonCandidateVerified
		return nil
	}
	execResp.AuthUser = saved.AuthUser
	if ctx.UserInputs != nil {
		replaceEntries(ctx.UserInputs, saved.UserInputs, "")
	}
	if saved.PromptDetails != "" {
		execResp.AdditionalData[common.DataLinkingPromptDetails] = saved.PromptDetails
	}
	execResp.RuntimeData[common.RuntimeKeyExternalIdentity] = ""
	execResp.Status = providers.ExecUserInputRequired
	execResp.Error = &ErrNonCandidateVerified
	return nil
}

// refuseCandidate drops the candidates and continues as a new user.
func (e *accountLinkingExecutor) refuseCandidate(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, logger *log.Logger) {
	logger.Debug(ctx.Context, "User refused to link the matched account, continuing as a new user")
	execResp.RuntimeData[common.RuntimeKeyLinkingCandidateUserIDs] = ""
	execResp.Status = providers.ExecComplete
}

// writeLink records the federated identity against the entity the AuthUser names. A rejected write
// sets a failure on the response. The identity comes from RuntimeData, since verification replaced
// the federated token on the AuthUser.
func (e *accountLinkingExecutor) writeLink(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	entityID string, logger *log.Logger) error {
	federated := federatedIdentityFrom(ctx)
	if federated == nil {
		return errors.New("no federated identity to link")
	}

	svcErr := e.authnProvider.LinkAccount(ctx.Context, execResp.AuthUser,
		federated.idpID, federated.sub)
	if svcErr == nil {
		return nil
	}

	if svcErr.Type == tidcommon.ServerErrorType {
		return fmt.Errorf("failed to link the federated identity: %s", svcErr.Code)
	}
	logger.Debug(ctx.Context, "The link write was rejected",
		log.MaskedString(log.LoggerKeyEntityID, entityID), log.String("idpId", federated.idpID),
		log.String("errorCode", svcErr.Code))
	execResp.Status = providers.ExecFailure
	execResp.Error = &ErrLinkingWriteFailed
	return nil
}

// publishPromptDetails publishes the matched attribute values for the prompt as a JSON array of
// {"label","value"} objects ordered by attribute name, and returns what it published.
func (e *accountLinkingExecutor) publishPromptDetails(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, candidates *providers.LinkCandidates, logger *log.Logger) string {
	labels := e.attributeDisplayNames(ctx, candidates.EntityTypes, logger)
	details := make([]map[string]string, 0, len(candidates.MatchedAttributes))
	for _, attribute := range slices.Sorted(maps.Keys(candidates.MatchedAttributes)) {
		label := labels[attribute]
		if label == "" {
			label = attribute
		}
		details = append(details, map[string]string{
			"label": label,
			"value": candidates.MatchedAttributes[attribute],
		})
	}
	if len(details) == 0 {
		return ""
	}

	encoded, _ := json.Marshal(details)
	execResp.AdditionalData[common.DataLinkingPromptDetails] = string(encoded)
	return string(encoded)
}

// attributeDisplayNames returns the schema display names of the candidates' user types, keyed by
// attribute name. The first type that names an attribute wins. A type that cannot be read leaves
// its attributes to fall back to their names.
func (e *accountLinkingExecutor) attributeDisplayNames(ctx *providers.NodeContext, entityTypes []string,
	logger *log.Logger) map[string]string {
	labels := map[string]string{}
	for _, entityType := range entityTypes {
		attrs, svcErr := e.entityTypeService.GetAttributes(ctx.Context, entitytype.TypeCategoryUser, entityType,
			entitytype.AttributeFilter{AllowNonCredential: true})
		if svcErr != nil {
			logger.Debug(ctx.Context, "Could not read the attribute display names",
				log.String("entityType", entityType), log.String("errorCode", svcErr.Code))
			continue
		}
		for _, attr := range attrs {
			if _, ok := labels[attr.Attribute]; !ok && attr.DisplayName != "" {
				labels[attr.Attribute] = attr.DisplayName
			}
		}
	}
	return labels
}

// namesAnEntity reports whether authUser resolves to an entity. A pending federated identity answers
// with a client error, which is nobody. A server error is returned.
func (e *accountLinkingExecutor) namesAnEntity(ctx *providers.NodeContext,
	authUser providers.AuthUser) (bool, error) {
	_, entityRef, svcErr := e.authnProvider.GetEntityReference(ctx.Context, authUser)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return false, fmt.Errorf("failed to resolve the saved AuthUser: %s", svcErr.Code)
		}
		return false, nil
	}
	return entityRef != nil, nil
}

// authenticatedEntityID is the entity the AuthUser resolves to, or "" when nobody authenticated yet.
// A pending federated identity resolves to nobody. A server error is returned.
func (e *accountLinkingExecutor) authenticatedEntityID(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) (string, error) {
	if !execResp.AuthUser.IsAuthenticated() {
		return "", nil
	}
	authUser, entityRef, svcErr := e.authnProvider.GetEntityReference(ctx.Context, execResp.AuthUser)
	execResp.AuthUser = authUser
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return "", fmt.Errorf("failed to resolve the authenticated entity: %s", svcErr.Code)
		}
		return "", nil
	}
	if entityRef == nil {
		return "", nil
	}
	return entityRef.EntityID, nil
}

// saveLinkingCheckpoint records the checkpoint and clears the external identity, so no claim reaches
// the verification steps. The candidates stay, so provisioning refuses to run if verification exits
// without returning here.
func saveLinkingCheckpoint(runtimeData, writes map[string]string, checkpoint linkingCheckpoint) error {
	saved := maps.Clone(runtimeData)
	if saved == nil {
		saved = make(map[string]string)
	}
	maps.Copy(saved, writes)
	delete(saved, common.RuntimeKeyLinkingCheckpoint)
	checkpoint.RuntimeData = saved

	encoded, err := json.Marshal(&checkpoint)
	if err != nil {
		return err
	}
	writes[common.RuntimeKeyLinkingCheckpoint] = string(encoded)
	writes[common.RuntimeKeyExternalIdentity] = ""
	return nil
}

// restoreLinkingCheckpoint puts the engine's RuntimeData back to what the first pass saved and keeps
// the checkpoint for a retry. A missing or malformed checkpoint is an internal error.
func restoreLinkingCheckpoint(runtimeData map[string]string) (linkingCheckpoint, error) {
	raw := runtimeData[common.RuntimeKeyLinkingCheckpoint]
	if raw == "" {
		return linkingCheckpoint{}, errors.New("the linking checkpoint is missing")
	}
	var saved linkingCheckpoint
	if err := json.Unmarshal([]byte(raw), &saved); err != nil {
		return linkingCheckpoint{}, fmt.Errorf("the linking checkpoint does not decode: %w", err)
	}
	// Restoring from a checkpoint without RuntimeData would empty the engine's map.
	if saved.RuntimeData == nil {
		return linkingCheckpoint{}, errors.New("the linking checkpoint holds no RuntimeData")
	}
	replaceEntries(runtimeData, saved.RuntimeData, common.RuntimeKeyLinkingCheckpoint)
	return saved, nil
}

// replaceEntries makes current hold exactly the entries of saved, leaving keep as it is.
func replaceEntries(current, saved map[string]string, keep string) {
	for key := range current {
		if _, ok := saved[key]; !ok && key != keep {
			delete(current, key)
		}
	}
	maps.Copy(current, saved)
}

// linkingCandidateIDs returns the candidates sent to verification, or nil when there are none.
func linkingCandidateIDs(runtimeData map[string]string) []string {
	raw := runtimeData[common.RuntimeKeyLinkingCandidateUserIDs]
	if raw == "" {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	return ids
}
