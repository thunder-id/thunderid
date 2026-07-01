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

package openid4vci

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// Route paths for the OpenID4VCI issuer endpoints.
const (
	metadataPath        = "/.well-known/openid-credential-issuer"
	offerPath           = "/openid4vci/offer"
	credentialOfferPath = "/openid4vci/credential-offer" //nolint:gosec
	noncePath           = "/openid4vci/nonce"
	credentialPath      = "/openid4vci/credential" //nolint:gosec
)

// offerConfigParam is the query parameter naming the credential configuration to offer.
const offerConfigParam = "credential_configuration_id"

// authSchemes are the Authorization header schemes the credential endpoint accepts.
// DPoP-bound access tokens (RFC 9449) are presented with the "DPoP" scheme.
var authSchemes = []string{"Bearer ", "DPoP "}

// maxCredentialRequestBytes bounds the credential request body size.
const maxCredentialRequestBytes = 1 << 20

// openID4VCIHandler serves the OpenID4VCI issuer endpoints.
type openID4VCIHandler struct {
	service            OpenID4VCIServiceInterface
	dpopVerifier       dpop.VerifierInterface
	credentialEndpoint string
}

// newOpenID4VCIHandler creates a new openID4VCIHandler with the given service, DPoP verifier, and credential endpoint.
func newOpenID4VCIHandler(
	svc OpenID4VCIServiceInterface, dpopVerifier dpop.VerifierInterface, credentialEndpoint string,
) *openID4VCIHandler {
	return &openID4VCIHandler{service: svc, dpopVerifier: dpopVerifier, credentialEndpoint: credentialEndpoint}
}

// HandleMetadata returns the credential issuer metadata document.
func (h *openID4VCIHandler) HandleMetadata(w http.ResponseWriter, r *http.Request) {
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, h.service.GetMetadata(r.Context()))
}

// HandleOffer returns an issuer-initiated credential offer and its
// openid-credential-offer:// deep link for the requested credential configuration.
func (h *openID4VCIHandler) HandleOffer(w http.ResponseWriter, r *http.Request) {
	configID := sysutils.SanitizeString(r.URL.Query().Get(offerConfigParam))
	if configID == "" {
		writeOID4VCIError(w, toOID4VCIError(ErrInvalidRequest))
		return
	}
	offer, deepLink, err := h.service.GenerateCredentialOffer(r.Context(), configID)
	if err != nil {
		writeOID4VCIError(w, toOID4VCIError(err))
		return
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"credential_offer":     offer,
		"credential_offer_uri": deepLink,
	})
}

// HandleCredentialOffer returns a stored credential offer by id (the target of
// credential_offer_uri).
func (h *openID4VCIHandler) HandleCredentialOffer(w http.ResponseWriter, r *http.Request) {
	id := sysutils.SanitizeString(r.PathValue("id"))
	if id == "" {
		writeOID4VCIError(w, toOID4VCIError(ErrInvalidRequest))
		return
	}
	offer, err := h.service.GetCredentialOffer(r.Context(), id)
	if err != nil {
		writeOID4VCIError(w, toOID4VCIError(err))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, offer)
}

// HandleNonce issues a fresh c_nonce for holder proofs.
func (h *openID4VCIHandler) HandleNonce(w http.ResponseWriter, r *http.Request) {
	nonce, err := h.service.GenerateNonce(r.Context())
	if err != nil {
		log.GetLogger().Error(r.Context(), "Failed to issue c_nonce", log.Error(err))
		writeOID4VCIError(w, oid4vciError{
			Status: http.StatusInternalServerError, Code: errCodeServerError,
			Description: "Failed to issue c_nonce",
		})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, NonceResponse{CNonce: nonce})
}

// HandleCredential issues an SD-JWT VC for the bearer-authorized subject.
func (h *openID4VCIHandler) HandleCredential(w http.ResponseWriter, r *http.Request) {
	// Reject access tokens presented in the query string (RFC 6750 §2).
	if r.URL.Query().Has("access_token") {
		writeOID4VCIError(w, toOID4VCIError(ErrInvalidToken))
		return
	}
	token := bearerToken(r)
	if token == "" {
		writeOID4VCIError(w, toOID4VCIError(ErrInvalidToken))
		return
	}
	if err := h.verifyDPoP(r, token); err != nil {
		writeOID4VCIError(w, toOID4VCIError(err))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxCredentialRequestBytes))
	if err != nil {
		writeOID4VCIError(w, toOID4VCIError(ErrInvalidRequest))
		return
	}

	resp, err := h.service.IssueCredential(r.Context(), token, body)
	if err != nil {
		e := toOID4VCIError(err)
		if e.Code == errCodeInvalidProof || e.Code == errCodeInvalidNonce {
			if nonce, nonceErr := h.service.GenerateNonce(r.Context()); nonceErr == nil {
				e.CNonce = nonce
				e.CNonceExpiresIn = int64(defaultNonceTTL.Seconds())
			}
		}
		writeOID4VCIError(w, e)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, resp)
}

// bearerToken extracts the access token from the Authorization header, accepting
// both the Bearer and DPoP schemes.
func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	for _, scheme := range authSchemes {
		if len(auth) > len(scheme) && strings.EqualFold(auth[:len(scheme)], scheme) {
			return strings.TrimSpace(auth[len(scheme):])
		}
	}
	return ""
}

// verifyDPoP enforces the DPoP proof (RFC 9449 §7) when the access token is
// sender-constrained via cnf.jkt. Bearer (unbound) tokens are left untouched.
func (h *openID4VCIHandler) verifyDPoP(r *http.Request, token string) error {
	claims, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		return ErrInvalidToken
	}
	cnfJkt, err := dpop.ExtractCnfJkt(claims)
	if err != nil {
		return ErrInvalidToken
	}
	if cnfJkt == "" {
		return nil
	}
	proof := r.Header.Get("DPoP")
	if h.dpopVerifier == nil || proof == "" {
		return ErrInvalidDPoP
	}
	if _, err := h.dpopVerifier.Verify(r.Context(), dpop.VerifyParams{
		Proof:       proof,
		HTM:         r.Method,
		HTU:         h.credentialEndpoint,
		AccessToken: token,
		ExpectedJkt: cnfJkt,
	}); err != nil {
		return ErrInvalidDPoP
	}
	return nil
}

// writeOID4VCIError writes the OpenID4VCI error body, adding WWW-Authenticate on 401.
func writeOID4VCIError(w http.ResponseWriter, e oid4vciError) {
	if e.Status == http.StatusUnauthorized {
		scheme := "Bearer"
		if e.Code == errCodeInvalidDPoPProof {
			scheme = "DPoP"
		}
		w.Header().Set("WWW-Authenticate",
			scheme+` error="`+e.Code+`", error_description="`+e.Description+`"`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(e)
}
