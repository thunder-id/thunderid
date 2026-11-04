// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	sharedrevocation "github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/revocationmock"
)

const (
	testGrantSubject  = "user-1"
	testGrantAudience = "https://api.example.com"
)

// denyListLookup answers like the criteria deny list: boundary rows match artifacts established by their cutoff.
func denyListLookup(rows ...revocation.CriteriaRevocation) func(context.Context, revocation.RevocationIdentity) error {
	return func(_ context.Context, identity revocation.RevocationIdentity) error {
		for _, row := range rows {
			if slices.Contains(identity.Criteria, row.Criterion) &&
				(!sharedrevocation.IsBoundaryReason(row.Reason) || !identity.EstablishedAt.After(row.Cutoff)) {
				return revocation.ErrTokenRevoked
			}
		}
		return nil
	}
}

type GrantHandlerUtilsTestSuite struct {
	suite.Suite
	mockEnforcement *revocationmock.EnforcementServiceInterfaceMock
	issuedAt        time.Time
	grant           revocation.RevocationIdentity
}

func TestGrantHandlerUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(GrantHandlerUtilsTestSuite))
}

func (s *GrantHandlerUtilsTestSuite) SetupTest() {
	s.mockEnforcement = revocationmock.NewEnforcementServiceInterfaceMock(s.T())
	s.issuedAt = time.Now().Add(-5 * time.Minute).UTC().Truncate(time.Second)
	s.grant = revocation.RevocationIdentity{
		EstablishedAt: s.issuedAt,
		Criteria: []revocation.Criterion{
			{Type: revocation.CriterionTypeSubject, Value: testGrantSubject},
			{Type: revocation.CriterionTypeApplicationKey, Value: testClientID},
		},
	}
}

func (s *GrantHandlerUtilsTestSuite) scopeRevokedAt(scope string, cutoff time.Time) revocation.CriteriaRevocation {
	return revocation.CriteriaRevocation{
		Criterion: revocation.Criterion{
			Type:  revocation.CriterionTypeEntityScope,
			Value: sharedrevocation.EntityScopeCriterionValue(testGrantSubject, testGrantAudience, scope),
		},
		Reason: revocation.RevocationReasonRoleAssignmentRemoved,
		Cutoff: cutoff,
	}
}

func (s *GrantHandlerUtilsTestSuite) exclude(oidcScopes, permissionScopes []string) (
	[]string, *model.ErrorResponse) {
	return excludeRevokedScopes(context.Background(), s.mockEnforcement, s.grant,
		testGrantSubject, []string{testGrantAudience}, oidcScopes, permissionScopes)
}

func (s *GrantHandlerUtilsTestSuite) TestExcludeRevokedScopes_WithoutEnforcementGrantsEveryScope() {
	scopes, errResp := excludeRevokedScopes(context.Background(), nil, s.grant, testGrantSubject,
		[]string{testGrantAudience}, []string{"openid"}, []string{"read"})

	s.Nil(errResp)
	s.Equal([]string{"openid", "read"}, scopes)
}

func (s *GrantHandlerUtilsTestSuite) TestExcludeRevokedScopes_NothingRevokedGrantsEveryScopeInOneLookup() {
	s.mockEnforcement.EXPECT().EnsureNotRevoked(mock.Anything, mock.Anything).RunAndReturn(denyListLookup())

	scopes, errResp := s.exclude([]string{"openid"}, []string{"read", "write"})

	s.Nil(errResp)
	s.Equal([]string{"openid", "read", "write"}, scopes)
	s.mockEnforcement.AssertNumberOfCalls(s.T(), "EnsureNotRevoked", 1)
}

func (s *GrantHandlerUtilsTestSuite) TestExcludeRevokedScopes_ScopeRevokedAfterIssueIsExcluded() {
	s.mockEnforcement.EXPECT().EnsureNotRevoked(mock.Anything, mock.Anything).
		RunAndReturn(denyListLookup(s.scopeRevokedAt("write", s.issuedAt.Add(time.Minute))))

	scopes, errResp := s.exclude([]string{"openid"}, []string{"read", "write"})

	s.Nil(errResp)
	s.Equal([]string{"openid", "read"}, scopes)
}

