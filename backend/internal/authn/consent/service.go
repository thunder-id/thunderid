// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package consent implements the consent enforcer authn service for runtime consent collection.
package consent

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authzdetail "github.com/thunder-id/thunderid/internal/authzdetail/common"
	"github.com/thunder-id/thunderid/internal/consent"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// ConsentEnforcerService is the consent enforcer. It exposes SetConsentService so the consent
// service can be injected after construction, since that service depends on components initialized
// after the enforcer.
type ConsentEnforcerService interface {
	providers.ConsentProvider
	// SetConsentService injects the consent service.
	SetConsentService(consentService consent.ConsentServiceInterface)
}

// consentEnforcerService is the default implementation of ConsentEnforcerService.
type consentEnforcerService struct {
	consentService consent.ConsentServiceInterface
	jwtService     jwt.JWTServiceInterface
	detailTypes    providers.AuthorizationDetailTypeProvider
	logger         *log.Logger
}

// newConsentEnforcerService creates a new instance of consentEnforcerService.
func newConsentEnforcerService(jwtSvc jwt.JWTServiceInterface,
	detailTypes providers.AuthorizationDetailTypeProvider) ConsentEnforcerService {
	return &consentEnforcerService{
		jwtService:  jwtSvc,
		detailTypes: detailTypes,
		logger:      log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ConsentEnforcerService")),
	}
}

// SetConsentService injects the consent service. See ConsentEnforcerService.
func (s *consentEnforcerService) SetConsentService(consentService consent.ConsentServiceInterface) {
	s.consentService = consentService
}

// ResolveConsent implements providers.ConsentProvider.ResolveConsent.
func (s *consentEnforcerService) ResolveConsent(ctx context.Context, ouID, appID, appName, userID string,
	essentialAttributes, optionalAttributes, authorizedPermissions []string,
	authorizationDetails []providers.AuthorizationDetail,
	availableAttributes *providers.AttributesResponse, forceReprompt bool,
	runtimeMetadata map[string][]string) (
	*providers.ConsentPromptData, *tidcommon.ServiceError) {
	logger := s.logger.With(log.String("appID", appID), log.MaskedString(log.LoggerKeyUserID, userID))
	logger.Debug(ctx, "Resolving consent for user")

	// List consent purposes for this application.
	purposeFilter := &consent.PurposeFilter{
		GroupID: appID,
	}

	// In current implementation, there will be atmost 1 purpose per app.
	purposes, svcErr := s.consentService.ListPurposes(ctx, *purposeFilter)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			logger.Debug(ctx, "Client error from consent service when listing purposes",
				log.Any("error", svcErr))
			return nil, &ErrorConsentPurposeFetchFailed
		}
		logger.Error(ctx, "Failed to list consent purposes", log.Any("error", svcErr))
		return nil, &tidcommon.InternalServerError
	}

	// Permissions purpose is dynamically built using runtime data
	permissionsPurpose := s.buildPermissionsPurpose(appID, appName, authorizedPermissions)
	if permissionsPurpose != nil {
		purposes = append(purposes, *permissionsPurpose)
	}

	if len(purposes) == 0 && len(authorizationDetails) == 0 {
		logger.Debug(ctx, "No consent purposes configured for application; skipping consent")
		return nil, nil
	}

	// Build the set of elements that already have active consent. When forceReprompt is set, existing
	// consent is ignored so every required claim is prompted again; the lookup is skipped entirely.
	var consentedElements map[string]bool
	var existingConsents []*consent.Consent
	if !forceReprompt {
		// Search for existing consent records for this user and application
		filter := consent.ConsentFilter{
			GroupID:       appID,
			UserID:        userID,
			ConsentStatus: consent.ConsentStatusActive,
		}
		var searchErr *tidcommon.ServiceError
		existingConsents, searchErr = s.consentService.SearchConsents(ctx, filter)
		if searchErr != nil {
			if searchErr.Type == tidcommon.ClientErrorType {
				logger.Debug(ctx, "Client error from consent service when searching consents",
					log.Any("error", searchErr))
				return nil, &ErrorConsentSearchFailed
			}
			logger.Error(ctx, "Failed to search existing consents", log.Any("error", searchErr))
			return nil, &tidcommon.InternalServerError
		}
		consentedElements = buildConsentedElementSet(existingConsents)
	}

	// Build a set of attributes present in the user's profile for profile filtering
	userAttributeSet := buildUserAttributeSet(availableAttributes)

	promptPurposes := buildPurposePrompts(purposes, essentialAttributes, optionalAttributes,
		consentedElements, userAttributeSet, authorizedPermissions)

	// RFC 9396 section 3 requires authorization details to be consented in the same prompt.
	detailPurpose, svcErr := s.authorizationDetailPurpose(ctx, appID, appName, authorizationDetails,
		existingConsents)
	if svcErr != nil {
		return nil, svcErr
	}
	if detailPurpose != nil {
		promptPurposes = append(promptPurposes, *detailPurpose)
	}
	if len(promptPurposes) == 0 {
		logger.Debug(ctx, "All required consents are active; no prompt needed")
		return nil, nil
	}

	promptData := &providers.ConsentPromptData{Purposes: promptPurposes}

	// Generate a signed session token capturing the prompted purposes and their elements.
	// This token should be verified in RecordConsent to ensure the user's decisions match what was prompted
	sessionToken, err := s.createConsentSessionToken(ctx, promptData, authorizationDetails)
	if err != nil {
		logger.Error(ctx, "Failed to create consent session token", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	promptData.SessionToken = sessionToken

	logger.Debug(ctx, "Consent prompt required", log.Int("purposeCount", len(promptPurposes)))
	return promptData, nil
}

// RecordConsent records the user's consent decisions. It first verifies the session token to
// determine what was prompted, fills in any missing purposes as denied, checks for essential
// attribute denials, and then persists the consent record.
func (s *consentEnforcerService) RecordConsent(ctx context.Context, ouID, appID, userID string,
	decisions *providers.ConsentDecisions, sessionToken string,
	validityPeriod int64, runtimeMetadata map[string][]string) (*providers.Consent, *tidcommon.ServiceError) {
	logger := s.logger.With(log.String("appID", appID), log.MaskedString(log.LoggerKeyUserID, userID))
	logger.Debug(ctx, "Recording consent for user")

	// Verify and decode the consent session token to retrieve the prompted purposes
	sessionData, err := s.verifyAndDecodeConsentSession(ctx, sessionToken)
	if err != nil {
		logger.Debug(ctx, "Failed to verify consent session token", log.Error(err))
		return nil, &ErrorConsentSessionInvalid
	}

	// Fill in any missing purposes as denied so incomplete submissions are treated as non-consented
	fillMissingDecisions(sessionData, decisions)

	// Push denials down the decision hierarchy so everything below reads the effective value
	applyDecisionHierarchy(decisions)

	// Build essential element lookup and check whether any essential attribute was denied
	essentialElements := buildEssentialElementSet(sessionData)
	hasEssentialDenial := hasEssentialDenials(decisions, essentialElements)

	filter := consent.ConsentFilter{
		GroupID:       appID,
		UserID:        userID,
		ConsentStatus: consent.ConsentStatusActive,
	}
	existingConsents, svcErr := s.consentService.SearchConsents(ctx, filter)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			logger.Debug(ctx, "Client error from consent service when searching consents for upsert",
				log.Any("error", svcErr))
			return nil, &ErrorConsentSearchFailed
		}
		logger.Error(ctx, "Failed to search existing consents for upsert", log.Any("error", svcErr))
		return nil, &tidcommon.InternalServerError
	}

	// Approved authorization details are taken from the signed session, never from the submission.
	grantedDetails, detailElements, detailTypes, svcErr := s.decideAuthorizationDetails(ctx, appID, sessionData,
		decisions, existingConsents)
	if svcErr != nil {
		return nil, svcErr
	}

	// Convert the user's consent decisions into the format needed for creating a consent record
	newPurposeItems := buildConsentElementApprovals(sessionData, decisions)

	// A prompt that presented only authorization details, none of which is kept, has nothing to record.
	if len(newPurposeItems) == 0 && len(detailElements) == 0 {
		result := &providers.Consent{}
		if len(existingConsents) > 0 {
			result = convertToProvidersConsent(existingConsents[0])
		}
		result.GrantedAuthorizationDetails = grantedDetails
		return result, nil
	}

	validityTime := int64(0)
	if validityPeriod > 0 {
		validityTime = time.Now().Unix() + validityPeriod
	}

	var consentRecord *consent.Consent
	if len(existingConsents) > 0 {
		consentRecord, svcErr = s.updateExistingConsent(ctx, appID, userID,
			existingConsents, newPurposeItems, detailElements, detailTypes, validityTime)
	} else {
		consentRecord, svcErr = s.createNewConsent(ctx, appID, userID,
			newPurposeItems, detailElements, detailTypes, validityTime)
	}
	if svcErr != nil {
		return nil, svcErr
	}

	// If the user denied any essential attribute, return an error after persisting
	if hasEssentialDenial {
		logger.Debug(ctx, "User denied essential attribute(s)", log.String("consentID", consentRecord.ID))
		return nil, &ErrorEssentialConsentDenied
	}

	result := convertToProvidersConsent(consentRecord)
	result.GrantedAuthorizationDetails = grantedDetails
	return result, nil
}

