// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/tests/mocks/cachemock"
)

// CacheBackedStoreTestSuite tests the cacheBackedStore decorator with a mocked cache and inner store.
type CacheBackedStoreTestSuite struct {
	suite.Suite
	mockStore *notificationTemplateStoreInterfaceMock
	cache     *cachemock.CacheInterfaceMock[templateDAO]
	cacheData map[string]templateDAO
	store     notificationTemplateStoreInterface
	ctx       context.Context
}

func TestCacheBackedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CacheBackedStoreTestSuite))
}

func (s *CacheBackedStoreTestSuite) SetupTest() {
	s.mockStore = newNotificationTemplateStoreInterfaceMock(s.T())
	s.cacheData = make(map[string]templateDAO)
	s.cache = cachemock.NewCacheInterfaceMock[templateDAO](s.T())
	setupTemplateCacheMock(s.cache, s.cacheData)
	s.store = newCacheBackedStore(s.cache, s.mockStore)
	s.ctx = context.Background()
}

// setupTemplateCacheMock configures a cache mock to track Set/Get/Delete against a backing map, so the
// decorator's cache effects are observable (mirrors entitytype's cache_backed_store_test).
func setupTemplateCacheMock(mockCache *cachemock.CacheInterfaceMock[templateDAO],
	data map[string]templateDAO) {
	mockCache.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, key cache.CacheKey, value templateDAO) error {
			data[key.Key] = value
			return nil
		}).Maybe()
	mockCache.EXPECT().Get(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, key cache.CacheKey) (templateDAO, bool) {
			v, ok := data[key.Key]
			return v, ok
		}).Maybe()
	mockCache.EXPECT().Delete(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, key cache.CacheKey) error {
			delete(data, key.Key)
			return nil
		}).Maybe()
	mockCache.EXPECT().Clear(mock.Anything).Return(nil).Maybe()
	mockCache.EXPECT().IsEnabled().Return(true).Maybe()
	mockCache.EXPECT().GetName().Return("mockCache").Maybe()
	mockCache.EXPECT().CleanupExpired().Maybe()
}

// emailDAO builds an email templateDAO for tests (handle defaults to the id for brevity).
func emailDAO(id, displayName string) templateDAO {
	return templateDAO{ID: id, Channel: ChannelTypeEmail, Handle: id, DisplayName: displayName,
		Content: TemplateContent{Subject: "s", Body: "b"}}
}

// cachedDAO reports the cached entry (if any) for an email handle, using the production key format.
func (s *CacheBackedStoreTestSuite) cachedDAO(handle string) (templateDAO, bool) {
	v, ok := s.cacheData[cacheKey(ChannelTypeEmail, handle).Key]
	return v, ok
}

// --- GetTemplateByHandle (runtime hot path, cached) ---

// TestGetTemplateByHandle_CacheHit_DoesNotCallStore verifies a hit is served from cache without the store.
func (s *CacheBackedStoreTestSuite) TestGetTemplateByHandle_CacheHit_DoesNotCallStore() {
	s.cacheData[cacheKey(ChannelTypeEmail, "h1").Key] = emailDAO("h1", "A")

	got, err := s.store.GetTemplateByHandle(s.ctx, ChannelTypeEmail, "h1")
	s.NoError(err)
	s.Equal("A", got.DisplayName)
	s.mockStore.AssertNotCalled(s.T(), "GetTemplateByHandle")
}

// TestGetTemplateByHandle_CacheMiss_LoadsAndCaches verifies a miss loads from the store and caches it.
func (s *CacheBackedStoreTestSuite) TestGetTemplateByHandle_CacheMiss_LoadsAndCaches() {
	s.mockStore.On("GetTemplateByHandle", mock.Anything, ChannelTypeEmail, "h1").
		Return(emailDAO("h1", "A"), nil).Once()

	got, err := s.store.GetTemplateByHandle(s.ctx, ChannelTypeEmail, "h1")
	s.NoError(err)
	s.Equal("A", got.DisplayName)
	_, ok := s.cachedDAO("h1")
	s.True(ok)
	s.mockStore.AssertExpectations(s.T())
}

// TestGetTemplateByHandle_StoreError_NotCached verifies a store error is surfaced and nothing is cached.
func (s *CacheBackedStoreTestSuite) TestGetTemplateByHandle_StoreError_NotCached() {
	s.mockStore.On("GetTemplateByHandle", mock.Anything, ChannelTypeEmail, "bad").
		Return(templateDAO{}, errTemplateNotFound).Once()

	_, err := s.store.GetTemplateByHandle(s.ctx, ChannelTypeEmail, "bad")
	s.ErrorIs(err, errTemplateNotFound)
	_, ok := s.cachedDAO("bad")
	s.False(ok)
}

// --- GetTemplate (management path, uncached) ---

// TestGetTemplate_ByID_Delegates_NotCached verifies by-id reads always delegate and never cache.
func (s *CacheBackedStoreTestSuite) TestGetTemplate_ByID_Delegates_NotCached() {
	s.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, "t1").Return(emailDAO("t1", "A"), nil).Once()

	got, err := s.store.GetTemplate(s.ctx, ChannelTypeEmail, "t1")
	s.NoError(err)
	s.Equal("A", got.DisplayName)
	s.Empty(s.cacheData)
	s.mockStore.AssertExpectations(s.T())
}

