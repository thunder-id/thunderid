// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/resource"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize wires the connection service over the identity-provider and notification-sender
// services, registers the /connections routes, loads declarative connection resources, and
// returns the connection exporter for the export API.
func Initialize(mux *http.ServeMux, idpService idp.IDPServiceInterface,
	notificationService notification.NotificationSenderMgtSvcInterface,
	resourceService resource.ResourceServiceInterface,
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface) (
	declarativeresource.ResourceExporter, error) {
	svc := newService(idpService, notificationService, resourceService, authZENPDPService)
	h := newHandler(svc)
	registerRoutes(mux, h)

	if err := loadDeclarativeResources(idpService); err != nil {
		return nil, err
	}

	return newConnectionExporter(idpService, notificationService, authZENPDPService), nil
}

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

	// Vendor metadata. A single route taking the vendor as a query parameter, rather than
	// /connections/{vendor}/meta, which would have to be registered per vendor and would shadow
	// an instance whose id is literally "meta".
	mux.HandleFunc(middleware.WithCORS("GET /connections/meta", h.handleGetConnectionMeta, listOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /connections/meta", noContent, listOpts))

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
	message := ncommon.NotificationSenderTypeMessage
	registerSenderVendorRoutes(mux, h, "/connections/twilio", message, ncommon.NotificationProviderTypeTwilio,
		createSenderHandler(h, twilioToSenderDTO, twilioFromSenderDTO),
		getSenderHandler(h, message, ncommon.NotificationProviderTypeTwilio, twilioFromSenderDTO),
		updateSenderHandler(h, message, ncommon.NotificationProviderTypeTwilio,
			twilioToSenderDTO, twilioFromSenderDTO),
		collectionOpts, itemOpts)
	registerSenderVendorRoutes(mux, h, "/connections/vonage", message, ncommon.NotificationProviderTypeVonage,
		createSenderHandler(h, vonageToSenderDTO, vonageFromSenderDTO),
		getSenderHandler(h, message, ncommon.NotificationProviderTypeVonage, vonageFromSenderDTO),
		updateSenderHandler(h, message, ncommon.NotificationProviderTypeVonage,
			vonageToSenderDTO, vonageFromSenderDTO),
		collectionOpts, itemOpts)
	registerSenderVendorRoutes(mux, h, "/connections/"+smsGatewayVendorName, message,
		ncommon.NotificationProviderTypeCustom,
		createSenderHandler(h, smsGatewayToSenderDTO, smsGatewayFromSenderDTO),
		getSenderHandler(h, message, ncommon.NotificationProviderTypeCustom, smsGatewayFromSenderDTO),
		updateSenderHandler(h, message, ncommon.NotificationProviderTypeCustom,
			smsGatewayToSenderDTO, smsGatewayFromSenderDTO),
		collectionOpts, itemOpts)

	// Email-backed vendors.
	email := ncommon.NotificationSenderTypeEmail
	registerSenderVendorRoutes(mux, h, "/connections/"+emailSMTPVendorName, email, ncommon.NotificationProviderTypeSMTP,
		createSenderHandler(h, emailSMTPToSenderDTO, emailSMTPFromSenderDTO),
		getSenderHandler(h, email, ncommon.NotificationProviderTypeSMTP, emailSMTPFromSenderDTO),
		updateSenderHandler(h, email, ncommon.NotificationProviderTypeSMTP,
			emailSMTPToSenderDTO, emailSMTPFromSenderDTO),
		collectionOpts, itemOpts)

	registerAuthZENPDPVendorRoutes(mux, h, "/connections/authzen-pdp", collectionOpts, itemOpts)
}

// registerVendorRoutes registers the collection (list/create) and item (get/update/delete)
// routes for a single vendor, plus their OPTIONS handlers.
//
//nolint:dupl // mirrors registerSenderVendorRoutes but scopes by IdP type, not sender type
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

// registerSenderVendorRoutes registers the collection (list/create) and item (get/update/delete)
// routes for a single notification-sender-backed vendor, plus their OPTIONS handlers. The sender
// type scopes every lookup alongside the provider, so an email endpoint cannot reach a message
// sender and vice versa.
//
//nolint:dupl // mirrors registerVendorRoutes but scopes by sender type and provider, not IdP type
func registerSenderVendorRoutes(mux *http.ServeMux, h *handler, base string,
	senderType ncommon.NotificationSenderType, provider ncommon.NotificationProviderType,
	create, get, update http.HandlerFunc, collectionOpts, itemOpts middleware.CORSOptions) {
	mux.HandleFunc(middleware.WithCORS("GET "+base, h.listSenderInstances(senderType, provider), collectionOpts))
	mux.HandleFunc(middleware.WithCORS("POST "+base, create, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base, noContent, collectionOpts))

	mux.HandleFunc(middleware.WithCORS("GET "+base+"/{id}", get, itemOpts))
	mux.HandleFunc(middleware.WithCORS("PUT "+base+"/{id}", update, itemOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE "+base+"/{id}",
		h.deleteSenderInstance(senderType, provider), itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base+"/{id}", noContent, itemOpts))

	usagesOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET "+base+"/{id}/usages",
		h.usagesSenderInstance(senderType, provider), usagesOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base+"/{id}/usages", noContent, usagesOpts))
}

// registerAuthZENPDPVendorRoutes registers CRUD, CORS, and usage routes for AuthZEN PDP connections.
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