// updateExistingConsent updates an existing consent record by merging new decisions into it.
// The existing record's approved elements are preserved, and new decisions override.
// Returns the updated consent record.
func (s *consentEnforcerService) updateExistingConsent(ctx context.Context, appID, userID string,
	existingConsents []*consent.Consent, newPurposeItems []providers.ConsentPurposeItem,
	detailElements map[string][]consent.ConsentElementApproval,
	detailTypes map[string]providers.AuthorizationDetailType, validityTime int64,
) (*consent.Consent, *tidcommon.ServiceError) {
	logger := s.logger.With(log.String("appID", appID), log.MaskedString(log.LoggerKeyUserID, userID),
		log.Int("existingConsentCount", len(existingConsents)))
	logger.Debug(ctx, "Existing consent record found; updating with new decisions")

	// Build the consent request payload
	req := &consent.ConsentRequest{
		GroupID:      appID,
		ValidityTime: validityTime,
		Authorizations: []consent.ConsentAuthorizationRequest{
			{
				UserID: userID,
				Type:   consent.AuthorizationTypeAuthorization,
				Status: consent.AuthorizationStatusApproved,
			},
		},
	}

	// Merge new decisions into the existing consent record.
	existing := existingConsents[0]
	req.Purposes = mergeAuthorizationDetailPurposes(mergeConsentPurposes(existing.Purposes, newPurposeItems),
		detailElements, detailTypes)

	updated, svcErr := s.consentService.UpdateConsent(ctx, existing.ID, req)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			logger.Debug(ctx, "Client error from consent service when updating consent record",
				log.Any("error", svcErr))
			return nil, &ErrorConsentUpdateFailed
		}
		logger.Error(ctx, "Failed to update consent record", log.Any("error", svcErr))
		return nil, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Consent record updated successfully", log.String("consentID", updated.ID))
	return updated, nil
}

