// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/observabilityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

// testContactValue is the stored work email of the user the tests pin.
const testContactValue = "Private.Contact@example.com"

type ServiceTestSuite struct {
	suite.Suite
	entities  *entitymock.EntityServiceInterfaceMock
	published *[]*providers.Event
	svc       *attributeVerificationService
	ctx       context.Context
	testErr   error
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	s.entities = entitymock.NewEntityServiceInterfaceMock(s.T())
	published := []*providers.Event{}
	s.published = &published
	obs := observabilityprovidermock.NewObservabilityProviderMock(s.T())
	obs.On("IsEnabled").Return(true).Maybe()
	obs.On("PublishEvent", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { published = append(published, args.Get(1).(*providers.Event)) }).Maybe()
	authorizer := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authorizer.EXPECT().IsActionAllowed(mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Maybe()
	s.svc = newAttributeVerificationService(s.entities, contactSchema(s.T()), obs, authorizer)
	s.ctx = context.Background()
	s.testErr = errors.New("store error")
}

// contactSchema declares scalar contacts and ineligible attribute types.
func contactSchema(t *testing.T) *entitytypemock.EntityTypeServiceInterfaceMock {
	t.Helper()
	schema := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
	schema.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, mock.Anything,
		entitytype.AttributeFilter{AllowNonCredential: true}).Return([]entitytype.AttributeInfo{
		{Attribute: "workEmail", Type: "email"}, {Attribute: "mobile", Type: "mobile"},
		{Attribute: "nickname", Type: "string"}, {Attribute: "nested", Type: "object"},
		{Attribute: "many", Type: "array"},
	}, nil).Maybe()
	return schema
}

// storedUser is user-1, a person with the given attributes.
func storedUser(attributes string) *providers.Entity {
	return &providers.Entity{ID: "user-1", Category: providers.EntityCategoryUser, Type: "person",
		Attributes: json.RawMessage(attributes)}
}

// contactUser is user-1, a person whose work email holds the pinned value.
func contactUser() *providers.Entity {
	return storedUser(`{"workEmail":"` + testContactValue + `"}`)
}

// contactPin is the pin of user-1's stored work email.
func contactPin() providers.AttributeVerificationPin {
	return providers.AttributeVerificationPin{Subject: "user-1", Attribute: "workEmail",
		Value: testContactValue, Channel: providers.VerificationChannelEmail}
}

// storedRequest asks to pin user-1's stored attribute.
func storedRequest(attribute string) providers.AttributeVerificationRequest {
	return providers.AttributeVerificationRequest{EntityID: "user-1", Attribute: attribute}
}

// pendingRequest asks to pin a value collected for a user of the type pending creation.
func pendingRequest(entityType, attribute, value string) providers.AttributeVerificationRequest {
	return providers.AttributeVerificationRequest{EntityType: entityType, Attribute: attribute, Value: value,
		PendingCreation: true}
}

// contactRecord matches the entity write of the pinned work email for a person, at a set time.
func contactRecord() interface{} {
	return mock.MatchedBy(func(record entity.AttributeVerificationRecord) bool {
		return record.EntityID == "user-1" && record.EntityCategory == providers.EntityCategoryUser &&
			record.EntityType == "person" && record.Attribute == "workEmail" &&
			record.Value == testContactValue && !record.VerifiedAt.IsZero()
	})
}

// requireNoContactValue checks that no event field carries the contact value.
func (s *ServiceTestSuite) requireNoContactValue() {
	for _, evt := range *s.published {
		for key, value := range evt.Data {
			s.Require().NotContains(fmt.Sprint(value), testContactValue, "field %s carries the contact", key)
		}
	}
}

// requireServiceError checks that err carries the code of want.
func (s *ServiceTestSuite) requireServiceError(err *tidcommon.ServiceError, want tidcommon.ServiceError) {
	s.Require().NotNil(err)
	s.Equal(want.Code, err.Code)
}

