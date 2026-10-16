// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package defaultprovider

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authn/otpmock"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernancemock"
)

// AccessTestSuite covers P1: the check after a secret is verified, the reset when it admits, and the
// count when it does not. The failure scope and the lock scope come from one derivation.
type AccessTestSuite struct {
	suite.Suite
	entitySvc  *entitymock.EntityServiceInterfaceMock
	otpSvc     *otpmock.OTPAuthnServiceInterfaceMock
	governance *identitygovernancemock.IdentityGovernanceServiceInterfaceMock
	provider   providers.AuthnProviderInterface
	ctx        context.Context
}

func TestAccessTestSuite(t *testing.T) {
	suite.Run(t, new(AccessTestSuite))
}

func (s *AccessTestSuite) SetupTest() {
	s.entitySvc = entitymock.NewEntityServiceInterfaceMock(s.T())
	s.otpSvc = otpmock.NewOTPAuthnServiceInterfaceMock(s.T())
	s.governance = identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(s.T())
	s.provider = Initialize(s.entitySvc, nil, s.otpSvc, nil, nil, nil, s.governance)
	s.ctx = context.Background()
}

// authenticate runs a password attempt through the whole provider.
func (s *AccessTestSuite) authenticate(
	credentials map[string]interface{}) *tidcommon.ServiceError {
	_, svcErr := s.provider.Authenticate(s.ctx,
		map[string]interface{}{"username": "alice"}, credentials, nil)
	return svcErr
}

// expectVerified makes the credential comparison succeed and the entity read return a live user, so
// that the attempt reaches the check with a resolved subject.
func (s *AccessTestSuite) expectVerified(entityID string, credentials map[string]interface{}) {
	s.entitySvc.On("AuthenticateEntity", mock.Anything, mock.Anything, credentials).
		Return(&entity.AuthenticateResult{
			EntityID:       entityID,
			EntityCategory: providers.EntityCategoryUser,
			EntityType:     "person",
		}, (*entity.CredentialFailure)(nil), nil).Once()
	s.entitySvc.On("GetEntityProfile", mock.Anything, entityID).
		Return(&providers.Entity{
			ID:         entityID,
			Category:   providers.EntityCategoryUser,
			Type:       "person",
			Attributes: json.RawMessage(`{}`),
		}, nil).Once()
}

// expectProvenMismatch makes the comparison fail with evidence naming the row it ran against.
func (s *AccessTestSuite) expectProvenMismatch(credentials map[string]interface{}) {
	s.entitySvc.On("AuthenticateEntity", mock.Anything, mock.Anything, credentials).
		Return(nil, &entity.CredentialFailure{EntityID: "u1"},
			entity.ErrAuthenticationFailed).Once()
}

func (s *AccessTestSuite) assertNothingRecorded() {
	s.governance.AssertNotCalled(s.T(), "RecordFailure",
		mock.Anything, mock.Anything, mock.Anything)
}

func (s *AccessTestSuite) assertNothingChecked() {
	s.governance.AssertNotCalled(s.T(), "AdmitAuthenticationStep",
		mock.Anything, mock.Anything, mock.Anything)
}

// A refusal that names no hold is refused after the secret verified, with the answer a wrong secret
// produces.
func (s *AccessTestSuite) TestAHeldAccountIsRefused() {
	credentials := map[string]interface{}{"password": "correct"}
	s.expectVerified("held-1", credentials)
	s.governance.On("AdmitAuthenticationStep", mock.Anything, "held-1",
		providers.AccessScopeCredential).
		Return(providers.AccessDecision{Admitted: false}, nil).Once()

	svcErr := s.authenticate(credentials)
	s.Require().NotNil(svcErr)
	s.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, svcErr.Code,
		"a refusal that names no hold is the wrong-password answer")
}