// createNewConsent creates a new consent record with the given purpose items and validity time.
func (s *consentEnforcerService) createNewConsent(ctx context.Context, appID, userID string,
	newPurposeItems []providers.ConsentPurposeItem, detailElements map[string][]consent.ConsentElementApproval,
	detailTypes map[string]providers.AuthorizationDetailType, validityTime int64) (*consent.Consent,
	*tidcommon.ServiceError) {
	logger := s.logger.With(log.String("appID", appID), log.MaskedString(log.LoggerKeyUserID, userID))
	logger.Debug(ctx, "Creating new consent record")

	// Build the consent request payload
	req := &consent.ConsentRequest{
		GroupID:      appID,
		ValidityTime: validityTime,
		Authorizations: []consent.ConsentAuthorizationRequest{
			{
				UserID: userID,
				Type:   consent.AuthorizationTypeAuthorization,
				Status: consent.AuthorizationStatusApproved,
			},
		},
	}
	req.Purposes = mergeAuthorizationDetailPurposes(mergeConsentPurposes(nil, newPurposeItems), detailElements,
		detailTypes)

	created, svcErr := s.consentService.CreateConsent(ctx, req)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			logger.Debug(ctx, "Client error from consent service when creating consent record",
				log.Any("error", svcErr))
			return nil, &ErrorConsentCreateFailed
		}
		logger.Error(ctx, "Failed to create consent record", log.Any("error", svcErr))
		return nil, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Consent recorded successfully", log.String("consentID", created.ID))
	return created, nil
}

// createConsentSessionToken creates a signed JWT containing the consent session data.
// The session captures the purposes and their essential/optional elements from the resolve step,
// so that the record step can verify completeness and enforce essential attribute rules.
func (s *consentEnforcerService) createConsentSessionToken(ctx context.Context,
	promptData *providers.ConsentPromptData, authorizationDetails []providers.AuthorizationDetail,
) (string, error) {
	sessionData := consentSessionData{
		Purposes: make([]consentSessionPurpose, 0, len(promptData.Purposes)),
	}
	for _, p := range promptData.Purposes {
		sessionData.Purposes = append(sessionData.Purposes, consentSessionPurpose{
			PurposeName: p.PurposeName,
			Essential:   elementNames(p.Essential),
			Optional:    elementNames(p.Optional),
			Details:     promptedDetails(p, authorizationDetails),
		})
	}
	sessionData.CoveredDetails = coveredDetails(sessionData.Purposes, authorizationDetails)

	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return "", err
	}

	issuer := config.GetServerRuntime().Config.JWT.Issuer
	claims := map[string]interface{}{
		consentSessionClaimKey: json.RawMessage(sessionJSON),
	}

	claims["aud"] = consentSessionTokenAudience
	token, _, svcErr := s.jwtService.GenerateJWT(
		ctx, "", issuer,
		consentSessionTokenValidityPeriod, claims, "", "")
	if svcErr != nil {
		return "", errors.New("failed to generate consent session token: " + svcErr.Error.DefaultValue)
	}

	return token, nil
}

// verifyAndDecodeConsentSession verifies the JWT consent session token and decodes the session data.
func (s *consentEnforcerService) verifyAndDecodeConsentSession(
	ctx context.Context, sessionToken string) (*consentSessionData, error) {
	issuer := config.GetServerRuntime().Config.JWT.Issuer

	if svcErr := s.jwtService.VerifyJWT(ctx, sessionToken, consentSessionTokenAudience, issuer); svcErr != nil {
		return nil, errors.New("consent session token verification failed: " + svcErr.Error.DefaultValue)
	}

	// Decode the claim as raw JSON so numbers in the authorization details keep their precision.
	sessionRaw, ok, err := jwt.DecodeJWTClaim(sessionToken, consentSessionClaimKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("missing consent session claim in JWT")
	}

	var sessionData consentSessionData
	if err := json.Unmarshal(sessionRaw, &sessionData); err != nil {
		return nil, err
	}

	return &sessionData, nil
}

// fillMissingDecisions treats an incomplete submission as non-consented rather than rejecting the
// request. A purpose entirely absent from the user's decisions is added as denied with every prompted
// element denied. A purpose that is present but denied at the root or at its own level has any
// omitted prompted elements added as denied too, so a minimal deny payload (e.g. root approved=false
// with no elements listed) still persists the individual denials and lets the essential-denial check
// see them.
func fillMissingDecisions(session *consentSessionData, decisions *providers.ConsentDecisions) {
	decisionIdx := make(map[string]int, len(decisions.Purposes))
	for i, pd := range decisions.Purposes {
		decisionIdx[pd.PurposeName] = i
	}

	for _, sp := range session.Purposes {
		idx, present := decisionIdx[sp.PurposeName]
		if !present {
			// Build element decisions marking all elements as denied
			elements := make([]providers.ElementDecision, 0, len(sp.Essential)+len(sp.Optional))
			for _, elem := range sp.Essential {
				elements = append(elements, providers.ElementDecision{Name: elem, Approved: false})
			}
			for _, elem := range sp.Optional {
				elements = append(elements, providers.ElementDecision{Name: elem, Approved: false})
			}
			decisions.Purposes = append(decisions.Purposes, providers.PurposeDecision{
				PurposeName: sp.PurposeName,
				Approved:    false,
				Elements:    elements,
			})
			continue
		}

		// Purpose present and approved at both levels: leave the client's element list untouched.
		// Approval is not propagated down (see applyDecisionHierarchy), so an omitted element under
		// an approved parent stays omitted and is treated as not-consented on subsequent reads.
		if decisions.Approved && decisions.Purposes[idx].Approved {
			continue
		}

		// Purpose present but denied at the root or at its own level: fill in any prompted elements
		// the client omitted so the deny is recorded explicitly against each element.
		existing := make(map[string]bool, len(decisions.Purposes[idx].Elements))
		for _, ed := range decisions.Purposes[idx].Elements {
			existing[ed.Name] = true
		}
		for _, elem := range sp.Essential {
			if !existing[elem] {
				decisions.Purposes[idx].Elements = append(decisions.Purposes[idx].Elements,
					providers.ElementDecision{Name: elem, Approved: false})
			}
		}
		for _, elem := range sp.Optional {
			if !existing[elem] {
				decisions.Purposes[idx].Elements = append(decisions.Purposes[idx].Elements,
					providers.ElementDecision{Name: elem, Approved: false})
			}
		}
	}
}

