// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"context"
	"errors"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// recordingCapturer records every resource handed to it.
type recordingCapturer struct {
	resourceTypes []string
	resources     []interface{}
}

func (r *recordingCapturer) CaptureValues(_ context.Context, resourceType string, resource interface{}) {
	r.resourceTypes = append(r.resourceTypes, resourceType)
	r.resources = append(r.resources, resource)
}

// recordCaptures hands the suite's service a capturer and returns it.
func (s *ServiceTestSuite) recordCaptures() *recordingCapturer {
	capturer := &recordingCapturer{}
	s.service.valueCapturer = capturer
	return capturer
}

// captured returns the one type handed to the capturer.
func (s *ServiceTestSuite) captured(capturer *recordingCapturer) *typeDeclarativeResource {
	s.Require().Len(capturer.resources, 1)
	s.Equal(resourceTypeAuthorizationDetailType, capturer.resourceTypes[0])
	resource, ok := capturer.resources[0].(*typeDeclarativeResource)
	s.Require().True(ok, "the capture was not handed the shape the exporter reads back")
	return resource
}

// withSentBearer returns a payment type carrying a bearer credential as the management API sends it.
func (s *ServiceTestSuite) withSentBearer(t providers.AuthorizationDetailType) providers.AuthorizationDetailType {
	properties, svcErr := sentAuthenticationProperties(bearer(testProcessorToken))
	s.Require().Nil(svcErr)
	t.Processor = &providers.AuthorizationDetailProcessor{Endpoint: testProcessorURL,
		AuthenticationProperties: properties}
	return t
}

// A created type is captured with its processor credential in clear, as its export refers to it.
func (s *ServiceTestSuite) TestCreateType_CapturesTheProcessorCredential() {
	withTestCrypto(s.T())
	capturer := s.recordCaptures()
	s.typeStore.EXPECT().GetTypeByValue(mock.Anything, testPaymentType).
		Return(providers.AuthorizationDetailType{}, errTypeNotFound)
	s.typeStore.EXPECT().CreateType(mock.Anything, mock.Anything).Return(nil)

	_, svcErr := s.service.CreateAuthorizationDetailType(s.ctx, apiType(s.withSentBearer(paymentType())))

	s.Require().Nil(svcErr)
	captured := s.captured(capturer)
	s.Equal(testPaymentType, captured.Type)
	s.Require().NotNil(captured.Processor)
	s.Require().NotNil(captured.Processor.Authentication)
	s.Equal(testProcessorToken, captured.Processor.Authentication.Properties["token"])
}

// An updated type is captured as it is stored after the update.
func (s *ServiceTestSuite) TestUpdateType_CapturesTheUpdatedType() {
	withTestCrypto(s.T())
	capturer := s.recordCaptures()
	stored := paymentType()
	stored.Processor = storedBearer(s.T())
	renamed := stored
	renamed.Name = renamedTypeName
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, stored.ID).Return(stored, nil).Once()
	s.typeStore.EXPECT().UpdateType(mock.Anything, mock.Anything, false).Return(nil).Once()
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, stored.ID).Return(renamed, nil).Once()

	_, svcErr := s.service.UpdateAuthorizationDetailType(s.ctx, stored.ID,
		s.withSentBearer(providers.AuthorizationDetailType{ID: stored.ID, Type: stored.Type, Name: renamedTypeName,
			Schema: stored.Schema, Reuse: stored.Reuse}))

	s.Require().Nil(svcErr)
	captured := s.captured(capturer)
	s.Equal(renamedTypeName, captured.Name)
	s.Equal(testProcessorToken, captured.Processor.Authentication.Properties["token"])
}

// A write that fails captures nothing.
func (s *ServiceTestSuite) TestCreateType_FailedWriteCapturesNothing() {
	capturer := s.recordCaptures()
	s.typeStore.EXPECT().GetTypeByValue(mock.Anything, testFlightType).
		Return(providers.AuthorizationDetailType{}, errTypeNotFound)
	s.typeStore.EXPECT().CreateType(mock.Anything, mock.Anything).Return(errors.New("db down"))

	_, svcErr := s.service.CreateAuthorizationDetailType(s.ctx, apiType(flightType()))

	s.Require().NotNil(svcErr)
	s.Empty(capturer.resources)
}
