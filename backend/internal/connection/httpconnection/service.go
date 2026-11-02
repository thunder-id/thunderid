// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/http/httpguts"

	"github.com/thunder-id/thunderid/internal/ou"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/httpauth"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// HTTPConnectionProvider resolves an HTTP connection for an outbound call, without the management
// operations.
type HTTPConnectionProvider interface {
	// GetResolvedConnection returns the connection's URL, timeout and headers with its
	// authentication applied, or ErrorNotFound when the connection does not exist.
	GetResolvedConnection(ctx context.Context, id string) (*ResolvedHTTPConnection, *tidcommon.ServiceError)
}

// HTTPConnectionServiceInterface defines HTTP connection management operations. A connection's
// authentication is taken as given: keeping a stored secret the caller omitted is the caller's
// decision, made before an update.
type HTTPConnectionServiceInterface interface {
	HTTPConnectionProvider
	CreateHTTPConnection(ctx context.Context, connection HTTPConnection) (*HTTPConnection, *tidcommon.ServiceError)
	ListHTTPConnections(ctx context.Context) ([]HTTPConnection, *tidcommon.ServiceError)
	GetHTTPConnection(ctx context.Context, id string) (*HTTPConnection, *tidcommon.ServiceError)
	UpdateHTTPConnection(ctx context.Context, id string, connection HTTPConnection) (
		*HTTPConnection, *tidcommon.ServiceError)
	DeleteHTTPConnection(ctx context.Context, id string) *tidcommon.ServiceError
	GetHTTPConnectionUsages(ctx context.Context, id string) (
		*resourcedependency.DependenciesResponse, *tidcommon.ServiceError)
	SetDependencyRegistry(registry resourcedependency.Registry)
	ValidateHTTPConnection(ctx context.Context, connection *HTTPConnection) *tidcommon.ServiceError
	ResolveConnectionOU(ctx context.Context, connection *HTTPConnection) *tidcommon.ServiceError
	ValidateHTTPConnectionCreate(ctx context.Context, connection *HTTPConnection) *tidcommon.ServiceError
	ValidateHTTPConnectionUpdate(ctx context.Context, existing *HTTPConnection,
		connection *HTTPConnection) *tidcommon.ServiceError
}

// httpConnectionService validates and manages HTTP connections.
type httpConnectionService struct {
	store              httpConnectionStoreInterface
	ouService          ou.OrganizationUnitServiceInterface
	dependencyRegistry resourcedependency.Registry
	logger             *log.Logger
}

// ownerErrors are the errors given when the organization unit of an HTTP connection is refused.
var ownerErrors = ou.OwnerErrors{
	Required: ErrorOURequired,
	NotFound: ErrorInvalidOU,
	Changed:  ErrorOUUpdateNotAllowed,
}

var _ HTTPConnectionServiceInterface = (*httpConnectionService)(nil)