// applyDecisionHierarchy denies every purpose and element sitting under a denied level, so callers
// can read the leaf Approved values directly instead of walking back up. Approval is not propagated
// downwards: an approved parent leaves its children's own decisions untouched.
func applyDecisionHierarchy(decisions *providers.ConsentDecisions) {
	for i := range decisions.Purposes {
		purpose := &decisions.Purposes[i]
		if !decisions.Approved {
			purpose.Approved = false
		}
		if purpose.Approved {
			continue
		}
		for j := range purpose.Elements {
			purpose.Elements[j].Approved = false
		}
	}
}

// buildEssentialElementSet builds a set of "purposeName:elementName" keys for essential elements
// from the consent session data.
func buildEssentialElementSet(session *consentSessionData) map[string]bool {
	set := make(map[string]bool, len(session.Purposes))
	for _, sp := range session.Purposes {
		for _, elem := range sp.Essential {
			set[purposeElementKey(sp.PurposeName, elem)] = true
		}
	}

	return set
}

// hasEssentialDenials checks whether any essential attribute was denied by the user.
// It does not modify the decisions — the consent record reflects the user's actual choices.
func hasEssentialDenials(decisions *providers.ConsentDecisions, essentialElements map[string]bool) bool {
	for _, p := range decisions.Purposes {
		for _, e := range p.Elements {
			if essentialElements[purposeElementKey(p.PurposeName, e.Name)] && !e.Approved {
				return true
			}
		}
	}

	return false
}

func convertToProvidersConsent(c *consent.Consent) *providers.Consent {
	if c == nil {
		return nil
	}

	purposes := make([]providers.ConsentPurposeItem, 0, len(c.Purposes))
	for _, p := range c.Purposes {
		elements := make([]providers.ConsentElementApproval, 0, len(p.Elements))
		for _, e := range p.Elements {
			elements = append(elements, providers.ConsentElementApproval{
				Name:           e.Name,
				Namespace:      providers.Namespace(e.Namespace),
				IsUserApproved: e.IsUserApproved,
			})
		}
		purposes = append(purposes, providers.ConsentPurposeItem{
			Name:     p.Name,
			Elements: elements,
		})
	}

	auths := make([]providers.ConsentAuthorization, 0, len(c.Authorizations))
	for _, a := range c.Authorizations {
		auths = append(auths, providers.ConsentAuthorization{
			ID:          a.ID,
			UserID:      a.UserID,
			Type:        providers.ConsentAuthorizationType(a.Type),
			Status:      providers.ConsentAuthorizationStatus(a.Status),
			UpdatedTime: a.UpdatedTime,
		})
	}

	return &providers.Consent{
		ID:             c.ID,
		Type:           providers.ConsentTypeAuthentication,
		GroupID:        c.GroupID,
		Status:         providers.ConsentStatus(c.Status),
		ValidityTime:   c.ValidityTime,
		Purposes:       purposes,
		Authorizations: auths,
	}
}

func buildConsentedElementSet(consents []*consent.Consent) map[string]bool {
	consentedSet := make(map[string]bool)
	for _, c := range consents {
		for _, p := range c.Purposes {
			for _, e := range p.Elements {
				// Authorization detail approvals are matched by value, never by name.
				if e.IsUserApproved && e.Namespace != consent.NamespaceAuthorizationDetail {
					consentedSet[purposeElementKey(p.Name, e.Name)] = true
				}
			}
		}
	}

	return consentedSet
}

// buildUserAttributeSet builds a set of attribute names present in the user's profile.
// When availableAttributes is nil, the returned set is empty — meaning no profile filtering is applied.
func buildUserAttributeSet(available *providers.AttributesResponse) map[string]bool {
	if available == nil || len(available.Attributes) == 0 {
		return nil
	}

	set := make(map[string]bool, len(available.Attributes))
	for name := range available.Attributes {
		set[name] = true
	}

	return set
}

// purposeElementKey constructs a unique key for a purpose-element pair.
func purposeElementKey(purposeName, elementName string) string {
	return purposeName + ":" + elementName
}