// publishedTypes returns the types of the published events, each with the given status.
func (s *ServiceTestSuite) publishedTypes(status string) []string {
	types := make([]string, 0, len(*s.published))
	for _, evt := range *s.published {
		types = append(types, evt.Type)
		s.Equal(status, evt.Status)
	}
	return types
}

// A user's stored contact is pinned by its canonical value and the request is published with the user
// as actor; a value collected for a user pending creation is pinned the same way without an event.
func (s *ServiceTestSuite) TestPreparePinsCanonicalContacts() {
	for _, tc := range []struct {
		attribute, value, canonical string
		channel                     providers.VerificationChannel
	}{
		{"workEmail", " User@Example.com ", "User@example.com", providers.VerificationChannelEmail},
		{"mobile", "+94771234567", "+94771234567", providers.VerificationChannelSMS},
	} {
		s.Run(tc.attribute, func() {
			s.SetupTest()
			values, err := json.Marshal(map[string]string{tc.attribute: tc.value})
			s.Require().NoError(err)
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(storedUser(string(values)), nil).Once()

			pin, svcErr := s.svc.PrepareAttributeVerification(s.ctx, storedRequest(tc.attribute))

			s.Require().Nil(svcErr)
			s.Equal(providers.AttributeVerificationPin{Subject: "user-1", Attribute: tc.attribute,
				Value: tc.canonical, Channel: tc.channel}, *pin)
			s.Require().Len(*s.published, 1)
			requested := (*s.published)[0]
			s.Equal(string(event.EventTypeAttributeVerificationRequested), requested.Type)
			s.Equal(event.ComponentAttributeVerification, requested.Component)
			s.Equal(providers.StatusSuccess, requested.Status)
			s.Equal("user-1", requested.Data[event.DataKey.Subject])
			s.Equal("user-1", requested.Data[event.DataKey.ActorSub])
			s.Equal(tc.attribute, requested.Data[event.DataKey.Attribute])

			pin, svcErr = s.svc.PrepareAttributeVerification(s.ctx, pendingRequest("person", tc.attribute, tc.value))

			s.Require().Nil(svcErr)
			s.Equal(providers.AttributeVerificationPin{Attribute: tc.attribute, Value: tc.canonical,
				Channel: tc.channel}, *pin)
			s.Len(*s.published, 1)
			s.entities.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
		})
	}
}

// The request is published with the caller as actor when the request carries one.
func (s *ServiceTestSuite) TestPrepareNamesTheCallerAsActor() {
	s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
	ctx := security.WithSecurityContextTest(s.ctx, security.NewSecurityContextForTest("operator", "ou1", "", nil,
		nil))

	_, svcErr := s.svc.PrepareAttributeVerification(ctx, storedRequest("workEmail"))

	s.Require().Nil(svcErr)
	s.Require().Len(*s.published, 1)
	s.Equal("user-1", (*s.published)[0].Data[event.DataKey.Subject])
	s.Equal("operator", (*s.published)[0].Data[event.DataKey.ActorSub])
}

