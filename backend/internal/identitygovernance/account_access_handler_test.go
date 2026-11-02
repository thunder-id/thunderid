// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type AccountAccessHandlerTestSuite struct {
	suite.Suite
	// handler has no base; precedence has a representative deployment base.
	handler    accountAccessHandler
	precedence accountAccessHandler
}

func TestAccountAccessHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(AccountAccessHandlerTestSuite))
}

func (s *AccountAccessHandlerTestSuite) SetupTest() {
	s.handler = accountAccessHandler{}
	s.precedence = accountAccessHandler{base: testAccountAccessConfig()}
}

// mergedOverBase merges the layers over the representative base.
func (s *AccountAccessHandlerTestSuite) mergedOverBase(readOnly,
	writable accountAccessValue) accountAccessValue {
	value, ok := s.precedence.Merge(readOnly, writable).(accountAccessValue)
	s.Require().True(ok)
	return value
}

// The exported constructor builds the handler over the deployment configuration.
func (s *AccountAccessHandlerTestSuite) TestNewAccountAccessHandlerMergesOverTheDeploymentBase() {
	handler := NewAccountAccessHandler(Config{AccountAccess: testAccountAccessConfig()})

	merged, ok := handler.Merge(nil, accountAccessValue{}).(accountAccessValue)

	s.Require().True(ok)
	s.Equal(testAccountAccessConfig(), merged)
}

func (s *AccountAccessHandlerTestSuite) decode(raw string) accountAccessValue {
	value, err := s.handler.Decode(json.RawMessage(raw))
	s.Require().NoError(err)
	cfg, ok := value.(accountAccessValue)
	s.Require().True(ok)
	return cfg
}

func (s *AccountAccessHandlerTestSuite) TestAnEmptyLayerDecodesToAnAbsentSection() {
	value, err := s.handler.Decode(nil)
	s.Require().NoError(err)
	s.Equal(accountAccessValue{}, value)
}

// Unlike other sections, unknown fields are rejected.
func (s *AccountAccessHandlerTestSuite) TestAnUnknownFieldIsRejected() {
	_, err := s.handler.Decode(json.RawMessage(`{"user":{"treshold":5}}`))
	s.Require().Error(err)
}

func (s *AccountAccessHandlerTestSuite) TestAnApplicationPolicyIsRejected() {
	_, err := s.handler.Decode(json.RawMessage(`{"application":{"default":{"enabled":true}}}`))
	s.Require().Error(err)
}

// Per-method overrides are not exposed, so a per-method block is an unknown field.
func (s *AccountAccessHandlerTestSuite) TestAPerMethodBlockIsRejected() {
	_, err := s.handler.Decode(json.RawMessage(`{"user":{"scopes":{"otp":{"threshold":3}}}}`))
	s.Require().Error(err)
	s.Contains(err.Error(), `unknown field "scopes"`)
}

// Disclosure is fixed in code, so there is no setting for it.
func (s *AccountAccessHandlerTestSuite) TestADisclosureSettingIsRejected() {
	_, err := s.handler.Decode(json.RawMessage(`{"user":{"default":{"discloseAccountHold":"always"}}}`))
	s.Require().Error(err)
	s.Contains(err.Error(), `unknown field "discloseAccountHold"`)
}

// The lock email goes only to the user type's primary email, so there is no recipient setting.
func (s *AccountAccessHandlerTestSuite) TestARecipientAttributeIsRejected() {
	_, err := s.handler.Decode(json.RawMessage(
		`{"user":{"notifications":{"onLock":{"email":{"recipientAttribute":"email"}}}}}`))
	s.Require().Error(err)
	s.Contains(err.Error(), `unknown field "recipientAttribute"`)
}

// The lock email templates are set through the section: a default pair, the account-wide lock's and
// per-method overrides.
func (s *AccountAccessHandlerTestSuite) TestTheLockEmailTemplatesAreConfigurableThroughTheSection() {
	incoming := s.decode(`{"user":{"notifications":{"onLock":{"email":{"templates":{` +
		`"temporary":"authentication-method-locked","permanent":"authentication-method-locked-permanent",` +
		`"account":{"permanent":"account-locked-permanent"},"methods":{"otp":{"temporary":"otp-locked"}}}}}}}}`)
	s.Require().NoError(s.handler.Validate(incoming, nil, nil))

	s.Equal(&accountAccessLockTemplates{
		lockEmailTemplatePair: lockEmailTemplatePair{
			Temporary: "authentication-method-locked", Permanent: "authentication-method-locked-permanent"},
		Account: &lockEmailTemplatePair{Permanent: "account-locked-permanent"},
		Methods: map[string]lockEmailTemplatePair{"otp": {Temporary: "otp-locked"}},
	}, incoming.User.Notifications.lockEmail().Templates)
}

