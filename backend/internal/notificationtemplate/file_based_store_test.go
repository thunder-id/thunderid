// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
)

type FileBasedStoreTestSuite struct {
	suite.Suite
	ctx       context.Context
	fileStore *templateFileBasedStore
}

func TestFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedStoreTestSuite))
}

func (s *FileBasedStoreTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.fileStore = newTestFileStore()
}

func (s *FileBasedStoreTestSuite) TestReadsBackWhatWasDeclared() {
	loadDeclared(s.T(), s.fileStore, declaredEmailYAML)

	got, err := s.fileStore.GetTemplateByHandle(s.ctx, ChannelTypeEmail, "welcome")
	s.Require().NoError(err)
	s.Require().Equal("Welcome", got.DisplayName)
	s.Require().Equal("Hi", got.Content.Subject)

	exists, _ := s.fileStore.IsHandleExists(s.ctx, ChannelTypeEmail, "welcome")
	s.Require().True(exists)

	_, err = s.fileStore.GetTemplate(s.ctx, ChannelTypeSMS, got.ID)
	s.Require().True(errors.Is(err, errTemplateNotFound), "expected not found for the wrong channel")
}

func (s *FileBasedStoreTestSuite) TestRefusesWrites() {
	s.Require().True(errors.Is(s.fileStore.CreateTemplate(s.ctx, templateDAO{}), errDeclarativeTemplate))
	s.Require().True(errors.Is(s.fileStore.UpdateTemplate(s.ctx, templateDAO{}), errDeclarativeTemplate))
	s.Require().True(errors.Is(s.fileStore.DeleteTemplate(s.ctx, ChannelTypeEmail, "x"), errDeclarativeTemplate))
}
