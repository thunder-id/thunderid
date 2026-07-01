/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package openid4vci

import (
	"context"
	"errors"
	"testing"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/openid4vci/credential"
)

type OfferTestSuite struct {
	suite.Suite
}

func TestOfferTestSuite(t *testing.T) {
	suite.Run(t, new(OfferTestSuite))
}

func (s *OfferTestSuite) newOfferStore() *openID4VCIStoreInterfaceMock {
	m := newOpenID4VCIStoreInterfaceMock(s.T())
	offers := map[string]*offerRecord{}
	m.EXPECT().SaveOffer(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, rec *offerRecord) error {
			offers[rec.ID] = rec
			return nil
		}).Maybe()
	m.EXPECT().GetOffer(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id string) (*offerRecord, bool) {
			rec, ok := offers[id]
			return rec, ok
		}).Maybe()
	return m
}

func (s *OfferTestSuite) TestGenerateCredentialOfferSuccess() {
	ctx := context.Background()
	store := s.newOfferStore()
	creds := newCredentialReaderMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
		Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
	svc := &service{
		cfg:   serviceConfig{CredentialIssuer: testIssuer, BaseURL: "https://issuer.example"},
		store: store,
		creds: creds,
	}

	offer, deepLink, err := svc.GenerateCredentialOffer(ctx, "eudi-pid")
	s.Require().NoError(err)
	s.Equal(testIssuer, offer["credential_issuer"])
	s.Equal([]string{"eudi-pid"}, offer["credential_configuration_ids"])
	s.Contains(deepLink, credentialOfferScheme)
	s.Contains(deepLink, "credential_offer_uri=")
}

func (s *OfferTestSuite) TestGenerateCredentialOfferUnknownConfig() {
	ctx := context.Background()
	creds := newCredentialReaderMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "missing").
		Return(nil, &tidcommon.ServiceError{Code: "not-found"})
	svc := &service{cfg: serviceConfig{CredentialIssuer: testIssuer}, creds: creds}

	_, _, err := svc.GenerateCredentialOffer(ctx, "missing")
	s.ErrorIs(err, ErrUnsupportedCredential)
}

func (s *OfferTestSuite) TestGenerateCredentialOfferStoreError() {
	ctx := context.Background()
	store := newOpenID4VCIStoreInterfaceMock(s.T())
	store.EXPECT().SaveOffer(mock.Anything, mock.Anything).Return(errors.New("store failed"))
	creds := newCredentialReaderMock(s.T())
	creds.EXPECT().GetCredentialConfigurationByHandle(ctx, "eudi-pid").
		Return(&credential.CredentialConfigurationDTO{Handle: "eudi-pid", VCT: "v"}, nil)
	svc := &service{
		cfg:   serviceConfig{CredentialIssuer: testIssuer, BaseURL: "https://i"},
		store: store,
		creds: creds,
	}

	_, _, err := svc.GenerateCredentialOffer(ctx, "eudi-pid")
	s.ErrorIs(err, ErrIssuance)
}

func (s *OfferTestSuite) TestGetCredentialOffer() {
	ctx := context.Background()

	s.Run("Success", func() {
		store := newOpenID4VCIStoreInterfaceMock(s.T())
		store.EXPECT().GetOffer(ctx, "o1").Return(
			&offerRecord{
				ID: "o1", Offer: map[string]interface{}{"k": "v"}, ExpiresAt: time.Now().Add(time.Minute),
			}, true)
		svc := &service{store: store}
		offer, err := svc.GetCredentialOffer(ctx, "o1")
		s.Require().NoError(err)
		s.Equal("v", offer["k"])
	})

	s.Run("NotFound", func() {
		store := newOpenID4VCIStoreInterfaceMock(s.T())
		store.EXPECT().GetOffer(ctx, "missing").Return(nil, false)
		svc := &service{store: store}
		_, err := svc.GetCredentialOffer(ctx, "missing")
		s.ErrorIs(err, ErrUnsupportedCredential)
	})

	s.Run("Expired", func() {
		store := newOpenID4VCIStoreInterfaceMock(s.T())
		store.EXPECT().GetOffer(ctx, "old").Return(
			&offerRecord{ID: "old", Offer: map[string]interface{}{}, ExpiresAt: time.Now().Add(-time.Minute)}, true)
		svc := &service{store: store}
		_, err := svc.GetCredentialOffer(ctx, "old")
		s.ErrorIs(err, ErrUnsupportedCredential)
	})
}
