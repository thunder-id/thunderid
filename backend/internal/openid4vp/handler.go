/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package openid4vp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const requestObjectContentType = "application/oauth-authz-req+jwt"

// Route paths for the OpenID4VP wallet- and RP-facing endpoints.
const (
	requestURIPath      = "/openid4vp/request"
	responseURIPath     = "/openid4vp/response"
	apiInitiatePath     = "/openid4vp/initiate"
	apiStatusPath       = "/openid4vp/status/{txn_id}"
	apiStatusPrefix     = "/openid4vp/status/"
	apiTrustAnchorsPath = "/openid4vp/trust-anchors"
)

const defaultResultTokenValidity = 300 * time.Second

// openID4VPHandler serves both the wallet-facing and RP-facing OpenID4VP endpoints.
type openID4VPHandler struct {
	service              OpenID4VPServiceInterface
	issuer               resultTokenIssuer
	rpStatusBase         string
	resultTokenValidity  time.Duration
	requestStateValidity time.Duration
	logger               *log.Logger
}

// newOpenID4VPHandler builds the handler. Zero resultTokenValidity falls back to
// defaultResultTokenValidity; zero requestStateValidity falls back to defaultStateTTL.
// A nil issuer disables COMPLETED result-token issuance — wallet endpoints continue to work.
func newOpenID4VPHandler(
	svc OpenID4VPServiceInterface,
	issuer resultTokenIssuer,
	baseURL string,
	resultTokenValidity, requestStateValidity time.Duration,
) *openID4VPHandler {
	if resultTokenValidity <= 0 {
		resultTokenValidity = defaultResultTokenValidity
	}
	if requestStateValidity <= 0 {
		requestStateValidity = defaultStateTTL
	}
	return &openID4VPHandler{
		service:              svc,
		issuer:               issuer,
		rpStatusBase:         strings.TrimRight(baseURL, "/") + apiStatusPrefix,
		resultTokenValidity:  resultTokenValidity,
		requestStateValidity: requestStateValidity,
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OpenID4VPHandler")),
	}
}

// HandleRequestObject returns the signed authorization request JWT to the wallet.
func (h *openID4VPHandler) HandleRequestObject(w http.ResponseWriter, r *http.Request) {
	state := sysutils.SanitizeString(r.URL.Query().Get("state"))
	if state == "" {
		writeServiceErrorResponse(r.Context(), w, &ErrorInvalidRequest)
		return
	}

	jar, svcErr := h.service.GetRequestObject(r.Context(), state)
	if svcErr != nil {
		writeServiceErrorResponse(r.Context(), w, svcErr)
		return
	}

	w.Header().Set("Content-Type", requestObjectContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, werr := w.Write([]byte(jar)); werr != nil {
		h.logger.Error(r.Context(), "Failed to write request object response", log.Error(werr))
	}
}

// HandleResponse ingests the wallet's encrypted VP response.
func (h *openID4VPHandler) HandleResponse(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeServiceErrorResponse(r.Context(), w, &ErrorInvalidRequest)
		return
	}

	state := sysutils.SanitizeString(r.FormValue("state"))
	if state == "" {
		writeServiceErrorResponse(r.Context(), w, &ErrorInvalidRequest)
		return
	}

	// A wallet may post an OAuth error to the response_uri instead of a vp_token (OpenID4VP §6.4).
	if errCode := sysutils.SanitizeString(r.FormValue("error")); errCode != "" {
		if svcErr := h.service.SubmitError(r.Context(), state, errCode,
			r.FormValue("error_description")); svcErr != nil {
			writeServiceErrorResponse(r.Context(), w, svcErr)
			return
		}
		sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, map[string]string{})
		return
	}

	response := r.FormValue("response")
	if response == "" {
		writeServiceErrorResponse(r.Context(), w, &ErrorInvalidRequest)
		return
	}

	if _, svcErr := h.service.SubmitResponse(r.Context(), state, []byte(response)); svcErr != nil {
		writeServiceErrorResponse(r.Context(), w, svcErr)
		return
	}

	body := map[string]string{}
	if redirect := h.service.GetResultRedirectURI(state); redirect != "" {
		body["redirect_uri"] = redirect
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, body)
}

