// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"gopkg.in/yaml.v3"
)

// ConnectionSharingType is the sharing framework's identifier for connections of every kind.
const ConnectionSharingType sharing.ResourceType = resourceTypeConnection

// connectionSharingDeclaration is how a connection participates in sharing.
//
// Sharing a connection makes the same connection available in the organization units a policy
// reaches, without copying it. The connection keeps living in its own organization unit, and the
// sharees can only use it as it is.
type connectionSharingDeclaration struct {
	service *service
}

// newConnectionSharingDeclaration returns the declaration registered with the sharing framework.
func newConnectionSharingDeclaration(service *service) *connectionSharingDeclaration {
	return &connectionSharingDeclaration{service: service}
}

// ResourceType identifies connections to the sharing framework.
func (d *connectionSharingDeclaration) ResourceType() sharing.ResourceType {
	return ConnectionSharingType
}

// Fields declares nothing: a connection's configuration is global, not per-organization-unit state
// layered on a shared definition. A policy can only govern fields the type declares, so this is what
// makes "a sharee may not override anything" true by construction.
func (d *connectionSharingDeclaration) Fields() []sharing.FieldDeclaration {
	return nil
}

// OwningOUID returns the organization unit that owns a connection.
func (d *connectionSharingDeclaration) OwningOUID(
	ctx context.Context, resourceID string,
) (string, *tidcommon.ServiceError) {
	return d.service.owningOUID(security.WithRuntimeContext(ctx), resourceID)
}

// owningOUID resolves the organization unit owning the connection with the given id. A connection
// is stored by the service of its kind, so each is asked in turn, and an id none of them holds is
// not a connection.
func (s *service) owningOUID(ctx context.Context, id string) (string, *tidcommon.ServiceError) {
	if dto, svcErr := s.idpService.GetIdentityProvider(ctx, id); svcErr == nil {
		return dto.OUID, nil
	} else if svcErr.Code != idp.ErrorIDPNotFound.Code {
		return "", svcErr
	}

	if dto, svcErr := s.notificationService.GetSender(ctx, id); svcErr == nil {
		return dto.OUID, nil
	} else if svcErr.Code != notification.ErrorSenderNotFound.Code {
		return "", svcErr
	}

	if s.authZENPDPService != nil {
		connection, svcErr := s.authZENPDPService.GetAuthZENPDP(ctx, id)
		if svcErr != nil {
			return "", svcErr
		}
		if connection != nil {
			return connection.OUID, nil
		}
	}

	if s.httpService != nil {
		connection, svcErr := s.httpService.GetHTTPConnection(ctx, id)
		if svcErr == nil {
			return connection.OUID, nil
		}
		if svcErr.Code != httpconnection.ErrorNotFound.Code {
			return "", svcErr
		}
	}
	return "", &authzenpdp.ErrorNotFound
}

// ownerLookup resolves the organization unit owning a connection as one vendor's route sees it. A
// vendor's route must answer not found for the id of another vendor's connection, the same as its
// get route does, so each vendor passes a lookup scoped to its own kind.
type ownerLookup func(ctx context.Context, connectionID string) (string, *tidcommon.ServiceError)

// idpOwner returns the lookup for identity-provider-backed connections of one type.
func (s *service) idpOwner(idpType providers.IDPType) ownerLookup {
	return func(ctx context.Context, id string) (string, *tidcommon.ServiceError) {
		dto, svcErr := s.getByType(ctx, idpType, id)
		if svcErr != nil {
			return "", svcErr
		}
		return dto.OUID, nil
	}
}

// senderOwner returns the lookup for notification-sender-backed connections of one provider.
func (s *service) senderOwner(
	senderType ncommon.NotificationSenderType, provider ncommon.NotificationProviderType,
) ownerLookup {
	return func(ctx context.Context, id string) (string, *tidcommon.ServiceError) {
		dto, svcErr := s.getSenderByProvider(ctx, senderType, provider, id)
		if svcErr != nil {
			return "", svcErr
		}
		return dto.OUID, nil
	}
}

