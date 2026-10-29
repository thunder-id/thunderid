// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// DiscoveryHandlerInterface defines the interface for discovery handlers
type discoveryHandlerInterface interface {
	HandleOAuth2AuthorizationServerMetadata(w http.ResponseWriter, r *http.Request)
	HandleOIDCDiscovery(w http.ResponseWriter, r *http.Request)
	HandleAuthorizationDetailsTypes(w http.ResponseWriter, r *http.Request)
	HandleAuthorizationDetailsType(w http.ResponseWriter, r *http.Request)
}

// discoveryHandler implements DiscoveryHandlerInterface
type discoveryHandler struct {
	discoveryService DiscoveryServiceInterface
}

// NewDiscoveryHandler creates a new discovery handler
func newDiscoveryHandler(discoveryService DiscoveryServiceInterface) discoveryHandlerInterface {
	return &discoveryHandler{
		discoveryService: discoveryService,
	}
}

// HandleOAuth2AuthorizationServerMetadata handles OAuth 2.0 Authorization Server Metadata requests
func (dh *discoveryHandler) HandleOAuth2AuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DiscoveryHandler"))

	metadata := dh.discoveryService.GetOAuth2AuthorizationServerMetadata(ctx)

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, metadata)
	logger.Debug(ctx, "OAuth 2.0 Authorization Server Metadata response sent successfully")
}

// HandleOIDCDiscovery handles OpenID Connect Discovery requests
func (dh *discoveryHandler) HandleOIDCDiscovery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DiscoveryHandler"))

	metadata, err := dh.discoveryService.GetOIDCMetadata(ctx)
	if err != nil {
		sysutils.WriteErrorResponse(ctx, w, http.StatusInternalServerError, apierror.ErrorResponse{})
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, metadata)
	logger.Debug(ctx, "OIDC discovery response sent successfully")
}

// HandleAuthorizationDetailsTypes serves the request view of every registered type.
func (dh *discoveryHandler) HandleAuthorizationDetailsTypes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DiscoveryHandler"))
	types, err := dh.discoveryService.ListAuthorizationDetailsTypeMetadata(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to read authorization details type metadata", log.Error(err))
		sysutils.WriteErrorResponse(ctx, w, http.StatusInternalServerError, internalServerError)
		return
	}
	writeCacheable(w, r, AuthorizationDetailsTypesResponse{Types: types})
}

// HandleAuthorizationDetailsType serves the request view of the type named by the URL-encoded path value.
func (dh *discoveryHandler) HandleAuthorizationDetailsType(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DiscoveryHandler"))
	metadata, err := dh.discoveryService.GetAuthorizationDetailsTypeMetadata(ctx, r.PathValue("type"))
	if err != nil {
		logger.Error(ctx, "Failed to read authorization details type metadata", log.Error(err))
		sysutils.WriteErrorResponse(ctx, w, http.StatusInternalServerError, internalServerError)
		return
	}
	if metadata == nil {
		sysutils.WriteErrorResponse(ctx, w, http.StatusNotFound, errorAuthorizationDetailsTypeNotFound)
		return
	}
	writeCacheable(w, r, metadata)
}

// writeCacheable writes a JSON response with an ETag, answering 304 Not Modified when it matches.
func writeCacheable(w http.ResponseWriter, r *http.Request, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DiscoveryHandler")).Error(r.Context(),
			"Failed to encode authorization details type metadata", log.Error(err))
		sysutils.WriteErrorResponse(r.Context(), w, http.StatusInternalServerError, internalServerError)
		return
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	w.Header().Set("ETag", etag)
	if etagListed(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set(constants.ContentTypeHeaderName, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// etagListed reports whether an If-None-Match header lists etag. The comparison is weak, as RFC 9110 section
// 13.1.2 requires, so the weak form of the tag a compressing proxy sends still matches.
func etagListed(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}
	return false
}
