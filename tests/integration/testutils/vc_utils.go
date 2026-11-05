// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// OpenID4VCI/OpenID4VP test helpers: management-API seeding, wallet-facing HTTP
// calls, the holder proof, and a self-contained wallet simulator (SD-JWT VC +
// ECDH-ES encrypted response) built from stdlib.
package testutils

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// VC management APIs are platform-protected, so these helpers use the
// admin-authenticated GetHTTPClient.

// Base paths for the VC management APIs.
const (
	credentialConfigurationsEndpoint = "/openid4vci/credential-configurations"
	presentationDefinitionsEndpoint  = "/openid4vp/presentation-definitions"
)

// CreateCredentialConfiguration creates a credential configuration via API and
// returns the generated configuration ID.
func CreateCredentialConfiguration(cfg CredentialConfiguration) (string, error) {
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal credential configuration: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+credentialConfigurationsEndpoint, bytes.NewReader(cfgJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var created map[string]any
	if err := json.Unmarshal(bodyBytes, &created); err != nil {
		return "", fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	id, ok := created["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id or id is not a string. Response: %s", string(bodyBytes))
	}
	return id, nil
}

// DeleteCredentialConfiguration deletes a credential configuration by ID.
func DeleteCredentialConfiguration(id string) error {
	req, err := http.NewRequest("DELETE", TestServerURL+credentialConfigurationsEndpoint+"/"+id, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 200 or 204, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// CreatePresentationDefinition creates a presentation definition via API and
// returns the generated definition ID.
func CreatePresentationDefinition(def PresentationDefinition) (string, error) {
	defJSON, err := json.Marshal(def)
	if err != nil {
		return "", fmt.Errorf("failed to marshal presentation definition: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+presentationDefinitionsEndpoint, bytes.NewReader(defJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var created map[string]any
	if err := json.Unmarshal(bodyBytes, &created); err != nil {
		return "", fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	id, ok := created["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id or id is not a string. Response: %s", string(bodyBytes))
	}
	return id, nil
}

// UpdatePresentationDefinition replaces the presentation definition with the given ID via API.
func UpdatePresentationDefinition(id string, def PresentationDefinition) error {
	defJSON, err := json.Marshal(def)
	if err != nil {
		return fmt.Errorf("failed to marshal presentation definition: %w", err)
	}

	req, err := http.NewRequest("PUT", TestServerURL+presentationDefinitionsEndpoint+"/"+id, bytes.NewReader(defJSON))
	if err != nil {
		return fmt.Errorf("failed to create update request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to send update request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// DeletePresentationDefinition deletes a presentation definition by ID.
func DeletePresentationDefinition(id string) error {
	req, err := http.NewRequest("DELETE", TestServerURL+presentationDefinitionsEndpoint+"/"+id, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 200 or 204, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// OpenID4VCI wallet-facing endpoints are public, so these use the raw client;
// the credential endpoint carries the OAuth access token set by the caller.

// GetVCIIssuerMetadata fetches the credential issuer metadata document.
func GetVCIIssuerMetadata() (*VCHTTPResult, map[string]any, error) {
	req, err := http.NewRequest("GET", TestServerURL+"/.well-known/openid-credential-issuer", nil)
	if err != nil {
		return nil, nil, err
	}
	res, raw, err := doRawVCRequest(req)
	if err != nil {
		return nil, nil, err
	}
	var parsed map[string]any
	if res.StatusCode == http.StatusOK {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return res, nil, fmt.Errorf("parse metadata response: %w. Body: %s", err, string(raw))
		}
	}
	return res, parsed, nil
}

// GetVCICredentialOffer fetches an issuer-initiated credential offer for a
// credential configuration handle.
func GetVCICredentialOffer(configID string) (*VCHTTPResult, map[string]any, error) {
	u := TestServerURL + "/openid4vci/offer?credential_configuration_id=" + url.QueryEscape(configID)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}
	res, raw, err := doRawVCRequest(req)
	if err != nil {
		return nil, nil, err
	}
	var parsed map[string]any
	if res.StatusCode == http.StatusOK {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return res, nil, fmt.Errorf("parse offer response: %w. Body: %s", err, string(raw))
		}
	}
	return res, parsed, nil
}

// GetVCIStoredOffer fetches a stored credential offer by id (the target of
// credential_offer_uri).
func GetVCIStoredOffer(id string) (*VCHTTPResult, map[string]any, error) {
	req, err := http.NewRequest("GET", TestServerURL+"/openid4vci/credential-offer/"+id, nil)
	if err != nil {
		return nil, nil, err
	}
	res, raw, err := doRawVCRequest(req)
	if err != nil {
		return nil, nil, err
	}
	var parsed map[string]any
	if res.StatusCode == http.StatusOK {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return res, nil, fmt.Errorf("parse stored offer response: %w. Body: %s", err, string(raw))
		}
	}
	return res, parsed, nil
}

// RequestVCINonce requests a fresh c_nonce for holder proofs.
func RequestVCINonce() (*VCHTTPResult, string, error) {
	req, err := http.NewRequest("POST", TestServerURL+"/openid4vci/nonce", nil)
	if err != nil {
		return nil, "", err
	}
	res, raw, err := doRawVCRequest(req)
	if err != nil {
		return nil, "", err
	}
	var parsed struct {
		CNonce string `json:"c_nonce"`
	}
	if res.StatusCode == http.StatusOK {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return res, "", fmt.Errorf("parse nonce response: %w. Body: %s", err, string(raw))
		}
	}
	return res, parsed.CNonce, nil
}

// RequestVCICredential posts a credential request. tokenScheme is "Bearer" or
// "DPoP"; dpopProof (when non-empty) is sent in the DPoP header. body is the
// credential request payload (any JSON-serializable value).
func RequestVCICredential(tokenScheme, accessToken, dpopProof string, body any) (*VCHTTPResult, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal credential request: %w", err)
	}
	req, err := http.NewRequest("POST", TestServerURL+"/openid4vci/credential", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		if tokenScheme == "" {
			tokenScheme = "Bearer"
		}
		req.Header.Set("Authorization", tokenScheme+" "+accessToken)
	}
	if dpopProof != "" {
		req.Header.Set("DPoP", dpopProof)
	}
	res, _, err := doRawVCRequest(req)
	return res, err
}

// OpenID4VCI holder proof and shared crypto helpers.

// vciProofType is the required "typ" header of an OpenID4VCI holder proof JWT.
const vciProofType = "openid4vci-proof+jwt"

// VCIHolderProofOptions tweaks the holder proof JWT produced by
// CreateVCIHolderProof. Zero values keep the RFC-compliant defaults.
type VCIHolderProofOptions struct {
	// Typ overrides the "typ" header (default openid4vci-proof+jwt).
	Typ string
	// OmitTyp drops the "typ" header entirely.
	OmitTyp bool
	// OmitJWK drops the embedded "jwk" header.
	OmitJWK bool
	// Aud overrides the "aud" claim (default: the supplied issuer).
	Aud string
	// OmitNonce drops the "nonce" claim.
	OmitNonce bool
	// Nonce overrides the "nonce" claim (default: the supplied nonce).
	Nonce string
	// IatOffset shifts the "iat" claim by the given seconds (negative = stale).
	IatOffset int64
}

// CreateVCIHolderProof mints an OpenID4VCI holder proof JWT bound to key. The
// proof carries the embedded holder JWK, aud=credentialIssuer and the supplied
// c_nonce, signed with the key's algorithm (ES256).
func CreateVCIHolderProof(key *DPoPKey, credentialIssuer, nonce string, opts VCIHolderProofOptions) (string, error) {
	header := map[string]any{
		"typ": vciProofType,
		"alg": key.Alg,
		"jwk": key.JWK,
	}
	if opts.Typ != "" {
		header["typ"] = opts.Typ
	}
	if opts.OmitTyp {
		delete(header, "typ")
	}
	if opts.OmitJWK {
		delete(header, "jwk")
	}

	aud := credentialIssuer
	if opts.Aud != "" {
		aud = opts.Aud
	}
	payload := map[string]any{
		"aud": aud,
		"iat": time.Now().Unix() + opts.IatOffset,
	}
	nonceVal := nonce
	if opts.Nonce != "" {
		nonceVal = opts.Nonce
	}
	if !opts.OmitNonce {
		payload["nonce"] = nonceVal
	}

	return signCompactJWS(header, payload, key.Private, key.Alg)
}

// signCompactJWS serializes header and payload, signs the signing input with the
// given signer and algorithm, and returns the compact JWS. ECDSA signatures use
// the fixed-length P1363 (r||s) form the server's verifier expects.
func signCompactJWS(header, payload map[string]any, priv crypto.Signer, alg string) (string, error) {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON)

	sig, err := signProof(priv, alg, signingInput)
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// ecPublicKeyFromJWK reconstructs an ECDSA P-256 public key from a JWK map with
// base64url x and y coordinates.
func ecPublicKeyFromJWK(jwk map[string]any) (*ecdsa.PublicKey, error) {
	xStr, _ := jwk["x"].(string)
	yStr, _ := jwk["y"].(string)
	if xStr == "" || yStr == "" {
		return nil, fmt.Errorf("jwk missing x or y coordinate")
	}
	xb, err := base64.RawURLEncoding.DecodeString(xStr)
	if err != nil {
		return nil, fmt.Errorf("decode x: %w", err)
	}
	yb, err := base64.RawURLEncoding.DecodeString(yStr)
	if err != nil {
		return nil, fmt.Errorf("decode y: %w", err)
	}
	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(xb),
		Y:     new(big.Int).SetBytes(yb),
	}, nil
}

// ecdhConcatKDF implements the Concat KDF (RFC 7518 §4.6.2) using SHA-256,
// mirroring the server's cryptolib derivation so a JWE encrypted here decrypts
// with the same CEK. algID is the direct content-encryption algorithm (e.g.
// "A128GCM"); apu/apv are nil for the direct ECDH-ES flow.
func ecdhConcatKDF(z []byte, algID string, keyLen int, apu, apv []byte) []byte {
	suppPubInfo := make([]byte, 4)
	binary.BigEndian.PutUint32(suppPubInfo, uint32(keyLen*8))

	otherInfo := lengthPrefixed([]byte(algID))
	otherInfo = append(otherInfo, lengthPrefixed(apu)...)
	otherInfo = append(otherInfo, lengthPrefixed(apv)...)
	otherInfo = append(otherInfo, suppPubInfo...)

	key := make([]byte, 0, keyLen)
	for counter := uint32(1); len(key) < keyLen; counter++ {
		h := sha256.New()
		counterBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(counterBuf, counter)
		h.Write(counterBuf)
		h.Write(z)
		h.Write(otherInfo)
		key = append(key, h.Sum(nil)...)
	}
	return key[:keyLen]
}

// lengthPrefixed returns data prefixed with its 4-byte big-endian length.
func lengthPrefixed(data []byte) []byte {
	res := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(res, uint32(len(data)))
	copy(res[4:], data)
	return res
}

// OpenID4VP wallet simulator.

// vpWalletIssuer is the "iss" claim placed in the self-signed SD-JWT VC. Issuer
// trust is established by pinning the wallet's issuer certificate on a definition,
// so any non-empty issuer is accepted.
const vpWalletIssuer = "https://integration-test-issuer.thunderid.local"

// VPWallet is a self-contained OpenID4VP wallet. It owns a self-signed issuer
// key/cert (placed in the SD-JWT x5c header) and a holder key (bound via cnf and
// used to sign the Key Binding JWT).
type VPWallet struct {
	issuerKey     *ecdsa.PrivateKey
	issuerCertDER []byte
	holderKey     *DPoPKey
}

// NewVPWallet builds a wallet with a fresh self-signed ES256 issuer certificate
// and a fresh ES256 holder key.
func NewVPWallet() (*VPWallet, error) {
	issuerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate issuer key: %w", err)
	}
	ski := make([]byte, 20)
	if _, err := rand.Read(ski); err != nil {
		return nil, fmt.Errorf("generate issuer key id: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ThunderID Integration Test Issuer"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		SubjectKeyId:          ski,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &issuerKey.PublicKey, issuerKey)
	if err != nil {
		return nil, fmt.Errorf("create issuer certificate: %w", err)
	}
	holderKey, err := GenerateDPoPKey("ES256")
	if err != nil {
		return nil, fmt.Errorf("generate holder key: %w", err)
	}
	return &VPWallet{issuerKey: issuerKey, issuerCertDER: der, holderKey: holderKey}, nil
}

// HolderKey exposes the wallet's holder key (for tests that need its JWK).
func (w *VPWallet) HolderKey() *DPoPKey { return w.holderKey }

// IssuerCertificatePEM returns the wallet's issuer certificate as PEM, for pinning
// as a trusted authority on a presentation definition.
func (w *VPWallet) IssuerCertificatePEM() string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: w.issuerCertDER}))
}