func (s *GrantHandlerUtilsTestSuite) TestExcludeRevokedScopes_RevocationInTheGrantsIssuingSecondApplies() {
	s.grant.EstablishedAt = s.issuedAt.Add(500 * time.Millisecond)
	s.mockEnforcement.EXPECT().EnsureNotRevoked(mock.Anything, mock.Anything).
		RunAndReturn(denyListLookup(s.scopeRevokedAt("write", s.issuedAt)))

	scopes, errResp := s.exclude(nil, []string{"read", "write"})

	s.Nil(errResp)
	s.Equal([]string{"read"}, scopes)
}

func (s *GrantHandlerUtilsTestSuite) TestExcludeRevokedScopes_GrantRevokedIsRefused() {
	s.mockEnforcement.EXPECT().EnsureNotRevoked(mock.Anything, mock.Anything).
		RunAndReturn(denyListLookup(revocation.CriteriaRevocation{
			Criterion: revocation.Criterion{Type: revocation.CriterionTypeSubject, Value: testGrantSubject},
			Reason:    revocation.RevocationReasonUserDeleted,
		}))

	scopes, errResp := s.exclude([]string{"openid"}, []string{"read"})

	s.Nil(scopes)
	s.Equal(&model.ErrorResponse{
		Error: constants.ErrorInvalidGrant, ErrorDescription: "The authorization grant has been revoked"}, errResp)
}

func (s *GrantHandlerUtilsTestSuite) TestExcludeRevokedScopes_EveryScopeRevokedIsRefused() {
	cutoff := s.issuedAt.Add(time.Minute)
	s.mockEnforcement.EXPECT().EnsureNotRevoked(mock.Anything, mock.Anything).
		RunAndReturn(denyListLookup(s.scopeRevokedAt("read", cutoff), s.scopeRevokedAt("write", cutoff)))

	scopes, errResp := s.exclude(nil, []string{"read", "write"})

	s.Nil(scopes)
	s.Equal(&model.ErrorResponse{
		Error: constants.ErrorInvalidGrant, ErrorDescription: "The authorization grant has been revoked"}, errResp)
}

func (s *GrantHandlerUtilsTestSuite) TestExcludeRevokedScopes_EveryPermissionRevokedKeepsOIDCScopes() {
	s.mockEnforcement.EXPECT().EnsureNotRevoked(mock.Anything, mock.Anything).
		RunAndReturn(denyListLookup(s.scopeRevokedAt("read", s.issuedAt.Add(time.Minute))))

	scopes, errResp := s.exclude([]string{"openid"}, []string{"read"})

	s.Nil(errResp)
	s.Equal([]string{"openid"}, scopes)
}

func (s *GrantHandlerUtilsTestSuite) TestExcludeRevokedScopes_NoScopesRequestedIsNotRefused() {
	s.mockEnforcement.EXPECT().EnsureNotRevoked(mock.Anything, mock.Anything).RunAndReturn(denyListLookup())

	scopes, errResp := s.exclude(nil, nil)

	s.Nil(errResp)
	s.Empty(scopes)
}

func (s *GrantHandlerUtilsTestSuite) TestExcludeRevokedScopes_DenyListReadErrorIsServerError() {
	revokedThenUnavailable := func(failOnCall int) func(context.Context, revocation.RevocationIdentity) error {
		call := 0
		return func(context.Context, revocation.RevocationIdentity) error {
			call++
			switch call {
			case failOnCall:
				return revocation.ErrEnforcementUnavailable
			case 1:
				return revocation.ErrTokenRevoked
			default:
				return nil
			}
		}
	}
	testCases := []struct {
		name       string
		failOnCall int
	}{
		{"FirstLookup", 1},
		{"GrantLookup", 2},
		{"ScopeLookup", 3},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.mockEnforcement.EXPECT().EnsureNotRevoked(mock.Anything, mock.Anything).
				RunAndReturn(revokedThenUnavailable(tc.failOnCall))

			scopes, errResp := s.exclude([]string{"openid"}, []string{"read"})

			s.Nil(scopes)
			s.Equal(&model.ErrorResponse{Error: constants.ErrorServerError,
				ErrorDescription: "Token revocation status could not be verified"}, errResp)
		})
	}
}

func (s *GrantHandlerUtilsTestSuite) TestGrantRevocationError() {
	revoked := grantRevocationError(revocation.ErrTokenRevoked)
	s.Equal(constants.ErrorInvalidGrant, revoked.Error)

	unavailable := grantRevocationError(errors.New("db down"))
	s.Equal(constants.ErrorServerError, unavailable.Error)
}