// buildPurposePrompts dispatches each purpose to the per-namespace builder and returns the
// prompts that still require user consent. Purposes whose Namespace was not inferred are skipped.
func buildPurposePrompts(purposes []consent.ConsentPurpose, essentialAttributes, optionalAttributes []string,
	consentedElements map[string]bool, userAttributeSet map[string]bool,
	authorizedPermissions []string) []providers.ConsentPurposePrompt {
	promptPurposes := make([]providers.ConsentPurposePrompt, 0, len(purposes))
	for _, purpose := range purposes {
		switch deriveConsentPromptTypeFromPurpose(purpose) {
		case consentPromptTypeAttributes:
			if prompt, ok := buildAttributePurposePrompt(purpose, essentialAttributes,
				optionalAttributes, consentedElements, userAttributeSet); ok {
				promptPurposes = append(promptPurposes, prompt)
			}
		case consentPromptTypePermissions:
			if prompt, ok := buildPermissionPurposePrompt(purpose, consentedElements,
				authorizedPermissions); ok {
				promptPurposes = append(promptPurposes, prompt)
			}
		}
	}
	return promptPurposes
}

func deriveConsentPromptTypeFromPurpose(purpose consent.ConsentPurpose) string {
	if strings.HasPrefix(purpose.Name, consent.PermissionPurposeNamePrefix) {
		return consentPromptTypePermissions
	}
	if strings.HasPrefix(purpose.Name, consent.AttributePurposeNamePrefix) {
		return consentPromptTypeAttributes
	}
	return ""
}

// buildAttributePurposePrompt builds a ConsentPurposePrompt for an attribute purpose. It applies
// the requested attribute filter, the user-profile presence filter, and skips elements that
// already have active consent.
func buildAttributePurposePrompt(purpose consent.ConsentPurpose,
	essentialAttributes, optionalAttributes []string,
	consentedElements, userAttributeSet map[string]bool) (providers.ConsentPurposePrompt, bool) {
	essential := make([]providers.PromptElement, 0, len(purpose.Elements))
	optional := make([]providers.PromptElement, 0, len(purpose.Elements))
	for _, elem := range purpose.Elements {
		// Skip elements not in the request's essential/optional sets
		if !slices.Contains(essentialAttributes, elem.Name) &&
			!slices.Contains(optionalAttributes, elem.Name) {
			continue
		}

		// Skip elements not present in the user profile
		if len(userAttributeSet) > 0 && !userAttributeSet[elem.Name] {
			continue
		}

		// Skip elements that already have active consent
		key := purposeElementKey(purpose.Name, elem.Name)
		if consentedElements[key] {
			continue
		}

		// Classify the element as essential or optional for prompting
		if slices.Contains(essentialAttributes, elem.Name) {
			essential = append(essential, providers.PromptElement{Name: elem.Name})
		} else {
			optional = append(optional, providers.PromptElement{Name: elem.Name})
		}
	}

	if len(essential) == 0 && len(optional) == 0 {
		return providers.ConsentPurposePrompt{}, false
	}
	return providers.ConsentPurposePrompt{
		PurposeName: purpose.Name,
		PurposeID:   purpose.ID,
		Description: purpose.Description,
		Type:        consentPromptTypeAttributes,
		Essential:   essential,
		Optional:    optional,
	}, true
}

// buildPermissionPurposePrompt builds a ConsentPurposePrompt for a permission purpose. Only
// elements that appear in the authorized permissions and are not already consented are included.
// Rollup parent linkage is computed server-side from the prompted-element set.
func buildPermissionPurposePrompt(purpose consent.ConsentPurpose,
	consentedElements map[string]bool, authorizedPermissions []string) (providers.ConsentPurposePrompt, bool) {
	prompted := make([]string, 0, len(purpose.Elements))
	for _, elem := range purpose.Elements {
		// Skip elements outside the user's authorized permissions or already consented
		if !slices.Contains(authorizedPermissions, elem.Name) {
			continue
		}
		if consentedElements[purposeElementKey(purpose.Name, elem.Name)] {
			continue
		}
		prompted = append(prompted, elem.Name)
	}
	if len(prompted) == 0 {
		return providers.ConsentPurposePrompt{}, false
	}

	parents := computePermissionParents(prompted)
	optional := make([]providers.PromptElement, 0, len(prompted))
	for _, name := range prompted {
		optional = append(optional, providers.PromptElement{
			Name:   name,
			Parent: parents[name],
		})
	}

	return providers.ConsentPurposePrompt{
		PurposeName: purpose.Name,
		PurposeID:   purpose.ID,
		Description: purpose.Description,
		Type:        consentPromptTypePermissions,
		Optional:    optional,
	}, true
}

// computePermissionParents returns each permission's rollup parent within the supplied set, or ""
// when no parent is present. P's parent is the longest other Q in the set such that P starts with
// Q followed by a permission-delimiter character.
func computePermissionParents(permissions []string) map[string]string {
	parents := make(map[string]string, len(permissions))
	for _, p := range permissions {
		var longestParent string
		for _, q := range permissions {
			if q == p {
				continue
			}
			if len(q) >= len(p) {
				continue
			}
			if !strings.HasPrefix(p, q) {
				continue
			}
			next := rune(p[len(q)])
			if resource.IsPermissionDelimiter(next) && len(q) > len(longestParent) {
				longestParent = q
			}
		}
		parents[p] = longestParent
	}
	return parents
}

