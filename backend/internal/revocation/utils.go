// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

// scopeValueSeparator ends each length prefix in a scope criterion digest.
const scopeValueSeparator = "|"

// EntityScopeCriterionValue returns the value denying one scope for one principal on one resource server.
func EntityScopeCriterionValue(entityID, audience, scope string) string {
	return digest(entityID, audience, scope)
}

// EntityScopeSubtreeCriterionValue binds a subtree to its principal, audience, and original delimiter.
func EntityScopeSubtreeCriterionValue(entityID, audience, root, delimiter string) string {
	return digest(entityID, audience, root, delimiter)
}

// ScopeCriterionValue returns the value denying one scope on one resource server for every principal.
func ScopeCriterionValue(audience, scope string) string {
	return digest(audience, scope)
}

// TokenAudiences reads the audiences of a token carrying scopes, failing closed when the claim is malformed.
func TokenAudiences(claims map[string]interface{}, scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return nil, nil
	}
	// A refresh token's own aud is the issuer; the resource servers it grants are in access_token_aud.
	audienceClaim := constants.ClaimAud
	if accessTokenSubject, _ := claims[constants.ClaimAccessTokenSubject].(string); accessTokenSubject != "" {
		audienceClaim = constants.ClaimAccessTokenAudience
	}
	switch value := claims[audienceClaim].(type) {
	case nil:
		return nil, nil
	case string:
		if value == "" {
			return nil, errMalformedAudience
		}
		return []string{value}, nil
	case []interface{}:
		if len(value) == 0 {
			return nil, errMalformedAudience
		}
		audiences := make([]string, 0, len(value))
		for _, entry := range value {
			audience, ok := entry.(string)
			if !ok || audience == "" {
				return nil, errMalformedAudience
			}
			audiences = append(audiences, audience)
		}
		return audiences, nil
	default:
		return nil, errMalformedAudience
	}
}

// ScopeCriteria returns the criteria revoking each scope on each audience, per principal when subject is set.
func ScopeCriteria(subject string, audiences, scopes []string) []Criterion {
	criteria := make([]Criterion, 0, 2*len(audiences)*len(scopes))
	for _, audience := range audiences {
		for _, scope := range scopes {
			if subject != "" {
				criteria = append(criteria, Criterion{
					Type: CriterionTypeEntityScope, Value: EntityScopeCriterionValue(subject, audience, scope),
				})
			}
			criteria = append(criteria, Criterion{
				Type: CriterionTypeScope, Value: ScopeCriterionValue(audience, scope),
			})
		}
	}
	return append(criteria, SubtreeScopeCriteria(subject, audiences, scopes)...)
}

// SubtreeScopeCriteria derives frozen subtree lookup keys without reading the live permission catalog.
func SubtreeScopeCriteria(subject string, audiences, scopes []string) []Criterion {
	if subject == "" {
		return nil
	}
	criteria := []Criterion{}
	seen := map[Criterion]bool{}
	for _, audience := range audiences {
		for _, scope := range scopes {
			for _, separator := range serverconst.ValidPermissionDelimiters {
				delimiter := string(separator)
				root := scope
				for root != "" {
					criterion := Criterion{
						Type:  CriterionTypeEntityScopeSubtree,
						Value: EntityScopeSubtreeCriterionValue(subject, audience, root, delimiter),
					}
					if !seen[criterion] {
						seen[criterion] = true
						criteria = append(criteria, criterion)
					}
					index := strings.LastIndex(root, delimiter)
					if index < 0 {
						break
					}
					root = root[:index]
				}
			}
		}
	}
	return criteria
}

// digest returns the hex SHA-256 of the length-prefixed parts, so differently split parts never collide.
func digest(parts ...string) string {
	var rendered strings.Builder
	for _, part := range parts {
		rendered.WriteString(strconv.Itoa(len(part)))
		rendered.WriteString(scopeValueSeparator)
		rendered.WriteString(part)
	}
	sum := sha256.Sum256([]byte(rendered.String()))
	return hex.EncodeToString(sum[:])
}