// --- CreateTemplate (write-through) ---

func (s *CacheBackedStoreTestSuite) TestCreateTemplate_CachesNewRow() {
	dao := emailDAO("h1", "A")
	s.mockStore.On("CreateTemplate", mock.Anything, dao).Return(nil).Once()

	s.NoError(s.store.CreateTemplate(s.ctx, dao))
	cached, ok := s.cachedDAO("h1")
	s.True(ok)
	s.Equal("A", cached.DisplayName)
}

func (s *CacheBackedStoreTestSuite) TestCreateTemplate_StoreError_NotCached() {
	dao := emailDAO("h1", "A")
	s.mockStore.On("CreateTemplate", mock.Anything, dao).Return(errors.New("db error")).Once()

	s.Error(s.store.CreateTemplate(s.ctx, dao))
	_, ok := s.cachedDAO("h1")
	s.False(ok)
}

// --- UpdateTemplate (write-through) ---

// TestUpdateTemplate_ReCachesFullRow verifies the decorator caches the entire row it is handed on
// update (including the immutable handle), so a subsequent read returns the whole committed template,
// not a partial one.
func (s *CacheBackedStoreTestSuite) TestUpdateTemplate_ReCachesFullRow() {
	s.cacheData[cacheKey(ChannelTypeEmail, "otp-verification").Key] = emailDAO("otp-verification", "A")
	updated := templateDAO{
		ID: "t1", Channel: ChannelTypeEmail, Handle: "otp-verification", DisplayName: "B",
		Content: TemplateContent{Subject: "s2", Body: "b2"},
	}
	s.mockStore.On("UpdateTemplate", mock.Anything, updated).Return(nil).Once()

	s.NoError(s.store.UpdateTemplate(s.ctx, updated))
	cached, ok := s.cachedDAO("otp-verification")
	s.True(ok)
	s.Equal(updated, cached)
}

func (s *CacheBackedStoreTestSuite) TestUpdateTemplate_StoreError_CacheUntouched() {
	s.cacheData[cacheKey(ChannelTypeEmail, "h1").Key] = emailDAO("h1", "A")
	s.mockStore.On("UpdateTemplate", mock.Anything, mock.Anything).Return(errors.New("db error")).Once()

	s.Error(s.store.UpdateTemplate(s.ctx, emailDAO("h1", "B")))
	cached, ok := s.cachedDAO("h1")
	s.True(ok)
	s.Equal("A", cached.DisplayName)
}

// --- DeleteTemplate (invalidate) ---

func (s *CacheBackedStoreTestSuite) TestDeleteTemplate_Invalidates() {
	s.cacheData[cacheKey(ChannelTypeEmail, "h1").Key] = emailDAO("h1", "A")
	// Delete addresses by id; the decorator reads the row first to resolve the handle for invalidation.
	s.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, "t1").Return(emailDAO("h1", "A"), nil).Once()
	s.mockStore.On("DeleteTemplate", mock.Anything, ChannelTypeEmail, "t1").Return(nil).Once()

	s.NoError(s.store.DeleteTemplate(s.ctx, ChannelTypeEmail, "t1"))
	_, ok := s.cachedDAO("h1")
	s.False(ok)
}

func (s *CacheBackedStoreTestSuite) TestDeleteTemplate_StoreError_CacheUntouched() {
	s.cacheData[cacheKey(ChannelTypeEmail, "h1").Key] = emailDAO("h1", "A")
	s.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, "t1").Return(emailDAO("h1", "A"), nil).Once()
	s.mockStore.On("DeleteTemplate", mock.Anything, ChannelTypeEmail, "t1").Return(errors.New("db error")).Once()

	s.Error(s.store.DeleteTemplate(s.ctx, ChannelTypeEmail, "t1"))
	_, ok := s.cachedDAO("h1")
	s.True(ok)
}

// TestDeleteTemplate_ReadError_AbortsWithoutDeleting verifies a non-not-found read error aborts the
// delete, so the row is never removed while its cache entry could not be resolved for invalidation.
func (s *CacheBackedStoreTestSuite) TestDeleteTemplate_ReadError_AbortsWithoutDeleting() {
	s.cacheData[cacheKey(ChannelTypeEmail, "h1").Key] = emailDAO("h1", "A")
	s.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, "t1").
		Return(templateDAO{}, errors.New("db error")).Once()

	s.Error(s.store.DeleteTemplate(s.ctx, ChannelTypeEmail, "t1"))
	s.mockStore.AssertNotCalled(s.T(), "DeleteTemplate")
	_, ok := s.cachedDAO("h1")
	s.True(ok)
}

