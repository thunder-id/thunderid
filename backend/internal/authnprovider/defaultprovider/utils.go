// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package defaultprovider

import (
	"context"
	"slices"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// credentialAuthenticationMethod returns the method a credential comparison is checked under. It is the
// system credential method only when every credential is a system credential.
func credentialAuthenticationMethod(credentials map[string]interface{}) providers.AuthenticationMethod {
	for key := range credentials {
		if !slices.Contains(authnprovidercm.SystemCredentialTypes, key) {
			return providers.AuthenticationMethodCredential
		}
	}
	return providers.AuthenticationMethodSystemCredential
}

// otpFailureOf turns a wrong one-time password into evidence naming the code's recipient.
func otpFailureOf(failure *otp.VerificationFailure) *attributedFailure {
	if failure == nil {
		return nil
	}
	return &attributedFailure{
		recipient: map[string]interface{}{failure.RecipientAttr: failure.Recipient},
	}
}

// credentialFailureOf turns the entity whose stored credential did not match into failure evidence, or
// nil when no comparison failed.
func credentialFailureOf(mismatchedEntityID string) *attributedFailure {
	if mismatchedEntityID == "" {
		return nil
	}
	return &attributedFailure{entityID: mismatchedEntityID}
}

// accessRefusal maps a governance answer to the authentication error, or nil when the identity is
// admitted. A governance failure is a server error.
func accessRefusal(decision providers.AccessDecision, svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr != nil {
		return &tidcommon.InternalServerError
	}
	if !decision.Admitted {
		return refusal(decision)
	}
	return nil
}

// proofPlane returns the plane a proof is submitted at: recovery inside a recovery flow, authentication
// otherwise.
func proofPlane(ctx context.Context) providers.AccessPlane {
	if authnprovidercm.IsRecoveryFlow(ctx) {
		return providers.AccessPlaneRecovery
	}
	return providers.AccessPlaneAuthentication
}

// refusal maps a governance refusal to the locked or suspended error, or to the authentication failure
// when it names no hold.
func refusal(decision providers.AccessDecision) *tidcommon.ServiceError {
	switch decision.Reason {
	case providers.AccessRefusalReasonSuspended:
		return newClientError(authnprovidercm.ErrorCodeAccountSuspended,
			"Account suspended", "The account is suspended")
	case providers.AccessRefusalReasonLocked:
		return newClientError(authnprovidercm.ErrorCodeAccountLocked,
			"Account locked", "The account is locked")
	default:
		return newClientError(authnprovidercm.ErrorCodeAuthenticationFailed,
			"Authentication failed", "Invalid credentials provided")
	}
}

// passkeyEnrollmentRefusal maps a passkey enrolment refused for a suspension to the suspended error, or
// returns nil for any other error.
func passkeyEnrollmentRefusal(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr.Code != authncommon.ErrorAccountSuspended.Code {
		return nil
	}
	return refusal(providers.AccessDecision{Reason: providers.AccessRefusalReasonSuspended})
}
