// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package idp provides the implementation for identity provider management operations.
package idp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/entitytype"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// IDPServiceInterface defines the interface for the IdP service.
type IDPServiceInterface interface {
	CreateIdentityProvider(ctx context.Context, idp *providers.IDPDTO) (*providers.IDPDTO, *tidcommon.ServiceError)
	GetIdentityProviderList(ctx context.Context) ([]BasicIDPDTO, *tidcommon.ServiceError)
	GetIdentityProvider(ctx context.Context, idpID string) (*providers.IDPDTO, *tidcommon.ServiceError)
	GetIdentityProviderByName(ctx context.Context, idpName string) (*providers.IDPDTO, *tidcommon.ServiceError)
	GetIdentityProvidersByProperty(ctx context.Context, propertyKey,
		propertyValue string) ([]providers.IDPDTO, *tidcommon.ServiceError)
	UpdateIdentityProvider(
		ctx context.Context,
		idpID string,
		idp *providers.IDPDTO,
	) (*providers.IDPDTO, *tidcommon.ServiceError)
	DeleteIdentityProvider(ctx context.Context, idpID string) *tidcommon.ServiceError
	GetIDPUsages(ctx context.Context, idpID string) (*resourcedependency.DependenciesResponse, *tidcommon.ServiceError)
	SetDependencyRegistry(r resourcedependency.Registry)
	ApplySchemaAwareDefaults(ctx context.Context, idp *providers.IDPDTO)
}

// idpService is the default implementation of the IdPServiceInterface.
type idpService struct {
	idpStore           idpStoreInterface
	entityTypeService  entitytype.EntityTypeServiceInterface
	transactioner      providers.Transactioner
	dependencyRegistry resourcedependency.Registry
	logger             *log.Logger
	uuidGenerator      func() (string, error)
}

// userTypeAttributes holds a user type's non-credential schema attributes.
type userTypeAttributes struct {
	name       string
	attributes []entitytype.AttributeInfo
}

// isUnique reports whether the user type declares attr as unique.
func (u userTypeAttributes) isUnique(attr string) bool {
	return slices.ContainsFunc(u.attributes, func(a entitytype.AttributeInfo) bool {
		return a.Attribute == attr && a.Unique
	})
}

// isRequired reports whether the user type declares attr as required.
func (u userTypeAttributes) isRequired(attr string) bool {
	return slices.ContainsFunc(u.attributes, func(a entitytype.AttributeInfo) bool {
		return a.Attribute == attr && a.Required
	})
}

// newIDPService creates a new instance of IdPService.
func newIDPService(idpStore idpStoreInterface, entityTypeService entitytype.EntityTypeServiceInterface,
	transactioner providers.Transactioner) IDPServiceInterface {
	return &idpService{
		idpStore:          idpStore,
		entityTypeService: entityTypeService,
		transactioner:     transactioner,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdPService")),
		uuidGenerator:     utils.GenerateUUIDv7,
	}
}

// CreateIdentityProvider creates a new Identity Provider.
func (is *idpService) CreateIdentityProvider(
	ctx context.Context, idp *providers.IDPDTO) (*providers.IDPDTO, *tidcommon.ServiceError) {
	logger := is.logger
	if isDeclarativeModeEnabled() {
		return nil, &declarativeresource.ErrorDeclarativeResourceCreateOperation
	}

	if svcErr := validateIDP(ctx, idp, logger); svcErr != nil {
		return nil, svcErr
	}
	// Seeded on create only: an update replaces the whole connection, so re-seeding there would
	// silently restore a section the administrator removed.
	is.ApplySchemaAwareDefaults(ctx, idp)
	if svcErr := is.validateAttributeConfiguration(ctx, idp); svcErr != nil {
		return nil, svcErr
	}

	if idp.ID == "" {
		id, genErr := is.uuidGenerator()
		if genErr != nil {
			logger.Error(ctx, "failed to generate ID for identity provider", log.Error(genErr))
			return nil, &tidcommon.InternalServerError
		}
		idp.ID = id
	}

	var (
		err    error
		svcErr *tidcommon.ServiceError
	)
	err = is.transactioner.Transact(ctx, func(txCtx context.Context) error {
		// Check if an identity provider with the same name already exists
		existingIDP, err := is.idpStore.GetIdentityProviderByName(txCtx, idp.Name)
		if err != nil && !errors.Is(err, ErrIDPNotFound) {
			return err
		}
		if existingIDP != nil {
			svcErr = &ErrorIDPAlreadyExists
			return errors.New("identity provider already exists")
		}

		// Create the IdP in the database.
		err = is.idpStore.CreateIdentityProvider(txCtx, *idp)
		if err != nil {
			return err
		}
		return nil
	})

	if svcErr != nil {
		return nil, svcErr
	}
	if err != nil {
		logger.Error(ctx, "Failed to create identity provider",
			log.Error(err), log.String("idpName", idp.Name))
		return nil, &tidcommon.InternalServerError
	}

	return idp, nil
}

