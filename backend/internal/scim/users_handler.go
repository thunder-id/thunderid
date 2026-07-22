package scim

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const loggerComponentName = "scim"

// scimUsersHandler handles all /scim/v2/Users HTTP requests.
type scimUsersHandler struct {
	svc     SCIMUsersServiceInterface
	baseURL string
}

// newSCIMUsersHandler creates a new scimUsersHandler.
func newSCIMUsersHandler(svc SCIMUsersServiceInterface, baseURL string) *scimUsersHandler {
	return &scimUsersHandler{svc: svc, baseURL: baseURL}
}

// HandleUsersListRequest handles GET /scim/v2/Users
func (h *scimUsersHandler) HandleUsersListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	// ?filter is not supported in this implementation.
	if r.URL.Query().Get("filter") != "" {
		h.handleSCIMError(w, r, &ErrorFilterNotSupported)
		return
	}
	startIndex := 1
	count := 20
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
	listResp, svcErr := h.svc.ListUsers(ctx, startIndex, count, h.baseURL)
	if svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}

	writeSCIMSuccessResponse(ctx, w, http.StatusOK, listResp)
	logger.Debug(ctx, "SCIM Users list sent", log.Int("totalResults", listResp.TotalResults))
}

// HandleUsersCreateRequest handles POST /scim/v2/Users
func (h *scimUsersHandler) HandleUsersCreateRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if svcErr := validateSCIMContentType(r); svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		h.handleSCIMError(w, r, &ErrorInvalidRequestBody)
		return
	}
	payload, svcErr := ValidateSCIMUserRequest(body)
	if svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}

	created, svcErr := h.svc.CreateUser(ctx, payload, h.baseURL)
	if svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}

	w.Header().Set("Location", created.Meta.Location)
	w.Header().Set("ETag", created.Meta.Version)
	writeSCIMSuccessResponse(ctx, w, http.StatusCreated, created)
	logger.Debug(ctx, "SCIM User created", log.String("userID", created.ID))
}

// HandleUsersGetRequest handles GET /scim/v2/Users/{id}
func (h *scimUsersHandler) HandleUsersGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	userID := r.PathValue("id")
	if userID == "" {
		h.handleSCIMError(w, r, &ErrorUserNotFound)
		return
	}
	scimUser, svcErr := h.svc.GetUser(ctx, userID, h.baseURL)
	if svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}
	w.Header().Set("ETag", scimUser.Meta.Version)
	writeSCIMSuccessResponse(ctx, w, http.StatusOK, scimUser)
	logger.Debug(ctx, "SCIM User GET sent", log.String("userID", userID))
}

// HandleUsersReplaceRequest handles PUT /scim/v2/Users/{id}
func (h *scimUsersHandler) HandleUsersReplaceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	userID := r.PathValue("id")

	if userID == "" {
		h.handleSCIMError(w, r, &ErrorUserNotFound)
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
	payload, svcErr := ValidateSCIMUserRequest(body)
	if svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}

	replaced, svcErr := h.svc.ReplaceUser(ctx, userID, payload, r.Header.Get("If-Match"), h.baseURL)
	if svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}
	w.Header().Set("ETag", replaced.Meta.Version)
	writeSCIMSuccessResponse(ctx, w, http.StatusOK, replaced)
	logger.Debug(ctx, "SCIM User replaced", log.String("userID", userID))
}

// HandleUsersDeleteRequest handles DELETE /scim/v2/Users/{id}
func (h *scimUsersHandler) HandleUsersDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	userID := r.PathValue("id")
	if userID == "" {
		h.handleSCIMError(w, r, &ErrorUserNotFound)
		return
	}
	svcErr := h.svc.DeleteUser(ctx, userID, r.Header.Get("If-Match"))
	if svcErr != nil {
		h.handleSCIMError(w, r, svcErr)
		return
	}
	writeSCIMSuccessResponse(ctx, w, http.StatusNoContent, nil)
	logger.Debug(ctx, "SCIM User deleted", log.String("userID", userID))
}

// handleSCIMError translates an internal ThunderID ServiceError into the
// SCIM-standard wire error response (RFC 7644 §3.12).
func (h *scimUsersHandler) handleSCIMError(w http.ResponseWriter, r *http.Request, svcErr *tidcommon.ServiceError) {
	ctx := r.Context()

	if svcErr.Type == tidcommon.ServerErrorType {
		writeSCIMErrorResponse(ctx, w, http.StatusInternalServerError, SCIMErrorResponse{
			Schemas: []string{SCIMErrorSchemaURN},
			Status:  "500",
			Detail:  svcErr.ErrorDescription.DefaultValue,
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