// authZENPDPOwner is the lookup for AuthZEN PDP connections.
func (s *service) authZENPDPOwner(ctx context.Context, id string) (string, *tidcommon.ServiceError) {
	connection, svcErr := s.getAuthZENPDP(ctx, id)
	if svcErr != nil {
		return "", svcErr
	}
	return connection.OUID, nil
}

// httpOwner is the lookup for HTTP connections.
func (s *service) httpOwner(ctx context.Context, id string) (string, *tidcommon.ServiceError) {
	connection, svcErr := s.getHTTP(ctx, id)
	if svcErr != nil {
		return "", svcErr
	}
	return connection.OUID, nil
}

// sharingErrors answers the sharing framework's client errors in this service's own codes.
var sharingErrors = sharing.ErrorTranslation{
	PolicyNotFound:  ErrorSharingPolicyNotFound,
	PolicyExists:    ErrorSharingPolicyExists,
	VersionMismatch: ErrorSharingPolicyVersionMismatch,
	PolicyDeclared:  ErrorSharingPolicyDeclared,
	InvalidLimit:    ErrorInvalidLimit,
	InvalidOffset:   ErrorInvalidOffset,
	InvalidPolicy:   ErrorInvalidSharingPolicy,
}

// translateSharingError maps a sharing framework error into this service's own codes.
func translateSharingError(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	return sharingErrors.Translate(svcErr)
}

// requireOwnPolicy fetches a policy and refuses one belonging to another resource. Without it a
// policy id alone would reach any policy in the deployment through any connection's path.
func (s *service) requireOwnPolicy(
	ctx context.Context, owner ownerLookup, connectionID, policyID string,
) (sharing.Policy, *tidcommon.ServiceError) {
	if _, svcErr := owner(ctx, connectionID); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	policy, svcErr := s.sharingService.GetPolicy(ctx, policyID)
	if svcErr != nil {
		return sharing.Policy{}, translateSharingError(svcErr)
	}
	if policy.ResourceType != ConnectionSharingType || policy.ResourceID != connectionID {
		return sharing.Policy{}, &ErrorSharingPolicyNotFound
	}
	return policy, nil
}

// createSharingPolicy records which organization units may use a connection.
func (s *service) createSharingPolicy(
	ctx context.Context, owner ownerLookup, connectionID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	owningOUID, svcErr := owner(ctx, connectionID)
	if svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	if svcErr := declarativeresource.CheckDeclarativeCreate(); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	policy, svcErr := s.sharingService.CreatePolicy(ctx, ConnectionSharingType, connectionID, owningOUID, req)
	return policy, translateSharingError(svcErr)
}

// getSharingPolicy returns one of the connection's sharing policies.
func (s *service) getSharingPolicy(
	ctx context.Context, owner ownerLookup, connectionID, policyID string,
) (sharing.Policy, *tidcommon.ServiceError) {
	return s.requireOwnPolicy(ctx, owner, connectionID, policyID)
}

// listSharingPolicies returns one page of the connection's sharing policies.
func (s *service) listSharingPolicies(
	ctx context.Context, owner ownerLookup, connectionID string, limit, offset int,
) (sharing.PolicyList, *tidcommon.ServiceError) {
	if _, svcErr := owner(ctx, connectionID); svcErr != nil {
		return sharing.PolicyList{}, svcErr
	}
	list, svcErr := s.sharingService.GetPolicyList(ctx, ConnectionSharingType, connectionID, limit, offset)
	return list, translateSharingError(svcErr)
}

// updateSharingPolicy replaces a sharing policy's contents.
func (s *service) updateSharingPolicy(
	ctx context.Context, owner ownerLookup, connectionID, policyID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	if _, svcErr := s.requireOwnPolicy(ctx, owner, connectionID, policyID); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	if svcErr := declarativeresource.CheckDeclarativeUpdate(); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	policy, svcErr := s.sharingService.UpdatePolicy(ctx, policyID, req)
	return policy, translateSharingError(svcErr)
}

