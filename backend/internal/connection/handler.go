// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// handler serves the connection HTTP endpoints. Each vendor file (google.go, ...) defines
// its own typed request/response structs, its toDTO/fromDTO mappers, and thin handler
// methods that delegate the request plumbing to the generic helpers below.
type handler struct {
	svc *service
}

// newHandler creates a new connection handler.
func newHandler(svc *service) *handler {
	return &handler{svc: svc}
}

// createConnection decodes a typed request, maps it to an IdP DTO via the vendor's mapper,
// delegates creation, and writes the encoded response.
func createConnection[Req any, Resp any](h *handler, w http.ResponseWriter, r *http.Request,
	toDTO func(Req) (*providers.IDPDTO, error), fromDTO func(providers.IDPDTO) (Resp, error)) {
	ctx := r.Context()
	req, err := sysutils.DecodeJSONBody[Req](r)
	if err != nil {
		writeInvalidBody(ctx, w)
		return
	}
	dto, err := toDTO(*req)
	if err != nil {
		writeServiceError(ctx, w, &tidcommon.InternalServerError)
		return
	}
	created, svcErr := h.svc.create(ctx, dto)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	resp, err := fromDTO(*created)
	if err != nil {
		writeServiceError(ctx, w, &tidcommon.InternalServerError)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, resp)
}

// getConnection fetches an instance of the given type and writes the encoded response.
func getConnection[Resp any](h *handler, w http.ResponseWriter, r *http.Request,
	idpType providers.IDPType, fromDTO func(providers.IDPDTO) (Resp, error)) {
	ctx := r.Context()
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeServiceError(ctx, w, &idp.ErrorInvalidIDPID)
		return
	}
	dto, svcErr := h.svc.getByType(ctx, idpType, id)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	resp, err := fromDTO(*dto)
	if err != nil {
		writeServiceError(ctx, w, &tidcommon.InternalServerError)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// updateConnection decodes a typed request, maps it, delegates the update (which preserves
// any secret the request omits), and writes the encoded response.
func updateConnection[Req any, Resp any](h *handler, w http.ResponseWriter, r *http.Request,
	idpType providers.IDPType, toDTO func(Req) (*providers.IDPDTO, error),
	fromDTO func(providers.IDPDTO) (Resp, error)) {
	ctx := r.Context()
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeServiceError(ctx, w, &idp.ErrorInvalidIDPID)
		return
	}
	req, err := sysutils.DecodeJSONBody[Req](r)
	if err != nil {
		writeInvalidBody(ctx, w)
		return
	}
	dto, err := toDTO(*req)
	if err != nil {
		writeServiceError(ctx, w, &tidcommon.InternalServerError)
		return
	}
	updated, svcErr := h.svc.update(ctx, idpType, id, dto)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	resp, err := fromDTO(*updated)
	if err != nil {
		writeServiceError(ctx, w, &tidcommon.InternalServerError)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// createHandler binds a vendor's mappers to createConnection, yielding a registerable handler.
func createHandler[Req any, Resp any](h *handler, toDTO func(Req) (*providers.IDPDTO, error),
	fromDTO func(providers.IDPDTO) (Resp, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		createConnection(h, w, r, toDTO, fromDTO)
	}
}

// getHandler binds a vendor's type and mapper to getConnection, yielding a registerable handler.
func getHandler[Resp any](h *handler, idpType providers.IDPType,
	fromDTO func(providers.IDPDTO) (Resp, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		getConnection(h, w, r, idpType, fromDTO)
	}
}

// updateHandler binds a vendor's type and mappers to updateConnection, yielding a registerable handler.
func updateHandler[Req any, Resp any](h *handler, idpType providers.IDPType,
	toDTO func(Req) (*providers.IDPDTO, error),
	fromDTO func(providers.IDPDTO) (Resp, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		updateConnection(h, w, r, idpType, toDTO, fromDTO)
	}
}

// listInstances returns a handler that lists the configured instances of a connection type.
func (h *handler) listInstances(idpType providers.IDPType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		instances, svcErr := h.svc.listByType(ctx, idpType)
		if svcErr != nil {
			writeServiceError(ctx, w, svcErr)
			return
		}
		summaries := make([]connectionInstanceSummary, 0, len(instances))
		for _, instance := range instances {
			summaries = append(summaries, connectionInstanceSummary{
				ID:          instance.ID,
				Name:        instance.Name,
				Description: instance.Description,
			})
		}
		sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, summaries)
	}
}

// deleteInstance returns a handler that deletes an instance of a connection type.
func (h *handler) deleteInstance(idpType providers.IDPType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := r.PathValue("id")
		if strings.TrimSpace(id) == "" {
			writeServiceError(ctx, w, &idp.ErrorInvalidIDPID)
			return
		}
		if svcErr := h.svc.deleteByType(ctx, idpType, id); svcErr != nil {
			writeServiceError(ctx, w, svcErr)
			return
		}
		sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
	}
}

