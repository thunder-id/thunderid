// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type ServerConfigTestSuite struct {
	suite.Suite
	// handler has no base; precedence has a representative deployment base.
	handler    accountAccessHandler
	precedence accountAccessHandler
}

func TestServerConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ServerConfigTestSuite))
}

func (s *ServerConfigTestSuite) SetupTest() {
	s.handler = accountAccessHandler{}
	s.precedence = accountAccessHandler{base: testAccountAccessConfig()}
}

// mergedOverBase merges the layers over the representative base.
func (s *ServerConfigTestSuite) mergedOverBase(readOnly,
	writable accountAccessValue) accountAccessValue {
	value, ok := s.precedence.Merge(readOnly, writable).(accountAccessValue)
	s.Require().True(ok)
	return value
}

func (s *ServerConfigTestSuite) decode(raw string) accountAccessValue {
	value, err := s.handler.Decode(json.RawMessage(raw))
	s.Require().NoError(err)
	cfg, ok := value.(accountAccessValue)
	s.Require().True(ok)
	return cfg
}

func (s *ServerConfigTestSuite) TestAnEmptyLayerDecodesToAnAbsentSection() {
	value, err := s.handler.Decode(nil)
	s.Require().NoError(err)
	s.Equal(accountAccessValue{}, value)
}

// Unlike other sections, unknown fields are rejected.
func (s *ServerConfigTestSuite) TestAnUnknownFieldIsRejected() {
	_, err := s.handler.Decode(json.RawMessage(`{"user":{"treshold":5}}`))
	s.Require().Error(err)
}

func (s *ServerConfigTestSuite) TestAnApplicationPolicyIsRejected() {
	_, err := s.handler.Decode(json.RawMessage(`{"application":{"default":{"enabled":true}}}`))
	s.Require().Error(err)
}

// Per-method overrides are not exposed, so a per-method block is an unknown field.
func (s *ServerConfigTestSuite) TestAPerMethodBlockIsRejected() {
	_, err := s.handler.Decode(json.RawMessage(`{"user":{"scopes":{"otp":{"threshold":3}}}}`))
	s.Require().Error(err)
	s.Contains(err.Error(), `unknown field "scopes"`)
}

// Disclosure is fixed in code, so there is no setting for it.
func (s *ServerConfigTestSuite) TestADisclosureSettingIsRejected() {
	_, err := s.handler.Decode(json.RawMessage(`{"user":{"default":{"discloseAccountHold":"always"}}}`))
	s.Require().Error(err)
	s.Contains(err.Error(), `unknown field "discloseAccountHold"`)
}

func (s *ServerConfigTestSuite) TestAnUnknownGranularityIsRejected() {
	incoming := s.decode(`{"user":{"lockGranularity":"entitiy"}}`)
	s.Require().Error(s.handler.Validate(incoming, nil, nil))
}

func (s *ServerConfigTestSuite) TestANegativeDurationIsRejected() {
	// The resolver would silently drop a negative duration.
	incoming := s.decode(`{"user":{"default":{"lockDurationsSeconds":[300,-1]}}}`)
	s.Require().Error(s.handler.Validate(incoming, nil, nil))
}

func (s *ServerConfigTestSuite) TestAZeroDurationIsAccepted() {
	// Zero means a permanent lock.
	incoming := s.decode(
		`{"user":{"default":{"enabled":true,"threshold":5,"failureWindowSeconds":900,"lockDurationsSeconds":[300,0]}}}`)
	s.Require().NoError(s.handler.Validate(incoming, nil, nil))
}