// IssuerKeyID returns the base64url SubjectKeyId of the wallet's issuer certificate,
// the value a verifier advertises as a DCQL "aki" trusted authority.
func (w *VPWallet) IssuerKeyID() (string, error) {
	cert, err := x509.ParseCertificate(w.issuerCertDER)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(cert.SubjectKeyId), nil
}

// NewSelfSignedCertificatePEM mints a self-signed ES256 CA certificate valid until
// notAfter and returns it PEM-encoded.
func NewSelfSignedCertificatePEM(commonName string, notAfter time.Time) (string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return "", fmt.Errorf("generate serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             notAfter.Add(-48 * time.Hour),
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", fmt.Errorf("create certificate: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), nil
}

// VPRequestObject is the decoded authorization request the wallet fetches from
// /openid4vp/request. Only the fields a wallet needs are surfaced.
type VPRequestObject struct {
	ClientID     string
	Nonce        string
	State        string
	ResponseURI  string
	CredentialID string
	VCT          string
	EncKey       *ecdsa.PublicKey
	EncAlg       string
	Raw          map[string]any
}

// ParseVPRequestObject decodes the compact request-object JWT without verifying
// its signature (the wallet trusts the transport) and extracts the fields needed
// to build and encrypt a response.
func ParseVPRequestObject(jar string) (*VPRequestObject, error) {
	parts := strings.Split(jar, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("request object is not a compact JWS (got %d segments)", len(parts))
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode request object payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal request object payload: %w", err)
	}

	req := &VPRequestObject{
		ClientID:    stringClaim(claims, "client_id"),
		Nonce:       stringClaim(claims, "nonce"),
		State:       stringClaim(claims, "state"),
		ResponseURI: stringClaim(claims, "response_uri"),
		EncAlg:      "A128GCM",
		Raw:         claims,
	}

	// dcql_query.credentials[0]: id (vp_token key) + meta.vct_values[0] (VCT).
	if dcql, ok := claims["dcql_query"].(map[string]any); ok {
		if creds, ok := dcql["credentials"].([]any); ok && len(creds) > 0 {
			if cred, ok := creds[0].(map[string]any); ok {
				req.CredentialID = stringClaim(cred, "id")
				if meta, ok := cred["meta"].(map[string]any); ok {
					if vcts, ok := meta["vct_values"].([]any); ok && len(vcts) > 0 {
						req.VCT, _ = vcts[0].(string)
					}
				}
			}
		}
	}

	// client_metadata: ephemeral encryption key + supported enc values.
	if cm, ok := claims["client_metadata"].(map[string]any); ok {
		if encVals, ok := cm["encrypted_response_enc_values_supported"].([]any); ok && len(encVals) > 0 {
			if enc, ok := encVals[0].(string); ok && enc != "" {
				req.EncAlg = enc
			}
		}
		if jwks, ok := cm["jwks"].(map[string]any); ok {
			if keys, ok := jwks["keys"].([]any); ok && len(keys) > 0 {
				if jwk, ok := keys[0].(map[string]any); ok {
					encKey, err := ecPublicKeyFromJWK(jwk)
					if err != nil {
						return nil, fmt.Errorf("parse verifier encryption key: %w", err)
					}
					req.EncKey = encKey
				}
			}
		}
	}

	if req.CredentialID == "" || req.VCT == "" || req.EncKey == nil {
		return nil, fmt.Errorf("request object missing credential id, vct, or encryption key")
	}
	return req, nil
}

