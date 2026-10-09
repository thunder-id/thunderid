// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type CompositeStoreTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestCompositeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CompositeStoreTestSuite))
}

func (s *CompositeStoreTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CompositeStoreTestSuite) TestMergesAndRefusesDeclaredWrites() {
	fileStore := newTestFileStore()
	loadDeclared(s.T(), fileStore, declaredEmailYAML)
	declaredID, err := fileStore.GetTemplateByHandle(s.ctx, ChannelTypeEmail, "welcome")
	s.Require().NoError(err)

	reset := templateDAO{ID: "db-1", Channel: ChannelTypeEmail, Handle: "reset", DisplayName: "Reset",
		Content: TemplateContent{Subject: "S", Body: "B"}}
	db := newNotificationTemplateStoreInterfaceMock(s.T())
	db.On("ListTemplates", mock.Anything, ChannelTypeEmail, mock.Anything, mock.Anything).
		Return([]templateDAO{reset}, nil)
	db.On("UpdateTemplate", mock.Anything, reset).Return(nil).Once()
	composite := newCompositeStore(fileStore, db)

	list, err := composite.ListTemplates(s.ctx, ChannelTypeEmail, 100, 0)
	s.Require().NoError(err)
	s.Require().Len(list, 2, "expected both sources merged")

	count, _ := composite.CountTemplates(s.ctx, ChannelTypeEmail)
	s.Require().Equal(2, count)

	// A DB write goes through to the DB store.
	s.Require().NoError(composite.UpdateTemplate(s.ctx, reset))
	// A declared write is refused.
	s.Require().True(errors.Is(composite.UpdateTemplate(s.ctx, declaredID), errDeclarativeTemplate))
	s.Require().True(errors.Is(composite.DeleteTemplate(s.ctx, ChannelTypeEmail, declaredID.ID), errDeclarativeTemplate))
	// Declared writes must never reach the DB store.
	db.AssertExpectations(s.T())
}