// The templates block has only its own keys, so the old flat keys are unknown fields.
func (s *AccountAccessHandlerTestSuite) TestAnUnknownTemplateKeyIsRejected() {
	for _, email := range []string{
		`{"template":"authentication-method-locked"}`,
		`{"templates":{"expiring":"authentication-method-locked"}}`,
		`{"templates":{"methods":{"otp":{"template":"otp-locked"}}}}`,
	} {
		_, err := s.handler.Decode(json.RawMessage(`{"user":{"notifications":{"onLock":{"email":` + email + `}}}}`))
		s.Require().Error(err, email)
		s.Contains(err.Error(), "unknown field", email)
	}
}

// A template handle must be a valid handle, and a method override must name a lockable method; the
// refusal lists the methods that can be set.
func (s *AccountAccessHandlerTestSuite) TestAnInvalidLockTemplateIsRejected() {
	const path = "user.notifications.onLock.email.templates"
	for want, templates := range map[string]string{
		path + ".temporary is not a valid handle":             `{"temporary":"Not A Handle"}`,
		path + ".permanent is not a valid handle":             `{"permanent":"a/b"}`,
		path + ".account.temporary is not a valid handle":     `{"account":{"temporary":"Not A Handle"}}`,
		path + ".methods.otp.permanent is not a valid handle": `{"methods":{"otp":{"permanent":"Not A Handle"}}}`,
		path + ".methods.passkey is not a lockable authentication method; use credential or otp": `{` +
			`"methods":{"passkey":{"temporary":"passkey-locked"}}}`,
		path + ".methods.account is not a lockable authentication method; use credential or otp": `{` +
			`"methods":{"account":{"temporary":"authentication-method-locked"}}}`,
	} {
		incoming := s.decode(`{"user":{"notifications":{"onLock":{"email":{"templates":` + templates + `}}}}}`)
		err := s.handler.Validate(incoming, nil, nil)
		s.Require().Error(err, want)
		s.Contains(err.Error(), want)
	}
}

func (s *AccountAccessHandlerTestSuite) TestAnUnknownGranularityIsRejected() {
	incoming := s.decode(`{"user":{"lockGranularity":"entitiy"}}`)
	s.Require().Error(s.handler.Validate(incoming, nil, nil))
}

func (s *AccountAccessHandlerTestSuite) TestEachKnownGranularityIsAccepted() {
	for _, value := range []granularity{granularityAuthenticationMethod, granularityAccount} {
		incoming := s.decode(`{"user":{"lockGranularity":"` + string(value) + `"}}`)
		s.Require().NoError(s.handler.Validate(incoming, nil, nil), value)
	}
}

// A default that states nothing configures no lock, so it is not checked for one.
func (s *AccountAccessHandlerTestSuite) TestADefaultStatingNothingIsAccepted() {
	s.Require().NoError(s.handler.Validate(s.decode(`{"user":{"default":{}}}`), nil, nil))
}

func (s *AccountAccessHandlerTestSuite) TestAZeroDurationIsAccepted() {
	// Zero means a permanent lock.
	incoming := s.decode(
		`{"user":{"default":{"enabled":true,"threshold":5,"failureWindowSeconds":900,"lockDurationsSeconds":[300,0]}}}`)
	s.Require().NoError(s.handler.Validate(incoming, nil, nil))
}

// A value that would overflow time.Duration is refused at write time.
func (s *AccountAccessHandlerTestSuite) TestADurationTooLargeToCarryIsRejected() {
	const complete = `{"user":{"default":{"enabled":true,"threshold":5,` +
		`"failureWindowSeconds":%d,"lockDecaySeconds":%d,"lockDurationsSeconds":[%d]}}}`

	cases := []struct {
		field    string
		document string
	}{
		{"failureWindowSeconds", fmt.Sprintf(complete, maxDurationSeconds+1, 86400, 300)},
		{"lockDecaySeconds", fmt.Sprintf(complete, 900, maxDurationSeconds+1, 300)},
		{"lockDurationsSeconds", fmt.Sprintf(complete, 900, 86400, maxDurationSeconds+1)},
		{"lastLoginResolutionSeconds",
			fmt.Sprintf(`{"user":{"lastLoginResolutionSeconds":%d}}`, maxDurationSeconds+1)},
	}
	for _, c := range cases {
		s.Run(c.field, func() {
			err := s.handler.Validate(s.decode(c.document), nil, nil)
			s.Require().Error(err)
			s.Contains(err.Error(), c.field)
		})
	}

	// The bound itself is accepted.
	s.Require().NoError(s.handler.Validate(
		s.decode(fmt.Sprintf(complete, maxDurationSeconds, maxDurationSeconds, maxDurationSeconds)),
		nil, nil))
	s.Require().NoError(s.handler.Validate(
		s.decode(fmt.Sprintf(`{"user":{"lastLoginResolutionSeconds":%d}}`, maxDurationSeconds)),
		nil, nil))
}

