// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) TestInitialize_RefusesANilEventBus() {
	service, resolver, err := Initialize(http.NewServeMux(), reversingCrypto{}, nil, nil)

	s.ErrorIs(err, errNilEventBus)
	s.Nil(service)
	s.Nil(resolver)
}
