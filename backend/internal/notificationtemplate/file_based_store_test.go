// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

// newTestFileStore creates a file store over an isolated in-memory backing store for tests.
func newTestFileStore() *templateFileBasedStore {
	return &templateFileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeNotificationTemplate),
	}
}

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

// TestDeclaredTemplateIsFlaggedReadOnly verifies put marks every declared template read-only, so the
// flag surfaces on each read path regardless of how the template is fetched.
func (s *FileBasedStoreTestSuite) TestDeclaredTemplateIsFlaggedReadOnly() {
	loadDeclared(s.T(), s.fileStore, declaredEmailYAML)

	byHandle, err := s.fileStore.GetTemplateByHandle(s.ctx, ChannelTypeEmail, "welcome")
	s.Require().NoError(err)
	s.Require().True(byHandle.IsReadOnly, "a declared template read by handle must be read-only")

	byID, err := s.fileStore.GetTemplate(s.ctx, ChannelTypeEmail, byHandle.ID)
	s.Require().NoError(err)
	s.Require().True(byID.IsReadOnly, "a declared template read by id must be read-only")

	listed, err := s.fileStore.ListTemplates(s.ctx, ChannelTypeEmail, 100, 0)
	s.Require().NoError(err)
	s.Require().Len(listed, 1)
	s.Require().True(listed[0].IsReadOnly, "a declared template in a listing must be read-only")
}

// declaredSMSYAML is a minimal valid SMS template declaration (no subject, no design).
const declaredSMSYAML = `resource_type: notification_template
id: otp-sms
channel: sms
handle: otp
ouId: 01900000-0000-7000-8000-000000000001
displayName: OTP
content:
  body: Your code is {{ctx(code)}}
`

// TestListByChannelOrdersByHandleAndFiltersChannel verifies a channel's templates come back sorted by
// handle, and templates of another channel are excluded.
func (s *FileBasedStoreTestSuite) TestListByChannelOrdersByHandleAndFiltersChannel() {
	loadDeclared(s.T(), s.fileStore, declaredZebraYAML)
	loadDeclared(s.T(), s.fileStore, declaredAlphaYAML)
	loadDeclared(s.T(), s.fileStore, declaredSMSYAML)

	email, err := s.fileStore.ListTemplates(s.ctx, ChannelTypeEmail, 100, 0)
	s.Require().NoError(err)
	s.Require().Equal([]string{"alpha", "zebra"}, handlesOf(email))

	sms, err := s.fileStore.ListTemplates(s.ctx, ChannelTypeSMS, 100, 0)
	s.Require().NoError(err)
	s.Require().Equal([]string{"otp"}, handlesOf(sms))
}

// TestListTemplatesPagination exercises the three windowing branches: a middle page, a clamped last
// page, and an offset past the end.
func (s *FileBasedStoreTestSuite) TestListTemplatesPagination() {
	loadDeclared(s.T(), s.fileStore, declaredZebraYAML)
	loadDeclared(s.T(), s.fileStore, declaredAlphaYAML)

	first, err := s.fileStore.ListTemplates(s.ctx, ChannelTypeEmail, 1, 0)
	s.Require().NoError(err)
	s.Require().Equal([]string{"alpha"}, handlesOf(first))

	// limit runs past the end: the window is clamped to what remains.
	last, err := s.fileStore.ListTemplates(s.ctx, ChannelTypeEmail, 10, 1)
	s.Require().NoError(err)
	s.Require().Equal([]string{"zebra"}, handlesOf(last))

	// offset past the end returns an empty (non-nil) page.
	beyond, err := s.fileStore.ListTemplates(s.ctx, ChannelTypeEmail, 10, 5)
	s.Require().NoError(err)
	s.Require().Empty(beyond)
}

// TestCountTemplatesByChannel verifies counts are scoped to the channel.
func (s *FileBasedStoreTestSuite) TestCountTemplatesByChannel() {
	loadDeclared(s.T(), s.fileStore, declaredAlphaYAML)
	loadDeclared(s.T(), s.fileStore, declaredSMSYAML)

	emailCount, err := s.fileStore.CountTemplates(s.ctx, ChannelTypeEmail)
	s.Require().NoError(err)
	s.Require().Equal(1, emailCount)

	smsCount, err := s.fileStore.CountTemplates(s.ctx, ChannelTypeSMS)
	s.Require().NoError(err)
	s.Require().Equal(1, smsCount)
}

// TestLookupsMissOnUnknownKeys verifies the not-found paths of the per-id, per-handle, and
// handle-existence lookups.
func (s *FileBasedStoreTestSuite) TestLookupsMissOnUnknownKeys() {
	loadDeclared(s.T(), s.fileStore, declaredAlphaYAML)

	_, err := s.fileStore.GetTemplate(s.ctx, ChannelTypeEmail, "no-such-id")
	s.Require().True(errors.Is(err, errTemplateNotFound))

	_, err = s.fileStore.GetTemplateByHandle(s.ctx, ChannelTypeEmail, "no-such-handle")
	s.Require().True(errors.Is(err, errTemplateNotFound))

	exists, err := s.fileStore.IsHandleExists(s.ctx, ChannelTypeEmail, "no-such-handle")
	s.Require().NoError(err)
	s.Require().False(exists)

	// A declared handle checked against the wrong channel does not match.
	exists, err = s.fileStore.IsHandleExists(s.ctx, ChannelTypeSMS, "alpha")
	s.Require().NoError(err)
	s.Require().False(exists)
}
