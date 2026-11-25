// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/system/jose/jws"
)

// agentCapabilities describes an agent's OAuth client the way the agent MCP tools describe it: how the
// agent proves who it is and the ways it acts for users.
type agentCapabilities struct {
	credential      agentCredential
	jwksURI         string
	keyIDs          []string
	actsForUsers    []actingMode
	redirectURIs    []string
	otherGrantTypes []string
}

// capabilityChange is what a tool call asks to change about how an agent proves who it is and acts for
// users. A nil credential, actsForUsers or redirectURIs leaves that part as it is.
type capabilityChange struct {
	credential   *agentCredential
	jwksURI      string
	jwks         map[string]any
	actsForUsers []actingMode
	redirectURIs []string
	// authFlowID is the authentication flow the agent uses after the change, which approval needs.
	authFlowID string
}

// capabilityTarget is the credential and ways of acting an agent ends up with after a change.
type capabilityTarget struct {
	credential   agentCredential
	actsForUsers []actingMode
	redirectURIs []string
}

// keySet is a JSON Web Key Set.
type keySet struct {
	Keys []map[string]any `json:"keys"`
}

// acts reports whether the target includes the given way of acting for users.
func (t capabilityTarget) acts(mode actingMode) bool {
	return slices.Contains(t.actsForUsers, mode)
}

// describeCapabilities reads an agent's OAuth settings as a credential and the ways it acts for users.
// An agent without OAuth settings has no credential and acts only on its own.
func describeCapabilities(cfg *providers.OAuthConfigWithSecret) agentCapabilities {
	caps := agentCapabilities{credential: credentialNone, actsForUsers: []actingMode{}}
	if cfg == nil {
		return caps
	}
	caps.credential = credentialOf(cfg)
	if caps.credential == credentialPrivateKeyJWT && cfg.Certificate != nil {
		switch cfg.Certificate.Type {
		case providers.CertificateTypeJWKSURI:
			caps.jwksURI = cfg.Certificate.Value
		case providers.CertificateTypeJWKS:
			caps.keyIDs = keyIDs(cfg.Certificate.Value)
		}
	}
	for _, mode := range actingModes {
		if slices.Contains(cfg.GrantTypes, actingModeGrantTypes[mode]) {
			caps.actsForUsers = append(caps.actsForUsers, mode)
		}
	}
	for _, grant := range cfg.GrantTypes {
		if !isDescribedGrantType(grant) {
			caps.otherGrantTypes = append(caps.otherGrantTypes, string(grant))
		}
	}
	caps.redirectURIs = cfg.RedirectURIs
	return caps
}

// applyCapabilityChange applies a requested change to an agent's OAuth settings. It starts from the
// stored settings, so what the change does not describe is kept, and it checks only what the change
// touches, so a state the services accept never blocks an unrelated edit. It returns every problem with
// the request at once, or the settings to store, which are nil when the agent needs no OAuth client.
func applyCapabilityChange(current *providers.OAuthConfigWithSecret, change capabilityChange) (
	*providers.OAuthConfigWithSecret, []string) {
	have := describeCapabilities(current)
	target := capabilityTarget{
		credential:   have.credential,
		actsForUsers: have.actsForUsers,
		redirectURIs: have.redirectURIs,
	}
	if change.credential != nil {
		target.credential = *change.credential
	}
	if change.actsForUsers != nil {
		target.actsForUsers = orderedModes(change.actsForUsers)
	}
	if change.redirectURIs != nil {
		target.redirectURIs = change.redirectURIs
	}

	if problems := checkCapabilityChange(current, have, change, target); len(problems) > 0 {
		return nil, problems
	}
	if target.credential == credentialNone && len(target.actsForUsers) == 0 {
		return nil, nil
	}
	return buildOAuthConfig(current, have, change, target), nil
}

// checkCapabilityChange returns every problem with a requested change, judged against what it changes.
func checkCapabilityChange(current *providers.OAuthConfigWithSecret, have agentCapabilities,
	change capabilityChange, target capabilityTarget) []string {
	isNew := current == nil
	credentialChanged := isNew || target.credential != have.credential
	modesChanged := isNew || !slices.Equal(have.actsForUsers, target.actsForUsers)

	problems := actingProblems(have, change, target, credentialChanged || modesChanged, modesChanged)
	problems = append(problems, keyProblems(have, change, target)...)
	if !isNew {
		problems = append(problems, grantProblems(current, have, target, credentialChanged, modesChanged)...)
	}
	return problems
}

