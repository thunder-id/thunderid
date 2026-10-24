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

// declaredZebraYAML and declaredAlphaYAML declare two email templates; the file store returns them
// handle-sorted (alpha, zebra), and the composite merge concatenates them ahead of the DB items.
const declaredZebraYAML = `resource_type: notification_template
id: zebra-email
channel: email
handle: zebra
ouId: 01900000-0000-7000-8000-000000000001
displayName: Zebra
content:
  subject: Z
  body: z
`

const declaredAlphaYAML = `resource_type: notification_template
id: alpha-email
channel: email
handle: alpha
ouId: 01900000-0000-7000-8000-000000000001
displayName: Alpha
content:
  subject: A
  body: a
`

// newOrderingComposite builds a composite store over two declared templates (zebra, alpha) and the
// given DB templates, with the DB store mocked to return them regardless of the requested window.
func (s *CompositeStoreTestSuite) newOrderingComposite(dbItems []templateDAO) notificationTemplateStoreInterface {
	fileStore := newTestFileStore()
	loadDeclared(s.T(), fileStore, declaredZebraYAML)
	loadDeclared(s.T(), fileStore, declaredAlphaYAML)

	db := newNotificationTemplateStoreInterfaceMock(s.T())
	db.On("CountTemplates", mock.Anything, ChannelTypeEmail).Return(len(dbItems), nil)
	db.On("ListTemplates", mock.Anything, ChannelTypeEmail, mock.Anything, mock.Anything).Return(dbItems, nil)
	return newCompositeStore(fileStore, db)
}

func handlesOf(items []templateDAO) []string {
	out := make([]string, len(items))
	for i := range items {
		out[i] = items[i].Handle
	}
	return out
}

// TestListConcatenatesDeclaredThenDB verifies the merged list places declared templates (in the
// file store's handle order) ahead of DB templates (in the DB's order), without a global re-sort —
// the concat-and-dedup contract shared by every composite store.
func (s *CompositeStoreTestSuite) TestListConcatenatesDeclaredThenDB() {
	composite := s.newOrderingComposite([]templateDAO{emailDAO("mango", "mango"), emailDAO("apple", "apple")})

	list, err := composite.ListTemplates(s.ctx, ChannelTypeEmail, 100, 0)
	s.Require().NoError(err)
	// Declared (file-store handle order: alpha, zebra) then DB (as returned: mango, apple).
	s.Require().Equal([]string{"alpha", "zebra", "mango", "apple"}, handlesOf(list))
}

// TestPaginationNoOverlapOrGap verifies consecutive pages partition the merged set exactly once
// each. Concatenation keeps each store's order stable across calls, so paged windows never overlap
// or drop rows.
func (s *CompositeStoreTestSuite) TestPaginationNoOverlapOrGap() {
	composite := s.newOrderingComposite([]templateDAO{emailDAO("mango", "mango"), emailDAO("apple", "apple")})

	page1, err := composite.ListTemplates(s.ctx, ChannelTypeEmail, 2, 0)
	s.Require().NoError(err)
	page2, err := composite.ListTemplates(s.ctx, ChannelTypeEmail, 2, 2)
	s.Require().NoError(err)

	combined := append(handlesOf(page1), handlesOf(page2)...)
	s.Require().Equal([]string{"alpha", "zebra", "mango", "apple"}, combined)
}

// TestDeclaredWinsOnHandleCollision verifies a declared template shadows a DB template that shares
// its handle, and the colliding DB row is dropped from the merge.
func (s *CompositeStoreTestSuite) TestDeclaredWinsOnHandleCollision() {
	dbAlpha := emailDAO("alpha", "DB Alpha") // collides with the declared "alpha"
	composite := s.newOrderingComposite([]templateDAO{dbAlpha, emailDAO("mango", "mango")})

	list, err := composite.ListTemplates(s.ctx, ChannelTypeEmail, 100, 0)
	s.Require().NoError(err)
	// Declared (alpha, zebra) then the surviving DB item (mango); the DB "alpha" is deduped out.
	s.Require().Equal([]string{"alpha", "zebra", "mango"}, handlesOf(list))
	for _, item := range list {
		if item.Handle == "alpha" {
			s.Require().Equal("Alpha", item.DisplayName, "declared template must win over the DB one")
		}
	}
}

func (s *CompositeStoreTestSuite) TestMergesAndRefusesDeclaredWrites() {
	fileStore := newTestFileStore()
	loadDeclared(s.T(), fileStore, declaredEmailYAML)
	declaredID, err := fileStore.GetTemplateByHandle(s.ctx, ChannelTypeEmail, "welcome")
	s.Require().NoError(err)

	reset := templateDAO{ID: "db-1", Channel: ChannelTypeEmail, Handle: "reset", DisplayName: "Reset",
		Content: TemplateContent{Subject: "S", Body: "B"}}
	db := newNotificationTemplateStoreInterfaceMock(s.T())
	db.On("CountTemplates", mock.Anything, ChannelTypeEmail).Return(1, nil)
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
	err = composite.DeleteTemplate(s.ctx, ChannelTypeEmail, declaredID.ID)
	s.Require().True(errors.Is(err, errDeclarativeTemplate))
	// Declared writes must never reach the DB store.
	db.AssertExpectations(s.T())
}

