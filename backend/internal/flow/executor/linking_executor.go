// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"unicode"
	"unicode/utf8"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// linkingExecutor decides what a federated authentication resolved to and what has to
// happen next, and records the link on every outcome where it settles an unlinked identity onto an
// account that already exists. ProvisioningExecutor writes only the link of the user it just
// created.
//
// A federated authentication resolves a local user only through a recorded link. This node is what
// matches an identity that has none against an existing account, on the connection's
// account-linking attributes. A match names an account without proving it, so it counts only
// once the End-User verifies that account.
//
// Three outcomes. Somebody is authenticated, so there is nothing to prove and the flow continues.
// Nothing matched, so this identity has no local account and provisioning creates one. One or more
// candidates matched and nobody proved any of them yet, so the node goes incomplete and forwards to
// whatever is wired on onIncomplete.
//
// That is a prompt that says an account matched and offers to link it or to carry on with a
// separate one. The link action starts verification, and whoever it authenticates proves the link,
// if they are one of the candidates; what it offers is the flow author's call. Verification is either
// a CALL node that runs another flow, or steps in this flow that lead back here. Either way this node
// saves a checkpoint of RuntimeData and moves the external identity into it before verification, and
// restores it after, so the steps see an ordinary sign-in with none of the claims the connection
// published, and nothing they write outlives them. An account that verifies without being a
// candidate gets the prompt again, with the AuthUser put back to the pending identity the checkpoint
// saved. Several candidates need no choice from the End-User: the account they verify is the one
// they meant. The reject action points back here, raised as an action type rather than collected as
// a field: the candidates are dropped and the flow carries on as a new user.
type linkingExecutor struct {
	providers.Executor
	authnProvider providers.AuthnProviderManager
	logger        *log.Logger
}

// newLinkingExecutor creates a new instance of linkingExecutor.
func newLinkingExecutor(
	flowFactory core.FlowFactoryInterface,
	authnProvider providers.AuthnProviderManager,
) *linkingExecutor {
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, ExecutorNameLinking),
		log.String(log.LoggerKeyExecutorName, ExecutorNameLinking))
	base := flowFactory.CreateExecutor(
		ExecutorNameLinking,
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
	return &linkingExecutor{
		Executor:      base,
		authnProvider: authnProvider,
		logger:        logger,
	}
}