// actingProblems returns the problems with the ways of acting a change asks for. accessChanged says the
// credential or the ways of acting change.
func actingProblems(have agentCapabilities, change capabilityChange, target capabilityTarget,
	accessChanged, modesChanged bool) []string {
	signIn := target.acts(actingSignIn)
	problems := []string{}
	if accessChanged && target.credential == credentialNone &&
		(target.acts(actingApproval) || target.acts(actingTokenExchange)) {
		problems = append(problems, "approval and token_exchange need a credential, so the agent can prove "+
			"who it is: use client_secret or private_key_jwt")
	}
	if signIn && len(target.redirectURIs) == 0 && (modesChanged || change.redirectURIs != nil) {
		problems = append(problems, "sign_in needs redirectUris, the addresses a user returns to after signing in")
	}
	if !signIn && len(change.redirectURIs) > 0 {
		problems = append(problems, "redirectUris apply only to sign_in")
	}
	if target.acts(actingApproval) && !slices.Contains(have.actsForUsers, actingApproval) &&
		change.authFlowID == "" {
		problems = append(problems, "approval needs authFlowId, an authentication flow that can reach a user "+
			"who isn't there")
	}
	return problems
}

// keyProblems returns the problems with the keys a change gives, or the keys it needs and lacks.
func keyProblems(have agentCapabilities, change capabilityChange, target capabilityTarget) []string {
	hasKeys := change.jwksURI != "" || change.jwks != nil
	problems := []string{}
	switch {
	case target.credential != credentialPrivateKeyJWT && hasKeys:
		problems = append(problems, "jwksUri and jwks apply only to credential private_key_jwt")
	case change.jwksURI != "" && change.jwks != nil:
		problems = append(problems, "give jwksUri or jwks, not both")
	case target.credential == credentialPrivateKeyJWT && have.credential != credentialPrivateKeyJWT && !hasKeys:
		problems = append(problems, "private_key_jwt needs the agent's public keys: give jwksUri or jwks")
	}
	if change.jwks != nil {
		if _, problem := keySetFromJWKS(change.jwks); problem != "" {
			problems = append(problems, problem)
		}
	}
	return problems
}

// grantProblems returns the problems a change to an existing agent causes for grants and settings the
// credential and ways of acting don't describe, but depend on them.
func grantProblems(current *providers.OAuthConfigWithSecret, have agentCapabilities, target capabilityTarget,
	credentialChanged, modesChanged bool) []string {
	problems := []string{}
	if credentialChanged && target.credential == credentialNone &&
		(slices.Contains(current.GrantTypes, providers.GrantTypeJWTBearer) || idJAGEnabled(current)) {
		problems = append(problems, "credential none can't keep the jwt-bearer grant or ID-JAG, which need "+
			"a credential")
	}
	if modesChanged && !target.acts(actingTokenExchange) &&
		slices.Contains(have.actsForUsers, actingTokenExchange) && idJAGEnabled(current) {
		problems = append(problems, "token_exchange can't be removed while ID-JAG is on")
	}
	return problems
}

