// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/sharing"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// sharingHandler serves the sharing-policy endpoints of the resource-server API.
type sharingHandler struct {
	resourceService ResourceServiceInterface
	sharingService  sharing.ServiceInterface
}

// newSharingHandler creates the resource-server sharing handler.
func newSharingHandler(
	resourceService ResourceServiceInterface, sharingService sharing.ServiceInterface,
) *sharingHandler {
	return &sharingHandler{resourceService: resourceService, sharingService: sharingService}
}

// HandleSharingPolicyPostRequest records a sharing policy for a resource server.
func (h *sharingHandler) HandleSharingPolicyPostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	serverID := r.PathValue("id")

	owner, svcErr := h.owningOUID(ctx, serverID)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	var req SharingPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(ctx, w, &ErrorInvalidRequestFormat)
		return
	}

	policy, svcErr := h.sharingService.CreatePolicy(
		ctx, ResourceServerSharingType, serverID, owner, req.toServiceRequest())
	if svcErr != nil {
		handleError(ctx, w, mapSharingError(svcErr))
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, toSharingPolicyResponse(policy))
}

// HandleSharingPolicyListRequest lists every sharing policy recorded for a resource server.
func (h *sharingHandler) HandleSharingPolicyListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	serverID := r.PathValue("id")

	if _, svcErr := h.owningOUID(ctx, serverID); svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	policies, svcErr := h.sharingService.ListPolicies(ctx, ResourceServerSharingType, serverID)
	if svcErr != nil {
		handleError(ctx, w, mapSharingError(svcErr))
		return
	}

	out := make([]SharingPolicyResponse, 0, len(policies))
	for _, p := range policies {
		out = append(out, toSharingPolicyResponse(p))
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, SharingPolicyListResponse{
		TotalResults: len(out),
		Policies:     out,
	})
}

// HandleSharingPolicyGetRequest returns one sharing policy.
func (h *sharingHandler) HandleSharingPolicyGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	policy, svcErr := h.policyOfServer(ctx, r)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, toSharingPolicyResponse(policy))
}

// HandleSharingPolicyPutRequest replaces a sharing policy's contents.
func (h *sharingHandler) HandleSharingPolicyPutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current, svcErr := h.policyOfServer(ctx, r)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	var req SharingPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(ctx, w, &ErrorInvalidRequestFormat)
		return
	}

	serviceReq := req.toServiceRequest()
	// If-Match carries the version the caller believes it is editing; the body field is the
	// fallback for clients that cannot set the header.
	if v, ok := ifMatchVersion(r); ok {
		serviceReq.Version = v
	}

	updated, svcErr := h.sharingService.UpdatePolicy(ctx, current.ID, serviceReq)
	if svcErr != nil {
		handleError(ctx, w, mapSharingError(svcErr))
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, toSharingPolicyResponse(updated))
}

// HandleSharingPolicyDeleteRequest removes a sharing policy.
func (h *sharingHandler) HandleSharingPolicyDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	policy, svcErr := h.policyOfServer(ctx, r)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	if svcErr := h.sharingService.DeletePolicy(ctx, policy.ID); svcErr != nil {
		handleError(ctx, w, mapSharingError(svcErr))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleSharingOverlayGetRequest returns what one organization unit may do with the server.
func (h *sharingHandler) HandleSharingOverlayGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	serverID := r.PathValue("id")

	if _, svcErr := h.owningOUID(ctx, serverID); svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	ouID := strings.TrimSpace(r.URL.Query().Get("ouId"))
	if ouID == "" {
		handleError(ctx, w, &ErrorInvalidRequestFormat)
		return
	}

	resolved, svcErr := h.sharingService.ResolveOverlayRules(
		ctx, ResourceServerSharingType, serverID, ouID)
	if svcErr != nil {
		handleError(ctx, w, mapSharingError(svcErr))
		return
	}

	// An organization unit no policy reached holds nothing here, and the resource server does not
	// exist as far as it is concerned. Answering with an empty rule set would describe a server it
	// was never told about, which is the one thing a withheld share must not do.
	if !resolved.Visible {
		handleError(ctx, w, &ErrorResourceServerNotFound)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, toSharingOverlayResponse(resolved))
}

// owningOUID resolves the server's owner, reporting not-found when the server does not exist so a
// sharing call cannot record a policy against nothing.
func (h *sharingHandler) owningOUID(
	ctx context.Context, serverID string,
) (string, *tidcommon.ServiceError) {
	server, svcErr := h.resourceService.GetResourceServer(ctx, serverID)
	if svcErr != nil {
		return "", svcErr
	}
	return server.OUID, nil
}

// policyOfServer loads a policy and refuses one recorded against a different resource, so a policy
// id from one server cannot be read or edited through another's path.
func (h *sharingHandler) policyOfServer(
	ctx context.Context, r *http.Request,
) (sharing.Policy, *tidcommon.ServiceError) {
	serverID := r.PathValue("id")
	if _, svcErr := h.owningOUID(ctx, serverID); svcErr != nil {
		return sharing.Policy{}, svcErr
	}

	policy, svcErr := h.sharingService.GetPolicy(ctx, r.PathValue("policyId"))
	if svcErr != nil {
		return sharing.Policy{}, mapSharingError(svcErr)
	}
	if policy.ResourceType != ResourceServerSharingType || policy.ResourceID != serverID {
		return sharing.Policy{}, &ErrorSharingPolicyNotFound
	}
	return policy, nil
}

// ifMatchVersion reads the optimistic-concurrency version from If-Match, tolerating the quoting an
// entity tag normally carries.
func ifMatchVersion(r *http.Request) (int, bool) {
	raw := strings.Trim(strings.TrimSpace(r.Header.Get("If-Match")), `"`)
	if raw == "" {
		return 0, false
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return v, true
}
