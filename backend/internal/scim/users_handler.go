package scim

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/database/utils"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const loggerComponentName = "scim"

var (
	scimFilterQuotedStringRe = regexp.MustCompile(`"([^"\\]|\\.)*"`)
	scimFilterEqRe           = regexp.MustCompile(`(?i)^((?:[A-Za-z0-9][A-Za-z0-9.\-_]*:)*)` +
		`([A-Za-z][A-Za-z0-9.\-_]*)\s+eq\s+(.+)$`)
)

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

	// Parse optional SCIM filter — only single "eq" expressions are supported.
	var parsedFilters map[string]interface{}
	if filterStr := r.URL.Query().Get("filter"); filterStr != "" {
		var err error
		parsedFilters, err = parseSCIMFilterForEq(filterStr)
		if err != nil {
			writeSCIMErrorResponse(ctx, w, http.StatusBadRequest, SCIMErrorResponse{
				Schemas:  []string{SCIMErrorSchemaURN},
				Status:   "400",
				ScimType: "invalidFilter",
				Detail:   err.Error(),
			})
			return
		}
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
	listResp, svcErr := h.svc.ListUsers(ctx, startIndex, count, parsedFilters, h.baseURL)
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

// parseSCIMFilterForEq parses a SCIM filter string that contains exactly one
// "eq" comparison and no logical operators, grouping, or square brackets.
// Returns a native filter map suitable for userService.GetUserList, or an
// error if the expression uses any unsupported syntax.
func parseSCIMFilterForEq(filterStr string) (map[string]interface{}, error) {
	filterStr = strings.TrimSpace(filterStr)
	if filterStr == "" {
		return nil, nil
	}
	sanitized := scimFilterQuotedStringRe.ReplaceAllString(filterStr, `""`)
	lower := strings.ToLower(sanitized)
	// Reject all compound/complex expressions up front (outside of quoted strings).
	if strings.Contains(lower, " and ") ||
		strings.Contains(lower, " or ") ||
		strings.HasPrefix(lower, "not ") ||
		strings.ContainsAny(sanitized, "()[]") {
		return nil, fmt.Errorf(
			"compound filter expressions are not supported; only a single 'eq' expression is supported",
		)
	}
	// Reject any operator that is not "eq".
	// These keywords may appear as part of an unsupported operator.
	unsupportedOps := []string{" ne ", " co ", " sw ", " ew ", " pr", " gt ", " lt ", " ge ", " le "}
	for _, op := range unsupportedOps {
		if strings.Contains(lower, op) {
			// Extract the actual operator token for the error message.
			return nil, fmt.Errorf(
				"the specified filter operator is not supported; only 'eq' is supported",
			)
		}
	}
	// Match: [optional-URN-prefix:]attrPath eq compValue
	// attrPath allows alphanumeric, underscore, hyphen, and dot (for sub-attributes).
	// compValue is a quoted string, a boolean literal, or a number.
	matches := scimFilterEqRe.FindStringSubmatch(filterStr)
	if len(matches) == 0 {
		return nil, fmt.Errorf(
			"invalid filter expression; expected format: 'attrPath eq value'",
		)
	}
	// matches[1] = optional URN prefix (e.g. "urn:thunderid:params:scim:schemas:employee:2.0:User:")
	// matches[2] = attribute path (e.g. "profile.manager.id")
	// matches[3] = raw comparison value
	if isUnsupportedSCIMFilterAttr(matches[2]) {
		return nil, fmt.Errorf("filtering on %q is not supported", matches[2])
	}
	attribute := translateSCIMFilterAttr(matches[2])
	if err := utils.ValidateKey(attribute); err != nil {
		return nil, fmt.Errorf("filtering on %q is not supported", matches[2])
	}
	rawValue := strings.TrimSpace(matches[3])
	value, err := parseSCIMCompValue(rawValue)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{attribute: value}, nil
}

// parseSCIMCompValue converts a raw SCIM compValue token into a typed Go value.
// compValue = false / null / true / number / string  (RFC 7159 JSON rules)
func parseSCIMCompValue(raw string) (interface{}, error) {
	// Quoted string — parse as a JSON string literal so escapes are handled correctly.
	if len(raw) > 0 && raw[0] == '"' {
		s, err := strconv.Unquote(raw)
		if err == nil {
			return s, nil
		}
		return nil, fmt.Errorf("invalid quoted string comparison value: %q", raw)
	}
	lower := strings.ToLower(raw)
	switch lower {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		// null comparisons are not meaningful for our store.
		return nil, fmt.Errorf("null comparison values are not supported")
	}
	// Integer
	if intVal, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return intVal, nil
	}
	// Decimal
	if floatVal, err := strconv.ParseFloat(raw, 64); err == nil {
		return floatVal, nil
	}
	return nil, fmt.Errorf("unrecognized comparison value: %q", raw)
}