// mergeConsentPurposes merges existing consent purposes with new decisions.
func mergeConsentPurposes(
	existing []consent.ConsentPurposeItem, incoming []providers.ConsentPurposeItem,
) []consent.ConsentPurposeItem {
	existingMap := make(map[string]*consent.ConsentPurposeItem, len(existing))
	for i := range existing {
		existingMap[existing[i].Name] = &existing[i]
	}
	coveredPurposes := make(map[string]bool, len(incoming))

	merged := make([]consent.ConsentPurposeItem, 0, len(existing)+len(incoming))
	for _, newPurpose := range incoming {
		coveredPurposes[newPurpose.Name] = true

		existingPurpose, hasExisting := existingMap[newPurpose.Name]

		mergedElemMap := make(map[string]consent.ConsentElementApproval)
		order := make([]string, 0, len(newPurpose.Elements))

		if hasExisting {
			for _, e := range existingPurpose.Elements {
				mergedElemMap[e.Name] = e
				order = append(order, e.Name)
			}
		}
		for _, e := range newPurpose.Elements {
			if _, seen := mergedElemMap[e.Name]; !seen {
				order = append(order, e.Name)
			}
			mergedElemMap[e.Name] = consent.ConsentElementApproval{
				Name:           e.Name,
				Namespace:      consent.Namespace(e.Namespace),
				IsUserApproved: e.IsUserApproved,
			}
		}

		if len(mergedElemMap) == 0 {
			continue
		}

		mergedElements := make([]consent.ConsentElementApproval, 0, len(mergedElemMap))
		for _, name := range order {
			mergedElements = append(mergedElements, mergedElemMap[name])
		}
		merged = append(merged, consent.ConsentPurposeItem{
			Name:     newPurpose.Name,
			Elements: mergedElements,
		})
	}

	// Carry forward existing purposes that aren't in the incoming decisions.
	for _, ep := range existing {
		if coveredPurposes[ep.Name] {
			continue
		}
		filtered := make([]consent.ConsentElementApproval, 0, len(ep.Elements))
		filtered = append(filtered, ep.Elements...)
		if len(filtered) == 0 {
			continue
		}
		merged = append(merged, consent.ConsentPurposeItem{Name: ep.Name, Elements: filtered})
	}

	return merged
}

// buildConsentElementApprovals converts the user's consent decisions into ConsentPurposeItem
// records, filtered to what the signed session prompted. Extra purposes or elements in the
// submission are dropped to prevent privilege escalation via crafted submissions.
func buildConsentElementApprovals(session *consentSessionData,
	decisions *providers.ConsentDecisions) []providers.ConsentPurposeItem {
	promptedElements := buildPromptedElementSet(session)
	promptedPurposes := buildPromptedPurposeSet(session)
	detailPurposes := make(map[string]bool, len(session.Purposes))
	for _, sp := range session.Purposes {
		if len(sp.Details) > 0 {
			detailPurposes[sp.PurposeName] = true
		}
	}

	purposeItems := make([]providers.ConsentPurposeItem, 0, len(decisions.Purposes))
	for _, pd := range decisions.Purposes {
		// Authorization detail decisions are handled by decideAuthorizationDetails.
		if !promptedPurposes[pd.PurposeName] || detailPurposes[pd.PurposeName] {
			continue
		}
		// Namespace is derived from the purpose name (the consent service does not echo it on
		// reads), so attribute decisions get NamespaceAttribute and permission decisions get
		// NamespacePermission.
		ns := namespaceFromPurposeName(pd.PurposeName)
		elementApprovals := make([]providers.ConsentElementApproval, 0, len(pd.Elements))
		for _, ed := range pd.Elements {
			if !promptedElements[purposeElementKey(pd.PurposeName, ed.Name)] {
				continue
			}
			elementApprovals = append(elementApprovals, providers.ConsentElementApproval{
				Name:           ed.Name,
				Namespace:      ns,
				IsUserApproved: ed.Approved,
			})
		}

		purposeItems = append(purposeItems, providers.ConsentPurposeItem{
			Name:     pd.PurposeName,
			Elements: elementApprovals,
		})
	}

	return purposeItems
}

// buildPromptedPurposeSet returns the set of purpose names included in the signed session prompt.
func buildPromptedPurposeSet(session *consentSessionData) map[string]bool {
	set := make(map[string]bool, len(session.Purposes))
	for _, sp := range session.Purposes {
		set[sp.PurposeName] = true
	}
	return set
}

// buildPromptedElementSet returns "purposeName:elementName" keys for every element prompted in
// the signed session.
func buildPromptedElementSet(session *consentSessionData) map[string]bool {
	set := make(map[string]bool, len(session.Purposes))
	for _, sp := range session.Purposes {
		for _, name := range sp.Essential {
			set[purposeElementKey(sp.PurposeName, name)] = true
		}
		for _, name := range sp.Optional {
			set[purposeElementKey(sp.PurposeName, name)] = true
		}
	}
	return set
}

// namespaceFromPurposeName derives the purpose namespace from the name prefix. Returns empty for
// names without a recognized prefix; callers filter such purposes out.
func namespaceFromPurposeName(name string) providers.Namespace {
	switch {
	case strings.HasPrefix(name, consent.PermissionPurposeNamePrefix):
		return providers.NamespacePermission
	case strings.HasPrefix(name, consent.AttributePurposeNamePrefix):
		return providers.NamespaceAttribute
	case strings.HasPrefix(name, consent.AuthorizationDetailPurposeNamePrefix):
		return providers.NamespaceAuthorizationDetail
	default:
		return ""
	}
}

func (s *consentEnforcerService) buildPermissionsPurpose(
	appID, appName string, authorizedPermissions []string,
) *consent.ConsentPurpose {
	if len(authorizedPermissions) == 0 {
		return nil
	}

	elements := permissionsToPurposeElements(authorizedPermissions)
	purposeName := consent.PermissionPurposeName(appID)
	purposeDescription := "Permission consent purpose for application " + appName

	return &consent.ConsentPurpose{
		Name:        purposeName,
		Description: purposeDescription,
		GroupID:     appID,
		Elements:    elements,
	}
}