// BuildPresentation issues a self-signed SD-JWT VC disclosing the given claims,
// binds it to the wallet's holder key, appends a Key Binding JWT over the
// request nonce and audience, and returns the combined presentation string.
func (w *VPWallet) BuildPresentation(req *VPRequestObject, claims map[string]any) (string, error) {
	now := time.Now()

	type disclosure struct{ raw, digest string }
	disclosures := make([]disclosure, 0, len(claims))
	digests := make([]string, 0, len(claims))
	for name, value := range claims {
		saltBytes := make([]byte, 16)
		if _, err := rand.Read(saltBytes); err != nil {
			return "", fmt.Errorf("generate salt: %w", err)
		}
		salt := base64.RawURLEncoding.EncodeToString(saltBytes)
		encoded, err := json.Marshal([]any{salt, name, value})
		if err != nil {
			return "", fmt.Errorf("marshal disclosure %q: %w", name, err)
		}
		raw := base64.RawURLEncoding.EncodeToString(encoded)
		sum := sha256.Sum256([]byte(raw))
		digest := base64.RawURLEncoding.EncodeToString(sum[:])
		disclosures = append(disclosures, disclosure{raw: raw, digest: digest})
		digests = append(digests, digest)
	}
	sort.Strings(digests)

	header := map[string]any{
		"alg": "ES256",
		"typ": "dc+sd-jwt",
		"x5c": []string{base64.StdEncoding.EncodeToString(w.issuerCertDER)},
	}
	payload := map[string]any{
		"iss":     vpWalletIssuer,
		"vct":     req.VCT,
		"iat":     now.Unix(),
		"exp":     now.Add(time.Hour).Unix(),
		"cnf":     map[string]any{"jwk": w.holderKey.JWK},
		"_sd":     digests,
		"_sd_alg": "sha-256",
	}
	issuerJWT, err := signCompactJWS(header, payload, w.issuerKey, "ES256")
	if err != nil {
		return "", fmt.Errorf("sign issuer JWT: %w", err)
	}

	combined := issuerJWT
	for _, d := range disclosures {
		combined += "~" + d.raw
	}
	combined += "~"

	sdHashSum := sha256.Sum256([]byte(combined))
	kbHeader := map[string]any{"alg": "ES256", "typ": "kb+jwt"}
	kbPayload := map[string]any{
		"aud":     req.ClientID,
		"nonce":   req.Nonce,
		"iat":     now.Unix(),
		"sd_hash": base64.RawURLEncoding.EncodeToString(sdHashSum[:]),
	}
	kbJWT, err := signCompactJWS(kbHeader, kbPayload, w.holderKey.Private, "ES256")
	if err != nil {
		return "", fmt.Errorf("sign key binding JWT: %w", err)
	}

	return combined + kbJWT, nil
}

