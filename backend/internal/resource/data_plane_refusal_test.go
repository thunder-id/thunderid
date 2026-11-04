// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type DataPlaneRefusalTestSuite struct {
	suite.Suite
}

func TestDataPlaneRefusalTestSuite(t *testing.T) {
	suite.Run(t, new(DataPlaneRefusalTestSuite))
}

// A create refused until the data plane is ready is answered as a conflict: the request is sound,
// but the deployment is not ready for it yet.
func (s *DataPlaneRefusalTestSuite) TestADataPlaneRefusalIsAConflict() {
	for _, refusal := range []tidcommon.ServiceError{
		declarativeresource.ErrorDataPlaneNotReady, declarativeresource.ErrorNoDataPlane,
	} {
		recorder := httptest.NewRecorder()
		handleError(context.Background(), recorder, &refusal)

		s.Equal(http.StatusConflict, recorder.Code, refusal.Code)
		s.Contains(recorder.Body.String(), refusal.Code)
	}
}