// deleteSharingPolicy removes a sharing policy and everything that depended on it.
func (s *service) deleteSharingPolicy(
	ctx context.Context, owner ownerLookup, connectionID, policyID string,
) *tidcommon.ServiceError {
	policy, svcErr := s.requireOwnPolicy(ctx, owner, connectionID, policyID)
	if svcErr != nil {
		if svcErr.Code == ErrorSharingPolicyNotFound.Code {
			return nil
		}
		return svcErr
	}
	if svcErr := declarativeresource.CheckDeclarativeDelete(); svcErr != nil {
		return svcErr
	}
	if policy.Declared {
		return &ErrorSharingPolicyDeclared
	}
	return translateSharingError(s.sharingService.DeletePolicy(ctx, policyID))
}

// resolveSharingOverlay answers what one organization unit may do with the connection. A connection
// declares no overlay fields, so the answer is visibility.
func (s *service) resolveSharingOverlay(
	ctx context.Context, owner ownerLookup, connectionID, ouID string,
) (sharing.ResolvedOverlay, *tidcommon.ServiceError) {
	if _, svcErr := owner(ctx, connectionID); svcErr != nil {
		return sharing.ResolvedOverlay{}, svcErr
	}
	if ouID == "" {
		return sharing.ResolvedOverlay{}, &ErrorMissingOUIDParameter
	}
	overlay, svcErr := s.sharingService.ResolveOverlayRules(ctx, ConnectionSharingType, connectionID, ouID)
	if svcErr != nil {
		// An id naming no organization unit leaves nothing to resolve either, and answering it as
		// a malformed policy would describe a mistake the caller did not make.
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			return sharing.ResolvedOverlay{}, &ErrorConnectionNotSharedToOU
		}
		return sharing.ResolvedOverlay{}, translateSharingError(svcErr)
	}
	if !overlay.Visible {
		return sharing.ResolvedOverlay{}, &ErrorConnectionNotSharedToOU
	}
	return overlay, nil
}

// deleteSharingPolicies removes every policy recorded against a connection that has just been deleted, so a
// connection later created with the same id does not come back already shared.
//
// It runs after the connection's own delete, because only that delete knows whether it will be refused: a
// connection a store holds as immutable, or one that blocking usages protect, must keep its policies. The two
// deletes cannot share a transaction, so a failure here leaves policies behind a connection that is gone. The
// caller is told, and retrying the delete removes them (see purgeOrphanedPolicies).
func (s *service) deleteSharingPolicies(ctx context.Context, connectionID string) *tidcommon.ServiceError {
	if s.sharingService == nil {
		return nil
	}
	return translateSharingError(
		s.sharingService.DeletePoliciesForResource(ctx, ConnectionSharingType, connectionID))
}

// purgeOrphanedPolicies finishes a delete whose policy cleanup failed. It is called when a delete finds the
// connection already gone. When policies are still recorded for it, they are removed and the delete succeeds,
// since the connection being gone is what the caller asked for. Otherwise it returns notFound, which is what the
// delete answered before sharing existed (nil for a kind that treats a missing connection as already deleted).
//
// A connection counts as gone only when none of the services that hold connections has it. A request on one
// vendor's path that names a connection of another kind must not remove that connection's policies.
func (s *service) purgeOrphanedPolicies(
	ctx context.Context, connectionID string, notFound *tidcommon.ServiceError,
) *tidcommon.ServiceError {
	if s.sharingService == nil {
		return notFound
	}
	if _, svcErr := s.owningOUID(ctx, connectionID); svcErr == nil || svcErr.Code != authzenpdp.ErrorNotFound.Code {
		return notFound
	}
	policies, svcErr := s.sharingService.ListPolicies(ctx, ConnectionSharingType, connectionID)
	if svcErr != nil {
		return translateSharingError(svcErr)
	}
	if len(policies) == 0 {
		return notFound
	}
	return translateSharingError(
		s.sharingService.DeletePoliciesForResource(ctx, ConnectionSharingType, connectionID))
}

