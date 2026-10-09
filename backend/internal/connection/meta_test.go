// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/smtpauth"
)

type MetaTestSuite struct {
	suite.Suite
	handler *handler
}

func TestMetaSuite(t *testing.T) {
	suite.Run(t, new(MetaTestSuite))
}

func (s *MetaTestSuite) SetupTest() {
	s.handler, _, _ = newConnectionTestHandler(s.T())
}

// get issues a metadata request for the given vendor and returns the recorder.
func (s *MetaTestSuite) get(vendor string) *httptest.ResponseRecorder {
	target := "/connections/meta"
	if vendor != "" {
		target += "?vendor=" + vendor
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rr := httptest.NewRecorder()
	s.handler.handleGetConnectionMeta(rr, req)
	return rr
}

// decode reads a successful metadata response.
func (s *MetaTestSuite) decode(rr *httptest.ResponseRecorder) connectionMetaResponse {
	s.Require().Equal(http.StatusOK, rr.Code)
	var resp connectionMetaResponse
	s.Require().NoError(json.NewDecoder(rr.Body).Decode(&resp))
	return resp
}

// describe requests the metadata of one vendor. Filtering narrows the list rather than changing
// its shape, so the result is a one-element list naming that vendor.
func (s *MetaTestSuite) describe(vendor string) connectionVendorMeta {
	resp := s.decode(s.get(vendor))
	s.Require().Len(resp.Vendors, 1)
	s.Require().Equal(vendor, resp.Vendors[0].Vendor)
	return resp.Vendors[0]
}

// A vendor that dials out with configurable credentials describes the methods it accepts, which
// is what lets the console render an authentication section it was not compiled against.
func (s *MetaTestSuite) TestDescribesAuthenticatingVendors() {
	for _, vendor := range []string{emailSMTPVendorName} {
		s.Run(vendor, func() {
			resp := s.describe(vendor)
			s.Require().Len(resp.Authentication.Methods, 2)
			s.Equal(outboundauth.TypeNone, resp.Authentication.Methods[0].Type)
			s.Equal(outboundauth.TypeBasic, resp.Authentication.Methods[1].Type)
		})
	}
}

// Field order is part of the contract: a credential form must show the username above the
// password, which a keyed object could not express because encoding/json sorts map keys.
func (s *MetaTestSuite) TestBasicMethodDescribesItsFieldsInRenderOrder() {
	resp := s.describe(emailSMTPVendorName)

	basic := resp.Authentication.Methods[1]
	s.Require().Len(basic.Fields, 2)
	s.Equal(outboundauth.FieldBasicUsername, basic.Fields[0].Key)
	s.Equal(outboundauth.FieldBasicPassword, basic.Fields[1].Key)

	// The console renders a credential field as a write-only masked input.
	s.False(basic.Fields[0].Credential)
	s.True(basic.Fields[1].Credential)
	s.True(basic.Fields[0].Required)
	s.True(basic.Fields[1].Required)
	s.Equal(outboundauth.FieldType("string"), basic.Fields[0].Type)
	s.NotEmpty(basic.Fields[0].DisplayName)
}

// A vendor whose credentials are a fixed contract has no choice to offer, so it advertises no
// methods and the console renders no authentication section for it.
func (s *MetaTestSuite) TestVendorsWithoutConfigurableAuthenticationAdvertiseNone() {
	for _, vendor := range []string{"twilio", "vonage", "google", "oidc"} {
		s.Run(vendor, func() {
			resp := s.describe(vendor)
			s.Empty(resp.Authentication.Methods)
		})
	}
}

func (s *MetaTestSuite) TestUnknownVendorIsRejected() {
	s.Equal(http.StatusBadRequest, s.get("nope").Code)
}

// Without a vendor filter, every registered vendor is described, in a stable order, each with
// the same entry a filtered request returns for it.
func (s *MetaTestSuite) TestNoVendorDescribesEveryVendor() {
	resp := s.decode(s.get(""))

	names := make([]string, 0, len(resp.Vendors))
	for _, vendor := range resp.Vendors {
		names = append(names, vendor.Vendor)
		s.Equal(s.describe(vendor.Vendor), vendor, vendor.Vendor)
	}
	s.Equal([]string{"google", "github", "oidc", "oauth", "twilio", "vonage", smsGatewayVendorName,
		emailSMTPVendorName, authZENPDPVendorName}, names)
}

// The endpoint must not be able to advertise a method the sender service would reject, so it
// reads the same set the validator does.
func (s *MetaTestSuite) TestAdvertisedMethodsMatchTheValidatedSet() {
	cases := map[string][]outboundauth.Type{
		emailSMTPVendorName:  smtpauth.SupportedTypes(),
		smsGatewayVendorName: {outboundauth.TypeNone, outboundauth.TypeAPIKey},
		authZENPDPVendorName: {outboundauth.TypeNone, outboundauth.TypeBearer, outboundauth.TypeBasic,
			outboundauth.TypeAPIKey},
	}

	for vendor, supported := range cases {
		s.Run(vendor, func() {
			resp := s.describe(vendor)
			advertised := make([]outboundauth.Type, 0, len(resp.Authentication.Methods))
			for _, method := range resp.Authentication.Methods {
				advertised = append(advertised, method.Type)
			}
			s.Equal(supported, advertised)
		})
	}
}