// EncryptResponse builds the {state, vp_token} authorization response and
// ECDH-ES/A128GCM encrypts it to the verifier's ephemeral key, returning the
// compact JWE for the `response` form field.
func (w *VPWallet) EncryptResponse(req *VPRequestObject, presentation string) (string, error) {
	responseObj := map[string]any{
		"state": req.State,
		"vp_token": map[string]any{
			req.CredentialID: []string{presentation},
		},
	}
	plaintext, err := json.Marshal(responseObj)
	if err != nil {
		return "", fmt.Errorf("marshal authorization response: %w", err)
	}

	recipient, err := req.EncKey.ECDH()
	if err != nil {
		return "", fmt.Errorf("convert recipient key: %w", err)
	}
	ephPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("generate ephemeral key: %w", err)
	}
	z, err := ephPriv.ECDH(recipient)
	if err != nil {
		return "", fmt.Errorf("ecdh agreement: %w", err)
	}
	cek := ecdhConcatKDF(z, "A128GCM", 16, nil, nil)

	ephBytes := ephPriv.PublicKey().Bytes() // 0x04 || x || y
	header := map[string]any{
		"alg": "ECDH-ES",
		"enc": "A128GCM",
		"epk": map[string]any{
			"kty": "EC",
			"crv": "P-256",
			"x":   base64.RawURLEncoding.EncodeToString(ephBytes[1:33]),
			"y":   base64.RawURLEncoding.EncodeToString(ephBytes[33:65]),
		},
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal JWE header: %w", err)
	}
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	block, err := aes.NewCipher(cek)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return "", fmt.Errorf("generate iv: %w", err)
	}
	sealed := gcm.Seal(nil, iv, plaintext, []byte(headerB64))
	ciphertext := sealed[:len(sealed)-16]
	tag := sealed[len(sealed)-16:]

	return strings.Join([]string{
		headerB64,
		"", // empty encrypted key for direct ECDH-ES
		base64.RawURLEncoding.EncodeToString(iv),
		base64.RawURLEncoding.EncodeToString(ciphertext),
		base64.RawURLEncoding.EncodeToString(tag),
	}, "."), nil
}