// An enabled policy that can never form a lock is refused.
func (s *AccountAccessHandlerTestSuite) TestAPolicyThatCannotFormALockIsRejected() {
	incoming := s.decode(`{"user":{"default":{"enabled":true,"threshold":5}}}`)
	err := s.handler.Validate(incoming, nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "failureWindowSeconds")
	s.Contains(err.Error(), "lockDurationsSeconds")
}

// A policy that says it is enabled and states nothing else cannot form a lock, so it is refused.
func (s *AccountAccessHandlerTestSuite) TestAnEnabledPolicyStatingNothingElseIsRejected() {
	err := s.handler.Validate(s.decode(`{"user":{"default":{"enabled":true}}}`), nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "threshold")
	s.Contains(err.Error(), "failureWindowSeconds")
	s.Contains(err.Error(), "lockDurationsSeconds")
}

// The consecutive ceiling may be zero (none) or at least the threshold; a negative one, or one below
// the threshold that would make the first lock permanent, is refused.
func (s *AccountAccessHandlerTestSuite) TestTheConsecutiveCeilingIsValidated() {
	policy := func(ceiling int) string {
		return fmt.Sprintf(`{"user":{"default":{"enabled":true,"threshold":5,"failureWindowSeconds":900,`+
			`"lockDurationsSeconds":[300],"maxConsecutiveFailures":%d}}}`, ceiling)
	}
	for _, accepted := range []int{0, 5, 50} {
		s.NoError(s.handler.Validate(s.decode(policy(accepted)), nil, nil), "ceiling %d", accepted)
	}
	for _, refused := range []int{-1, 4} {
		err := s.handler.Validate(s.decode(policy(refused)), nil, nil)
		s.Require().Error(err, "ceiling %d", refused)
		s.Contains(err.Error(), "maxConsecutiveFailures")
	}
}

// A writable layer that states only the ceiling is checked against the base's threshold and keeps
// every other base field.
func (s *AccountAccessHandlerTestSuite) TestTheConsecutiveCeilingIsLaidOverTheBase() {
	s.Require().NoError(s.precedence.Validate(
		s.decode(`{"user":{"default":{"maxConsecutiveFailures":20}}}`), accountAccessValue{}, accountAccessValue{}))
	s.Require().Error(s.precedence.Validate(
		s.decode(`{"user":{"default":{"maxConsecutiveFailures":3}}}`), accountAccessValue{}, accountAccessValue{}),
		"the base threshold is 5")

	merged := s.mergedOverBase(accountAccessValue{}, s.decode(`{"user":{"default":{"maxConsecutiveFailures":20}}}`))
	s.Equal(20, *merged.User.Default.MaxConsecutiveFailures)
	s.Equal(5, *merged.User.Default.Threshold, "an unstated field keeps the base value")
}

// A window of zero means the counter never resets on time, so the policy can still form a lock.
func (s *AccountAccessHandlerTestSuite) TestAZeroWindowIsAccepted() {
	incoming := s.decode(
		`{"user":{"default":{"enabled":true,"threshold":5,"failureWindowSeconds":0,"lockDurationsSeconds":[300]}}}`)
	s.Require().NoError(s.handler.Validate(incoming, nil, nil))
}

func (s *AccountAccessHandlerTestSuite) TestAnIncompleteWritableLayerIsCoherentAgainstTheBase() {
	handler := accountAccessHandler{base: s.decode(
		`{"user":{"default":{"enabled":true,"threshold":5,"failureWindowSeconds":900,` +
			`"lockDurationsSeconds":[300]}}}`)}
	// Only a threshold; the base supplies the rest.
	incoming := s.decode(`{"user":{"default":{"threshold":3}}}`)
	s.Require().NoError(handler.Validate(incoming, nil, nil))
}