// newHTTPConnectionService creates an HTTP connection service.
func newHTTPConnectionService(store httpConnectionStoreInterface,
	ouService ou.OrganizationUnitServiceInterface) *httpConnectionService {
	return &httpConnectionService{
		store:     store,
		ouService: ouService,
		logger:    log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// GetResolvedConnection returns a connection ready to call, its headers carrying the decrypted
// credential. The authentication refuses a credential that would cross the network in clear.
func (s *httpConnectionService) GetResolvedConnection(ctx context.Context, id string) (
	*ResolvedHTTPConnection, *tidcommon.ServiceError) {
	connection, svcErr := s.GetHTTPConnection(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	endpoint, err := url.Parse(connection.URL)
	if err != nil {
		s.logger.Error(ctx, "Failed to parse HTTP connection URL", log.Error(err), log.String("connectionID", id))
		return nil, &tidcommon.InternalServerError
	}
	authConfig, err := outboundauth.FromProperties(connection.AuthProperties)
	if err != nil {
		s.logger.Error(ctx, "Failed to read HTTP connection authentication",
			log.Error(err), log.String("connectionID", id))
		return nil, &tidcommon.InternalServerError
	}
	authenticator, err := httpauth.New(authConfig)
	if err != nil {
		s.logger.Error(ctx, "HTTP connection authentication cannot be presented",
			log.Error(err), log.String("connectionID", id))
		return nil, &tidcommon.InternalServerError
	}

	headers := make(http.Header, len(connection.Headers)+1)
	for _, header := range connection.Headers {
		headers.Set(header.Name, header.Value)
	}
	if err := authenticator.Authenticate(ctx, &http.Request{URL: endpoint, Header: headers}); err != nil {
		s.logger.Error(ctx, "Failed to apply HTTP connection authentication",
			log.Error(err), log.String("connectionID", id))
		return nil, &tidcommon.InternalServerError
	}
	return &ResolvedHTTPConnection{
		URL:     connection.URL,
		Timeout: time.Duration(connection.TimeoutMS) * time.Millisecond,
		Headers: headers,
	}, nil
}

// CreateHTTPConnection validates and stores a connection.
func (s *httpConnectionService) CreateHTTPConnection(ctx context.Context, connection HTTPConnection) (
	*HTTPConnection, *tidcommon.ServiceError) {
	if svcErr := s.checkCreate(ctx, &connection); svcErr != nil {
		return nil, svcErr
	}
	if connection.ID == "" {
		id, err := sysutils.GenerateUUIDv7()
		if err != nil {
			s.logger.Error(ctx, "Failed to generate HTTP connection ID", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		connection.ID = id
	}

	if err := s.store.CreateHTTPConnection(ctx, connection); err != nil {
		switch {
		case errors.Is(err, errNameConflict), errors.Is(err, errIDConflict):
			return nil, &ErrorAlreadyExists
		case errors.Is(err, errHTTPConnectionIsImmutable):
			return nil, &declarativeresource.ErrorDeclarativeResourceCreateOperation
		}
		s.logger.Error(ctx, "Failed to create HTTP connection",
			log.Error(err), log.String("connectionName", connection.Name))
		return nil, &tidcommon.InternalServerError
	}
	return &connection, nil
}

// ListHTTPConnections returns all HTTP connections.
func (s *httpConnectionService) ListHTTPConnections(ctx context.Context) ([]HTTPConnection,
	*tidcommon.ServiceError) {
	connections, err := s.store.ListHTTPConnections(ctx)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrorResultLimitExceededInCompositeMode
		}
		s.logger.Error(ctx, "Failed to list HTTP connections", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return connections, nil
}

// GetHTTPConnection returns a connection by ID, or ErrorNotFound when it does not exist.
func (s *httpConnectionService) GetHTTPConnection(ctx context.Context, id string) (*HTTPConnection,
	*tidcommon.ServiceError) {
	if strings.TrimSpace(id) == "" {
		return nil, &ErrorNotFound
	}
	connection, err := s.store.GetHTTPConnection(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "Failed to get HTTP connection", log.Error(err), log.String("connectionID", id))
		return nil, &tidcommon.InternalServerError
	}
	if connection == nil {
		return nil, &ErrorNotFound
	}
	return connection, nil
}

// UpdateHTTPConnection validates a connection and replaces the stored one.
func (s *httpConnectionService) UpdateHTTPConnection(ctx context.Context, id string, connection HTTPConnection) (
	*HTTPConnection, *tidcommon.ServiceError) {
	if svcErr := s.checkUpdate(ctx, &connection); svcErr != nil {
		return nil, svcErr
	}
	current, svcErr := s.GetHTTPConnection(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.checkOwnerUnchanged(ctx, &connection, current.OUID); svcErr != nil {
		return nil, svcErr
	}
	connection.ID = id

	if err := s.store.UpdateHTTPConnection(ctx, connection); err != nil {
		switch {
		case errors.Is(err, errNameConflict):
			return nil, &ErrorAlreadyExists
		case errors.Is(err, errConnectionNotFound):
			return nil, &ErrorNotFound
		case errors.Is(err, errHTTPConnectionIsImmutable):
			return nil, &declarativeresource.ErrorDeclarativeResourceUpdateOperation
		}
		s.logger.Error(ctx, "Failed to update HTTP connection", log.Error(err), log.String("connectionID", id))
		return nil, &tidcommon.InternalServerError
	}
	return &connection, nil
}

// DeleteHTTPConnection deletes a connection that no resource uses. Deleting a missing connection
// succeeds.
func (s *httpConnectionService) DeleteHTTPConnection(ctx context.Context, id string) *tidcommon.ServiceError {
	if svcErr := declarativeresource.CheckDeclarativeDelete(); svcErr != nil {
		return svcErr
	}
	if svcErr := s.ensureNoBlockingDependencies(ctx, id); svcErr != nil {
		return svcErr
	}
	if err := s.store.DeleteHTTPConnection(ctx, id); err != nil {
		switch {
		case errors.Is(err, errConnectionNotFound):
			return nil
		case errors.Is(err, errHTTPConnectionIsImmutable):
			return &declarativeresource.ErrorDeclarativeResourceDeleteOperation
		}
		s.logger.Error(ctx, "Failed to delete HTTP connection", log.Error(err), log.String("connectionID", id))
		return &tidcommon.InternalServerError
	}
	return nil
}

// GetHTTPConnectionUsages returns the resources that use a connection.
func (s *httpConnectionService) GetHTTPConnectionUsages(ctx context.Context, id string) (
	*resourcedependency.DependenciesResponse, *tidcommon.ServiceError) {
	if _, svcErr := s.GetHTTPConnection(ctx, id); svcErr != nil {
		return nil, svcErr
	}
	if s.dependencyRegistry == nil {
		s.logger.Warn(ctx, "Dependency registry not set; returning unknown usages", log.String("connectionID", id))
		return &resourcedependency.DependenciesResponse{Usages: []resourcedependency.ResourceDependency{}}, nil
	}
	result, err := s.dependencyRegistry.GetDependencies(ctx, resourcedependency.ResourceTypeHTTPConnection, id)
	if err != nil {
		s.logger.Error(ctx, "Failed to get HTTP connection usages", log.Error(err), log.String("connectionID", id))
		return nil, &tidcommon.InternalServerError
	}
	return result, nil
}

// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (s *httpConnectionService) SetDependencyRegistry(registry resourcedependency.Registry) {
	s.dependencyRegistry = registry
}

// ensureNoBlockingDependencies refuses deletion while a resource uses the connection, or when its
// usages cannot be determined.
func (s *httpConnectionService) ensureNoBlockingDependencies(ctx context.Context, id string) *tidcommon.ServiceError {
	if s.dependencyRegistry == nil {
		s.logger.Error(ctx, "Dependency registry not set; refusing to delete HTTP connection",
			log.String("connectionID", id))
		return &tidcommon.InternalServerError
	}
	deps, err := s.dependencyRegistry.GetDependencies(ctx, resourcedependency.ResourceTypeHTTPConnection, id)
	if err != nil {
		s.logger.Error(ctx, "Failed to evaluate HTTP connection dependencies",
			log.Error(err), log.String("connectionID", id))
		return &tidcommon.InternalServerError
	}
	if deps == nil || deps.TotalResults == nil {
		s.logger.Error(ctx, "HTTP connection dependency data unavailable; refusing to delete",
			log.String("connectionID", id))
		return &tidcommon.InternalServerError
	}
	blocking := resourcedependency.BlockingUsages(deps)
	if len(blocking) == 0 {
		return nil
	}
	return ErrorHasBlockingDependencies.WithParams(map[string]string{
		"dependencies": resourcedependency.SummarizeBlockingUsages(blocking),
	})
}

// ValidateHTTPConnection runs the checks create and update run on the connection alone, normalizing it
// in place.
func (s *httpConnectionService) ValidateHTTPConnection(ctx context.Context,
	connection *HTTPConnection) *tidcommon.ServiceError {
	return s.validate(ctx, connection)
}

// ValidateHTTPConnectionCreate reports what CreateHTTPConnection would refuse, without writing,
// normalizing the connection in place. The store's refusals of a taken ID or name are read rather than
// written.
func (s *httpConnectionService) ValidateHTTPConnectionCreate(ctx context.Context,
	connection *HTTPConnection) *tidcommon.ServiceError {
	if svcErr := s.checkCreate(ctx, connection); svcErr != nil {
		return svcErr
	}
	if _, svcErr := s.GetHTTPConnection(ctx, connection.ID); svcErr == nil {
		return &ErrorAlreadyExists
	} else if svcErr.Code != ErrorNotFound.Code {
		return svcErr
	}
	return s.checkNameFree(ctx, *connection)
}

// ValidateHTTPConnectionUpdate reports what UpdateHTTPConnection would refuse of an update to existing,
// as read by the caller, without writing, normalizing the connection in place. The store's refusals are
// read rather than written.
func (s *httpConnectionService) ValidateHTTPConnectionUpdate(ctx context.Context, existing *HTTPConnection,
	connection *HTTPConnection) *tidcommon.ServiceError {
	if svcErr := s.checkUpdate(ctx, connection); svcErr != nil {
		return svcErr
	}
	if svcErr := s.checkOwnerUnchanged(ctx, connection, existing.OUID); svcErr != nil {
		return svcErr
	}
	connection.ID = existing.ID
	if existing.IsReadOnly {
		return &declarativeresource.ErrorDeclarativeResourceUpdateOperation
	}
	return s.checkNameFree(ctx, *connection)
}

// checkCreate runs the checks a create makes before it writes, normalizing the connection in place and resolving
// its owner.
func (s *httpConnectionService) checkCreate(ctx context.Context, connection *HTTPConnection) *tidcommon.ServiceError {
	if svcErr := declarativeresource.CheckDeclarativeCreate(); svcErr != nil {
		return svcErr
	}
	if svcErr := s.validate(ctx, connection); svcErr != nil {
		return svcErr
	}
	return s.ResolveConnectionOU(ctx, connection)
}

// checkUpdate runs the checks an update makes before it writes, normalizing the connection in place.
func (s *httpConnectionService) checkUpdate(ctx context.Context, connection *HTTPConnection) *tidcommon.ServiceError {
	if svcErr := declarativeresource.CheckDeclarativeUpdate(); svcErr != nil {
		return svcErr
	}
	return s.validate(ctx, connection)
}

// checkNameFree returns ErrorAlreadyExists when another connection, stored or declared, has the
// connection's name.
func (s *httpConnectionService) checkNameFree(ctx context.Context,
	connection HTTPConnection) *tidcommon.ServiceError {
	named, err := s.store.GetHTTPConnectionByName(ctx, connection.Name)
	if err != nil {
		s.logger.Error(ctx, "Failed to get HTTP connection by name", log.Error(err),
			log.String("connectionName", connection.Name))
		return &tidcommon.InternalServerError
	}
	if named != nil && named.ID != connection.ID {
		return &ErrorAlreadyExists
	}
	return nil
}

// validate checks a connection and normalizes it in place for storage.
func (s *httpConnectionService) validate(ctx context.Context, connection *HTTPConnection) *tidcommon.ServiceError {
	connection.Name = strings.TrimSpace(connection.Name)
	if connection.Name == "" || utf8.RuneCountInString(connection.Name) > maxNameLength {
		return &ErrorInvalidName
	}
	if utf8.RuneCountInString(connection.Description) > maxDescriptionLength {
		return &ErrorInvalidDescription
	}
	endpoint, ok := httpauth.ParseEndpointURL(strings.TrimSpace(connection.URL))
	if !ok {
		return &ErrorInvalidURL
	}
	connection.URL = canonicalURL(endpoint)
	if connection.TimeoutMS == 0 {
		connection.TimeoutMS = defaultTimeoutMS
	}
	if connection.TimeoutMS < 1 || connection.TimeoutMS > maxTimeoutMS {
		return &ErrorInvalidTimeout
	}
	if svcErr := s.validateHeaders(connection); svcErr != nil {
		return svcErr
	}
	return s.validateAuthentication(ctx, connection, endpoint)
}

// validateHeaders trims plain headers and checks that each is valid, not repeated and not reserved.
func (s *httpConnectionService) validateHeaders(connection *HTTPConnection) *tidcommon.ServiceError {
	headers := make([]Header, 0, len(connection.Headers))
	seen := make(map[string]bool, len(connection.Headers))
	for _, header := range connection.Headers {
		name := strings.TrimSpace(header.Name)
		canonical := http.CanonicalHeaderKey(name)
		if _, reserved := reservedHeaderNames[strings.ToLower(name)]; reserved || seen[canonical] ||
			!httpguts.ValidHeaderFieldName(name) || !httpguts.ValidHeaderFieldValue(header.Value) {
			return &ErrorInvalidHeaders
		}
		seen[canonical] = true
		headers = append(headers, Header{Name: name, Value: header.Value})
	}
	connection.Headers = headers
	return nil
}

// validateAuthentication checks the connection's outbound authentication against its method, and
// that no plain header repeats a header the method sets.
func (s *httpConnectionService) validateAuthentication(ctx context.Context, connection *HTTPConnection,
	endpoint *url.URL) *tidcommon.ServiceError {
	authConfig, err := outboundauth.FromProperties(connection.AuthProperties)
	if err != nil {
		s.logger.Error(ctx, "Failed to read HTTP connection authentication", log.Error(err))
		return &tidcommon.InternalServerError
	}
	// The HTTP binding validates the method's fields and API-key headers, and refuses the mask, which
	// by now has either been resolved to a stored secret or has nothing to resolve to.
	canonical, err := httpauth.Canonicalize(authConfig)
	if err != nil {
		return &ErrorInvalidAuthentication
	}
	// HTTP Basic joins user-id and password with a colon (RFC 7617), so a colon in the user-id would
	// shift part of it into the password on the receiving side.
	if authConfig.Type == outboundauth.TypeBasic &&
		strings.Contains(authConfig.Properties[outboundauth.FieldBasicUsername], ":") {
		return &ErrorInvalidAuthentication
	}
	// A stored credential must never cross the network in clear.
	if authConfig.Enabled() && endpoint.Scheme != urlSchemeHTTPS {
		return &ErrorInvalidURL
	}
	if canonical.Type == outboundauth.TypeAPIKey {
		for _, header := range connection.Headers {
			if _, found := canonical.Properties[http.CanonicalHeaderKey(header.Name)]; found {
				return &ErrorInvalidHeaders
			}
		}
		// A connection read from a file or an import document carries the header names as written; an
		// update matches a kept header by its stored name, so it is stored canonical.
		if !maps.Equal(canonical.Properties, authConfig.Properties) {
			if connection.AuthProperties, err = outboundauth.ToProperties(canonical); err != nil {
				s.logger.Error(ctx, "Failed to store HTTP connection authentication", log.Error(err))
				return &tidcommon.InternalServerError
			}
		}
	}
	return nil
}

// ResolveConnectionOU sets the organization unit an HTTP connection is owned by. The owner is named by id or by
// handle, and a handle is resolved to the id of the unit it names; when both are given the id wins. A connection has
// to name an organization unit, and it has to exist.
func (s *httpConnectionService) ResolveConnectionOU(
	ctx context.Context, connection *HTTPConnection,
) *tidcommon.ServiceError {
	ouID, svcErr := s.ouService.ResolveOwner(ctx, connection.OUID, connection.OUHandle, ownerErrors)
	if svcErr != nil {
		return svcErr
	}
	connection.OUID = ouID
	return nil
}

// checkOwnerUnchanged keeps the owner of a connection being updated. An owner cannot change; leaving it out keeps the
// one the connection has.
func (s *httpConnectionService) checkOwnerUnchanged(ctx context.Context, connection *HTTPConnection,
	currentOUID string) *tidcommon.ServiceError {
	ouID, svcErr := s.ouService.ResolveOwnerForUpdate(
		ctx, connection.OUID, connection.OUHandle, currentOUID, ownerErrors)
	if svcErr != nil {
		return svcErr
	}
	connection.OUID = ouID
	return nil
}
