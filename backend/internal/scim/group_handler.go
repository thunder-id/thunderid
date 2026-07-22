package scim

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type scimGroupsHandler struct {
	svc     SCIMGroupsServiceInterface
	baseURL string
}

func newSCIMGroupsHandler(svc SCIMGroupsServiceInterface, baseURL string) *scimGroupsHandler {
	return &scimGroupsHandler{svc: svc, baseURL: baseURL}
}

func (h *scimGroupsHandler) HandleGroupsListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.URL.Query().Get("filter") != "" {
		h.handleSCIMError(w, r, &ErrorFilterNotSupported)
		return
	}
	startIndex, count := 1, 20
	if v := r.URL.Query().Get("startIndex"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			startIndex = n
		}
	}
	if v := r.URL.Query().Get("count"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			count = n
		}
	}
	resp, svcErr := h.svc.ListGroups(ctx, startIndex, count, h.baseURL)
	if svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}
	writeSCIMSuccessResponse(ctx, w, http.StatusOK, resp)
}

func (h *scimGroupsHandler) HandleGroupsCreateRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if svcErr := validateSCIMContentType(r); svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		h.handleSCIMError(w, r, &ErrorInvalidRequestBody)
		return
	}

	var payload struct {
		Schemas     []string          `json:"schemas"`
		DisplayName string            `json:"displayName"`
		Members     []SCIMGroupMember `json:"members"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.DisplayName == "" {
		h.handleSCIMError(w, r, &ErrorInvalidRequestBody)
		return
	}
	hasGroupSchema := false
	for _, s := range payload.Schemas {
		if strings.EqualFold(s, SCIMCoreGroupSchemaURN) {
			hasGroupSchema = true
			break
		}
	}
	if !hasGroupSchema {
		h.handleSCIMError(w, r, &ErrorMissingSchemas)
		return
	}
	created, svcErr := h.svc.CreateGroup(ctx, payload.DisplayName, payload.Members, h.baseURL)
	if svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}
	w.Header().Set("Location", created.Meta.Location)
	w.Header().Set("ETag", created.Meta.Version)
	writeSCIMSuccessResponse(ctx, w, http.StatusCreated, created)
}

func (h *scimGroupsHandler) HandleGroupsGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	groupID := r.PathValue("id")
	if groupID == "" {
		h.handleSCIMError(w, r, &ErrorResourceNotFound)
		return
	}
	g, svcErr := h.svc.GetGroup(ctx, groupID, h.baseURL)
	if svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}
	w.Header().Set("ETag", g.Meta.Version)
	writeSCIMSuccessResponse(ctx, w, http.StatusOK, g)
}

func (h *scimGroupsHandler) HandleGroupsReplaceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	groupID := r.PathValue("id")
	if groupID == "" {
		h.handleSCIMError(w, r, &ErrorResourceNotFound)
		return
	}
	if svcErr := validateSCIMContentType(r); svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		h.handleSCIMError(w, r, &ErrorInvalidRequestBody)
		return
	}
	var payload struct {
		DisplayName string            `json:"displayName"`
		Members     []SCIMGroupMember `json:"members"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.DisplayName == "" {
		h.handleSCIMError(w, r, &ErrorInvalidRequestBody)
		return
	}
	replaced, svcErr := h.svc.ReplaceGroup(ctx, groupID, payload.DisplayName, payload.Members,
		r.Header.Get("If-Match"), h.baseURL)
	if svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}
	w.Header().Set("ETag", replaced.Meta.Version)
	writeSCIMSuccessResponse(ctx, w, http.StatusOK, replaced)
}

func (h *scimGroupsHandler) HandleGroupsPatchRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	groupID := r.PathValue("id")
	if groupID == "" {
		h.handleSCIMError(w, r, &ErrorResourceNotFound)
		return
	}
	if svcErr := validateSCIMContentType(r); svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		h.handleSCIMError(w, r, &ErrorInvalidRequestBody)
		return
	}
	actions, svcErr := ValidateSCIMGroupPatchRequest(body)
	if svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}
	patched, svcErr := h.svc.PatchGroup(ctx, groupID, actions, r.Header.Get("If-Match"), h.baseURL)
	if svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}
	w.Header().Set("ETag", patched.Meta.Version)
	writeSCIMSuccessResponse(ctx, w, http.StatusOK, patched)
}

func (h *scimGroupsHandler) HandleGroupsDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	groupID := r.PathValue("id")
	if groupID == "" {
		h.handleSCIMError(w, r, &ErrorResourceNotFound)
		return
	}
	svcErr := h.svc.DeleteGroup(ctx, groupID, r.Header.Get("If-Match"))
	if svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}
	writeSCIMSuccessResponse(ctx, w, http.StatusNoContent, nil)
}

func (h *scimGroupsHandler) handleSCIMError(w http.ResponseWriter, r *http.Request, svcErr *tidcommon.ServiceError) {
	ctx := r.Context()
	log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)).
		Debug(ctx, "SCIM Groups error", log.Any("error", svcErr))
	if svcErr.Type == tidcommon.ServerErrorType {
		writeSCIMErrorResponse(ctx, w, http.StatusInternalServerError, SCIMErrorResponse{
			Schemas: []string{SCIMErrorSchemaURN}, Status: "500", Detail: svcErr.ErrorDescription.DefaultValue,
		})
		return
	}
	httpStatus, scimType := mapSCIMError(svcErr)
	writeSCIMErrorResponse(ctx, w, httpStatus, SCIMErrorResponse{
		Schemas:  []string{SCIMErrorSchemaURN},
		Status:   fmt.Sprintf("%d", httpStatus),
		ScimType: scimType,
		Detail:   svcErr.ErrorDescription.DefaultValue,
	})
}