// A value that would overflow time.Duration is refused at write time.
func (s *ServerConfigTestSuite) TestADurationTooLargeToCarryIsRejected() {
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
func (s *ServerConfigTestSuite) TestAPolicyThatCannotFormALockIsRejected() {
	incoming := s.decode(`{"user":{"default":{"enabled":true,"threshold":5}}}`)
	err := s.handler.Validate(incoming, nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "failureWindowSeconds")
	s.Contains(err.Error(), "lockDurationsSeconds")
}

// A policy that says it is enabled and states nothing else cannot form a lock, so it is refused.
func (s *ServerConfigTestSuite) TestAnEnabledPolicyStatingNothingElseIsRejected() {
	err := s.handler.Validate(s.decode(`{"user":{"default":{"enabled":true}}}`), nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "threshold")
	s.Contains(err.Error(), "failureWindowSeconds")
	s.Contains(err.Error(), "lockDurationsSeconds")
}

// A window of zero means the counter never resets on time, so the policy can still form a lock.
func (s *ServerConfigTestSuite) TestAZeroWindowIsAccepted() {
	incoming := s.decode(
		`{"user":{"default":{"enabled":true,"threshold":5,"failureWindowSeconds":0,"lockDurationsSeconds":[300]}}}`)
	s.Require().NoError(s.handler.Validate(incoming, nil, nil))
}

func (s *ServerConfigTestSuite) TestAnIncompleteWritableLayerIsCoherentAgainstTheBase() {
	handler := accountAccessHandler{base: s.decode(
		`{"user":{"default":{"enabled":true,"threshold":5,"failureWindowSeconds":900,` +
			`"lockDurationsSeconds":[300]}}}`)}
	// Only a threshold; the base supplies the rest.
	incoming := s.decode(`{"user":{"default":{"threshold":3}}}`)
	s.Require().NoError(handler.Validate(incoming, nil, nil))
}

func (s *ServerConfigTestSuite) TestSwitchingACategoryOffNeedsNoOtherField() {
	incoming := s.decode(`{"agent":{"default":{"enabled":false}}}`)
	s.Require().NoError(s.handler.Validate(incoming, nil, nil))
}

func (s *ServerConfigTestSuite) TestAWritableCategoryDoesNotDeleteAnotherCategory() {
	handler := accountAccessHandler{base: s.decode(
		`{"user":{"default":{"threshold":5}},"agent":{"default":{"threshold":10}}}`)}
	writable := s.decode(`{"user":{"default":{"threshold":3}}}`)

	merged := handler.Merge(nil, writable).(accountAccessValue)
	s.Require().NotNil(merged.Agent)
	s.Equal(10, *merged.Agent.Default.Threshold)
	s.Equal(3, *merged.User.Default.Threshold)
}

func (s *ServerConfigTestSuite) TestTheWritableLayerCanStateAFalseThatTheBaseDoesNot() {
	// A pointer field lets an explicit false differ from unset.
	handler := accountAccessHandler{base: s.decode(`{"user":{"default":{"enabled":true,"threshold":5,` +
		`"failureWindowSeconds":900,"lockDurationsSeconds":[300]}}}`)}
	writable := s.decode(`{"user":{"default":{"enabled":false}}}`)

	merged := handler.Merge(nil, writable).(accountAccessValue)
	s.False(*merged.User.Default.Enabled)
	s.Equal(5, *merged.User.Default.Threshold)
}

func (s *ServerConfigTestSuite) TestNoBaseAndNoSectionHasNoPolicy() {
	empty := accountAccessValue{}
	merged, ok := accountAccessHandler{}.Merge(empty, empty).(accountAccessValue)
	s.Require().True(ok)
	s.Nil(merged.User)
	s.Nil(merged.Agent)
}

func (s *ServerConfigTestSuite) TestTheBaseAppliesWithNoSection() {
	merged := s.mergedOverBase(accountAccessValue{}, accountAccessValue{})
	s.Equal(5, *merged.User.Default.Threshold)
	s.Equal(effectiveAccountAccess(s.precedence.base, accountAccessValue{}), merged)
}

func (s *ServerConfigTestSuite) TestWritableOverridesTheBase() {
	merged := s.mergedOverBase(accountAccessValue{}, s.decode(`{"user":{"default":{"threshold":2}}}`))
	s.Equal(2, *merged.User.Default.Threshold)
	s.Equal(900, *merged.User.Default.FailureWindowSeconds)
	s.Equal([]int{300, 900, 3600, 86400}, merged.User.Default.LockDurationsSeconds)
}

func (s *ServerConfigTestSuite) TestPartialWriteValidatesAgainstTheBase() {
	s.Require().NoError(s.precedence.Validate(
		s.decode(`{"user":{"default":{"threshold":3}}}`), accountAccessValue{},
		accountAccessValue{}))
}

func (s *ServerConfigTestSuite) TestIncompleteEnabledPolicyWithoutABaseIsRefused() {
	value := s.decode(`{"user":{"default":{"enabled":true,"threshold":3}}}`)
	s.Require().Error(accountAccessHandler{}.Validate(value, nil, nil))
}

func (s *ServerConfigTestSuite) TestWritableCanDisableTheBasePolicy() {
	merged := s.mergedOverBase(accountAccessValue{},
		s.decode(`{"user":{"default":{"enabled":false}}}`))
	s.False(*merged.User.Default.Enabled)
}

// A declarative document wins over the writable layer and is laid over the base.
func (s *ServerConfigTestSuite) TestADeclarativeDocumentWinsOverTheWritableLayer() {
	merged := s.mergedOverBase(s.decode(`{"user":{"default":{"threshold":7}}}`),
		s.decode(`{"user":{"default":{"threshold":2}}}`))
	s.Equal(7, *merged.User.Default.Threshold)
	s.Equal(900, *merged.User.Default.FailureWindowSeconds, "the base supplies what the document does not state")
	s.Require().NotNil(merged.Agent, "a category the document does not name keeps the base")
}

// While a declarative document exists the section is locked, so a write cannot be a silent no-op.
func (s *ServerConfigTestSuite) TestADeclarativeDocumentRefusesWrites() {
	err := s.precedence.Validate(s.decode(`{"user":{"default":{"threshold":2}}}`),
		s.decode(`{"user":{"default":{"threshold":7}}}`), accountAccessValue{})
	s.Require().ErrorIs(err, errDeclarativeAccountAccessLocked)
}

// At load time a partial declarative document is checked laid over the base.
func (s *ServerConfigTestSuite) TestTheDeclarativeLoadValidatesAgainstTheBase() {
	s.Require().NoError(s.precedence.Validate(s.decode(`{"user":{"default":{"threshold":7}}}`), nil, nil))
}

// Last-login settings are configurable through the section.
func (s *ServerConfigTestSuite) TestLastLoginSettingsAreConfigurableThroughTheSection() {
	incoming := s.decode(`{"user":{"recordLastLogin":true,"lastLoginResolutionSeconds":300},` +
		`"agent":{"recordLastLogin":false}}`)
	s.Require().NoError(s.handler.Validate(incoming, nil, nil))

	s.True(*incoming.User.RecordLastLogin)
	s.Equal(300, *incoming.User.LastLoginResolutionSeconds)
	s.False(*incoming.Agent.RecordLastLogin,
		"an explicit false must survive decoding, or a category could not be switched back off")

	// The setting reaches the resolver.
	activity := activityPolicyFor(categoryConfig(incoming, providers.EntityCategoryUser))
	s.True(activity.record)
	s.Equal(300*time.Second, activity.resolution)
}

// A negative resolution is refused at write time rather than clamped.
func (s *ServerConfigTestSuite) TestANegativeLastLoginResolutionIsRejected() {
	incoming := s.decode(`{"user":{"lastLoginResolutionSeconds":-1}}`)
	err := s.handler.Validate(incoming, nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "lastLoginResolutionSeconds")
}

// Last-login settings overlay field by field across layers.
func (s *ServerConfigTestSuite) TestLastLoginSettingsOverlayFieldByField() {
	readOnly := s.decode(`{"user":{"recordLastLogin":false,"lastLoginResolutionSeconds":3600}}`)
	writable := s.decode(`{"user":{"recordLastLogin":true}}`)

	merged := mergeAccountAccessValue(readOnly, writable)

	s.True(*merged.User.RecordLastLogin, "the writable layer wins where it states a value")
	s.Equal(3600, *merged.User.LastLoginResolutionSeconds,
		"and the layer beneath supplies what it does not state")
}

func (s *ServerConfigTestSuite) TestEffectiveLockEmailDefaultsWithoutChangingStoredLayer() {
	for _, recipient := range []string{"", " ", "workEmail"} {
		s.Run(recipient, func() {
			enabled := true
			incoming := notificationValue(&accountAccessLockEmail{
				Enabled: &enabled, RecipientAttribute: &recipient,
			})
			handler := accountAccessHandler{}
			if err := handler.Validate(incoming, nil, nil); err != nil {
				s.T().Fatal(err)
			}
			merged := handler.Merge(nil, incoming).(accountAccessValue)
			want := recipient
			if recipient == "" || recipient == " " {
				want = "email"
			}
			if got := *merged.User.Notifications.LockEmail().RecipientAttribute; got != want {
				s.T().Errorf("effective attribute = %q; want %q", got, want)
			}
			if got := *incoming.User.Notifications.LockEmail().RecipientAttribute; got != recipient {
				s.T().Errorf("stored layer changed to %q", got)
			}
		})
	}
}

// notificationValue builds a section stating only the user notification block.
func notificationValue(email *accountAccessLockEmail) accountAccessValue {
	return accountAccessValue{User: &accountAccessCategory{
		Notifications: &accountAccessNotifications{
			OnLock: &accountAccessLockNotification{Email: email},
		},
	}}
}

// An omitted notification override keeps the read-only settings.
func (s *ServerConfigTestSuite) TestAnOmittedNotificationOverrideKeepsTheReadOnlySettings() {
	readOnly := notificationValue(&accountAccessLockEmail{Enabled: new(true)})

	merged := mergeAccountAccessValue(readOnly,
		accountAccessValue{User: &accountAccessCategory{}})

	s.True(derefOr(merged.User.Notifications.LockEmail().Enabled, false))
}

// A notification override replaces only the fields it states.
func (s *ServerConfigTestSuite) TestANotificationOverrideReplacesOnlyWhatItStates() {
	readOnly := notificationValue(&accountAccessLockEmail{
		Enabled: new(true), RecipientAttribute: new("workEmail"),
	})

	merged := mergeAccountAccessValue(readOnly,
		notificationValue(&accountAccessLockEmail{Enabled: new(false)}))

	email := merged.User.Notifications.LockEmail()
	s.False(derefOr(email.Enabled, true))
	s.Equal("workEmail", derefOr(email.RecipientAttribute, ""))
}

// A default named in both layers is overlaid field by field.
func (s *ServerConfigTestSuite) TestADefaultInBothLayersIsOverlaid() {
	readOnly := testAccountAccessConfig()
	writable := accountAccessValue{User: &accountAccessCategory{
		Default: &accountAccessPolicy{Threshold: new(4)},
	}}

	merged := mergeAccountAccessValue(readOnly, writable).User.Default

	s.Equal(4, *merged.Threshold)
	s.Equal(900, *merged.FailureWindowSeconds, "an unstated field keeps the read-only value")
}

// A category absent from either layer is taken from the other.
func (s *ServerConfigTestSuite) TestACategoryAbsentFromOneLayerIsTakenFromTheOther() {
	readOnly := testAccountAccessConfig()

	merged := mergeAccountAccessValue(readOnly, accountAccessValue{})

	s.Same(readOnly.User, merged.User)
	s.Same(readOnly.Agent, merged.Agent)
}

func (s *ServerConfigTestSuite) TestLockEmailToleratesMissingLevels() {
	var notifications *accountAccessNotifications
	s.Nil(notifications.LockEmail())
	s.Nil((&accountAccessNotifications{}).LockEmail())
}

func (s *ServerConfigTestSuite) TestPolicyIsEmpty() {
	s.True(accountAccessPolicy{}.IsEmpty())
	s.False(accountAccessPolicy{Enabled: new(false)}.IsEmpty())
}