func (s *ServiceTestSuite) TestPrepareRefusesIneligibleProfiles() {
	for name, tc := range map[string]struct {
		attribute, attributes string
		readOnly              bool
		category              providers.EntityCategory
	}{
		"no attributes":    {"workEmail", "", false, providers.EntityCategoryUser},
		"missing":          {"workEmail", `{}`, false, providers.EntityCategoryUser},
		"blank":            {"workEmail", `{"workEmail":"  "}`, false, providers.EntityCategoryUser},
		"not a string":     {"workEmail", `{"workEmail":42}`, false, providers.EntityCategoryUser},
		"undeclared":       {"other", `{"other":"a@example.com"}`, false, providers.EntityCategoryUser},
		"string":           {"nickname", `{"nickname":"a@example.com"}`, false, providers.EntityCategoryUser},
		"object":           {"nested", `{"nested":{"email":"a@example.com"}}`, false, providers.EntityCategoryUser},
		"array":            {"many", `{"many":["a@example.com"]}`, false, providers.EntityCategoryUser},
		"read only":        {"workEmail", `{"workEmail":"a@example.com"}`, true, providers.EntityCategoryUser},
		"changed category": {"workEmail", `{"workEmail":"a@example.com"}`, false, providers.EntityCategoryAgent},
		"app":              {"workEmail", `{"workEmail":"a@example.com"}`, false, providers.EntityCategoryApp},
	} {
		s.Run(name, func() {
			s.SetupTest()
			user := storedUser(tc.attributes)
			user.Category, user.IsReadOnly = tc.category, tc.readOnly
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(user, nil).Once()

			pin, svcErr := s.svc.PrepareAttributeVerification(s.ctx, storedRequest(tc.attribute))

			s.Nil(pin)
			s.requireServiceError(svcErr, ErrorAttributeNotVerifiable)
			s.Empty(*s.published)
		})
	}
}

func (s *ServiceTestSuite) TestPrepareRefusesAnUnknownEntity() {
	s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(nil, entity.ErrEntityNotFound).Once()

	pin, svcErr := s.svc.PrepareAttributeVerification(s.ctx, storedRequest("workEmail"))

	s.Nil(pin)
	s.requireServiceError(svcErr, ErrorAttributeNotVerifiable)
	s.Empty(*s.published)
}

func (s *ServiceTestSuite) TestPendingCreationPrepareRefusesIneligibleValues() {
	for name, tc := range map[string]struct{ entityType, attribute, value string }{
		"no type":    {"", "workEmail", "a@example.com"},
		"blank":      {"person", "workEmail", "  "},
		"empty":      {"person", "workEmail", ""},
		"undeclared": {"person", "other", "a@example.com"},
		"string":     {"person", "nickname", "a@example.com"},
		"object":     {"person", "nested", "a@example.com"},
		"array":      {"person", "many", "a@example.com"},
	} {
		s.Run(name, func() {
			s.SetupTest()

			pin, svcErr := s.svc.PrepareAttributeVerification(s.ctx,
				pendingRequest(tc.entityType, tc.attribute, tc.value))

			s.Nil(pin)
			s.requireServiceError(svcErr, ErrorAttributeNotVerifiable)
			s.Empty(*s.published)
			s.entities.AssertNotCalled(s.T(), "GetEntityProfile", mock.Anything, mock.Anything)
		})
	}
}

// Without an entity type service nothing is a contact; a schema that cannot be read is a server error.
func (s *ServiceTestSuite) TestPreparationSchemaFailures() {
	for _, existing := range []bool{false, true} {
		for _, missing := range []bool{false, true} {
			s.Run(fmt.Sprintf("existing=%t/missing=%t", existing, missing), func() {
				s.SetupTest()
				s.svc.entityTypeService = nil
				want := ErrorAttributeNotVerifiable
				if !missing {
					schema := entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
					schema.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person", mock.Anything).
						Return(nil, &tidcommon.InternalServerError).Once()
					s.svc.entityTypeService = schema
					want = tidcommon.InternalServerError
				}
				request := pendingRequest("person", "workEmail", testContactValue)
				if existing {
					s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
					request = storedRequest("workEmail")
				}

				pin, svcErr := s.svc.PrepareAttributeVerification(s.ctx, request)

				s.Nil(pin)
				s.requireServiceError(svcErr, want)
				s.Empty(*s.published)
			})
		}
	}
}

