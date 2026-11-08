// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

// The OAuth services audit every token request, revocation and refused client on the bus, so they are
// never built without one, and no route is registered.
func (s *InitTestSuite) TestInitialize_RefusesANilEventBus() {
	mux := http.NewServeMux()

	validator, dispatcher, err := Initialize(mux, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, oauthconfig.Config{}, nil)

	s.ErrorIs(err, errNilEventBus)
	s.Nil(validator)
	s.Nil(dispatcher)
	_, pattern := mux.Handler(httptest.NewRequest(http.MethodPost, "/oauth2/token", nil))
	s.Empty(pattern)
}
