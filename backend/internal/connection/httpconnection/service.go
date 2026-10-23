// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/http/httpguts"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/httpauth"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// HTTPConnectionResolver resolves an HTTP connection for an outbound call. It is the seam a caller
// of a connection depends on, so the caller does not see the management operations.
type HTTPConnectionResolver interface {
	// GetResolvedConnection returns the connection's URL, timeout and headers with its
	// authentication applied, or ErrorNotFound when the connection does not exist.
	GetResolvedConnection(ctx context.Context, id string) (*ResolvedHTTPConnection, *tidcommon.ServiceError)
}

// HTTPConnectionServiceInterface defines HTTP connection management operations. A connection's
// authentication is taken as given: keeping a stored secret the caller omitted is the caller's
// decision, made before an update.
type HTTPConnectionServiceInterface interface {
	HTTPConnectionResolver
	CreateHTTPConnection(ctx context.Context, connection HTTPConnection) (*HTTPConnection, *tidcommon.ServiceError)
	ListHTTPConnections(ctx context.Context) ([]HTTPConnection, *tidcommon.ServiceError)
	GetHTTPConnection(ctx context.Context, id string) (*HTTPConnection, *tidcommon.ServiceError)
	UpdateHTTPConnection(ctx context.Context, id string, connection HTTPConnection) (
		*HTTPConnection, *tidcommon.ServiceError)
	DeleteHTTPConnection(ctx context.Context, id string) *tidcommon.ServiceError
	GetHTTPConnectionUsages(ctx context.Context, id string) (
		*resourcedependency.DependenciesResponse, *tidcommon.ServiceError)
	SetDependencyRegistry(registry resourcedependency.Registry)
}

// httpConnectionService validates and manages HTTP connections.
type httpConnectionService struct {
	store              httpConnectionStoreInterface
	dependencyRegistry resourcedependency.Registry
	logger             *log.Logger
}

var _ HTTPConnectionServiceInterface = (*httpConnectionService)(nil)

// newHTTPConnectionService creates an HTTP connection service.
func newHTTPConnectionService(store httpConnectionStoreInterface) *httpConnectionService {
	return &httpConnectionService{
		store:  store,
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// GetResolvedConnection returns a connection ready to call. The plain headers are set first and
// the authentication's HTTP binding then adds the credential, decrypted only into the returned
// headers. The binding applies the credential to a request for the connection's URL, so it refuses
// a credential that would cross the network in clear.
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
	if svcErr := declarativeresource.CheckDeclarativeCreate(); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.validate(ctx, &connection); svcErr != nil {
		return nil, svcErr
	}
	id, err := sysutils.GenerateUUIDv7()
	if err != nil {
		s.logger.Error(ctx, "Failed to generate HTTP connection ID", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	connection.ID = id

	if err := s.store.CreateHTTPConnection(ctx, connection); err != nil {
		if errors.Is(err, errNameConflict) {
			return nil, &ErrorAlreadyExists
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
	if svcErr := declarativeresource.CheckDeclarativeUpdate(); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.validate(ctx, &connection); svcErr != nil {
		return nil, svcErr
	}
	connection.ID = id

	if err := s.store.UpdateHTTPConnection(ctx, connection); err != nil {
		switch {
		case errors.Is(err, errNameConflict):
			return nil, &ErrorAlreadyExists
		case errors.Is(err, errConnectionNotFound):
			return nil, &ErrorNotFound
		}
		s.logger.Error(ctx, "Failed to update HTTP connection", log.Error(err), log.String("connectionID", id))
		return nil, &tidcommon.InternalServerError
	}
	return &connection, nil
}

// DeleteHTTPConnection deletes a connection that no flow references.
func (s *httpConnectionService) DeleteHTTPConnection(ctx context.Context, id string) *tidcommon.ServiceError {
	if svcErr := declarativeresource.CheckDeclarativeDelete(); svcErr != nil {
		return svcErr
	}
	if _, svcErr := s.GetHTTPConnection(ctx, id); svcErr != nil {
		return svcErr
	}
	if svcErr := s.ensureNoBlockingDependencies(ctx, id); svcErr != nil {
		return svcErr
	}
	if err := s.store.DeleteHTTPConnection(ctx, id); err != nil {
		if errors.Is(err, errConnectionNotFound) {
			return &ErrorNotFound
		}
		s.logger.Error(ctx, "Failed to delete HTTP connection", log.Error(err), log.String("connectionID", id))
		return &tidcommon.InternalServerError
	}
	return nil
}

// GetHTTPConnectionUsages returns the resources that reference a connection, such as flows that
// call it. It is informational; deletion is gated separately by ensureNoBlockingDependencies.
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

// ensureNoBlockingDependencies refuses deletion while a resource, such as a flow, references the
// connection. Deletion is destructive, so it fails closed: when usage cannot be determined the
// deletion is refused rather than allowed.
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

// validate checks a connection and normalizes it in place for storage.
func (s *httpConnectionService) validate(ctx context.Context, connection *HTTPConnection) *tidcommon.ServiceError {
	connection.Name = strings.TrimSpace(connection.Name)
	if connection.Name == "" || utf8.RuneCountInString(connection.Name) > maxNameLength {
		return &ErrorInvalidName
	}
	if utf8.RuneCountInString(connection.Description) > maxDescriptionLength {
		return &ErrorInvalidDescription
	}
	endpoint, ok := parseEndpointURL(strings.TrimSpace(connection.URL))
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
	}
	return nil
}