// HandleInitiate starts a verifier transaction on behalf of an RP.
func (h *openID4VPHandler) HandleInitiate(w http.ResponseWriter, r *http.Request) {
	var req initiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeServiceErrorResponse(r.Context(), w, &ErrorInvalidRequest)
		return
	}
	if strings.TrimSpace(req.DefinitionID) == "" || strings.TrimSpace(req.RPID) == "" {
		writeServiceErrorResponse(r.Context(), w, &ErrorInvalidRequest)
		return
	}
	init, svcErr := h.service.InitiateForRP(r.Context(), req.DefinitionID, req.RPID)
	if svcErr != nil {
		if svcErr.Type != tidcommon.ClientErrorType {
			h.logger.Error(r.Context(), "Failed to initiate OpenID4VP transaction",
				log.String("errorCode", svcErr.Code))
		}
		writeServiceErrorResponse(r.Context(), w, svcErr)
		return
	}

	rs, lookupErr := h.service.LookupState(r.Context(), init.State)
	expiresAt := time.Now().Add(h.requestStateValidity)
	if lookupErr == nil && rs != nil {
		expiresAt = rs.ExpiresAt
	}

	resp := initiateResponse{
		TxnID:     init.State,
		WalletURL: WalletAuthorizationURI(init.ClientID, init.RequestURI),
		StatusURL: h.rpStatusBase + init.State,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, resp)
}

// HandleStatus issues a result token on COMPLETED; FAILED/EXPIRED carry a diagnostic but no token.
func (h *openID4VPHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	txnID := strings.TrimSpace(extractTxnID(r))
	if txnID == "" {
		writeServiceErrorResponse(r.Context(), w, &ErrorInvalidRequest)
		return
	}

	rs, svcErr := h.service.LookupState(r.Context(), txnID)
	switch {
	case svcErr == nil:
	case svcErr.Code == ErrorUnknownState.Code:
		writeServiceErrorResponse(r.Context(), w, &ErrorUnknownState)
		return
	case svcErr.Code == ErrorExpiredState.Code:
		sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, statusResponse{Status: "EXPIRED"})
		return
	default:
		writeServiceErrorResponse(r.Context(), w, svcErr)
		return
	}

	switch rs.Status {
	case StatusPending:
		sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, statusResponse{Status: "PENDING"})
	case StatusFailed:
		sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, statusResponse{
			Status: "FAILED",
			Error:  rs.FailureReason,
		})
	case StatusCompleted:
		if h.issuer == nil {
			h.logger.Error(r.Context(), "Result token issuer not configured")
			writeServiceErrorResponse(r.Context(), w, &tidcommon.InternalServerError)
			return
		}
		rpID := rs.RPID
		if rpID == "" {
			rpID = rs.ClientID
		}
		token, tokenErr := h.issuer.issueResultToken(
			r.Context(), rpID, rs, int64(h.resultTokenValidity.Seconds()))
		if tokenErr != nil {
			h.logger.Error(r.Context(), "Failed to issue result token", log.Error(tokenErr))
			writeServiceErrorResponse(r.Context(), w, &tidcommon.InternalServerError)
			return
		}
		sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, statusResponse{
			Status:      "COMPLETED",
			ResultToken: token,
		})
	default:
		writeServiceErrorResponse(r.Context(), w, &tidcommon.InternalServerError)
	}
}

// HandleTrustAnchors returns the configured trust anchors (root CAs) as JSON.
func (h *openID4VPHandler) HandleTrustAnchors(w http.ResponseWriter, r *http.Request) {
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, h.service.GetTrustAnchors())
}

// extractTxnID resolves txn_id from a Go-1.22 path value or the trailing path segment.
func extractTxnID(r *http.Request) string {
	if v := r.PathValue("txn_id"); v != "" {
		return v
	}
	return strings.TrimPrefix(r.URL.Path, apiStatusPrefix)
}

// writeServiceErrorResponse writes a service error to the response with the appropriate HTTP status code.
func writeServiceErrorResponse(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		statusCode = clientErrorStatusCode(svcErr.Code)
	}
	sysutils.WriteErrorResponse(ctx, w, statusCode, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}

// clientErrorStatusCode maps a client error code to its HTTP status code.
func clientErrorStatusCode(code string) int {
	if code == ErrorUnknownState.Code {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}