func (s *AccountAccessHandlerTestSuite) TestSwitchingACategoryOffNeedsNoOtherField() {
	incoming := s.decode(`{"agent":{"default":{"enabled":false}}}`)
	s.Require().NoError(s.handler.Validate(incoming, nil, nil))
}

func (s *AccountAccessHandlerTestSuite) TestAWritableCategoryDoesNotDeleteAnotherCategory() {
	handler := accountAccessHandler{base: s.decode(
		`{"user":{"default":{"threshold":5}},"agent":{"default":{"threshold":10}}}`)}
	writable := s.decode(`{"user":{"default":{"threshold":3}}}`)

	merged := handler.Merge(nil, writable).(accountAccessValue)
	s.Require().NotNil(merged.Agent)
	s.Equal(10, *merged.Agent.Default.Threshold)
	s.Equal(3, *merged.User.Default.Threshold)
}

func (s *AccountAccessHandlerTestSuite) TestTheWritableLayerCanStateAFalseThatTheBaseDoesNot() {
	// A pointer field lets an explicit false differ from unset.
	handler := accountAccessHandler{base: s.decode(`{"user":{"default":{"enabled":true,"threshold":5,` +
		`"failureWindowSeconds":900,"lockDurationsSeconds":[300]}}}`)}
	writable := s.decode(`{"user":{"default":{"enabled":false}}}`)

	merged := handler.Merge(nil, writable).(accountAccessValue)
	s.False(*merged.User.Default.Enabled)
	s.Equal(5, *merged.User.Default.Threshold)
}

func (s *AccountAccessHandlerTestSuite) TestNoBaseAndNoSectionHasNoPolicy() {
	empty := accountAccessValue{}
	merged, ok := accountAccessHandler{}.Merge(empty, empty).(accountAccessValue)
	s.Require().True(ok)
	s.Nil(merged.User)
	s.Nil(merged.Agent)
}

func (s *AccountAccessHandlerTestSuite) TestTheBaseAppliesWithNoSection() {
	merged := s.mergedOverBase(accountAccessValue{}, accountAccessValue{})
	s.Equal(5, *merged.User.Default.Threshold)
	s.Equal(s.precedence.base, merged)
}

func (s *AccountAccessHandlerTestSuite) TestWritableOverridesTheBase() {
	merged := s.mergedOverBase(accountAccessValue{}, s.decode(`{"user":{"default":{"threshold":2}}}`))
	s.Equal(2, *merged.User.Default.Threshold)
	s.Equal(900, *merged.User.Default.FailureWindowSeconds)
	s.Equal([]int{300, 900, 3600, 86400}, merged.User.Default.LockDurationsSeconds)
}

func (s *AccountAccessHandlerTestSuite) TestPartialWriteValidatesAgainstTheBase() {
	s.Require().NoError(s.precedence.Validate(
		s.decode(`{"user":{"default":{"threshold":3}}}`), accountAccessValue{},
		accountAccessValue{}))
}

func (s *AccountAccessHandlerTestSuite) TestIncompleteEnabledPolicyWithoutABaseIsRefused() {
	value := s.decode(`{"user":{"default":{"enabled":true,"threshold":3}}}`)
	s.Require().Error(accountAccessHandler{}.Validate(value, nil, nil))
}

func (s *AccountAccessHandlerTestSuite) TestWritableCanDisableTheBasePolicy() {
	merged := s.mergedOverBase(accountAccessValue{},
		s.decode(`{"user":{"default":{"enabled":false}}}`))
	s.False(*merged.User.Default.Enabled)
}

// A declarative document wins over the writable layer and is laid over the base.
func (s *AccountAccessHandlerTestSuite) TestADeclarativeDocumentWinsOverTheWritableLayer() {
	merged := s.mergedOverBase(s.decode(`{"user":{"default":{"threshold":7}}}`),
		s.decode(`{"user":{"default":{"threshold":2}}}`))
	s.Equal(7, *merged.User.Default.Threshold)
	s.Equal(900, *merged.User.Default.FailureWindowSeconds, "the base supplies what the document does not state")
	s.Require().NotNil(merged.Agent, "a category the document does not name keeps the base")
}