// TestDeleteTemplate_NotFound_StillDeletes verifies a missing row is tolerated: the delete still runs
// (idempotent) and there is nothing to invalidate.
func (s *CacheBackedStoreTestSuite) TestDeleteTemplate_NotFound_StillDeletes() {
	s.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, "t1").
		Return(templateDAO{}, errTemplateNotFound).Once()
	s.mockStore.On("DeleteTemplate", mock.Anything, ChannelTypeEmail, "t1").Return(nil).Once()

	s.NoError(s.store.DeleteTemplate(s.ctx, ChannelTypeEmail, "t1"))
}

// --- Pass-through operations ---

func (s *CacheBackedStoreTestSuite) TestListTemplates_Delegates() {
	s.mockStore.On("ListTemplates", mock.Anything, ChannelTypeEmail, 10, 0).
		Return([]templateDAO{emailDAO("t1", "A")}, nil).Once()

	out, err := s.store.ListTemplates(s.ctx, ChannelTypeEmail, 10, 0)
	s.NoError(err)
	s.Len(out, 1)
	s.mockStore.AssertExpectations(s.T())
}

func (s *CacheBackedStoreTestSuite) TestCountTemplates_Delegates() {
	s.mockStore.On("CountTemplates", mock.Anything, ChannelTypeEmail).Return(3, nil).Once()

	n, err := s.store.CountTemplates(s.ctx, ChannelTypeEmail)
	s.NoError(err)
	s.Equal(3, n)
	s.mockStore.AssertExpectations(s.T())
}

func (s *CacheBackedStoreTestSuite) TestIsHandleExists_Delegates() {
	s.mockStore.On("IsHandleExists", mock.Anything, ChannelTypeEmail, "otp-verification").Return(true, nil).Once()

	ok, err := s.store.IsHandleExists(s.ctx, ChannelTypeEmail, "otp-verification")
	s.NoError(err)
	s.True(ok)
	s.mockStore.AssertExpectations(s.T())
}

// --- Keying, skip, and defensive branches ---

// TestCacheKey_ChannelScoped verifies the same handle on different channels maps to distinct keys.
func (s *CacheBackedStoreTestSuite) TestCacheKey_ChannelScoped() {
	s.Equal("email:x", cacheKey(ChannelTypeEmail, "x").Key)
	s.Equal("sms:x", cacheKey(ChannelTypeSMS, "x").Key)
	s.NotEqual(cacheKey(ChannelTypeEmail, "x").Key, cacheKey(ChannelTypeSMS, "x").Key)
}

// TestCreateTemplate_EmptyHandle_NotCached covers the set() guard that skips a dao with no handle.
func (s *CacheBackedStoreTestSuite) TestCreateTemplate_EmptyHandle_NotCached() {
	dao := templateDAO{ID: "t1", Channel: ChannelTypeEmail, Content: TemplateContent{Subject: "s", Body: "b"}}
	s.mockStore.On("CreateTemplate", mock.Anything, dao).Return(nil).Once()

	s.NoError(s.store.CreateTemplate(s.ctx, dao))
	s.Empty(s.cacheData)
}

// TestCreateTemplate_CacheSetError_DoesNotFail verifies a cache Set failure is logged, not surfaced.
func (s *CacheBackedStoreTestSuite) TestCreateTemplate_CacheSetError_DoesNotFail() {
	errCache := cachemock.NewCacheInterfaceMock[templateDAO](s.T())
	errCache.EXPECT().Set(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("cache down")).Maybe()
	store := newCacheBackedStore(errCache, s.mockStore)
	s.mockStore.On("CreateTemplate", mock.Anything, mock.Anything).Return(nil).Once()

	s.NoError(store.CreateTemplate(s.ctx, emailDAO("h1", "A")))
}

// TestDeleteTemplate_CacheDeleteError_DoesNotFail verifies a cache Delete failure is logged, not surfaced.
func (s *CacheBackedStoreTestSuite) TestDeleteTemplate_CacheDeleteError_DoesNotFail() {
	errCache := cachemock.NewCacheInterfaceMock[templateDAO](s.T())
	errCache.EXPECT().Delete(mock.Anything, mock.Anything).Return(errors.New("cache down")).Maybe()
	store := newCacheBackedStore(errCache, s.mockStore)
	s.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, "t1").Return(emailDAO("h1", "A"), nil).Once()
	s.mockStore.On("DeleteTemplate", mock.Anything, ChannelTypeEmail, "t1").Return(nil).Once()

	s.NoError(store.DeleteTemplate(s.ctx, ChannelTypeEmail, "t1"))
}

// TestTemplateDAO_JSONRoundTrip guards the Redis path, which marshals cached values with encoding/json:
// every field must survive a marshal/unmarshal round trip (no unexported or json:"-" fields that blank).
func (s *CacheBackedStoreTestSuite) TestTemplateDAO_JSONRoundTrip() {
	original := templateDAO{
		ID:          "t1",
		Channel:     ChannelTypeEmail,
		Handle:      "otp-verification",
		DisplayName: "OTP",
		Description: "desc",
		Content:     TemplateContent{Subject: "sub", Body: "body"},
		Design:      &TemplateDesign{ColorScheme: colorSchemeDark},
	}

	data, err := json.Marshal(original)
	s.Require().NoError(err)
	var round templateDAO
	s.Require().NoError(json.Unmarshal(data, &round))
	s.Equal(original, round)
}