// The refusal keeps the code of the hold that refused it, so the executors can state it.
func (s *AccessTestSuite) TestADisclosedHoldKeepsItsOwnCode() {
	credentials := map[string]interface{}{"password": "correct"}
	s.expectVerified("held-1", credentials)
	s.governance.On("AdmitAuthenticationStep", mock.Anything, "held-1",
		providers.AccessScopeCredential).
		Return(providers.AccessDecision{
			Admitted: false,
			Reason:   providers.AccessRefusalReasonSuspended,
		}, nil).Once()

	svcErr := s.authenticate(credentials)
	s.Require().NotNil(svcErr)
	s.Equal(authnprovidercm.ErrorCodeAccountSuspended, svcErr.Code)
	s.Equal(tidcommon.ClientErrorType, svcErr.Type)
}

// The reason picks the code, and the two holds do not share one. Everything a client eventually reads
// is the manager's, so what this asserts is the selection, not the wording.
func (s *AccessTestSuite) TestTheReasonPicksTheHeldCode() {
	suspended := refusal(providers.AccessDecision{Reason: providers.AccessRefusalReasonSuspended})
	locked := refusal(providers.AccessDecision{Reason: providers.AccessRefusalReasonLocked})
	plain := refusal(providers.AccessDecision{})

	s.Equal(authnprovidercm.ErrorCodeAccountSuspended, suspended.Code)
	s.Equal(authnprovidercm.ErrorCodeAccountLocked, locked.Code)
	s.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, plain.Code)
	s.Equal(tidcommon.ClientErrorType, suspended.Type)
	s.Equal(tidcommon.ClientErrorType, locked.Type)
}

// A reason this mapping does not know states nothing, so a new hold stays hidden until it is mapped.
func (s *AccessTestSuite) TestAnUnknownReasonStatesNothing() {
	s.Equal(authnprovidercm.ErrorCodeAuthenticationFailed,
		refusal(providers.AccessDecision{Reason: "some_future_hold"}).Code)
}

// A verified subject reaches governance on a runtime context.
func (s *AccessTestSuite) TestAnAdmittedAuthenticationSucceeds() {
	credentials := map[string]interface{}{"password": "correct"}
	s.expectVerified("u1", credentials)
	s.governance.On("AdmitAuthenticationStep", mock.MatchedBy(security.IsRuntimeContext), "u1",
		providers.AccessScopeCredential).
		Return(providers.AccessDecision{Admitted: true}, nil).Once()

	s.Nil(s.authenticate(credentials))
}

// A recovery flow's proof is admitted at the recovery plane: the lock it is about to release does not
// refuse it, and the step's method entry is left for the release node.
func (s *AccessTestSuite) TestARecoveryProofIsAdmittedAtTheRecoveryPlane() {
	credentials := map[string]interface{}{"password": "correct"}
	s.expectVerified("u1", credentials)
	s.governance.On("EvaluateAccess", mock.MatchedBy(security.IsRuntimeContext), "u1",
		providers.AccessPlaneRecovery, providers.AccessScopeCredential).
		Return(providers.AccessDecision{Admitted: true}, nil).Once()
	s.ctx = authnprovidercm.WithRecoveryFlow(context.Background())

	s.Nil(s.authenticate(credentials))
	s.assertNothingChecked()
}

// A suspension still refuses a recovery proof.
func (s *AccessTestSuite) TestARecoveryProofOfASuspendedAccountIsRefused() {
	credentials := map[string]interface{}{"password": "correct"}
	s.expectVerified("held-1", credentials)
	s.governance.On("EvaluateAccess", mock.Anything, "held-1",
		providers.AccessPlaneRecovery, providers.AccessScopeCredential).
		Return(providers.AccessDecision{Admitted: false}, nil).Once()
	s.ctx = authnprovidercm.WithRecoveryFlow(context.Background())

	svcErr := s.authenticate(credentials)
	s.Require().NotNil(svcErr)
	s.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, svcErr.Code)
}