func permissionsToPurposeElements(permissions []string) []consent.PurposeElement {
	out := make([]consent.PurposeElement, 0, len(permissions))
	for _, p := range permissions {
		out = append(out, consent.PurposeElement{
			Name:        p,
			Namespace:   consent.NamespacePermission,
			IsMandatory: false,
		})
	}
	return out
}

// elementNames extracts the Name field from each PromptElement.
func elementNames(elements []providers.PromptElement) []string {
	if len(elements) == 0 {
		return nil
	}
	names := make([]string, 0, len(elements))
	for _, e := range elements {
		names = append(names, e.Name)
	}
	return names
}

// authorizationDetailPurpose builds the authorization details purpose prompt for the details no earlier
// approval covers, or returns nil when all are covered.
func (s *consentEnforcerService) authorizationDetailPurpose(ctx context.Context, appID, appName string,
	details []providers.AuthorizationDetail,
	records []*consent.Consent) (*providers.ConsentPurposePrompt, *tidcommon.ServiceError) {
	if len(details) == 0 {
		return nil, nil
	}
	purposeName := consent.AuthorizationDetailPurposeName(appID)
	approvals := approvedAuthorizationDetails(records, purposeName)
	elements := make([]providers.PromptElement, 0, len(details))
	for _, detail := range details {
		t, svcErr := authzdetail.StartedType(ctx, s.detailTypes, s.logger, detail)
		if svcErr != nil {
			return nil, svcErr
		}
		obj, err := authzdetail.DetailFromValue(detail.ID, detail.Type, detail.Value)
		if err != nil {
			s.logger.Error(ctx, "Failed to read authorization detail", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if authzdetail.IsCovered(obj, t, approvals) {
			continue
		}
		elements = append(elements, authzdetail.DetailElement(obj, t))
	}
	if len(elements) == 0 {
		return nil, nil
	}
	return &providers.ConsentPurposePrompt{
		PurposeName: purposeName,
		PurposeID:   purposeName,
		Description: "Authorization details consent purpose for application " + appName,
		Type:        consentPromptTypeAuthorizationDetails,
		Essential:   []providers.PromptElement{},
		Optional:    elements,
	}, nil
}

// decideAuthorizationDetails returns the granted authorization details: when the prompt is approved, those
// earlier approvals cover, as requested, and the prompted details the user approved; their reusable consent
// elements keyed by purpose, and the types of those elements keyed by type value.
func (s *consentEnforcerService) decideAuthorizationDetails(ctx context.Context, appID string,
	session *consentSessionData, decisions *providers.ConsentDecisions,
	records []*consent.Consent) ([]providers.AuthorizationDetail,
	map[string][]consent.ConsentElementApproval, map[string]providers.AuthorizationDetailType,
	*tidcommon.ServiceError) {
	granted := []providers.AuthorizationDetail{}
	elements := map[string][]consent.ConsentElementApproval{}
	types := map[string]providers.AuthorizationDetailType{}
	// Every detail's type must still be the one the request started on, whether or not it is granted, and an
	// approval must still cover a covered detail, since one may have been revoked while the prompt was open.
	approvals := approvedAuthorizationDetails(records, consent.AuthorizationDetailPurposeName(appID))
	for _, detail := range session.CoveredDetails {
		t, svcErr := authzdetail.StartedType(ctx, s.detailTypes, s.logger, detail)
		if svcErr != nil {
			return nil, nil, nil, svcErr
		}
		if !decisions.Approved {
			continue
		}
		obj, err := authzdetail.DetailFromValue(detail.ID, detail.Type, detail.Value)
		if err != nil {
			s.logger.Error(ctx, "Failed to read authorization detail", log.Error(err))
			return nil, nil, nil, &tidcommon.InternalServerError
		}
		if !authzdetail.IsCovered(obj, t, approvals) {
			s.logger.Debug(ctx, "Authorization detail is no longer covered", log.String("type", detail.Type))
			continue
		}
		granted = append(granted, detail)
	}
	for _, purpose := range session.Purposes {
		for _, detail := range purpose.Details {
			t, svcErr := authzdetail.StartedType(ctx, s.detailTypes, s.logger, detail)
			if svcErr != nil {
				return nil, nil, nil, svcErr
			}
			decision, approved := approvedDetailDecision(decisions, purpose.PurposeName, detail.ID)
			if !approved {
				continue
			}
			if len(decision.Modified) > 0 {
				value, svcErr := s.modifyDetail(ctx, detail, t, decision.Modified)
				if svcErr != nil {
					return nil, nil, nil, svcErr
				}
				detail.Value = value
			}
			granted = append(granted, detail)
			if !t.Reuse {
				continue
			}
			types[t.Type] = t
			elements[purpose.PurposeName] = append(elements[purpose.PurposeName], consent.ConsentElementApproval{
				Name:           detail.ID,
				Namespace:      consent.NamespaceAuthorizationDetail,
				IsUserApproved: true,
				TypeID:         detail.TypeID,
				TypeVersion:    detail.TypeVersion,
				Granted:        detail.Value,
			})
		}
	}
	return granted, elements, types, nil
}

// modifyDetail applies and validates the user's changes to an authorization detail.
func (s *consentEnforcerService) modifyDetail(ctx context.Context, detail providers.AuthorizationDetail,
	t providers.AuthorizationDetailType,
	modified map[string]json.RawMessage) (json.RawMessage, *tidcommon.ServiceError) {
	base, err := authzdetail.DetailFromValue(detail.ID, detail.Type, detail.Value)
	if err != nil {
		s.logger.Error(ctx, "Failed to read authorization detail", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	candidate, err := authzdetail.ApplyChanges(base, t, modified, authzdetail.PartyUser)
	if err != nil {
		if !errors.Is(err, authzdetail.ErrChangeRefused) {
			s.logger.Error(ctx, "Registered authorization details type schema cannot be compiled", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		s.logger.Debug(ctx, "User change to the authorization detail refused", log.String("type", t.Type),
			log.Error(err))
		return nil, &ErrorAuthorizationDetailModificationRefused
	}
	value, err := candidate.Value()
	if err != nil {
		s.logger.Error(ctx, "Failed to encode authorization detail", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return value, nil
}

// promptedDetails returns the requested details an authorization details purpose prompts for.
func promptedDetails(purpose providers.ConsentPurposePrompt,
	details []providers.AuthorizationDetail) []providers.AuthorizationDetail {
	if purpose.Type != consentPromptTypeAuthorizationDetails {
		return nil
	}
	prompted := make([]providers.AuthorizationDetail, 0, len(purpose.Optional))
	for _, element := range purpose.Optional {
		for i := range details {
			if details[i].ID == element.Name {
				prompted = append(prompted, details[i])
			}
		}
	}
	return prompted
}

// coveredDetails returns the requested authorization details no purpose prompts for, which earlier
// approvals cover.
func coveredDetails(purposes []consentSessionPurpose,
	details []providers.AuthorizationDetail) []providers.AuthorizationDetail {
	prompted := map[string]bool{}
	for _, purpose := range purposes {
		for _, detail := range purpose.Details {
			prompted[detail.ID] = true
		}
	}
	var covered []providers.AuthorizationDetail
	for _, detail := range details {
		if !prompted[detail.ID] {
			covered = append(covered, detail)
		}
	}
	return covered
}

// approvedDetailDecision returns the user's decision on a prompted detail when it is an approval.
func approvedDetailDecision(decisions *providers.ConsentDecisions,
	purposeName, detailID string) (providers.ElementDecision, bool) {
	for _, purpose := range decisions.Purposes {
		if purpose.PurposeName != purposeName {
			continue
		}
		for _, element := range purpose.Elements {
			if element.Name == detailID && element.Approved {
				return element, true
			}
		}
	}
	return providers.ElementDecision{}, false
}

// approvedAuthorizationDetails returns the authorization details approved in the given consent records.
func approvedAuthorizationDetails(records []*consent.Consent, purposeName string) []authzdetail.Approval {
	var approved []authzdetail.Approval
	for _, record := range records {
		for _, purpose := range record.Purposes {
			if purpose.Name != purposeName {
				continue
			}
			for _, element := range purpose.Elements {
				if element.Namespace != consent.NamespaceAuthorizationDetail || !element.IsUserApproved {
					continue
				}
				approved = append(approved, authzdetail.Approval{TypeID: element.TypeID,
					TypeVersion: element.TypeVersion, Granted: element.Granted})
			}
		}
	}
	return approved
}

// elementDetailType returns the type value of the detail a consent element approves.
func elementDetailType(element consent.ConsentElementApproval) string {
	var detail struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(element.Granted, &detail); err != nil {
		return ""
	}
	return detail.Type
}

// supersedes reports whether an added approval of type t makes an existing approval redundant.
func supersedes(added, existing consent.ConsentElementApproval, t providers.AuthorizationDetailType) bool {
	typ := elementDetailType(added)
	if existing.Namespace != consent.NamespaceAuthorizationDetail || elementDetailType(existing) != typ {
		return false
	}
	if existing.TypeID != added.TypeID || existing.TypeVersion != added.TypeVersion {
		return true
	}
	addedDetail, err := authzdetail.DetailFromValue("", typ, added.Granted)
	if err != nil {
		return false
	}
	existingDetail, err := authzdetail.DetailFromValue("", typ, existing.Granted)
	if err != nil {
		return false
	}
	return authzdetail.CheckModification(addedDetail, existingDetail, authzdetail.Policies(t.Schema), "") == nil
}

// mergeAuthorizationDetailPurposes adds the elements to their purposes, dropping existing elements they
// supersede.
func mergeAuthorizationDetailPurposes(existing []consent.ConsentPurposeItem,
	elements map[string][]consent.ConsentElementApproval,
	types map[string]providers.AuthorizationDetailType) []consent.ConsentPurposeItem {
	if len(elements) == 0 {
		return existing
	}
	merged := make([]consent.ConsentPurposeItem, 0, len(existing)+len(elements))
	for _, purpose := range existing {
		if added, ok := elements[purpose.Name]; ok {
			superseded := func(e consent.ConsentElementApproval) bool {
				return slices.ContainsFunc(added, func(a consent.ConsentElementApproval) bool {
					return supersedes(a, e, types[elementDetailType(a)])
				})
			}
			purpose.Elements = slices.DeleteFunc(slices.Clone(purpose.Elements), superseded)
			purpose.Elements = append(purpose.Elements, added...)
		}
		merged = append(merged, purpose)
	}
	for _, name := range slices.Sorted(maps.Keys(elements)) {
		if !slices.ContainsFunc(existing, func(p consent.ConsentPurposeItem) bool { return p.Name == name }) {
			merged = append(merged, consent.ConsentPurposeItem{Name: name, Elements: elements[name]})
		}
	}
	return merged
}