// usagesInstance returns a handler that lists the resources referencing an instance of a
// connection type. Drives the pre-delete confirmation dialog.
func (h *handler) usagesInstance(idpType providers.IDPType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := r.PathValue("id")
		if strings.TrimSpace(id) == "" {
			writeServiceError(ctx, w, &idp.ErrorInvalidIDPID)
			return
		}
		usages, svcErr := h.svc.usagesByType(ctx, idpType, id)
		if svcErr != nil {
			writeServiceError(ctx, w, svcErr)
			return
		}
		sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, usages)
	}
}

// parsePaginationParams parses the limit and offset query parameters, applying the default
// page size when limit is omitted.
func parsePaginationParams(query url.Values) (int, int, *tidcommon.ServiceError) {
	limit := serverconst.DefaultPageSize
	offset := 0

	if limitStr := query.Get("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil {
			return 0, 0, &ErrorInvalidLimit
		}
		limit = parsedLimit
	}

	if offsetStr := query.Get("offset"); offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err != nil {
			return 0, 0, &ErrorInvalidOffset
		}
		offset = parsedOffset
	}

	return limit, offset, nil
}

// handleListConnections handles GET /connections, returning a paginated flat list of configured
// connection instances across both IdP- and sender-backed services, optionally filtered by
// the category query parameter.
func (h *handler) handleListConnections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	category, ok := parseConnectionCategory(r.URL.Query().Get("category"))
	if !ok {
		writeServiceError(ctx, w, &ErrorInvalidConnectionCategory)
		return
	}
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	resp, svcErr := h.svc.listInstances(ctx, category, limit, offset)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// createAuthZENPDPConnection creates an external AuthZEN PDP connection.
func (h *handler) createAuthZENPDPConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, err := sysutils.DecodeJSONBody[authZENPDPConnectionRequest](r)
	if err != nil {
		writeInvalidBody(ctx, w)
		return
	}
	created, svcErr := h.svc.createAuthZENPDP(ctx, authZENPDPFromRequest(*req))
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, authZENPDPToResponse(*created))
}

// listAuthZENPDPConnections lists configured external AuthZEN PDP connections.
func (h *handler) listAuthZENPDPConnections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	connections, svcErr := h.svc.listAuthZENPDP(ctx)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	summaries := make([]connectionInstanceSummary, 0, len(connections))
	for _, connection := range connections {
		summaries = append(summaries, connectionInstanceSummary{
			ID:          connection.ID,
			Name:        connection.Name,
			Description: connection.Description,
		})
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, summaries)
}

// getAuthZENPDPConnection returns an external AuthZEN PDP connection by ID.
func (h *handler) getAuthZENPDPConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeServiceError(ctx, w, &ErrorConnectionNotFound)
		return
	}
	connection, svcErr := h.svc.getAuthZENPDP(ctx, id)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, authZENPDPToResponse(*connection))
}

// updateAuthZENPDPConnection updates an external AuthZEN PDP connection by ID.
func (h *handler) updateAuthZENPDPConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeServiceError(ctx, w, &ErrorConnectionNotFound)
		return
	}
	req, err := sysutils.DecodeJSONBody[authZENPDPConnectionRequest](r)
	if err != nil {
		writeInvalidBody(ctx, w)
		return
	}
	updated, svcErr := h.svc.updateAuthZENPDP(ctx, id, authZENPDPFromRequest(*req))
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, authZENPDPToResponse(*updated))
}

// deleteAuthZENPDPConnection deletes an external AuthZEN PDP connection by ID.
func (h *handler) deleteAuthZENPDPConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeServiceError(ctx, w, &ErrorConnectionNotFound)
		return
	}
	if svcErr := h.svc.deleteAuthZENPDP(ctx, id); svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
}

// usagesAuthZENPDPConnection lists resources that reference an external AuthZEN PDP connection.
func (h *handler) usagesAuthZENPDPConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeServiceError(ctx, w, &ErrorConnectionNotFound)
		return
	}
	usages, svcErr := h.svc.usagesAuthZENPDP(ctx, id)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, usages)
}

// createSMSConnection decodes a typed request, maps it to a notification-sender DTO via the
// vendor's mapper, delegates creation, and writes the encoded response.
func createSMSConnection[Req any, Resp any](h *handler, w http.ResponseWriter, r *http.Request,
	toDTO func(Req) (*ncommon.NotificationSenderDTO, error),
	fromDTO func(ncommon.NotificationSenderDTO) (Resp, error)) {
	ctx := r.Context()
	req, err := sysutils.DecodeJSONBody[Req](r)
	if err != nil {
		writeInvalidBody(ctx, w)
		return
	}
	dto, err := toDTO(*req)
	if err != nil {
		writeServiceError(ctx, w, &tidcommon.InternalServerError)
		return
	}
	created, svcErr := h.svc.createSMS(ctx, *dto)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	resp, err := fromDTO(*created)
	if err != nil {
		writeServiceError(ctx, w, &tidcommon.InternalServerError)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, resp)
}