// buildOAuthConfig builds the OAuth settings for a checked change from the stored ones. Only the settings
// the change touches are rewritten.
func buildOAuthConfig(current *providers.OAuthConfigWithSecret, have agentCapabilities,
	change capabilityChange, target capabilityTarget) *providers.OAuthConfigWithSecret {
	cfg := &providers.OAuthConfigWithSecret{}
	if current != nil {
		copied := *current
		cfg = &copied
	}
	// A secret is only ever generated by the service.
	cfg.ClientSecret = ""

	switch target.credential {
	case credentialClientSecret:
		// An existing client keeps its secret method, since the token endpoint refuses any other.
		if have.credential != credentialClientSecret || current == nil {
			cfg.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodClientSecretBasic
		}
		cfg.PublicClient = false
	case credentialPrivateKeyJWT:
		cfg.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodPrivateKeyJWT
		cfg.PublicClient = false
		if change.jwksURI != "" {
			cfg.Certificate = &providers.Certificate{Type: providers.CertificateTypeJWKSURI, Value: change.jwksURI}
		} else if change.jwks != nil {
			value, _ := keySetFromJWKS(change.jwks)
			cfg.Certificate = &providers.Certificate{Type: providers.CertificateTypeJWKS, Value: value}
		}
	case credentialNone:
		cfg.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodNone
		cfg.PublicClient = true
	}
	if have.credential == credentialPrivateKeyJWT && target.credential != credentialPrivateKeyJWT &&
		!needsEncryptionCertificate(cfg) {
		cfg.Certificate = nil
	}

	signIn := target.acts(actingSignIn)
	signInAdded := signIn && !slices.Contains(have.actsForUsers, actingSignIn)
	grants := make([]providers.GrantType, 0, len(target.actsForUsers)+len(have.otherGrantTypes)+2)
	if target.credential != credentialNone {
		grants = append(grants, providers.GrantTypeClientCredentials)
	}
	for _, mode := range target.actsForUsers {
		grants = append(grants, actingModeGrantTypes[mode])
	}
	// A refresh token needs the authorization code or CIBA grant. Adding sign-in brings one, and an
	// existing one stays while either grant remains.
	hadRefresh := current != nil && slices.Contains(current.GrantTypes, providers.GrantTypeRefreshToken)
	if (signIn || target.acts(actingApproval)) && (hadRefresh || signInAdded) {
		grants = append(grants, providers.GrantTypeRefreshToken)
	}
	for _, grant := range have.otherGrantTypes {
		grants = append(grants, providers.GrantType(grant))
	}
	cfg.GrantTypes = grants

	if signIn {
		if !slices.Contains(cfg.ResponseTypes, providers.ResponseTypeCode) {
			cfg.ResponseTypes = append(slices.Clone(cfg.ResponseTypes), providers.ResponseTypeCode)
		}
		if signInAdded {
			cfg.PKCERequired = true
		}
		cfg.RedirectURIs = target.redirectURIs
	} else {
		// Response types, redirect URIs and PKCE belong to the authorization code grant.
		cfg.ResponseTypes = nil
		cfg.RedirectURIs = nil
		cfg.PKCERequired = false
	}
	if target.credential == credentialNone {
		// A public client signs users in with PKCE only.
		cfg.PKCERequired = true
	}
	return cfg
}

// credentialOf returns how an OAuth client proves who it is, read from its authentication method.
func credentialOf(cfg *providers.OAuthConfigWithSecret) agentCredential {
	if cfg.PublicClient {
		return credentialNone
	}
	switch cfg.TokenEndpointAuthMethod {
	case providers.TokenEndpointAuthMethodPrivateKeyJWT:
		return credentialPrivateKeyJWT
	case providers.TokenEndpointAuthMethodNone:
		return credentialNone
	case providers.TokenEndpointAuthMethodClientSecretBasic, providers.TokenEndpointAuthMethodClientSecretPost:
		return credentialClientSecret
	}
	// A client stored without a method authenticates with client_secret_basic.
	return credentialClientSecret
}

// isDescribedGrantType reports whether a grant type is described by the credential and the ways of
// acting for users, rather than reported as is.
func isDescribedGrantType(grant providers.GrantType) bool {
	if grant == providers.GrantTypeClientCredentials || grant == providers.GrantTypeRefreshToken {
		return true
	}
	for _, mode := range actingModes {
		if actingModeGrantTypes[mode] == grant {
			return true
		}
	}
	return false
}

// orderedModes returns the given ways of acting once each, in the order they are reported.
func orderedModes(modes []actingMode) []actingMode {
	ordered := make([]actingMode, 0, len(modes))
	for _, mode := range actingModes {
		if slices.Contains(modes, mode) {
			ordered = append(ordered, mode)
		}
	}
	return ordered
}

// idJAGEnabled reports whether OAuth settings let the client take ID-JAG assertions.
func idJAGEnabled(cfg *providers.OAuthConfigWithSecret) bool {
	return cfg != nil && cfg.Token != nil && cfg.Token.IDJAG != nil && cfg.Token.IDJAG.Enabled
}