// An unreadable authority refuses with the server error an unreadable entity gets here, not the
// wrong-password answer: an outage is not a credential failure.
func (s *AccessTestSuite) TestAnUnreadableAuthorityRefuses() {
	credentials := map[string]interface{}{"password": "correct"}
	s.expectVerified("u1", credentials)
	s.governance.On("AdmitAuthenticationStep", mock.Anything, "u1",
		mock.Anything).Return(providers.AccessDecision{Admitted: false}, &tidcommon.InternalServerError).Once()

	svcErr := s.authenticate(credentials)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.Equal(tidcommon.ServerErrorType, svcErr.Type)
}

// The scope reaching the authority is the one the dispatch selected, not the first key in the map.
// Getting this wrong reads a lock off the wrong counter, in both directions.
func (s *AccessTestSuite) TestTheScopeCheckedIsTheOneTheDispatchSelected() {
	credentials := map[string]interface{}{
		"password": "correct", authnprovidercm.CredentialTypeOTP: "123456",
	}
	s.otpSvc.On("Authenticate", mock.Anything, "session-1", "123456").
		Return(&authncommon.AuthnResult{
			Token: map[string]interface{}{authnprovidercm.UserAttributeUserID: "u1"},
		}, (*otp.VerificationFailure)(nil), (*tidcommon.ServiceError)(nil)).Once()
	s.entitySvc.On("GetEntityProfile", mock.Anything, "u1").
		Return(&providers.Entity{ID: "u1", Attributes: json.RawMessage(`{}`)}, nil).Once()
	s.governance.On("AdmitAuthenticationStep", mock.Anything, "u1",
		providers.AccessScopeOTP).
		Return(providers.AccessDecision{Admitted: true}, nil).Once()

	credentials[authnprovidercm.CredentialTypeOTP] = map[string]interface{}{
		"sessionToken": "session-1", "otp": "123456",
	}
	s.Nil(s.authenticate(credentials))
}

// A subject with no entity yet is the provisioning path, not a refusal; it is checked later at P2.
func (s *AccessTestSuite) TestASubjectWithNoEntityYetIsNotChecked() {
	credentials := map[string]interface{}{"password": "correct"}
	s.entitySvc.On("AuthenticateEntity", mock.Anything, mock.Anything, credentials).
		Return(&entity.AuthenticateResult{EntityID: ""}, (*entity.CredentialFailure)(nil), nil).Once()
	s.entitySvc.On("IdentifyEntity", mock.Anything, mock.Anything).
		Return(nil, entity.ErrEntityNotFound).Once()

	s.Nil(s.authenticate(credentials))
	s.assertNothingChecked()
}

// A wrong password is counted once, against the compared identity under the dispatched scope. The
// client gets the ordinary refusal, and nothing is checked or reset.
func (s *AccessTestSuite) TestAWrongPasswordIsCountedOnce() {
	credentials := map[string]interface{}{"password": "wrong"}
	s.expectProvenMismatch(credentials)
	s.governance.On("RecordFailure", mock.Anything, "u1",
		providers.AccessScopeCredential).Return(nil).Once()

	svcErr := s.authenticate(credentials)
	s.Require().NotNil(svcErr)
	s.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, svcErr.Code)
	s.assertNothingChecked()
}

// N attributable failures produce exactly N increments: one per attempt, never per candidate, per
// credential key or per retry.
func (s *AccessTestSuite) TestEachAttemptCountsExactlyOnce() {
	credentials := map[string]interface{}{"password": "wrong"}
	const attempts = 4
	for range attempts {
		s.expectProvenMismatch(credentials)
	}
	s.governance.On("RecordFailure", mock.Anything, "u1",
		providers.AccessScopeCredential).Return(nil).Times(attempts)

	for range attempts {
		s.Require().NotNil(s.authenticate(credentials))
	}
}