// A user type that no longer resolves makes the attribute not verifiable rather than a server error.
func (s *ServiceTestSuite) TestPreparationRefusesAnUnknownUserType() {
	schema := entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	schema.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person", mock.Anything).
		Return(nil, &entitytype.ErrorEntityTypeNotFound).Once()
	s.svc.entityTypeService = schema

	pin, svcErr := s.svc.PrepareAttributeVerification(s.ctx, pendingRequest("person", "workEmail",
		testContactValue))

	s.Nil(pin)
	s.requireServiceError(svcErr, ErrorAttributeNotVerifiable)
	s.Empty(*s.published)
}

func (s *ServiceTestSuite) TestPrepareProfileFaultsAreServerErrors() {
	for name, read := range map[string]func(){
		"read": func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(nil, s.testErr).Once()
		},
		"malformed attributes": func() {
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(storedUser(`[`), nil).Once()
		},
	} {
		s.Run(name, func() {
			s.SetupTest()
			read()

			pin, svcErr := s.svc.PrepareAttributeVerification(s.ctx, storedRequest("workEmail"))

			s.Nil(pin)
			s.requireServiceError(svcErr, tidcommon.InternalServerError)
			s.Empty(*s.published)
		})
	}
}

// A pin whose contact is still stored is written through the entity service and published with the user
// as actor.
func (s *ServiceTestSuite) TestRecordPublishesTheVerification() {
	before := time.Now()
	s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
	s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).Return(true, nil).Once()

	s.Require().Nil(s.svc.RecordAttributeVerification(s.ctx, contactPin()))

	record := s.entities.Calls[len(s.entities.Calls)-1].Arguments.Get(1).(entity.AttributeVerificationRecord)
	s.False(record.VerifiedAt.Before(before))
	s.Require().Len(*s.published, 1)
	evt := (*s.published)[0]
	s.Equal(string(event.EventTypeAttributeVerificationVerified), evt.Type)
	s.Equal(event.ComponentAttributeVerification, evt.Component)
	s.Equal(providers.StatusSuccess, evt.Status)
	s.Equal("user-1", evt.Data[event.DataKey.Subject])
	s.Equal(event.PrincipalTypeUser, evt.Data[event.DataKey.SubjectType])
	s.Equal("user-1", evt.Data[event.DataKey.ActorSub])
	s.Equal("workEmail", evt.Data[event.DataKey.Attribute])
	s.requireNoContactValue()
}

func (s *ServiceTestSuite) TestRecordWithoutObservabilityPublishesNothing() {
	s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
	s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).Return(true, nil).Once()
	s.svc.observability = nil

	s.Nil(s.svc.RecordAttributeVerification(s.ctx, contactPin()))
}

// A contact the entity service finds changed under the lock is refused and published; a failed write is
// a server error and is not.
func (s *ServiceTestSuite) TestRecordRefusalsUnderTheLock() {
	refused := []string{string(event.EventTypeAttributeVerificationRefused)}
	for name, tc := range map[string]struct {
		writeErr  error
		want      tidcommon.ServiceError
		published []string
	}{
		"stale": {nil, ErrorAttributeNotVerifiable, refused},
		"fault": {errors.New("write failed"), tidcommon.InternalServerError, []string{}},
	} {
		s.Run(name, func() {
			s.SetupTest()
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
			s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).
				Return(false, tc.writeErr).Once()

			s.requireServiceError(s.svc.RecordAttributeVerification(s.ctx, contactPin()), tc.want)

			s.ElementsMatch(tc.published, s.publishedTypes(providers.StatusFailure))
			s.requireNoContactValue()
		})
	}
}