// needsEncryptionCertificate reports whether OAuth settings encrypt ID tokens or userinfo responses, which
// uses the client's certificate whatever its credential.
func needsEncryptionCertificate(cfg *providers.OAuthConfigWithSecret) bool {
	return (cfg.Token != nil && cfg.Token.IDToken != nil && cfg.Token.IDToken.EncryptionAlg != "") ||
		(cfg.UserInfo != nil && cfg.UserInfo.EncryptionAlg != "")
}

// keySetFromJWKS checks a key set given as a tool input and returns the value the certificate store
// keeps, or a problem. Every key must be public and have a unique kid, which client authentication
// matches keys by.
func keySetFromJWKS(jwks map[string]any) (string, string) {
	raw, err := json.Marshal(jwks)
	if err != nil {
		return "", "jwks is not a valid JSON Web Key Set"
	}
	var set keySet
	if err := json.Unmarshal(raw, &set); err != nil || len(set.Keys) == 0 {
		return "", "jwks must be a JSON Web Key Set with at least one key under keys"
	}
	seen := make([]string, 0, len(set.Keys))
	for _, key := range set.Keys {
		if problem := checkPublicKey(key, seen); problem != "" {
			return "", problem
		}
		kid, _ := key["kid"].(string)
		seen = append(seen, kid)
	}
	return encodeKeySet(set)
}

// addKey adds a public key to a stored key set, and returns the new value or a problem.
func addKey(value string, key map[string]any) (string, string) {
	var set keySet
	if err := json.Unmarshal([]byte(value), &set); err != nil {
		return "", "the agent's stored key set can't be read"
	}
	if problem := checkPublicKey(key, keyIDs(value)); problem != "" {
		return "", problem
	}
	set.Keys = append(set.Keys, key)
	return encodeKeySet(set)
}

// removeKey removes the key with the given kid from a stored key set, and returns the new value or a
// problem. The last key can't be removed, since the agent would have no way to sign in.
func removeKey(value, kid string) (string, string) {
	var set keySet
	if err := json.Unmarshal([]byte(value), &set); err != nil {
		return "", "the agent's stored key set can't be read"
	}
	index := slices.IndexFunc(set.Keys, func(key map[string]any) bool {
		keyID, _ := key["kid"].(string)
		return keyID == kid
	})
	if index < 0 {
		return "", fmt.Sprintf("the agent's key set has no key with kid %s", kid)
	}
	if len(set.Keys) == 1 {
		return "", "the agent's last key can't be removed: add its new key with addKey first"
	}
	set.Keys = slices.Delete(set.Keys, index, index+1)
	return encodeKeySet(set)
}

// checkPublicKey returns a problem when a key holds private material, or lacks a kid or repeats one of
// the given kids.
func checkPublicKey(key map[string]any, existingIDs []string) string {
	if member, found := jws.ContainsPrivateMember(key); found {
		return fmt.Sprintf("keys must be public, but a key has the private member %s", member)
	}
	kid, _ := key["kid"].(string)
	if kid == "" {
		return "every key needs a kid, which client authentication matches keys by"
	}
	if slices.Contains(existingIDs, kid) {
		return fmt.Sprintf("the kid %s is already used by another key", kid)
	}
	return ""
}

// encodeKeySet serializes a key set for the certificate store, or returns a problem when it is too long.
func encodeKeySet(set keySet) (string, string) {
	raw, err := json.Marshal(set)
	if err != nil {
		return "", "the key set is not valid JSON"
	}
	if len(raw) > maxKeySetLength {
		return "", fmt.Sprintf("the key set is %d characters, more than the %d the certificate store accepts",
			len(raw), maxKeySetLength)
	}
	return string(raw), ""
}

// keyIDs returns the kid of each key in a stored key set.
func keyIDs(value string) []string {
	var set keySet
	if err := json.Unmarshal([]byte(value), &set); err != nil {
		return nil
	}
	ids := make([]string, 0, len(set.Keys))
	for _, key := range set.Keys {
		if kid, ok := key["kid"].(string); ok && kid != "" {
			ids = append(ids, kid)
		}
	}
	return ids
}
