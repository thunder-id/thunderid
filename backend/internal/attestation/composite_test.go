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

package attestation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/attestationprovidermock"
)

type CompositeVerifierTestSuite struct {
	suite.Suite
	android  *attestationprovidermock.AttestationProviderMock
	apple    *attestationprovidermock.AttestationProviderMock
	verifier providers.AttestationProvider
}

func TestCompositeVerifierTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeVerifierTestSuite))
}

func (s *CompositeVerifierTestSuite) SetupTest() {
	s.android = attestationprovidermock.NewAttestationProviderMock(s.T())
	s.apple = attestationprovidermock.NewAttestationProviderMock(s.T())
	s.verifier = newCompositeVerifier(s.android, s.apple)
}

// An Android-configured request dispatches to the Android provider only.
func (s *CompositeVerifierTestSuite) TestVerify_DispatchesToAndroid() {
	cfg := &providers.AttestationConfig{Android: &providers.AndroidAttestationConfig{PackageName: "com.example.app"}}
	s.android.EXPECT().Verify(mock.Anything, cfg, "android-token").Return(true, nil)

	ok, svcErr := s.verifier.Verify(context.Background(), cfg, "android-token")

	s.True(ok)
	s.Nil(svcErr)
	s.apple.AssertNotCalled(s.T(), "Verify", mock.Anything, mock.Anything, mock.Anything)
}

// An Apple-configured request dispatches to the Apple provider only.
func (s *CompositeVerifierTestSuite) TestVerify_DispatchesToApple() {
	cfg := &providers.AttestationConfig{
		Apple: &providers.AppleAttestationConfig{TeamID: "T1", BundleID: "com.example.app"},
	}
	s.apple.EXPECT().Verify(mock.Anything, cfg, "apple-token").Return(true, nil)

	ok, svcErr := s.verifier.Verify(context.Background(), cfg, "apple-token")

	s.True(ok)
	s.Nil(svcErr)
	s.android.AssertNotCalled(s.T(), "Verify", mock.Anything, mock.Anything, mock.Anything)
}

// A definitive rejection or operational error from the dispatched platform provider is passed
// through unchanged.
func (s *CompositeVerifierTestSuite) TestVerify_PropagatesProviderResult() {
	cfg := &providers.AttestationConfig{Android: &providers.AndroidAttestationConfig{PackageName: "com.example.app"}}

	s.Run("definitive rejection", func() {
		s.SetupTest()
		s.android.EXPECT().Verify(mock.Anything, cfg, "bad-token").Return(false, nil)

		ok, svcErr := s.verifier.Verify(context.Background(), cfg, "bad-token")

		s.False(ok)
		s.Nil(svcErr)
	})

	s.Run("operational error", func() {
		s.SetupTest()
		s.android.EXPECT().Verify(mock.Anything, cfg, "any-token").Return(false, &tidcommon.InternalServerError)

		ok, svcErr := s.verifier.Verify(context.Background(), cfg, "any-token")

		s.False(ok)
		s.Equal(&tidcommon.InternalServerError, svcErr)
	})
}

// A configuration with neither platform set (nil, or an empty AttestationConfig) is an operational
// error; neither platform provider is called since there is nothing to dispatch to.
func (s *CompositeVerifierTestSuite) TestVerify_NoConfiguredPlatform() {
	cases := map[string]*providers.AttestationConfig{
		"nil config":   nil,
		"empty config": {},
	}
	for name, cfg := range cases {
		s.Run(name, func() {
			s.SetupTest()

			ok, svcErr := s.verifier.Verify(context.Background(), cfg, "anything")

			s.False(ok)
			s.Equal(&tidcommon.InternalServerError, svcErr)
			s.android.AssertNotCalled(s.T(), "Verify", mock.Anything, mock.Anything, mock.Anything)
			s.apple.AssertNotCalled(s.T(), "Verify", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}