func (s *GrantHandlerUtilsTestSuite) TestGrantPermits() {
	oidc := map[string]struct{}{"openid": {}, "profile": {}, "reservations:history": {}}

	tests := []struct {
		name      string
		requested string
		granted   []string
		delimiter string
		want      bool
	}{
		{
			name:      "a grant of the resource carries what sits beneath it",
			requested: "reservations:view", granted: []string{"reservations"}, delimiter: ":", want: true,
		},
		{
			name:      "and carries it at any depth",
			requested: "reservations:online:view", granted: []string{"reservations"}, delimiter: ":", want: true,
		},
		{
			name:      "an exactly granted scope is permitted",
			requested: "reservations:view", granted: []string{"reservations:view"}, delimiter: ":", want: true,
		},
		{
			// Narrowing never works upwards: the resource carries every other descendant too.
			name:      "a grant of a descendant does not carry the resource it sits under",
			requested: "reservations", granted: []string{"reservations:view"}, delimiter: ":", want: false,
		},
		{
			name:      "a sibling is not carried",
			requested: "reservations:create", granted: []string{"reservations:view"}, delimiter: ":", want: false,
		},
		{
			name:      "a shared prefix that is not an ancestor is not carried",
			requested: "reservationsarchive", granted: []string{"reservations"}, delimiter: ":", want: false,
		},
		{
			// An OIDC scope is not a permission on any resource server, so it stands for itself only.
			name:      "an OIDC scope in the grant never acts as an ancestor",
			requested: "profile:secret", granted: []string{"profile"}, delimiter: ":", want: false,
		},
		{
			name:      "an OIDC scope is still permitted exactly",
			requested: "openid", granted: []string{"openid", "reservations"}, delimiter: ":", want: true,
		},
		{
			name:      "a requested custom OIDC scope cannot inherit a permission grant",
			requested: "reservations:history", granted: []string{"reservations"}, delimiter: ":", want: false,
		},
		{
			name:      "a requested custom OIDC scope is permitted when explicitly granted",
			requested: "reservations:history", granted: []string{"reservations:history"}, delimiter: ":", want: true,
		},

		{
			name:      "a non-colon delimiter is honored",
			requested: "reservations/view", granted: []string{"reservations"}, delimiter: "/", want: true,
		},
		{
			// An unbound grant has no resource server and therefore no delimiter.
			name:      "no delimiter reduces the test to equality",
			requested: "reservations:view", granted: []string{"reservations"}, delimiter: "", want: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, grantPermits(tt.requested, tt.granted, oidc, tt.delimiter))
		})
	}
}

func (s *GrantHandlerUtilsTestSuite) TestCompactPermissionScopes() {
	for _, tc := range []struct {
		name               string
		scopes, oidc, want []string
		delimiter          string
		hierarchy          bool
	}{
		{"root and child", []string{"reservations:create", "reservations"}, nil,
			[]string{"reservations"}, ":", true},
		{"exact unchanged", []string{"reservations", "reservations:create"}, nil,
			[]string{"reservations", "reservations:create"}, ":", false},
		{"child only", []string{"reservations:create"}, nil, []string{"reservations:create"}, ":", true},
		{"siblings", []string{"reservations:create", "reservations:view"}, nil,
			[]string{"reservations:create", "reservations:view"}, ":", true},
		{"custom OIDC child", []string{"reservations", "reservations:profile"}, []string{"reservations:profile"},
			[]string{"reservations", "reservations:profile"}, ":", true},
		{"OIDC cannot cover permission", []string{"profile", "profile:edit"}, []string{"profile"},
			[]string{"profile", "profile:edit"}, ":", true},
		{"machine permission", []string{"profile", "profile:edit"}, nil, []string{"profile"}, ":", true},
		{"custom delimiter", []string{"reservations/create", "reservations", "reservation"}, nil,
			[]string{"reservations", "reservation"}, "/", true},
	} {
		s.Run(tc.name, func() {
			original := slices.Clone(tc.scopes)
			server := &providers.ResourceServer{HierarchicalPermissions: tc.hierarchy, Delimiter: tc.delimiter}
			s.Equal(tc.want, compactPermissionScopes(tc.scopes, tc.oidc, server))
			s.Equal(original, tc.scopes)
		})
	}
}