// GetIdentityProviderList retrieves the list of all Identity Providers.
func (is *idpService) GetIdentityProviderList(ctx context.Context) ([]BasicIDPDTO, *tidcommon.ServiceError) {
	logger := is.logger
	idps, err := is.idpStore.GetIdentityProviderList(ctx)
	if err != nil {
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorResultLimitExceededInCompositeMode
		}
		logger.Error(ctx, "Failed to get identity provider list", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return idps, nil
}

// GetIdentityProvider retrieves an identity provider by its ID.
func (is *idpService) GetIdentityProvider(
	ctx context.Context,
	idpID string,
) (*providers.IDPDTO, *tidcommon.ServiceError) {
	logger := is.logger
	if strings.TrimSpace(idpID) == "" {
		return nil, &ErrorInvalidIDPID
	}

	idp, err := is.idpStore.GetIdentityProvider(ctx, idpID)
	if err != nil {
		if errors.Is(err, ErrIDPNotFound) {
			return nil, &ErrorIDPNotFound
		}
		logger.Error(ctx, "Failed to get identity provider", log.String("idpID", idpID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return idp, nil
}

// GetIdentityProviderByName retrieves an identity provider by its name.
func (is *idpService) GetIdentityProviderByName(ctx context.Context,
	idpName string) (*providers.IDPDTO, *tidcommon.ServiceError) {
	logger := is.logger
	if strings.TrimSpace(idpName) == "" {
		return nil, &ErrorInvalidIDPName
	}

	idp, err := is.idpStore.GetIdentityProviderByName(ctx, idpName)
	if err != nil {
		if errors.Is(err, ErrIDPNotFound) {
			return nil, &ErrorIDPNotFound
		}
		logger.Error(ctx, "Failed to get identity provider by name",
			log.String("idpName", idpName), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return idp, nil
}

// GetIdentityProvidersByProperty retrieves identity providers matching a given property key and value.
func (is *idpService) GetIdentityProvidersByProperty(ctx context.Context,
	propertyKey, propertyValue string) ([]providers.IDPDTO, *tidcommon.ServiceError) {
	logger := is.logger
	if strings.TrimSpace(propertyKey) == "" || strings.TrimSpace(propertyValue) == "" {
		return nil, &ErrorInvalidIDPID
	}

	idps, err := is.idpStore.GetIdentityProvidersByProperty(ctx, propertyKey, propertyValue)
	if err != nil {
		if errors.Is(err, ErrIDPNotFound) {
			return nil, &ErrorIDPNotFound
		}
		logger.Error(ctx, "Failed to get identity providers by property",
			log.String("propertyKey", propertyKey),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return idps, nil
}

// UpdateIdentityProvider updates an existing Identity Provider.
func (is *idpService) UpdateIdentityProvider(
	ctx context.Context,
	idpID string,
	idp *providers.IDPDTO,
) (*providers.IDPDTO,
	*tidcommon.ServiceError) {
	// A resource applied from the control plane is owned there. Changing it here would last only
	// until the next promotion overwrote it, so the change is refused instead.
	if svcErr := managedresource.Guard(ctx, managedresource.TypeConnection, idpID); svcErr != nil {
		return nil, svcErr
	}
	logger := is.logger
	// Block updates only in declarative-only mode; allow in composite and mutable modes
	// In composite mode, the store will check if the resource is immutable and return appropriate error
	if isDeclarativeModeEnabled() {
		return nil, &declarativeresource.ErrorDeclarativeResourceUpdateOperation
	}

	if strings.TrimSpace(idpID) == "" {
		return nil, &ErrorInvalidIDPID
	}
	if svcErr := validateIDP(ctx, idp, logger); svcErr != nil {
		return nil, svcErr
	}
	// Defaults are seeded on create only. An update replaces the whole connection, so an omitted
	// account-linking or mapping section is indistinguishable from one the administrator deliberately
	// removed; re-seeding here would silently undo that removal.
	if svcErr := is.validateAttributeConfiguration(ctx, idp); svcErr != nil {
		return nil, svcErr
	}

	idp.ID = idpID
	var svcErr *tidcommon.ServiceError
	err := is.transactioner.Transact(ctx, func(txCtx context.Context) error {
		// Check if the identity provider exists
		existingIDP, err := is.idpStore.GetIdentityProvider(txCtx, idpID)
		if err != nil {
			if errors.Is(err, ErrIDPNotFound) {
				svcErr = &ErrorIDPNotFound
				return err
			}
			return err
		}

		// If the name is being updated, check whether another IdP with the same name exists
		if existingIDP.Name != idp.Name {
			existingIDPByName, err := is.idpStore.GetIdentityProviderByName(txCtx, idp.Name)
			if err != nil && !errors.Is(err, ErrIDPNotFound) {
				return err
			}
			if existingIDPByName != nil {
				svcErr = &ErrorIDPAlreadyExists
				return errors.New("identity provider already exists")
			}
		}

		err = is.idpStore.UpdateIdentityProvider(txCtx, idp)
		if err != nil {
			// Check if it's the immutable error from composite store
			if errors.Is(err, ErrIDPIsImmutable) {
				svcErr = &ErrorIDPDeclarativeReadOnly
				return err
			}
			return err
		}
		return nil
	})

	if svcErr != nil {
		return nil, svcErr
	}
	if err != nil {
		logger.Error(ctx, "Failed to update identity provider", log.Error(err), log.String("idpID", idpID))
		return nil, &tidcommon.InternalServerError
	}

	return idp, nil
}

// DeleteIdentityProvider deletes an identity provider.
func (is *idpService) DeleteIdentityProvider(ctx context.Context, idpID string) *tidcommon.ServiceError {
	// A resource applied from the control plane is owned there. Changing it here would last only
	// until the next promotion overwrote it, so the change is refused instead.
	if svcErr := managedresource.Guard(ctx, managedresource.TypeConnection, idpID); svcErr != nil {
		return svcErr
	}
	logger := is.logger
	// Block deletes only in declarative-only mode; allow in composite and mutable modes
	// In composite mode, the store will check if the resource is immutable and return appropriate error
	if isDeclarativeModeEnabled() {
		return &declarativeresource.ErrorDeclarativeResourceDeleteOperation
	}

	if strings.TrimSpace(idpID) == "" {
		return &ErrorInvalidIDPID
	}

	// Refuse deletion while other resources block it (e.g. flows that reference the identity provider).
	if svcErr := is.ensureNoBlockingDependencies(ctx, idpID); svcErr != nil {
		return svcErr
	}

	var svcErr *tidcommon.ServiceError
	err := is.transactioner.Transact(ctx, func(txCtx context.Context) error {
		// Check if the identity provider exists
		_, err := is.idpStore.GetIdentityProvider(txCtx, idpID)
		if err != nil {
			if errors.Is(err, ErrIDPNotFound) {
				return nil
			}
			return err
		}

		err = is.idpStore.DeleteIdentityProvider(txCtx, idpID)
		if err != nil {
			// Check if it's the immutable error from composite store
			if errors.Is(err, ErrIDPIsImmutable) {
				svcErr = &ErrorIDPDeclarativeReadOnly
				return err
			}
			return err
		}
		return nil
	})

	if svcErr != nil {
		return svcErr
	}
	if err != nil {
		logger.Error(ctx, "Failed to delete identity provider", log.Error(err), log.String("idpID", idpID))
		return &tidcommon.InternalServerError
	}

	return nil
}

// ApplySchemaAwareDefaults seeds account-linking and username-mapping defaults derived from user-type
// schemas. Explicit configuration wins, and schema lookup failures leave the connection unchanged
// rather than blocking the operation. Entity-type reads are authorized against ctx and never elevated.
func (is *idpService) ApplySchemaAwareDefaults(ctx context.Context, idp *providers.IDPDTO) {
	if idp == nil || is.entityTypeService == nil {
		return
	}

	attributeConfig := idp.AttributeConfiguration
	needsAccountLinking := attributeConfig == nil || attributeConfig.AccountLinking == nil
	needsAttributeMappings := attributeConfig == nil || len(attributeConfig.UserTypeAttributeMappings) == 0
	if !needsAccountLinking && !needsAttributeMappings {
		return
	}

	// Every user type is a candidate: the per-type criteria the seeding helpers apply (a unique email,
	// a required username) decide what can actually be seeded. Self registration is deliberately not a
	// filter, because attribute mappings are read on login as well as during provisioning, so a type
	// that only ever receives manually created users still needs them.
	candidateUserTypes := is.loadCandidateUserTypes(ctx)
	if len(candidateUserTypes) == 0 {
		is.logger.Debug(ctx, "No user type to seed connection defaults from")
		return
	}

	// Linking is a flat attribute list with no user type attached, so it is seeded independently of
	// whether the mapping target can be decided.
	if needsAccountLinking {
		is.seedEmailAccountLinking(ctx, idp, candidateUserTypes)
	}
	if needsAttributeMappings {
		is.seedUserTypeDefaults(ctx, idp, candidateUserTypes)
	}
}

// loadCandidateUserTypes returns every user type visible to the caller with its non-credential
// attributes, or nil when any of it cannot be read. AttributeInfo carries both the required and the
// unique flag, so one read per type answers everything the seeding helpers ask. A partial read yields
// nothing rather than a subset, because seeding on an incomplete view of the deployment could pick a
// linking attribute that another type allows duplicates of.
func (is *idpService) loadCandidateUserTypes(ctx context.Context) []userTypeAttributes {
	response, svcErr := is.entityTypeService.GetEntityTypeList(
		ctx, entitytype.TypeCategoryUser, serverconst.MaxPageSize, 0, false)
	if svcErr != nil || response == nil {
		is.logger.Warn(ctx, "Could not list user types, skipping connection default seeding")
		return nil
	}

	candidates := make([]userTypeAttributes, 0, len(response.Types))
	for _, userType := range response.Types {
		attributes, attrErr := is.entityTypeService.GetAttributes(
			ctx, entitytype.TypeCategoryUser, userType.Name,
			entitytype.AttributeFilter{AllowNonCredential: true})
		if attrErr != nil {
			is.logger.Warn(ctx, "Could not read user type attributes, skipping connection default seeding",
				log.String("userType", userType.Name))
			return nil
		}
		candidates = append(candidates, userTypeAttributes{name: userType.Name, attributes: attributes})
	}
	return candidates
}

// seedEmailAccountLinking configures email as the account-linking attribute when the connection's
// scopes can yield one and email is unique on every candidate. Uniqueness on all of them matters
// because the linking list carries no user type: the lookup must identify a single user whichever
// type an identity provisions into.
func (is *idpService) seedEmailAccountLinking(
	ctx context.Context, idp *providers.IDPDTO, candidateUserTypes []userTypeAttributes,
) {
	scopes := utils.ParseStringArray(GetPropertyValue(idp.Properties, PropScopes), ",")
	if !scopesGrantEmail(idp.Type, scopes) {
		return
	}

	for _, userType := range candidateUserTypes {
		if !userType.isUnique(defaultAccountLinkingAttribute) {
			is.logger.Debug(ctx, "Email is not unique on a candidate user type, skipping linking default",
				log.String("userType", userType.name))
			return
		}
	}

	ensureAttributeConfiguration(idp).AccountLinking = &providers.AccountLinking{
		Attributes: []string{defaultAccountLinkingAttribute},
	}
}

// seedUserTypeDefaults records which local user type an incoming identity resolves to, and maps a
// provider claim onto the local username for every candidate that requires one. Without the mapping,
// provisioning prompts for a username on every first federated sign-in, since no provider emits a
// claim under that name. The default must name a type the mappings cover, because GetAttributeMappings
// looks up the entry keyed to it; with nothing to map it names a type email can match instead.
func (is *idpService) seedUserTypeDefaults(
	ctx context.Context, idp *providers.IDPDTO, candidateUserTypes []userTypeAttributes,
) {
	sourceAttribute := defaultUsernameSourceAttribute(idp.Type)
	if sourceAttribute == "" {
		return
	}

	// Google and OIDC derive the username from the email claim, which a connection only receives when
	// its scopes ask for one. Seeding the mapping regardless would leave an entry that resolves to
	// nothing: provisioning would still prompt for a username while the connection looks configured.
	if sourceAttribute == emailClaim {
		scopes := utils.ParseStringArray(GetPropertyValue(idp.Properties, PropScopes), ",")
		if !scopesGrantEmail(idp.Type, scopes) {
			is.logger.Debug(ctx, "Scopes cannot yield an email, skipping username mapping default")
			return
		}
	}

	usernameRequiredUserTypes := make([]string, 0, len(candidateUserTypes))
	for _, userType := range candidateUserTypes {
		if userType.isRequired(localUsernameAttribute) {
			usernameRequiredUserTypes = append(usernameRequiredUserTypes, userType.name)
		}
	}

	if len(usernameRequiredUserTypes) == 0 {
		emailMatchableUserType := firstUserTypeMatchableByEmail(candidateUserTypes)
		if emailMatchableUserType == "" {
			return
		}
		setDefaultUserType(ensureAttributeConfiguration(idp), emailMatchableUserType)
		return
	}

	mappings := make([]providers.UserTypeAttributeMapping, 0, len(usernameRequiredUserTypes))
	for _, userType := range usernameRequiredUserTypes {
		mappings = append(mappings, providers.UserTypeAttributeMapping{
			UserType: userType,
			Attributes: []providers.AttributeMapping{
				{ExternalAttribute: sourceAttribute, LocalAttribute: localUsernameAttribute},
			},
		})
	}

	attributeConfig := ensureAttributeConfiguration(idp)
	// Candidates arrive ordered by name, so taking the first is stable across restarts. This only
	// selects which mapping entry applies; the type provisioning targets is still decided by the flow.
	setDefaultUserType(attributeConfig, usernameRequiredUserTypes[0])
	attributeConfig.UserTypeAttributeMappings = mappings
}

// firstUserTypeMatchableByEmail returns the first candidate that email can identify a single user on,
// or "" when none can. Candidates arrive ordered by name, so the choice is stable across restarts.
func firstUserTypeMatchableByEmail(candidateUserTypes []userTypeAttributes) string {
	for _, userType := range candidateUserTypes {
		if userType.isUnique(defaultAccountLinkingAttribute) {
			return userType.name
		}
	}
	return ""
}

// setDefaultUserType records the resolution default, leaving a claim-driven resolution the
// administrator already configured intact: replacing the whole value would drop their external
// attribute and value mapping.
func setDefaultUserType(attributeConfig *providers.AttributeConfiguration, userType string) {
	if attributeConfig.UserTypeResolution == nil {
		attributeConfig.UserTypeResolution = &providers.UserTypeResolution{}
	}
	if strings.TrimSpace(attributeConfig.UserTypeResolution.Default) == "" {
		attributeConfig.UserTypeResolution.Default = userType
	}
}

// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (is *idpService) SetDependencyRegistry(r resourcedependency.Registry) {
	is.dependencyRegistry = r
}

// GetIDPUsages returns the resources that reference this identity provider, such as flows that use
// it. It is informational — it drives the pre-delete confirmation dialog and does not gate deletion
// on the server (deletion is gated separately by ensureNoBlockingDependencies).
func (is *idpService) GetIDPUsages(
	ctx context.Context, idpID string,
) (*resourcedependency.DependenciesResponse, *tidcommon.ServiceError) {
	if strings.TrimSpace(idpID) == "" {
		return nil, &ErrorInvalidIDPID
	}

	if _, err := is.idpStore.GetIdentityProvider(ctx, idpID); err != nil {
		if errors.Is(err, ErrIDPNotFound) {
			return nil, &ErrorIDPNotFound
		}
		is.logger.Error(ctx, "Failed to retrieve identity provider", log.Error(err), log.String("idpID", idpID))
		return nil, &tidcommon.InternalServerError
	}

	if is.dependencyRegistry == nil {
		is.logger.Warn(ctx, "Dependency registry not set; returning unknown dependencies",
			log.String("idpID", idpID))
		return &resourcedependency.DependenciesResponse{
			TotalResults: nil,
			Count:        0,
			Summary:      nil,
			Usages:       []resourcedependency.ResourceDependency{},
		}, nil
	}

	result, err := is.dependencyRegistry.GetDependencies(ctx, resourcedependency.ResourceTypeIDP, idpID)
	if err != nil {
		is.logger.Error(ctx, "Failed to get identity provider usages", log.Error(err), log.String("idpID", idpID))
		return nil, &tidcommon.InternalServerError
	}

	return result, nil
}

// ensureNoBlockingDependencies refuses deletion when other resources depend on the identity provider
// in a way that forbids it (behaviorOnDelete == restrict), such as flows that reference it. Because
// deletion is destructive, it fails closed: if dependency data cannot be determined, the deletion is
// refused rather than allowed.
func (is *idpService) ensureNoBlockingDependencies(ctx context.Context, idpID string) *tidcommon.ServiceError {
	if is.dependencyRegistry == nil {
		is.logger.Error(ctx, "Dependency registry not set; refusing to delete identity provider",
			log.String("idpID", idpID))
		return &tidcommon.InternalServerError
	}

	deps, err := is.dependencyRegistry.GetDependencies(ctx, resourcedependency.ResourceTypeIDP, idpID)
	if err != nil {
		is.logger.Error(ctx, "Failed to evaluate identity provider dependencies",
			log.Error(err), log.String("idpID", idpID))
		return &tidcommon.InternalServerError
	}
	// Fail closed: nil TotalResults means a provider failed to report, so usage is unknown.
	if deps == nil || deps.TotalResults == nil {
		is.logger.Error(ctx, "Identity provider dependency data unavailable; refusing to delete",
			log.String("idpID", idpID))
		return &tidcommon.InternalServerError
	}

	blocking := resourcedependency.BlockingUsages(deps)
	if len(blocking) == 0 {
		return nil
	}

	is.logger.Debug(ctx, "Identity provider has blocking dependencies; deletion refused",
		log.String("idpID", idpID), log.Int("blockingCount", len(blocking)))
	return tidcommon.CustomServiceError(ErrorIDPHasBlockingDependencies, tidcommon.I18nMessage{
		Key: "error.idpservice.idp_has_blocking_dependencies_description",
		DefaultValue: fmt.Sprintf(
			"The identity provider cannot be deleted because %s depend on it. Remove or reassign them first.",
			resourcedependency.SummarizeBlockingUsages(blocking)),
	})
}

// validateAttributeConfiguration validates the IDP's attribute configuration: a default user type is
// required only when user-type attribute mappings are configured (it selects which mapping profile
// applies), and for each user type's attributes a valid claim-mapping shape with every local (target)
// claim a non-credential attribute defined in that user type's schema. No-op when no profile is
// configured.
func (is *idpService) validateAttributeConfiguration(
	ctx context.Context,
	idp *providers.IDPDTO,
) *tidcommon.ServiceError {
	profile := idp.AttributeConfiguration
	if profile == nil {
		return nil
	}
	if len(profile.UserTypeAttributeMappings) > 0 &&
		(profile.UserTypeResolution == nil || strings.TrimSpace(profile.UserTypeResolution.Default) == "") {
		return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
			Key:          "error.idpservice.attribute_configuration_user_type_required_description",
			DefaultValue: "attribute configuration requires an user type",
		})
	}

	if svcErr := is.validateUserTypeResolution(ctx, profile.UserTypeResolution); svcErr != nil {
		return svcErr
	}

	seenUserTypes := make(map[string]bool, len(profile.UserTypeAttributeMappings))
	for i := range profile.UserTypeAttributeMappings {
		entry := profile.UserTypeAttributeMappings[i]
		if strings.TrimSpace(entry.UserType) == "" {
			return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
				Key:          "error.idpservice.attribute_configuration_entry_user_type_required_description",
				DefaultValue: "each user type attributes entry requires an user type",
			})
		}
		if seenUserTypes[entry.UserType] {
			return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
				Key:          "error.idpservice.attribute_configuration_duplicate_user_type_description",
				DefaultValue: "user type '{{param(userType)}}' is configured more than once",
				Params:       map[string]string{"userType": entry.UserType},
			})
		}
		seenUserTypes[entry.UserType] = true

		if len(entry.Attributes) > 0 {
			if svcErr := validateAttributeMappingShape(entry.Attributes); svcErr != nil {
				return svcErr
			}
		}

		// Local targets must be non-credential attributes defined in the user type's schema.
		attributes, svcErr := is.entityTypeService.GetAttributes(
			ctx, entitytype.TypeCategoryUser, entry.UserType, entitytype.AttributeFilter{AllowNonCredential: true})
		if svcErr != nil {
			return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
				Key: "error.idpservice.attribute_configuration_user_type_invalid_description",
				DefaultValue: fmt.Sprintf("invalid user type '%s' for attribute configuration: %s",
					entry.UserType, svcErr.ErrorDescription.DefaultValue),
			})
		}
		validTargets := make(map[string]bool, len(attributes))
		for _, attr := range attributes {
			validTargets[attr.Attribute] = true
		}
		for _, m := range entry.Attributes {
			if !validTargets[m.LocalAttribute] {
				return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
					Key: "error.idpservice.attribute_configuration_target_not_in_schema_description",
					DefaultValue: "local claim '{{param(claim)}}' is not an attribute of " +
						"user type '{{param(userType)}}'",
					Params: map[string]string{"claim": m.LocalAttribute, "userType": entry.UserType},
				})
			}
		}
	}
	return nil
}