// ParseConnectionSharingPolicies decodes the sharing policies a connection document carries. Used by the
// runtime import service, which replays them once the connection itself is written.
func ParseConnectionSharingPolicies(node *yaml.Node) ([]sharing.PolicyRequest, error) {
	var model connectionExportModel
	if err := node.Decode(&model); err != nil {
		return nil, fmt.Errorf("failed to parse connection document: %w", err)
	}
	return model.SharingPolicies, nil
}

// makeConnectionSharingConfig creates the loader config the sharing framework reads a connection's declared
// policies with. It names the connections directory, because a policy is carried inside the connection that
// declares it rather than in a document of its own.
func makeConnectionSharingConfig(
	idpService idp.IDPServiceInterface,
	notificationService notification.NotificationSenderMgtSvcInterface,
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface,
	httpService httpconnection.HTTPConnectionServiceInterface,
) sharing.DeclarativeLoaderConfig {
	return sharing.DeclarativeLoaderConfig{
		ResourceType:  ConnectionSharingType,
		DirectoryName: "connections",
		Parser:        makeConnectionSharingParser(idpService, notificationService, authZENPDPService, httpService),
	}
}

// makeConnectionSharingParser returns a parser that reads the sharing half of one connection document.
//
// It validates the connection because a document may name its organization unit by handle, and only the
// service of the connection's kind resolves that to the id a policy is declared against.
func makeConnectionSharingParser(
	idpService idp.IDPServiceInterface,
	notificationService notification.NotificationSenderMgtSvcInterface,
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface,
	httpService httpconnection.HTTPConnectionServiceInterface,
) func([]byte) (*sharing.DeclaredResourcePolicies, error) {
	return func(data []byte) (*sharing.DeclaredResourcePolicies, error) {
		var model connectionExportModel
		if err := yaml.Unmarshal(data, &model); err != nil {
			return nil, fmt.Errorf("failed to parse connection YAML: %w", err)
		}
		// Most documents declare no policy, and validating one that has nothing to declare would be a
		// second pass for an answer already known.
		if len(model.SharingPolicies) == 0 {
			return &sharing.DeclaredResourcePolicies{ResourceID: model.ID}, nil
		}

		dto, err := parseToConnectionDTOWrapper(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse connection '%s': %w", model.Name, err)
		}
		err = validateConnectionDTOWrapper(dto, idpService, notificationService, authZENPDPService, httpService)
		if err != nil {
			return nil, fmt.Errorf("error validating connection '%s': %w", model.Name, err)
		}
		id, name, ouID := declaredConnectionIdentity(dto)
		return &sharing.DeclaredResourcePolicies{
			ResourceID:   id,
			ResourceName: name,
			OwningOUID:   ouID,
			Policies:     model.SharingPolicies,
		}, nil
	}
}

// declaredConnectionIdentity returns the id, name and owning organization unit of a parsed connection.
func declaredConnectionIdentity(dto interface{}) (string, string, string) {
	switch d := dto.(type) {
	case *providers.IDPDTO:
		return d.ID, d.Name, d.OUID
	case *ncommon.NotificationSenderDTO:
		return d.ID, d.Name, d.OUID
	case *authzenpdp.AuthZENPDPConnection:
		return d.ID, d.Name, d.OUID
	case *httpconnection.HTTPConnection:
		return d.ID, d.Name, d.OUID
	}
	return "", "", ""
}

// loadDeclarativeSharing seeds the sharing policies the connection files declare. It runs under the same
// condition as the connections themselves.
func loadDeclarativeSharing(
	sharingService sharing.SharingServiceInterface,
	idpService idp.IDPServiceInterface,
	notificationService notification.NotificationSenderMgtSvcInterface,
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface,
	httpService httpconnection.HTTPConnectionServiceInterface,
) error {
	if !shouldLoadDeclarativeConnections() {
		return nil
	}
	return sharingService.LoadDeclarativeResources(context.Background(),
		makeConnectionSharingConfig(idpService, notificationService, authZENPDPService, httpService))
}
