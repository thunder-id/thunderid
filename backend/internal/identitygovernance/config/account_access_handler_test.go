// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type AccountAccessHandlerTestSuite struct {
	suite.Suite
	handler AccountAccessHandler
}

func TestAccountAccessHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(AccountAccessHandlerTestSuite))
}

func (s *AccountAccessHandlerTestSuite) SetupTest() {
	// Layer precedence is tested in AccountAccessPrecedenceTestSuite.
	s.handler = AccountAccessHandler{}
}

func (s *AccountAccessHandlerTestSuite) decode(raw string) AccountAccessValue {
	value, err := s.handler.Decode(json.RawMessage(raw))
	s.Require().NoError(err)
	cfg, ok := value.(AccountAccessValue)
	s.Require().True(ok)
	return cfg
}

func (s *AccountAccessHandlerTestSuite) TestAnEmptyLayerDecodesToAnAbsentSection() {
	value, err := s.handler.Decode(nil)
	s.Require().NoError(err)
	s.Equal(AccountAccessValue{}, value)
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

func (s *AccountAccessHandlerTestSuite) TestAnUnknownSignInMethodIsRejected() {
	incoming := s.decode(`{"user":{"scopes":{"telepathy":{"threshold":3}}}}`)
	err := s.handler.Validate(incoming, nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "telepathy")
}

func (s *AccountAccessHandlerTestSuite) TestAnUnknownGranularityIsRejected() {
	incoming := s.decode(`{"user":{"lockGranularity":"entitiy"}}`)
	s.Require().Error(s.handler.Validate(incoming, nil, nil))
}

func (s *AccountAccessHandlerTestSuite) TestANegativeDurationIsRejected() {
	// The resolver would silently drop a negative duration.
	incoming := s.decode(`{"user":{"default":{"lockDurationsSeconds":[300,-1]}}}`)
	s.Require().Error(s.handler.Validate(incoming, nil, nil))
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

func (s *AccountAccessHandlerTestSuite) TestAnIncompleteWritableLayerIsCoherentAgainstTheBase() {
	handler := AccountAccessHandler{base: s.decode(
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

func (s *AccountAccessHandlerTestSuite) TestAnOverrideIsCheckedAsTheResolverWouldOverlayIt() {
	// The override sets only a threshold; the category default supplies the rest.
	incoming := s.decode(
		`{"user":{"default":{"enabled":true,"threshold":5,"failureWindowSeconds":900,` +
			`"lockDurationsSeconds":[300]},"scopes":{"otp":{"threshold":3}}}}`)
	s.Require().NoError(s.handler.Validate(incoming, nil, nil))
}

// Scopes merge as a union across layers.
func (s *AccountAccessHandlerTestSuite) TestAWritableScopeDoesNotDeleteABaseOne() {
	handler := AccountAccessHandler{base: s.decode(
		`{"user":{"scopes":{"credential":{"threshold":5},"otp":{"threshold":3}}}}`)}
	writable := s.decode(`{"user":{"scopes":{"otp":{"threshold":2}}}}`)

	merged, ok := handler.Merge(nil, writable).(AccountAccessValue)
	s.Require().True(ok)
	s.Require().NotNil(merged.User)
	s.Len(merged.User.Scopes, 2)
	s.Equal(5, *merged.User.Scopes["credential"].Threshold)
	s.Equal(2, *merged.User.Scopes["otp"].Threshold)
}

func (s *AccountAccessHandlerTestSuite) TestAnOverlaidScopeKeepsTheFieldsItDoesNotState() {
	handler := AccountAccessHandler{base: s.decode(
		`{"user":{"scopes":{"otp":{"threshold":3,"failureWindowSeconds":600}}}}`)}
	writable := s.decode(`{"user":{"scopes":{"otp":{"threshold":2}}}}`)

	merged := handler.Merge(nil, writable).(AccountAccessValue)
	s.Equal(2, *merged.User.Scopes["otp"].Threshold)
	s.Equal(600, *merged.User.Scopes["otp"].FailureWindowSeconds)
}

func (s *AccountAccessHandlerTestSuite) TestAWritableCategoryDoesNotDeleteAnotherCategory() {
	handler := AccountAccessHandler{base: s.decode(
		`{"user":{"default":{"threshold":5}},"agent":{"default":{"threshold":10}}}`)}
	writable := s.decode(`{"user":{"default":{"threshold":3}}}`)

	merged := handler.Merge(nil, writable).(AccountAccessValue)
	s.Require().NotNil(merged.Agent)
	s.Equal(10, *merged.Agent.Default.Threshold)
	s.Equal(3, *merged.User.Default.Threshold)
}

func (s *AccountAccessHandlerTestSuite) TestTheWritableLayerCanStateAFalseThatTheBaseDoesNot() {
	// A pointer field lets an explicit false differ from unset.
	handler := AccountAccessHandler{base: s.decode(`{"user":{"default":{"enabled":true,"threshold":5,` +
		`"failureWindowSeconds":900,"lockDurationsSeconds":[300]}}}`)}
	writable := s.decode(`{"user":{"default":{"enabled":false}}}`)

	merged := handler.Merge(nil, writable).(AccountAccessValue)
	s.False(*merged.User.Default.Enabled)
	s.Equal(5, *merged.User.Default.Threshold)
}

// AccountAccessPrecedenceTestSuite checks the order: the deployment base, then a declarative
// document when one exists (which locks the section), otherwise the writable layer.
type AccountAccessPrecedenceTestSuite struct {
	suite.Suite
	handler AccountAccessHandler
}

func TestAccountAccessPrecedenceTestSuite(t *testing.T) {
	suite.Run(t, new(AccountAccessPrecedenceTestSuite))
}

func (s *AccountAccessPrecedenceTestSuite) SetupTest() {
	s.handler = AccountAccessHandler{base: testAccountAccessConfig()}
}

func (s *AccountAccessPrecedenceTestSuite) decode(raw string) AccountAccessValue {
	value, err := s.handler.Decode(json.RawMessage(raw))
	s.Require().NoError(err)
	return value.(AccountAccessValue)
}

func (s *AccountAccessPrecedenceTestSuite) merged(readOnly, writable AccountAccessValue) AccountAccessValue {
	value, ok := s.handler.Merge(readOnly, writable).(AccountAccessValue)
	s.Require().True(ok)
	return value
}

func (s *AccountAccessPrecedenceTestSuite) TestNoBaseAndNoSectionHasNoPolicy() {
	merged, ok := AccountAccessHandler{}.Merge(AccountAccessValue{}, AccountAccessValue{}).(AccountAccessValue)
	s.Require().True(ok)
	s.Nil(merged.User)
	s.Nil(merged.Agent)
}

func (s *AccountAccessPrecedenceTestSuite) TestTheBaseAppliesWithNoSection() {
	merged := s.merged(AccountAccessValue{}, AccountAccessValue{})
	s.Equal(5, *merged.User.Default.Threshold)
	s.Equal(s.handler.Base(), merged)
}

func (s *AccountAccessPrecedenceTestSuite) TestWritableOverridesTheBase() {
	merged := s.merged(AccountAccessValue{}, s.decode(`{"user":{"default":{"threshold":2}}}`))
	s.Equal(2, *merged.User.Default.Threshold)
	s.Equal(900, *merged.User.Default.FailureWindowSeconds)
	s.Equal([]int{300, 900, 3600, 86400}, merged.User.Default.LockDurationsSeconds)
}

func (s *AccountAccessPrecedenceTestSuite) TestPartialWriteValidatesAgainstTheBase() {
	s.Require().NoError(s.handler.Validate(
		s.decode(`{"user":{"default":{"threshold":3}}}`), AccountAccessValue{}, AccountAccessValue{}))
}

func (s *AccountAccessPrecedenceTestSuite) TestIncompleteEnabledPolicyWithoutABaseIsRefused() {
	value := s.decode(`{"user":{"default":{"enabled":true,"threshold":3}}}`)
	s.Require().Error(AccountAccessHandler{}.Validate(value, nil, nil))
}

func (s *AccountAccessPrecedenceTestSuite) TestWritableCanDisableTheBasePolicy() {
	merged := s.merged(AccountAccessValue{}, s.decode(`{"user":{"default":{"enabled":false}}}`))
	s.False(*merged.User.Default.Enabled)
}

// A declarative document wins over the writable layer and is laid over the base.
func (s *AccountAccessPrecedenceTestSuite) TestADeclarativeDocumentWinsOverTheWritableLayer() {
	merged := s.merged(s.decode(`{"user":{"default":{"threshold":7}}}`),
		s.decode(`{"user":{"default":{"threshold":2}}}`))
	s.Equal(7, *merged.User.Default.Threshold)
	s.Equal(900, *merged.User.Default.FailureWindowSeconds, "the base supplies what the document does not state")
	s.Require().NotNil(merged.Agent, "a category the document does not name keeps the base")
}

// While a declarative document exists the section is locked, so a write cannot be a silent no-op.
func (s *AccountAccessPrecedenceTestSuite) TestADeclarativeDocumentRefusesWrites() {
	err := s.handler.Validate(s.decode(`{"user":{"default":{"threshold":2}}}`),
		s.decode(`{"user":{"default":{"threshold":7}}}`), AccountAccessValue{})
	s.Require().ErrorIs(err, errDeclarativeAccountAccessLocked)
}

// At load time a partial declarative document is checked laid over the base.
func (s *AccountAccessPrecedenceTestSuite) TestTheDeclarativeLoadValidatesAgainstTheBase() {
	s.Require().NoError(s.handler.Validate(s.decode(`{"user":{"default":{"threshold":7}}}`), nil, nil))
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

	// The setting reaches the resolver.
	activity := activityPolicyFor(categoryConfig(incoming, providers.EntityCategoryUser))
	s.True(activity.Record)
	s.Equal(300*time.Second, activity.Resolution)
}

// A negative resolution is refused at write time rather than clamped.
func (s *AccountAccessHandlerTestSuite) TestANegativeLastLoginResolutionIsRejected() {
	incoming := s.decode(`{"user":{"lastLoginResolutionSeconds":-1}}`)
	err := s.handler.Validate(incoming, nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "lastLoginResolutionSeconds")
}

// Last-login settings overlay field by field across layers.
func (s *AccountAccessHandlerTestSuite) TestLastLoginSettingsOverlayFieldByField() {
	readOnly := s.decode(`{"user":{"recordLastLogin":false,"lastLoginResolutionSeconds":3600}}`)
	writable := s.decode(`{"user":{"recordLastLogin":true}}`)

	merged := mergeAccountAccessValue(readOnly, writable)

	s.True(*merged.User.RecordLastLogin, "the writable layer wins where it states a value")
	s.Equal(3600, *merged.User.LastLoginResolutionSeconds,
		"and the layer beneath supplies what it does not state")
}

func (s *AccountAccessHandlerTestSuite) TestEffectiveLockEmailDefaultsWithoutChangingStoredLayer() {
	for _, recipient := range []string{"", " ", "workEmail"} {
		s.Run(recipient, func() {
			enabled := true
			incoming := AccountAccessValue{User: &AccountAccessCategory{Notifications: &AccountAccessNotifications{
				OnLock: &AccountAccessLockNotification{Email: &AccountAccessLockEmail{
					Enabled: &enabled, RecipientAttribute: &recipient,
				}},
			}}}
			handler := AccountAccessHandler{}
			if err := handler.Validate(incoming, nil, nil); err != nil {
				s.T().Fatal(err)
			}
			merged := handler.Merge(nil, incoming).(AccountAccessValue)
			want := recipient
			if recipient == "" || recipient == " " {
				want = "email"
			}
			if got := *LockEmailOf(merged.User.Notifications).RecipientAttribute; got != want {
				s.T().Errorf("effective attribute = %q; want %q", got, want)
			}
			if got := *LockEmailOf(incoming.User.Notifications).RecipientAttribute; got != recipient {
				s.T().Errorf("stored layer changed to %q", got)
			}
		})
	}
}