// The stored contact is checked against the pin before the write, so a changed value, a changed channel,
// an attribute that is no longer a contact, an entity that is not a user and a deleted user are refused
// without writing.
func (s *ServiceTestSuite) TestRecordRefusesAContactThatNoLongerMatchesThePin() {
	app := contactUser()
	app.Category = providers.EntityCategoryApp
	for name, tc := range map[string]struct {
		stored  *providers.Entity
		readErr error
		mutate  func(*providers.AttributeVerificationPin)
	}{
		"other value": {stored: storedUser(`{"workEmail":"other@example.com"}`)},
		"other channel": {stored: contactUser(),
			mutate: func(p *providers.AttributeVerificationPin) { p.Channel = providers.VerificationChannelSMS }},
		"not a contact": {stored: storedUser(`{"nickname":"` + testContactValue + `"}`),
			mutate: func(p *providers.AttributeVerificationPin) { p.Attribute = "nickname" }},
		"removed":    {stored: storedUser(`{}`)},
		"not a user": {stored: app},
		"deleted":    {readErr: entity.ErrEntityNotFound},
	} {
		s.Run(name, func() {
			s.SetupTest()
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(tc.stored, tc.readErr).Once()
			pin := contactPin()
			if tc.mutate != nil {
				tc.mutate(&pin)
			}

			s.requireServiceError(s.svc.RecordAttributeVerification(s.ctx, pin), ErrorAttributeNotVerifiable)

			s.Equal([]string{string(event.EventTypeAttributeVerificationRefused)},
				s.publishedTypes(providers.StatusFailure))
			s.requireNoContactValue()
			s.entities.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
		})
	}
}

func (s *ServiceTestSuite) TestRecordProfileReadFaultIsAServerError() {
	s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(nil, s.testErr).Once()

	s.requireServiceError(s.svc.RecordAttributeVerification(s.ctx, contactPin()), tidcommon.InternalServerError)

	s.Empty(*s.published)
	s.entities.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
}

// Every server fault is logged with its cause, without the contact value.
func (s *ServiceTestSuite) TestServerFaultsAreLogged() {
	for name, tc := range map[string]struct {
		arrange func()
		call    func() *tidcommon.ServiceError
		message string
	}{
		"profile read": {
			arrange: func() {
				s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(nil, s.testErr).Once()
			},
			call: func() *tidcommon.ServiceError {
				_, err := s.svc.PrepareAttributeVerification(s.ctx, storedRequest("workEmail"))
				return err
			},
			message: "Failed to read the contact for attribute verification",
		},
		"record write": {
			arrange: func() {
				s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(contactUser(), nil).Once()
				s.entities.On("RecordAttributeVerification", mock.Anything, contactRecord()).
					Return(false, s.testErr).Once()
			},
			call: func() *tidcommon.ServiceError {
				return s.svc.RecordAttributeVerification(s.ctx, contactPin())
			},
			message: "Failed to record attribute verification",
		},
	} {
		s.Run(name, func() {
			s.SetupTest()
			tc.arrange()

			var svcErr *tidcommon.ServiceError
			logged := testhelpers.CaptureLogs(s.T(), "DEBUG", func() { svcErr = tc.call() })

			s.requireServiceError(svcErr, tidcommon.InternalServerError)
			s.Contains(logged, tc.message)
			s.NotContains(logged, testContactValue)
		})
	}
}

// verifiedUser is a loaded user-1 with the given attributes and system attributes.
func verifiedUser(attributes, systemAttributes string) providers.Entity {
	return providers.Entity{ID: "user-1", Category: providers.EntityCategoryUser, Type: "person",
		Attributes: json.RawMessage(attributes), SystemAttributes: json.RawMessage(systemAttributes)}
}

// Every recorded verification with a valid time is returned, whatever the attribute now holds, and no
// schema is read.
func (s *ServiceTestSuite) TestGetVerifiedAttributes() {
	user := verifiedUser(`{"workEmail":"a@example.com","mobile":"","nickname":"n",`+
		`"homeEmail":"h@example.com"}`, `{"attributeVerification":{`+
		`"workEmail":{"verifiedAt":"2026-10-15T00:00:00Z"},`+
		`"mobile":{"verifiedAt":"2026-10-15T00:00:00Z"},`+
		`"nickname":{"verifiedAt":"2026-10-15T00:00:00Z"},`+
		`"homeEmail":{"verifiedAt":"forged"},`+
		`"removedEmail":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`)

	verified := s.svc.GetVerifiedAttributes(s.ctx, user)

	s.Equal(map[string]providers.VerifiedAttribute{
		"workEmail":    {VerifiedAt: "2026-10-15T00:00:00Z"},
		"mobile":       {VerifiedAt: "2026-10-15T00:00:00Z"},
		"nickname":     {VerifiedAt: "2026-10-15T00:00:00Z"},
		"removedEmail": {VerifiedAt: "2026-10-15T00:00:00Z"},
	}, verified)
}

