// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/sharing"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize wires the connection service over the identity-provider and notification-sender
// services, registers the /connections routes, loads declarative connection resources, and
// returns the connection exporter for the export API and the provider of the connections that block the deletion of
// the organization unit that owns them. Given an MCP server, it also registers the HTTP endpoint connection tools.
//
// A declarative connection names its owner, and the service of its kind resolves and checks it.
//
// valueCapturer is optional. Given one, the service hands it every connection it creates or changes,
// so the values the connection's export refers to are kept where a reference finds them. It also
// returns the HTTP connection writer.
//
// Connections of every kind register with the sharing framework as one resource type, so a connection
// can be shared with other organization units. A sharee can use the connection as it is and may not
// override any of it.
func Initialize(mux *http.ServeMux, mcpServer *mcp.Server, idpService idp.IDPServiceInterface,
	notificationService notification.NotificationSenderMgtSvcInterface,
	resourceService resource.ResourceServiceInterface,
	authZENPDPService authzenpdp.AuthZENPDPServiceInterface,
	httpService httpconnection.HTTPConnectionServiceInterface,
	valueCapturer declarativeresource.ValueCapturer,
	sharingService sharing.SharingServiceInterface) (
	declarativeresource.ResourceExporter, resourcedependency.Provider, HTTPConnectionWriterInterface, error) {
	svc := newService(idpService, notificationService, resourceService, authZENPDPService, httpService)
	svc.valueCapturer = valueCapturer
	svc.sharingService = sharingService
	sharingService.RegisterResourceType(newConnectionSharingDeclaration(svc))
	h := newHandler(svc)
	registerRoutes(mux, h)

	if mcpServer != nil {
		registerMCPTools(mcpServer, svc)
	}

	if err := loadDeclarativeResources(idpService, notificationService, authZENPDPService, httpService); err != nil {
		return nil, nil, nil, err
	}
	if err := loadDeclarativeSharing(sharingService, idpService, notificationService, authZENPDPService,
		httpService); err != nil {
		return nil, nil, nil, err
	}

	exporter := newConnectionExporter(idpService, notificationService, authZENPDPService, httpService)
	exporter.sharingService = sharingService
	return exporter,
		newOUDependencyProvider(idpService, notificationService, authZENPDPService, httpService),
		&httpConnectionWriter{svc: svc}, nil
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
			smsGatewayUpdateToSenderDTO, smsGatewayFromSenderDTO),
		collectionOpts, itemOpts)

	// Email-backed vendors.
	email := ncommon.NotificationSenderTypeEmail
	registerSenderVendorRoutes(mux, h, "/connections/"+emailSMTPVendorName, email, ncommon.NotificationProviderTypeSMTP,
		createSenderHandler(h, emailSMTPToSenderDTO, emailSMTPFromSenderDTO),
		getSenderHandler(h, email, ncommon.NotificationProviderTypeSMTP, emailSMTPFromSenderDTO),
		updateSenderHandler(h, email, ncommon.NotificationProviderTypeSMTP,
			emailSMTPUpdateToSenderDTO, emailSMTPFromSenderDTO),
		collectionOpts, itemOpts)

	registerAuthZENPDPVendorRoutes(mux, h, "/connections/authzen-pdp", collectionOpts, itemOpts)
	registerHTTPConnectionRoutes(mux, h, "/connections/"+httpVendorName, collectionOpts, itemOpts)

	// Vendor metadata. A single route taking the vendor as a query parameter, rather than
	// /connections/{vendor}/meta, which would have to be registered per vendor and would shadow
	// an instance whose id is literally "meta".
	mux.HandleFunc(middleware.WithCORS("GET /connections/meta", h.handleGetConnectionMeta, listOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /connections/meta", noContent, listOpts))
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

	registerUsagesRoute(mux, base, h.usagesInstance(idpType))
	registerSharingRoutes(mux, h, base, h.svc.idpOwner(idpType))
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

	registerUsagesRoute(mux, base, h.usagesSenderInstance(senderType, provider))
	registerSharingRoutes(mux, h, base, h.svc.senderOwner(senderType, provider))
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

	registerUsagesRoute(mux, base, h.usagesAuthZENPDPConnection)
	registerSharingRoutes(mux, h, base, h.svc.authZENPDPOwner)
}

// registerHTTPConnectionRoutes registers CRUD, CORS, and usage routes for HTTP endpoint connections.
func registerHTTPConnectionRoutes(mux *http.ServeMux, h *handler, base string,
	collectionOpts, itemOpts middleware.CORSOptions) {
	mux.HandleFunc(middleware.WithCORS("GET "+base, h.listHTTPConnections, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("POST "+base, h.createHTTPConnection, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base, noContent, collectionOpts))

	mux.HandleFunc(middleware.WithCORS("GET "+base+"/{id}", h.getHTTPConnection, itemOpts))
	mux.HandleFunc(middleware.WithCORS("PUT "+base+"/{id}", h.updateHTTPConnection, itemOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE "+base+"/{id}", h.deleteHTTPConnection, itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base+"/{id}", noContent, itemOpts))

	registerUsagesRoute(mux, base, h.usagesHTTPConnection)
	registerSharingRoutes(mux, h, base, h.svc.httpOwner)
}

// registerUsagesRoute registers the GET and CORS routes that list the resources using an instance
// under base.
func registerUsagesRoute(mux *http.ServeMux, base string, usages http.HandlerFunc) {
	usagesOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET "+base+"/{id}/usages", usages, usagesOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base+"/{id}/usages", noContent, usagesOpts))
}

// registerSharingRoutes registers the sharing policy routes of one vendor's connections. They sit under
// each vendor's path rather than under a common /connections/{id}, which would be ambiguous with the
// vendor routes. Sharing policies live under the connection they govern: a policy has no life of its
// own.
func registerSharingRoutes(mux *http.ServeMux, h *handler, base string, owner ownerLookup) {
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
	overlayOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	policies := base + "/{id}/sharing-policies"
	mux.HandleFunc(middleware.WithCORS("POST "+policies, h.handleSharingPolicyPost(owner), collectionOpts))
	mux.HandleFunc(middleware.WithCORS("GET "+policies, h.handleSharingPolicyList(owner), collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+policies, noContent, collectionOpts))

	policy := policies + "/{policyId}"
	mux.HandleFunc(middleware.WithCORS("GET "+policy, h.handleSharingPolicyGet(owner), itemOpts))
	mux.HandleFunc(middleware.WithCORS("PUT "+policy, h.handleSharingPolicyPut(owner), itemOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE "+policy, h.handleSharingPolicyDelete(owner), itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+policy, noContent, itemOpts))

	overlay := base + "/{id}/overlay-rules"
	mux.HandleFunc(middleware.WithCORS("GET "+overlay, h.handleOverlayRuleGet(owner), overlayOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+overlay, noContent, overlayOpts))
}
