// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize wires the connection service over the identity-provider and notification-sender
// services, registers the /connections routes, loads declarative connection resources, and
// returns the connection exporter for the export API.
func Initialize(mux *http.ServeMux, idpService idp.IDPServiceInterface,
	notificationService notification.NotificationSenderMgtSvcInterface,
	resourceService resourceServerLister) (
	declarativeresource.ResourceExporter, error) {
	authZENPDPStore := newAuthZENPDPStoreForService()
	svc := newService(idpService, notificationService, resourceService, authZENPDPStore)
	h := newHandler(svc)
	registerRoutes(mux, h)

	if err := loadDeclarativeResources(idpService, authZENPDPStore); err != nil {
		return nil, err
	}

	return newConnectionExporter(idpService, notificationService, authZENPDPStore), nil
}

var newAuthZENPDPStoreForService = newAuthZENPDPStore

func noContent(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// registerRoutes registers the listing route and each IdP-backed vendor's CRUD routes.
func registerRoutes(mux *http.ServeMux, h *handler) {
	collectionOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	itemOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	// Listing.
	listOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /connections", h.handleListConnections, listOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /connections", noContent, listOpts))

	// IdP-backed vendors.
	registerVendorRoutes(mux, h, "/connections/google", providers.IDPTypeGoogle,
		createHandler(h, googleToIDPDTO, googleFromIDPDTO),
		getHandler(h, providers.IDPTypeGoogle, googleFromIDPDTO),
		updateHandler(h, providers.IDPTypeGoogle, googleToIDPDTO, googleFromIDPDTO),
		collectionOpts, itemOpts)
	registerVendorRoutes(mux, h, "/connections/github", providers.IDPTypeGitHub,
		createHandler(h, githubToIDPDTO, githubFromIDPDTO),
		getHandler(h, providers.IDPTypeGitHub, githubFromIDPDTO),
		updateHandler(h, providers.IDPTypeGitHub, githubToIDPDTO, githubFromIDPDTO),
		collectionOpts, itemOpts)
	registerVendorRoutes(mux, h, "/connections/oidc", providers.IDPTypeOIDC,
		createHandler(h, oidcToIDPDTO, oidcFromIDPDTO),
		getHandler(h, providers.IDPTypeOIDC, oidcFromIDPDTO),
		updateHandler(h, providers.IDPTypeOIDC, oidcToIDPDTO, oidcFromIDPDTO),
		collectionOpts, itemOpts)
	registerVendorRoutes(mux, h, "/connections/oauth", providers.IDPTypeOAuth,
		createHandler(h, oauthToIDPDTO, oauthFromIDPDTO),
		getHandler(h, providers.IDPTypeOAuth, oauthFromIDPDTO),
		updateHandler(h, providers.IDPTypeOAuth, oauthToIDPDTO, oauthFromIDPDTO),
		collectionOpts, itemOpts)

	// SMS-backed vendors.
	registerSMSVendorRoutes(mux, h, "/connections/twilio", ncommon.MessageProviderTypeTwilio,
		createSMSHandler(h, twilioToSenderDTO, twilioFromSenderDTO),
		getSMSHandler(h, ncommon.MessageProviderTypeTwilio, twilioFromSenderDTO),
		updateSMSHandler(h, ncommon.MessageProviderTypeTwilio, twilioToSenderDTO, twilioFromSenderDTO),
		collectionOpts, itemOpts)
	registerSMSVendorRoutes(mux, h, "/connections/vonage", ncommon.MessageProviderTypeVonage,
		createSMSHandler(h, vonageToSenderDTO, vonageFromSenderDTO),
		getSMSHandler(h, ncommon.MessageProviderTypeVonage, vonageFromSenderDTO),
		updateSMSHandler(h, ncommon.MessageProviderTypeVonage, vonageToSenderDTO, vonageFromSenderDTO),
		collectionOpts, itemOpts)
	registerSMSVendorRoutes(mux, h, "/connections/"+smsGatewayVendorName, ncommon.MessageProviderTypeCustom,
		createSMSHandler(h, smsGatewayToSenderDTO, smsGatewayFromSenderDTO),
		getSMSHandler(h, ncommon.MessageProviderTypeCustom, smsGatewayFromSenderDTO),
		updateSMSHandler(h, ncommon.MessageProviderTypeCustom, smsGatewayToSenderDTO, smsGatewayFromSenderDTO),
		collectionOpts, itemOpts)

	registerAuthZENPDPVendorRoutes(mux, h, "/connections/"+authZENPDPVendorName, collectionOpts, itemOpts)
}