// getSMSConnection fetches a message sender of the given provider and writes the encoded response.
func getSMSConnection[Resp any](h *handler, w http.ResponseWriter, r *http.Request,
	provider ncommon.MessageProviderType, fromDTO func(ncommon.NotificationSenderDTO) (Resp, error)) {
	ctx := r.Context()
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeServiceError(ctx, w, &notification.ErrorInvalidSenderID)
		return
	}
	dto, svcErr := h.svc.getSMSByProvider(ctx, provider, id)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	resp, err := fromDTO(*dto)
	if err != nil {
		writeServiceError(ctx, w, &tidcommon.InternalServerError)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// updateSMSConnection decodes a typed request, maps it, delegates the update (which preserves
// any secret the request omits), and writes the encoded response.
func updateSMSConnection[Req any, Resp any](h *handler, w http.ResponseWriter, r *http.Request,
	provider ncommon.MessageProviderType, toDTO func(Req) (*ncommon.NotificationSenderDTO, error),
	fromDTO func(ncommon.NotificationSenderDTO) (Resp, error)) {
	ctx := r.Context()
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeServiceError(ctx, w, &notification.ErrorInvalidSenderID)
		return
	}
	req, err := sysutils.DecodeJSONBody[Req](r)
	if err != nil {
		writeInvalidBody(ctx, w)
		return
	}
	dto, err := toDTO(*req)
	if err != nil {
		writeServiceError(ctx, w, &tidcommon.InternalServerError)
		return
	}
	updated, svcErr := h.svc.updateSMS(ctx, provider, id, *dto)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	resp, err := fromDTO(*updated)
	if err != nil {
		writeServiceError(ctx, w, &tidcommon.InternalServerError)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// createSMSHandler binds a vendor's mappers to createSMSConnection, yielding a registerable handler.
func createSMSHandler[Req any, Resp any](h *handler,
	toDTO func(Req) (*ncommon.NotificationSenderDTO, error),
	fromDTO func(ncommon.NotificationSenderDTO) (Resp, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		createSMSConnection(h, w, r, toDTO, fromDTO)
	}
}

// getSMSHandler binds a vendor's provider and mapper to getSMSConnection, yielding a handler.
func getSMSHandler[Resp any](h *handler, provider ncommon.MessageProviderType,
	fromDTO func(ncommon.NotificationSenderDTO) (Resp, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		getSMSConnection(h, w, r, provider, fromDTO)
	}
}

// updateSMSHandler binds a vendor's provider and mappers to updateSMSConnection.
func updateSMSHandler[Req any, Resp any](h *handler, provider ncommon.MessageProviderType,
	toDTO func(Req) (*ncommon.NotificationSenderDTO, error),
	fromDTO func(ncommon.NotificationSenderDTO) (Resp, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		updateSMSConnection(h, w, r, provider, toDTO, fromDTO)
	}
}

// listSMSInstances returns a handler that lists the configured senders of a message provider.
func (h *handler) listSMSInstances(provider ncommon.MessageProviderType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		instances, svcErr := h.svc.listSMSByProvider(ctx, provider)
		if svcErr != nil {
			writeServiceError(ctx, w, svcErr)
			return
		}
		summaries := make([]connectionInstanceSummary, 0, len(instances))
		for _, instance := range instances {
			summaries = append(summaries, connectionInstanceSummary{
				ID:          instance.ID,
				Name:        instance.Name,
				Description: instance.Description,
			})
		}
		sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, summaries)
	}
}

// deleteSMSInstance returns a handler that deletes a sender of a message provider.
func (h *handler) deleteSMSInstance(provider ncommon.MessageProviderType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := r.PathValue("id")
		if strings.TrimSpace(id) == "" {
			writeServiceError(ctx, w, &notification.ErrorInvalidSenderID)
			return
		}
		if svcErr := h.svc.deleteSMSByProvider(ctx, provider, id); svcErr != nil {
			writeServiceError(ctx, w, svcErr)
			return
		}
		sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
	}
}

// usagesSMSInstance returns a handler that lists the resources referencing a sender of a message
// provider. Drives the pre-delete confirmation dialog.
func (h *handler) usagesSMSInstance(provider ncommon.MessageProviderType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := r.PathValue("id")
		if strings.TrimSpace(id) == "" {
			writeServiceError(ctx, w, &notification.ErrorInvalidSenderID)
			return
		}
		usages, svcErr := h.svc.usagesSMSByProvider(ctx, provider, id)
		if svcErr != nil {
			writeServiceError(ctx, w, svcErr)
			return
		}
		sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, usages)
	}
}