// OpenID4VP wallet/verifier endpoints are public, so these use the raw client.

// InitiateVP starts a VP verification session for the given definition handle.
func InitiateVP(definitionID string) (*VCHTTPResult, *VPInitiateResponse, error) {
	body, _ := json.Marshal(map[string]string{"definition_id": definitionID})
	req, err := http.NewRequest("POST", TestServerURL+"/openid4vp/initiate", bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, raw, err := doRawVCRequest(req)
	if err != nil {
		return nil, nil, err
	}
	var parsed VPInitiateResponse
	if res.StatusCode == http.StatusOK {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return res, nil, fmt.Errorf("parse initiate response: %w. Body: %s", err, string(raw))
		}
	}
	return res, &parsed, nil
}

// FetchVPRequestObject retrieves the signed request object (JAR) for a state.
func FetchVPRequestObject(state string) (*VCHTTPResult, string, string, error) {
	u := TestServerURL + "/openid4vp/request?state=" + url.QueryEscape(state)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, "", "", err
	}
	client := GetRawHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return &VCHTTPResult{StatusCode: resp.StatusCode, Body: raw},
		string(raw), resp.Header.Get("Content-Type"), nil
}

// SubmitVPResponse posts an encrypted VP response for a state.
func SubmitVPResponse(state, response string) (*VCHTTPResult, error) {
	form := url.Values{}
	form.Set("state", state)
	form.Set("response", response)

	req, err := http.NewRequest("POST", TestServerURL+"/openid4vp/response", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, _, err := doRawVCRequest(req)
	return res, err
}

// SubmitVPError posts a wallet error to the response endpoint for a state.
func SubmitVPError(state, errCode, errDescription string) (*VCHTTPResult, error) {
	form := url.Values{}
	form.Set("state", state)
	form.Set("error", errCode)
	if errDescription != "" {
		form.Set("error_description", errDescription)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/openid4vp/response", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, _, err := doRawVCRequest(req)
	return res, err
}

// GetVPStatus polls a verification session by transaction id.
func GetVPStatus(txnID string) (*VCHTTPResult, *VPStatusResponse, error) {
	req, err := http.NewRequest("GET", TestServerURL+"/openid4vp/status/"+txnID, nil)
	if err != nil {
		return nil, nil, err
	}
	res, raw, err := doRawVCRequest(req)
	if err != nil {
		return nil, nil, err
	}
	var parsed VPStatusResponse
	if res.StatusCode == http.StatusOK {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return res, nil, fmt.Errorf("parse status response: %w. Body: %s", err, string(raw))
		}
	}
	return res, &parsed, nil
}

// PollVPStatusUntilTerminal polls the status endpoint until the session reaches
// a terminal state (not PENDING) or the attempts are exhausted.
func PollVPStatusUntilTerminal(txnID string, attempts int, interval time.Duration) (*VPStatusResponse, error) {
	var last *VPStatusResponse
	for i := 0; i < attempts; i++ {
		res, parsed, err := GetVPStatus(txnID)
		if err != nil {
			return nil, err
		}
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("status endpoint returned %d: %s", res.StatusCode, string(res.Body))
		}
		last = parsed
		if parsed.Status != "PENDING" {
			return parsed, nil
		}
		time.Sleep(interval)
	}
	return last, nil
}