// TestCreateTemplateGoesToDB verifies a create is delegated to the DB store unchanged.
func (s *CompositeStoreTestSuite) TestCreateTemplateGoesToDB() {
	dao := emailDAO("db-new", "New")
	db := newNotificationTemplateStoreInterfaceMock(s.T())
	db.On("CreateTemplate", mock.Anything, dao).Return(nil).Once()
	composite := newCompositeStore(newTestFileStore(), db)

	s.Require().NoError(composite.CreateTemplate(s.ctx, dao))
	db.AssertExpectations(s.T())
}

// TestGetTemplatePrefersDBThenFile verifies GetTemplate returns the DB row when present and falls
// back to the declared file store otherwise.
func (s *CompositeStoreTestSuite) TestGetTemplatePrefersDBThenFile() {
	fileStore := newTestFileStore()
	loadDeclared(s.T(), fileStore, declaredEmailYAML)
	declared, err := fileStore.GetTemplateByHandle(s.ctx, ChannelTypeEmail, "welcome")
	s.Require().NoError(err)

	dbRow := emailDAO("db-1", "DB One")
	db := newNotificationTemplateStoreInterfaceMock(s.T())
	db.On("GetTemplate", mock.Anything, ChannelTypeEmail, "db-1").Return(dbRow, nil)
	db.On("GetTemplate", mock.Anything, ChannelTypeEmail, declared.ID).
		Return(templateDAO{}, errTemplateNotFound)
	composite := newCompositeStore(fileStore, db)

	// DB hit.
	got, err := composite.GetTemplate(s.ctx, ChannelTypeEmail, "db-1")
	s.Require().NoError(err)
	s.Require().Equal("DB One", got.DisplayName)

	// DB miss falls back to the declared template.
	got, err = composite.GetTemplate(s.ctx, ChannelTypeEmail, declared.ID)
	s.Require().NoError(err)
	s.Require().Equal("Welcome", got.DisplayName)
}

// TestGetTemplateByHandlePrefersDeclared verifies the handle lookup checks the file store first so a
// declared template wins, then falls back to the DB store.
func (s *CompositeStoreTestSuite) TestGetTemplateByHandlePrefersDeclared() {
	fileStore := newTestFileStore()
	loadDeclared(s.T(), fileStore, declaredEmailYAML)

	dbRow := emailDAO("db-reset", "DB Reset")
	dbRow.Handle = "reset"
	db := newNotificationTemplateStoreInterfaceMock(s.T())
	db.On("GetTemplateByHandle", mock.Anything, ChannelTypeEmail, "reset").Return(dbRow, nil)
	composite := newCompositeStore(fileStore, db)

	// Declared handle served from the file store without touching the DB.
	got, err := composite.GetTemplateByHandle(s.ctx, ChannelTypeEmail, "welcome")
	s.Require().NoError(err)
	s.Require().Equal("Welcome", got.DisplayName)

	// Unknown-to-file handle falls back to the DB store.
	got, err = composite.GetTemplateByHandle(s.ctx, ChannelTypeEmail, "reset")
	s.Require().NoError(err)
	s.Require().Equal("DB Reset", got.DisplayName)
}

// TestIsHandleExistsChecksBothStores verifies the handle-existence check reports true when either
// store has the handle and false when neither does.
func (s *CompositeStoreTestSuite) TestIsHandleExistsChecksBothStores() {
	fileStore := newTestFileStore()
	loadDeclared(s.T(), fileStore, declaredEmailYAML)

	db := newNotificationTemplateStoreInterfaceMock(s.T())
	db.On("IsHandleExists", mock.Anything, ChannelTypeEmail, "reset").Return(true, nil)
	db.On("IsHandleExists", mock.Anything, ChannelTypeEmail, "missing").Return(false, nil)
	composite := newCompositeStore(fileStore, db)

	// Declared handle: file store short-circuits to true.
	exists, err := composite.IsHandleExists(s.ctx, ChannelTypeEmail, "welcome")
	s.Require().NoError(err)
	s.Require().True(exists)

	// DB-only handle.
	exists, err = composite.IsHandleExists(s.ctx, ChannelTypeEmail, "reset")
	s.Require().NoError(err)
	s.Require().True(exists)

	// Neither store has it.
	exists, err = composite.IsHandleExists(s.ctx, ChannelTypeEmail, "missing")
	s.Require().NoError(err)
	s.Require().False(exists)
}

// TestDeleteTemplateDBPath verifies a non-declared delete reaches the DB store.
func (s *CompositeStoreTestSuite) TestDeleteTemplateDBPath() {
	db := newNotificationTemplateStoreInterfaceMock(s.T())
	db.On("DeleteTemplate", mock.Anything, ChannelTypeEmail, "db-1").Return(nil).Once()
	// The file store is empty, so the id is not declared and the delete falls through to the DB.
	composite := newCompositeStore(newTestFileStore(), db)

	s.Require().NoError(composite.DeleteTemplate(s.ctx, ChannelTypeEmail, "db-1"))
	db.AssertExpectations(s.T())
}
