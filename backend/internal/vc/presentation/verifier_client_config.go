// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

import (
	"context"
	"crypto/x509"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// GetVCVerifierClientConfigByID returns the verifier client configuration for the presentation
// definition whose handle is verifierID.
func (s *definitionService) GetVCVerifierClientConfigByID(
	ctx context.Context, verifierID string,
) (*providers.VCVerifierClientConfig, *tidcommon.ServiceError) {
	if strings.TrimSpace(verifierID) == "" {
		return nil, &ErrorDefinitionInvalidRequest
	}
	dto, svcErr := s.GetPresentationDefinitionByHandle(ctx, verifierID)
	if svcErr != nil {
		return nil, svcErr
	}
	anchors, err := s.resolveTrustedAuthorities(dto.TrustedAuthorities)
	if err != nil {
		s.logger.Error(ctx, "Failed to resolve trusted authorities", log.String("handle", verifierID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	cfg, err := toVCVerifierClientConfig(*dto, anchors, s.config)
	if err != nil {
		s.logger.Error(ctx, "Invalid presentation definition", log.String("handle", verifierID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	verifierInfo, err := loadVerifierInfo(s.config.RegistrationCertFile, s.serverHome)
	if err != nil {
		s.logger.Error(ctx, "Invalid verifier info", log.String("handle", verifierID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	cfg.VerifierInfo = verifierInfo
	cfg.ClientIDScheme = s.config.ClientIDScheme

	return cfg, nil
}

// resolveTrustedAuthorities resolves trust anchor names to their certificates keyed by key ID.
// Unknown names, or names without a trust store, fail closed.
func (s *definitionService) resolveTrustedAuthorities(names []string) (map[string]*x509.Certificate, error) {
	if len(names) == 0 {
		return nil, nil
	}
	if s.trustStore == nil {
		return nil, fmt.Errorf("trust store is not configured")
	}
	return s.trustStore.AnchorsFor(names)
}

// toVCVerifierClientConfig converts a presentation definition into a verifier client configuration
// whose DCQL query requests a single credential, keyed by the definition handle. trustedAuthorities
// holds the definition's trust anchor certificates keyed by key ID; their key IDs are emitted as the
// query's aki trusted_authorities entry.
func toVCVerifierClientConfig(
	dto PresentationDefinitionDTO, trustedAuthorities map[string]*x509.Certificate, vpConfig config.OpenID4VPConfig,
) (*providers.VCVerifierClientConfig, error) {
	claimPaths := dto.RequestedClaims
	var claimSets [][]string
	if len(claimPaths) == 0 {
		claimPaths = slices.Concat(dto.MandatoryClaims, dto.OptionalClaims)
		claimSets = dcqlClaimSets(len(dto.MandatoryClaims), len(dto.OptionalClaims))
	}
	claims := make([]providers.DCQLClaimsQuery, 0, len(claimPaths))
	for i, path := range claimPaths {
		segments, err := claimPathToSegments(path)
		if err != nil {
			return nil, err
		}
		claim := providers.DCQLClaimsQuery{Path: segments, Values: dcqlValues(dto.ClaimValues[path])}
		if claimSets != nil {
			claim.ID = dcqlClaimID(i)
		}
		claims = append(claims, claim)
	}

	credential := providers.DCQLCredentialQuery{
		ID:                                dto.Handle,
		Format:                            dto.Format,
		Meta:                              dcqlMeta(dto.Format, dto.VCT),
		Claims:                            claims,
		ClaimSets:                         claimSets,
		RequireCryptographicHolderBinding: vpConfig.EnforceKeyBindingEnabled(),
	}
	if len(trustedAuthorities) > 0 {
		credential.TrustedAuthorities = []providers.DCQLTrustedAuthority{
			{Type: "aki", Values: slices.Sorted(maps.Keys(trustedAuthorities))},
		}
	}

	return &providers.VCVerifierClientConfig{
		VerifierID:           dto.Handle,
		DCQL:                 providers.DCQLQuery{Credentials: []providers.DCQLCredentialQuery{credential}},
		TrustedAuthorities:   trustedAuthorities,
		EnforceTrustedIssuer: dto.EnforceTrustedIssuer,
	}, nil
}

// dcqlMeta builds the format-specific DCQL meta constraints for a credential query. It returns nil
// for formats that define no meta constraints.
func dcqlMeta(format, vct string) *providers.DCQLMeta {
	switch format {
	case DefaultCredentialFormat:
		return &providers.DCQLMeta{VCTValues: []string{vct}}
	default:
		return nil
	}
}

// dcqlClaimSets expresses claim optionality as DCQL claim_sets over claims ordered mandatory first,
// then optional. It returns nil when there are no optional claims, since DCQL then requires every
// claim. The preferred set requests all claims; the fallback requests only the mandatory claims, or
// any single optional claim when none are mandatory, because a claim set cannot be empty.
func dcqlClaimSets(mandatory, optional int) [][]string {
	if optional == 0 {
		return nil
	}
	all := make([]string, 0, mandatory+optional)
	for i := range mandatory + optional {
		all = append(all, dcqlClaimID(i))
	}
	sets := [][]string{all}
	if mandatory > 0 {
		return append(sets, all[:mandatory])
	}
	for i := range optional {
		sets = append(sets, []string{dcqlClaimID(i)})
	}
	return sets
}

// dcqlClaimID returns the DCQL claim ID for the claim at index i of a credential query.
func dcqlClaimID(i int) string {
	return "claim_" + strconv.Itoa(i)
}

// claimPathToSegments converts a dotted claim path into DCQL path segments.
func claimPathToSegments(path string) ([]interface{}, error) {
	if path == "" {
		return nil, fmt.Errorf("empty claim path")
	}
	parts := strings.Split(path, ".")
	segments := make([]interface{}, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("malformed claim path %q", path)
		}
		segments = append(segments, part)
	}
	return segments, nil
}

// dcqlValues converts allowed claim values to the DCQL "values" array; nil when unconstrained.
func dcqlValues(values []string) []interface{} {
	if len(values) == 0 {
		return nil
	}
	out := make([]interface{}, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	return out
}

// loadVerifierInfo reads the Registration Certificate JWT from path and wraps it
// as the single entry of a verifier_info array. An empty path returns (nil, nil).
func loadVerifierInfo(path, serverHome string) ([]interface{}, error) {
	if path == "" {
		return nil, nil
	}
	resolved := filepath.Clean(resolvePath(serverHome, path))
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to read registration certificate %q: %w", resolved, err)
	}
	jwt := strings.TrimSpace(string(data))
	if jwt == "" {
		return nil, fmt.Errorf("VerifierInfo: registration certificate %q is empty", resolved)
	}
	return []interface{}{
		map[string]interface{}{
			"format": "registration_cert",
			"data":   jwt,
		},
	}, nil
}

// resolvePath joins a relative path with the server home directory.
func resolvePath(serverHome, path string) string {
	if path == "" || filepath.IsAbs(path) || serverHome == "" {
		return path
	}
	return filepath.Join(serverHome, path)
}