// VPVerificationResult is the outcome of a full OpenID4VP verification run by RunVPVerification.
type VPVerificationResult struct {
	// Request is the request object the wallet received.
	Request *VPRequestObject
	// SubmitStatusCode is the HTTP status of the wallet's response submission.
	SubmitStatusCode int
	// Status is the terminal status of the verification session.
	Status *VPStatusResponse
}

// RunVPVerification drives a verification for definitionID end to end: it initiates a session,
// fetches the request object, submits the wallet's encrypted presentation of claims, and polls
// the session to a terminal status.
func RunVPVerification(definitionID string, wallet *VPWallet, claims map[string]any) (*VPVerificationResult, error) {
	res, init, err := InitiateVP(definitionID)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("initiate returned %d: %s", res.StatusCode, string(res.Body))
	}
	_, jar, _, err := FetchVPRequestObject(init.TxnID)
	if err != nil {
		return nil, err
	}
	req, err := ParseVPRequestObject(jar)
	if err != nil {
		return nil, err
	}
	presentation, err := wallet.BuildPresentation(req, claims)
	if err != nil {
		return nil, err
	}
	response, err := wallet.EncryptResponse(req, presentation)
	if err != nil {
		return nil, err
	}
	submitRes, err := SubmitVPResponse(init.TxnID, response)
	if err != nil {
		return nil, err
	}
	status, err := PollVPStatusUntilTerminal(init.TxnID, 10, 200*time.Millisecond)
	if err != nil {
		return nil, err
	}
	return &VPVerificationResult{Request: req, SubmitStatusCode: submitRes.StatusCode, Status: status}, nil
}

// Shared low-level helpers.

// doRawVCRequest executes an HTTP request with the raw (unauthenticated) client
// and returns the status/body.
func doRawVCRequest(req *http.Request) (*VCHTTPResult, []byte, error) {
	client := GetRawHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return &VCHTTPResult{StatusCode: resp.StatusCode, Body: raw}, raw, nil
}

// stringClaim reads a string claim from a decoded JSON object.
func stringClaim(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}