// The lock email templates come from the layer in force: a declarative document's replace the
// writable layer's rather than merging with them, and without one the writable layer's apply.
func (s *AccountAccessHandlerTestSuite) TestMergeTakesTheLockEmailTemplatesOfTheLayerInForce() {
	declarative := s.decode(`{"user":{"notifications":{"onLock":{"email":{"templates":{` +
		`"temporary":"file-locked","account":{"permanent":"file-account-locked-permanent"}}}}}}}`)
	writable := s.decode(`{"user":{"notifications":{"onLock":{"email":{"enabled":true,"templates":{` +
		`"temporary":"db-locked","permanent":"db-locked-permanent",` +
		`"methods":{"otp":{"temporary":"db-otp-locked"}}}}}}}}`)

	merged := s.mergedOverBase(declarative, writable)
	s.Equal(&accountAccessLockTemplates{
		lockEmailTemplatePair: lockEmailTemplatePair{Temporary: "file-locked"},
		Account:               &lockEmailTemplatePair{Permanent: "file-account-locked-permanent"},
	}, merged.User.Notifications.lockEmail().Templates, "no writable template may fill a declarative gap")
	s.Nil(merged.User.Notifications.lockEmail().Enabled, "the writable layer's switch must not apply")

	merged = s.mergedOverBase(accountAccessValue{}, writable)
	s.Equal(writable.User.Notifications.lockEmail(), merged.User.Notifications.lockEmail())
	s.Equal(5, *merged.User.Default.Threshold, "the base still supplies the policy")
}

// While a declarative document exists the section is locked, so a write cannot be a silent no-op.
func (s *AccountAccessHandlerTestSuite) TestADeclarativeDocumentRefusesWrites() {
	err := s.precedence.Validate(s.decode(`{"user":{"default":{"threshold":2}}}`),
		s.decode(`{"user":{"default":{"threshold":7}}}`), accountAccessValue{})
	s.Require().ErrorIs(err, errDeclarativeAccountAccessLocked)
}

// At load time a partial declarative document is checked laid over the base.
func (s *AccountAccessHandlerTestSuite) TestTheDeclarativeLoadValidatesAgainstTheBase() {
	s.Require().NoError(s.precedence.Validate(s.decode(`{"user":{"default":{"threshold":7}}}`), nil, nil))
}

// Last-login settings are configurable through the section.
func (s *AccountAccessHandlerTestSuite) TestLastLoginSettingsAreConfigurableThroughTheSection() {
	incoming := s.decode(`{"user":{"recordLastLogin":true,"lastLoginResolutionSeconds":300},` +
		`"agent":{"recordLastLogin":false}}`)
	s.Require().NoError(s.handler.Validate(incoming, nil, nil))

	s.True(*incoming.User.RecordLastLogin)
	s.Equal(300, *incoming.User.LastLoginResolutionSeconds)
	s.False(*incoming.Agent.RecordLastLogin,
		"an explicit false must survive decoding, or a category could not be switched back off")
}

// A negative resolution is refused at write time rather than clamped.
func (s *AccountAccessHandlerTestSuite) TestANegativeLastLoginResolutionIsRejected() {
	incoming := s.decode(`{"user":{"lastLoginResolutionSeconds":-1}}`)
	err := s.handler.Validate(incoming, nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "lastLoginResolutionSeconds")
}

func (s *AccountAccessHandlerTestSuite) TestAWrongValueTypeIsRefused() {
	err := s.handler.Validate("not a section", nil, nil)

	s.Require().Error(err)
	s.Contains(err.Error(), "unexpected config type string")
}

// A writable layer of the wrong type merges as an absent layer, leaving the base.
func (s *AccountAccessHandlerTestSuite) TestAWrongWritableTypeMergesAsAbsent() {
	s.Equal(s.mergedOverBase(accountAccessValue{}, accountAccessValue{}),
		s.precedence.Merge(nil, "not a section"))
}

// Each negative count and seconds value is refused, naming the field. Policy resolution would silently
// drop a negative duration.
func (s *AccountAccessHandlerTestSuite) TestNegativeValuesAreRejected() {
	for path, document := range map[string]string{
		"user.default.threshold":               `{"user":{"default":{"threshold":-1}}}`,
		"agent.default.maxConsecutiveFailures": `{"agent":{"default":{"maxConsecutiveFailures":-1}}}`,
		"user.default.failureWindowSeconds":    `{"user":{"default":{"failureWindowSeconds":-1}}}`,
		"user.default.lockDecaySeconds":        `{"user":{"default":{"lockDecaySeconds":-1}}}`,
		"user.default.lockDurationsSeconds[1]": `{"user":{"default":{"lockDurationsSeconds":[300,-1]}}}`,
	} {
		err := s.handler.Validate(s.decode(document), nil, nil)
		s.Require().Error(err, path)
		s.Contains(err.Error(), path+" must be >= 0")
	}
}