// Execute branches on what the federated authentication resolved. The first pass resolves the
// identity; the pass that follows a verification step settles what came back.
//
// A settling pass ends the cycle unless the wrong account verified, the one outcome the End-User
// can retry. Ending it clears the flag and the checkpoint, so a later linking node in the same
// execution starts a first pass of its own.
func (e *linkingExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing linking executor")

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

// resolveIdentity is the first pass: whoever is already authenticated wins, otherwise the
// connection's account-linking attributes are matched against existing accounts, and the matches
// are sent to verification.
func (e *linkingExecutor) resolveIdentity(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, logger *log.Logger) (*providers.ExecutorResponse, error) {
	// Every provider state resolved, so the federated identity already carries its link: nothing to
	// decide and nothing to record. A state that does not resolve fails the whole lookup, so a
	// pending identity alongside an earlier authentication lands below rather than here.
	if e.authenticatedEntityID(ctx, execResp) != "" {
		execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateExists
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	candidates, svcErr := e.authnProvider.ResolveFederatedCandidates(ctx.Context, execResp.AuthUser)
	if svcErr != nil {
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

	encoded, err := json.Marshal(candidates.EntityIDs)
	if err != nil {
		logger.Error(ctx.Context, "Failed to encode the linking candidates", log.Error(err))
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, entitytype.TypeCategoryUser)
		return execResp, nil
	}
	execResp.RuntimeData[common.RuntimeKeyLinkingCandidateUserIDs] = string(encoded)
	execResp.RuntimeData[common.RuntimeKeyLinkingVerificationRequested] = dataValueTrue
	checkpoint := linkingCheckpoint{
		AuthUser:      ctx.AuthUser,
		UserInputs:    maps.Clone(ctx.UserInputs),
		PromptDetails: e.publishPromptDetails(ctx, execResp, candidates.MatchedAttributes, logger),
	}
	if err := saveLinkingCheckpoint(ctx.RuntimeData, execResp.RuntimeData, checkpoint); err != nil {
		logger.Error(ctx.Context, "Failed to save the linking checkpoint", log.Error(err))
		execResp.Status = providers.ExecFailure
		execResp.Error = errForEntityCategory(ErrFailedToIdentifyEntity, entitytype.TypeCategoryUser)
		return execResp, nil
	}
	execResp.Status = providers.ExecUserInputRequired

	logger.Debug(ctx.Context, "Candidates matched, requesting verification",
		log.Int("candidateCount", len(candidates.EntityIDs)))
	return execResp, nil
}

// linkingCheckpoint is what the first pass saves before verification: RuntimeData as it will stand
// once that pass's writes are merged, and what a retry after the wrong account needs to start over
// from the same place. That is the AuthUser as the pass found it, which holds the pending federated
// identity, the UserInputs before verification collected any, and the encoded prompt details, since
// AdditionalData does not outlive the response it was published on.
type linkingCheckpoint struct {
	RuntimeData   map[string]string  `json:"runtimeData"`
	AuthUser      providers.AuthUser `json:"authUser"`
	UserInputs    map[string]string  `json:"userInputs,omitempty"`
	PromptDetails string             `json:"promptDetails,omitempty"`
}

// saveLinkingCheckpoint records the checkpoint, with RuntimeData taken from runtimeData and this
// pass's writes, and clears the external identity for the verification steps. With it gone, no claim
// fills a verification input, and a federated verification step finds no earlier subject for the
// consistency check to hold its own against. The candidates stay, so provisioning still refuses to
// run if a path leaves verification without coming back here.
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

// restoreLinkingCheckpoint puts RuntimeData back to what the first pass saved: entries the
// verification steps added are removed and entries they changed are put back, the original external
// identity included. The checkpoint itself stays, so a retry after the wrong account can restore from
// it again. It edits the engine's own map, since an executor response can add entries but not remove
// them, and returns the checkpoint for that retry. A pass that asked for verification always saved a
// checkpoint, so a missing or malformed one is a fault rather than something to carry on from.
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

// replaceEntries makes current hold exactly the entries of saved, other than keep, which is left as it
// is. It edits current in place, since it is a map the engine owns.
func replaceEntries(current, saved map[string]string, keep string) {
	for key := range current {
		if _, ok := saved[key]; !ok && key != keep {
			delete(current, key)
		}
	}
	maps.Copy(current, saved)
}

// settleVerification is the pass that follows the verification this node asked for, after the
// checkpoint is restored. The candidates come from the checkpoint rather than a fresh match: a step
// that authenticated somebody replaced the pending federated identity, so there is nothing left to
// match on.
func (e *linkingExecutor) settleVerification(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	saved linkingCheckpoint, logger *log.Logger) error {
	// The prompt raises a rejection as an action type, which the prompt node hands to whatever the
	// reject action points at: the End-User has said the matched account is not theirs. The type
	// survives exactly one hop, which is why the reject action must point straight back at this
	// node. A verification step reaching here forwards no type at all, and any other type is not a
	// decision: both fall through to what authenticated.
	actionType, _ := ctx.ForwardedData[common.ForwardedDataKeyActionType].(string)
	if common.ActionType(actionType) == common.ActionTypeReject {
		e.refuseCandidate(ctx, execResp, logger)
		return nil
	}

	// Nobody authenticated and nobody refused, so the verification path ended without a sign-in.
	authenticatedID := e.authenticatedEntityID(ctx, execResp)
	if authenticatedID == "" {
		logger.Debug(ctx.Context, "Verification was offered but nobody authenticated")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrVerificationNotCompleted
		return nil
	}
	if !slices.Contains(linkingCandidateIDs(ctx.RuntimeData), authenticatedID) {
		logger.Debug(ctx.Context, "Verification was completed by an account that is not a candidate",
			log.MaskedString(log.LoggerKeyUserID, authenticatedID))
		e.retryVerification(ctx, execResp, saved, logger)
		return nil
	}

	failed, err := e.writeLink(ctx, execResp, authenticatedID, logger)
	if err != nil || failed {
		return err
	}
	execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateExists
	execResp.RuntimeData[common.RuntimeKeyLinkingCandidateUserIDs] = ""
	execResp.Status = providers.ExecComplete
	return nil
}

// retryVerification answers a verification by an account that is not a candidate by offering the
// prompt again, from the state the first attempt started from. The AuthUser goes back to the one the
// checkpoint saved, which the engine adopts because a pending federated identity reads as
// authenticated. Without that, the account that verified would stay signed in: OTP and magic-link
// steps would target it, a federated step would fail the consistency check against it, and a refusal
// would continue as it. The UserInputs go back too, edited in the engine's own map, or the identifier
// the first attempt collected would satisfy the next one and verify the same account again. The
// external identity is cleared again for the verification steps, and the prompt details are
// published again.
//
// A saved AuthUser always holds only the pending identity, since a first pass that found somebody
// authenticated completes without saving a checkpoint. One that does not read as authenticated would
// be ignored by the engine, and one that names an entity would sign the flow in as it, so either
// ends the attempt instead.
func (e *linkingExecutor) retryVerification(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	saved linkingCheckpoint, logger *log.Logger) {
	if !saved.AuthUser.IsAuthenticated() || e.namesAnEntity(ctx, saved.AuthUser) {
		logger.Error(ctx.Context, "The linking checkpoint holds an AuthUser that cannot be restored")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrCandidateNotVerified
		return
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
	execResp.Error = &ErrCandidateNotVerified
}

// namesAnEntity reports whether authUser resolves to an entity. A pending federated identity answers
// with a client error, which is nobody. A server error leaves the answer unknown, so it counts as
// naming one and the caller fails closed.
func (e *linkingExecutor) namesAnEntity(ctx *providers.NodeContext, authUser providers.AuthUser) bool {
	_, entityRef, svcErr := e.authnProvider.GetEntityReference(ctx.Context, authUser)
	if svcErr != nil {
		return svcErr.Type == tidcommon.ServerErrorType
	}
	return entityRef != nil
}

// authenticatedEntityID is the entity the AuthUser resolves to, or "" when nobody authenticated
// yet. A pending federated identity does not resolve by design, which is the same answer.
func (e *linkingExecutor) authenticatedEntityID(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) string {
	if !execResp.AuthUser.IsAuthenticated() {
		return ""
	}
	authUser, entityRef, svcErr := e.authnProvider.GetEntityReference(ctx.Context, execResp.AuthUser)
	execResp.AuthUser = authUser
	if svcErr != nil || entityRef == nil {
		return ""
	}
	return entityRef.EntityID
}

// refuseCandidate drops the matched accounts and continues as a new user, which is the End-User's
// answer to "is this you". Clearing the candidates is what lets the flow carry on, since candidates
// nobody settled are what provisioning refuses to provision over. Provisioning then decides whether
// they may have an account of their own, and the matched attribute being unique is what stops a
// duplicate.
func (e *linkingExecutor) refuseCandidate(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, logger *log.Logger) {
	logger.Debug(ctx.Context, "User refused to link the matched account, continuing as a new user")
	execResp.RuntimeData[common.RuntimeKeyLinkingCandidateUserIDs] = ""
	execResp.Status = providers.ExecComplete
}

// writeLink records the federated identity being linked against the entity the AuthUser now names,
// and reports whether the executor response was set to a failure. A flow carrying no federated
// identity has nothing to record and skips, and an entity that already holds the subject is a
// no-op, so a returning user costs one write that changes nothing.
//
// The identity is read from RuntimeData rather than the AuthUser because verification replaces the
// pending federated token with the local entity's own: by the time a link is worth writing, the
// AuthUser no longer names the connection or the subject. RuntimeData also keeps the identity being
// linked distinct from a verification step that authenticated at some other connection.
//
// A failed write is not survivable by completing anyway. The sign-in would succeed with nothing
// recorded, and the next one would resolve nothing and provision a duplicate. A write the provider
// rejects fails the node; a server error is returned, which ends the flow with an internal error.
func (e *linkingExecutor) writeLink(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	entityID string, logger *log.Logger) (bool, error) {
	federated := federatedIdentityFrom(ctx)
	if federated == nil {
		return false, nil
	}

	svcErr := e.authnProvider.LinkFederatedIdentity(ctx.Context, execResp.AuthUser,
		federated.idpID, federated.sub)
	if svcErr == nil {
		return false, nil
	}

	logger.Error(ctx.Context, "Failed to link the federated identity",
		log.MaskedString(log.LoggerKeyUserID, entityID), log.String("idpId", federated.idpID),
		log.String("errorCode", svcErr.Code),
		log.String("description", svcErr.ErrorDescription.DefaultValue))
	if svcErr.Type == tidcommon.ServerErrorType {
		return false, fmt.Errorf("failed to link the federated identity: %s", svcErr.Code)
	}
	execResp.Status = providers.ExecFailure
	execResp.Error = &ErrLinkingWriteFailed
	return true, nil
}

// publishPromptDetails surfaces the attribute values the candidates matched on for the linking
// prompt to render, as a JSON array of {"label","value"} objects under a single AdditionalData key,
// which is what a KEY_VALUE_LIST component bound to that key renders. Each row is labeled with its
// attribute name, first letter capitalized, and rows are ordered by attribute name.
//
// The values are the ones the federated identity supplied rather than ones read from the matched
// accounts: the match is exact, so they are the same, and the prompt then discloses nothing about
// an account the viewer has not proved beyond the fact that one exists. Nor does it say how many
// matched. It returns what it published, for the checkpoint to publish again on a retry.
func (e *linkingExecutor) publishPromptDetails(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse, matched map[string]string, logger *log.Logger) string {
	details := make([]map[string]string, 0, len(matched))
	for _, attribute := range slices.Sorted(maps.Keys(matched)) {
		details = append(details, map[string]string{
			"label": capitalizeFirst(attribute),
			"value": matched[attribute],
		})
	}
	if len(details) == 0 {
		return ""
	}

	encoded, err := json.Marshal(details)
	if err != nil {
		logger.Debug(ctx.Context, "Could not encode the prompt details")
		return ""
	}
	execResp.AdditionalData[common.DataLinkingPromptDetails] = string(encoded)
	return string(encoded)
}

// linkingCandidateIDs returns the candidates the linking executor sent to verification, or nil when
// there are none.
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

// capitalizeFirst returns s with its first letter upper-cased.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