// An OTP failure is counted against the recipient from the validated session, not the request's own
// identifiers, so a request cannot choose whose account its failures hit.
func (s *AccessTestSuite) TestAnOTPFailureIsCountedAgainstItsSessionRecipient() {
	credentials := map[string]interface{}{
		authnprovidercm.CredentialTypeOTP: map[string]interface{}{
			"sessionToken": "session-1", "otp": "000000",
		},
	}
	s.otpSvc.On("Authenticate", mock.Anything, "session-1", "000000").
		Return(nil, &otp.VerificationFailure{
			RecipientAttr: "mobile_number", Recipient: "+1234567890",
		}, &otp.ErrorIncorrectOTP).Once()
	recipientID := "recipient-1"
	s.entitySvc.On("IdentifyEntity", mock.Anything,
		map[string]interface{}{"mobile_number": "+1234567890"}).
		Return(&recipientID, nil).Once()
	s.governance.On("RecordFailure", mock.Anything, "recipient-1",
		providers.AccessScopeOTP).Return(nil).Once()

	s.Require().NotNil(s.authenticate(credentials))
}

// A recipient attribute that is the user id is the subject as it stands, with no attribute lookup.
// A lookup would match nothing, and no OTP failure in the shipped SMS flow would count.
func (s *AccessTestSuite) TestARecipientNamedByUserIDIsTheSubject() {
	credentials := map[string]interface{}{
		authnprovidercm.CredentialTypeOTP: map[string]interface{}{
			"sessionToken": "session-1", "otp": "000000",
		},
	}
	s.otpSvc.On("Authenticate", mock.Anything, "session-1", "000000").
		Return(nil, &otp.VerificationFailure{
			RecipientAttr: authnprovidercm.UserAttributeUserID, Recipient: "u1",
		}, &otp.ErrorIncorrectOTP).Once()
	s.governance.On("RecordFailure", mock.Anything, "u1",
		providers.AccessScopeOTP).Return(nil).Once()

	s.Require().NotNil(s.authenticate(credentials))
	s.entitySvc.AssertNotCalled(s.T(), "IdentifyEntity", mock.Anything, mock.Anything)
}

// A recipient that resolves to nobody, or to several, counts nothing.
func (s *AccessTestSuite) TestAnUnresolvableSubjectCountsNothing() {
	credentials := map[string]interface{}{
		authnprovidercm.CredentialTypeOTP: map[string]interface{}{
			"sessionToken": "session-1", "otp": "000000",
		},
	}
	s.otpSvc.On("Authenticate", mock.Anything, "session-1", "000000").
		Return(nil, &otp.VerificationFailure{
			RecipientAttr: "mobile_number", Recipient: "+15550000000",
		}, &otp.ErrorIncorrectOTP).Once()
	s.entitySvc.On("IdentifyEntity", mock.Anything, mock.Anything).
		Return(nil, entity.ErrEntityNotFound).Once()

	s.Require().NotNil(s.authenticate(credentials))
	s.assertNothingRecorded()
}

// A refusal that established no proof counts nothing, and costs no lookup.
func (s *AccessTestSuite) TestARefusalWithoutEvidenceCountsNothing() {
	credentials := map[string]interface{}{"password": "wrong"}
	s.entitySvc.On("AuthenticateEntity", mock.Anything, mock.Anything, credentials).
		Return(nil, (*entity.CredentialFailure)(nil), entity.ErrAuthenticationFailed).Once()

	s.Require().NotNil(s.authenticate(credentials))
	s.assertNothingRecorded()
	s.entitySvc.AssertNotCalled(s.T(), "IdentifyEntity", mock.Anything, mock.Anything)
}