// validateUserTypeResolution validates claim-driven user-type resolution. A value mapping requires an
// external attribute to evaluate it against, but an external attribute may be configured on its own
// (every identity then resolves to Default until value mappings are added). No mapping entry may be
// empty, and every mapped local user type must be a valid user type. A no-op for static (default-only)
// resolution.
func (is *idpService) validateUserTypeResolution(
	ctx context.Context,
	resolution *providers.UserTypeResolution,
) *tidcommon.ServiceError {
	if resolution == nil {
		return nil
	}
	hasExternal := strings.TrimSpace(resolution.ExternalAttribute) != ""
	hasMapping := len(resolution.ValueMapping) > 0
	if !hasExternal && !hasMapping {
		return nil
	}
	if hasMapping && !hasExternal {
		return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
			Key:          "error.idpservice.attribute_configuration_resolution_mapping_without_external_description",
			DefaultValue: "user type resolution value mapping requires an external attribute",
		})
	}
	// A default user type is the required fallback when the claim is missing or its value is unmapped.
	if strings.TrimSpace(resolution.Default) == "" {
		return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
			Key: "error.idpservice.attribute_configuration_resolution_default_required_description",
			DefaultValue: "claim-driven user type resolution requires a default user type as the " +
				"fallback",
		})
	}

	for value, userType := range resolution.ValueMapping {
		trimmedUserType := strings.TrimSpace(userType)
		if strings.TrimSpace(value) == "" || trimmedUserType == "" {
			return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
				Key:          "error.idpservice.attribute_configuration_resolution_empty_mapping_description",
				DefaultValue: "user type resolution value mapping must not contain empty values",
			})
		}
		if _, svcErr := is.entityTypeService.GetAttributes(
			ctx, entitytype.TypeCategoryUser, trimmedUserType,
			entitytype.AttributeFilter{AllowNonCredential: true}); svcErr != nil {
			return tidcommon.CustomServiceError(ErrorInvalidAttributeConfiguration, tidcommon.I18nMessage{
				Key: "error.idpservice.attribute_configuration_resolution_target_invalid_description",
				DefaultValue: "user type resolution maps to invalid user type " +
					"'{{param(userType)}}'",
				Params: map[string]string{"userType": trimmedUserType},
			})
		}
	}
	return nil
}
