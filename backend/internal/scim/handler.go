package scim

import (
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// scimHandler handles SCIM HTTP requests.
type scimHandler struct {
	// svc is named differently from the concrete *scimService type to avoid
	// confusion — this field holds the SCIMServiceInterface, not the struct.
	svc     SCIMServiceInterface
	baseURL string
}

// newSCIMHandler creates a new scimHandler instance.
func newSCIMHandler(svc SCIMServiceInterface, baseURL string) *scimHandler {
	return &scimHandler{
		svc:     svc,
		baseURL: baseURL,
	}
}

// HandleServiceProviderConfigGetRequest handles GET /scim/v2/ServiceProviderConfig.
func (sh *scimHandler) HandleServiceProviderConfigGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	config := sh.svc.GetServiceProviderConfig(ctx, sh.baseURL)
	writeSCIMSuccessResponse(ctx, w, http.StatusOK, config)

	logger.Debug(ctx, "SCIM ServiceProviderConfig GET response sent")
}

// HandleUnsupportedRequest returns a SCIM-standard 501 for unimplemented endpoints.
// Delegates to handleSCIMError so that all error paths go through the same translator.
func (sh *scimHandler) HandleUnsupportedRequest(w http.ResponseWriter, r *http.Request) {
	handleSCIMError(w, r, &ErrorUnsupportedOperation)
}

// handleSCIMError translates an internal ThunderID ServiceError into the
// SCIM-standard wire error response (RFC 7644 §3.12).
// Internal codes (SCIM-1001 etc.) are NEVER sent to the client.
func handleSCIMError(w http.ResponseWriter, r *http.Request, svcErr *serviceerror.ServiceError) {
	ctx := r.Context()

	// Server errors always map to 500 with no scimType.
	if svcErr.Type == serviceerror.ServerErrorType {
		writeSCIMErrorResponse(ctx, w, http.StatusInternalServerError, SCIMErrorResponse{
			Schemas: []string{SCIMErrorSchemaURN},
			Status:  "500",
			Detail:  "An unexpected error occurred",
		})
		return
	}

	// Map internal client error codes → SCIM standard HTTP status + scimType.
	var httpStatus int
	var scimType string

	switch svcErr.Code {

	// 400 invalidSyntax — body could not be parsed at all.
	case ErrorInvalidRequestBody.Code:
		httpStatus = http.StatusBadRequest
		scimType = "invalidSyntax"

	// 400 invalidValue — missing or malformed fields/schemas/URNs.
	case ErrorMissingSchemas.Code,
		ErrorDuplicateSchemas.Code,
		ErrorMissingCoreUserSchema.Code,
		ErrorMissingCustomSchema.Code,
		ErrorMultipleCustomSchemas.Code,
		ErrorInvalidCustomSchemaURN.Code,
		ErrorMissingCustomSchemaObject.Code,
		ErrorUnknownUserType.Code:
		httpStatus = http.StatusBadRequest
		scimType = "invalidValue"

	// 404 — resource not found.
	case ErrorUserNotFound.Code,
		ErrorSchemaNotFound.Code:
		httpStatus = http.StatusNotFound
		scimType = ""

	// 501 — unsupported operation.
	case ErrorUnsupportedOperation.Code:
		httpStatus = http.StatusNotImplemented
		scimType = "notImplemented"

	// 403 — authorization failure.
	case serviceerror.ErrorUnauthorized.Code:
		httpStatus = http.StatusForbidden
		scimType = ""

	default:
		httpStatus = http.StatusBadRequest
		scimType = "invalidValue"
	}

	writeSCIMErrorResponse(ctx, w, httpStatus, SCIMErrorResponse{
		Schemas:  []string{SCIMErrorSchemaURN},
		Status:   fmt.Sprintf("%d", httpStatus),
		ScimType: scimType,
		Detail:   svcErr.ErrorDescription.DefaultValue,
	})
}