// A server fault, an invalid request or an unknown subject counts nothing even with evidence; none
// is a failed proof, and counting them would let an outage lock accounts.
func (s *AccessTestSuite) TestANonProofErrorCountsNothing() {
	for name, err := range map[string]error{
		"unknown subject": entity.ErrEntityNotFound,
		"server fault":    assertAnError(),
	} {
		s.Run(name, func() {
			entitySvc := entitymock.NewEntityServiceInterfaceMock(s.T())
			governance := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(s.T())
			provider := Initialize(entitySvc, nil, nil, nil, nil, nil, governance)

			credentials := map[string]interface{}{"password": "wrong"}
			entitySvc.On("AuthenticateEntity", mock.Anything, mock.Anything, credentials).
				Return(nil, &entity.CredentialFailure{EntityID: "u1"}, err).Once()

			_, svcErr := provider.Authenticate(s.ctx,
				map[string]interface{}{"username": "alice"}, credentials, nil)
			s.Require().NotNil(svcErr)
			governance.AssertNotCalled(s.T(), "RecordFailure",
				mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// A step that verifies no secret counts nothing and checks nothing. Provisioning an entity earlier
// in the same flow is not an attempt at a secret.
func (s *AccessTestSuite) TestAStepThatVerifiesNoSecretIsNeitherCountedNorChecked() {
	credentials := map[string]interface{}{
		authnprovidercm.CredentialTypeProvisionedEntityID: "u1",
	}
	s.entitySvc.On("GetEntityProfile", mock.Anything, "u1").
		Return(&providers.Entity{ID: "u1", Attributes: json.RawMessage(`{}`)}, nil).Once()

	s.Nil(s.authenticate(credentials))
	s.assertNothingRecorded()
	s.assertNothingChecked()
}

// A failed count does not change the answer the client gets. The authentication was already refused;
// answering differently on the attempt that could not be counted would say which attempt mattered.
func (s *AccessTestSuite) TestAFailedCountDoesNotChangeTheRefusal() {
	credentials := map[string]interface{}{"password": "wrong"}
	s.expectProvenMismatch(credentials)
	s.governance.On("RecordFailure", mock.Anything, "u1",
		providers.AccessScopeCredential).Return(&tidcommon.InternalServerError).Once()

	svcErr := s.authenticate(credentials)
	s.Require().NotNil(svcErr)
	s.Equal(authnprovidercm.ErrorCodeAuthenticationFailed, svcErr.Code)
}

// Evidence never travels outwards. A refusal returns no result at all, so nothing about the subject
// of a failed attempt can become an entity reference, an attribute or a claim.
func (s *AccessTestSuite) TestARefusalCarriesNoResult() {
	credentials := map[string]interface{}{"password": "wrong"}
	s.expectProvenMismatch(credentials)
	s.governance.On("RecordFailure", mock.Anything, "u1", mock.Anything).Return(nil).Once()

	result, svcErr := s.provider.Authenticate(s.ctx,
		map[string]interface{}{"username": "alice"}, credentials, nil)
	s.Require().NotNil(svcErr)
	s.Nil(result)
}

// No governance at all counts nothing, checks nothing and performs no extra lookup. A wrong password
// in a deployment that asked for none of this costs exactly what it cost before.
func (s *AccessTestSuite) TestNoGovernanceCostsNoLookup() {
	entitySvc := entitymock.NewEntityServiceInterfaceMock(s.T())
	provider := Initialize(entitySvc, nil, nil, nil, nil, nil, nil)

	credentials := map[string]interface{}{"password": "wrong"}
	entitySvc.On("AuthenticateEntity", mock.Anything, mock.Anything, credentials).
		Return(nil, &entity.CredentialFailure{EntityID: "u1"}, entity.ErrAuthenticationFailed).Once()

	_, svcErr := provider.Authenticate(s.ctx,
		map[string]interface{}{"username": "alice"}, credentials, nil)
	s.Require().NotNil(svcErr)
	entitySvc.AssertNotCalled(s.T(), "IdentifyEntity", mock.Anything, mock.Anything)
}

// Enrollment is not an authentication attempt: nothing it refuses is counted, and nothing it
// completes is checked here.
func (s *AccessTestSuite) TestEnrollmentIsNeitherCountedNorChecked() {
	credentials := map[string]interface{}{authnprovidercm.CredentialTypePasskey: "registration"}

	_, svcErr := s.provider.Enroll(s.ctx, map[string]interface{}{"username": "alice"},
		credentials, nil)
	s.Require().NotNil(svcErr)
	s.assertNothingRecorded()
	s.assertNothingChecked()
}

// assertAnError is a plain operational fault, distinct from the classified authentication errors.
func assertAnError() error {
	return errAnOperationalFault
}

var errAnOperationalFault = errors.New("the authority is unreachable")