// registerVendorRoutes registers the collection (list/create) and item (get/update/delete)
// routes for a single vendor, plus their OPTIONS handlers.
//
//nolint:dupl // mirrors registerSMSVendorRoutes but scopes deletion by IdP type, not message provider
func registerVendorRoutes(mux *http.ServeMux, h *handler, base string, idpType providers.IDPType,
	create, get, update http.HandlerFunc, collectionOpts, itemOpts middleware.CORSOptions) {
	mux.HandleFunc(middleware.WithCORS("GET "+base, h.listInstances(idpType), collectionOpts))
	mux.HandleFunc(middleware.WithCORS("POST "+base, create, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base, noContent, collectionOpts))

	mux.HandleFunc(middleware.WithCORS("GET "+base+"/{id}", get, itemOpts))
	mux.HandleFunc(middleware.WithCORS("PUT "+base+"/{id}", update, itemOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE "+base+"/{id}", h.deleteInstance(idpType), itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base+"/{id}", noContent, itemOpts))

	usagesOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET "+base+"/{id}/usages", h.usagesInstance(idpType), usagesOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base+"/{id}/usages", noContent, usagesOpts))
}

// registerSMSVendorRoutes registers the collection (list/create) and item (get/update/delete)
// routes for a single SMS-backed vendor, plus their OPTIONS handlers.
//
//nolint:dupl // mirrors registerVendorRoutes but scopes deletion by message provider, not IdP type
func registerSMSVendorRoutes(mux *http.ServeMux, h *handler, base string, provider ncommon.MessageProviderType,
	create, get, update http.HandlerFunc, collectionOpts, itemOpts middleware.CORSOptions) {
	mux.HandleFunc(middleware.WithCORS("GET "+base, h.listSMSInstances(provider), collectionOpts))
	mux.HandleFunc(middleware.WithCORS("POST "+base, create, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base, noContent, collectionOpts))

	mux.HandleFunc(middleware.WithCORS("GET "+base+"/{id}", get, itemOpts))
	mux.HandleFunc(middleware.WithCORS("PUT "+base+"/{id}", update, itemOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE "+base+"/{id}", h.deleteSMSInstance(provider), itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base+"/{id}", noContent, itemOpts))

	usagesOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET "+base+"/{id}/usages", h.usagesSMSInstance(provider), usagesOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base+"/{id}/usages", noContent, usagesOpts))
}

func registerAuthZENPDPVendorRoutes(
	mux *http.ServeMux,
	h *handler,
	base string,
	collectionOpts middleware.CORSOptions,
	itemOpts middleware.CORSOptions,
) {
	mux.HandleFunc(middleware.WithCORS("GET "+base, h.listAuthZENPDPConnections, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("POST "+base, h.createAuthZENPDPConnection, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base, noContent, collectionOpts))

	mux.HandleFunc(middleware.WithCORS("GET "+base+"/{id}", h.getAuthZENPDPConnection, itemOpts))
	mux.HandleFunc(middleware.WithCORS("PUT "+base+"/{id}", h.updateAuthZENPDPConnection, itemOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE "+base+"/{id}", h.deleteAuthZENPDPConnection, itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base+"/{id}", noContent, itemOpts))

	usagesOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET "+base+"/{id}/usages", h.usagesAuthZENPDPConnection, usagesOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base+"/{id}/usages", noContent, usagesOpts))
}