// A user with no recorded verification, a declarative user and an entity that is not a user have none.
func (s *ServiceTestSuite) TestGetVerifiedAttributesIsEmptyWithoutAVerification() {
	user := verifiedUser(`{"workEmail":"a@example.com"}`,
		`{"attributeVerification":{"workEmail":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`)
	declarative := user
	declarative.IsReadOnly = true
	app := user
	app.Category = providers.EntityCategoryApp

	for name, entity := range map[string]providers.Entity{"declarative": declarative, "app": app,
		"unverified":  verifiedUser(`{"workEmail":"a@example.com"}`, `{"attributeVerification":{}}`),
		"no metadata": verifiedUser(`{"workEmail":"a@example.com"}`, ``)} {
		verified := s.svc.GetVerifiedAttributes(s.ctx, entity)

		s.NotNil(verified, name)
		s.Empty(verified, name)
	}
}

// Verification metadata that cannot be read is reported as unreadable rather than as no verification.
func (s *ServiceTestSuite) TestGetVerifiedAttributesReportsAnUnreadableState() {
	for name, metadata := range map[string]string{"system attributes": `[`,
		"entries": `{"attributeVerification":[]}`, "null entries": `{"attributeVerification":null}`} {
		verified := s.svc.GetVerifiedAttributes(s.ctx, verifiedUser(`{}`, metadata))

		s.Nil(verified, name)
	}
}

// userUpdateIn is the authorizer's check that the caller may update user-1 in ou-1.
var userUpdateIn = &sysauthz.ActionContext{ResourceType: security.ResourceTypeUser, OUID: "ou-1", ResourceID: "user-1"}

// ouUser is contactUser in organization unit ou-1.
func ouUser() *providers.Entity {
	user := contactUser()
	user.OUID = "ou-1"
	return user
}

// Prepare and record both confine the caller to users it may update in the user's organization unit.
func (s *ServiceTestSuite) TestPrepareAndRecordAuthorizeTheCallerForTheUsersUnit() {
	for name, test := range map[string]struct {
		allowed bool
		authErr *tidcommon.ServiceError
		want    *tidcommon.ServiceError
	}{
		"allowed":            {allowed: true},
		"not allowed":        {want: &tidcommon.ErrorUnauthorized},
		"authorizer failure": {authErr: &tidcommon.InternalServerError, want: &tidcommon.InternalServerError},
	} {
		s.Run(name, func() {
			s.SetupTest()
			authorizer := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
			authorizer.EXPECT().IsActionAllowed(mock.Anything, security.ActionUpdateUser, userUpdateIn).
				Return(test.allowed, test.authErr).Twice()
			s.svc.authorizer = authorizer
			s.entities.On("GetEntityProfile", mock.Anything, "user-1").Return(ouUser(), nil).Twice()
			if test.want == nil {
				s.entities.On("RecordAttributeVerification", mock.Anything, mock.Anything).Return(true, nil).Once()
			}

			_, prepareErr := s.svc.PrepareAttributeVerification(s.ctx, storedRequest("workEmail"))
			recordErr := s.svc.RecordAttributeVerification(s.ctx, contactPin())

			s.Equal(test.want, prepareErr)
			s.Equal(test.want, recordErr)
			if test.want != nil {
				s.entities.AssertNotCalled(s.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)
			}
		})
	}
}
