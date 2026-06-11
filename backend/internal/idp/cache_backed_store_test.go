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

package idp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/tests/mocks/cachemock"
)

type CacheBackedIDPStoreTestSuite struct {
	suite.Suite
	mockInner     *idpStoreInterfaceMock
	idCache       *cachemock.CacheInterfaceMock[*IDPDTO]
	propertyCache *cachemock.CacheInterfaceMock[[]IDPDTO]
	cachedStore   *cacheBackedIDPStore
}

func TestCacheBackedIDPStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CacheBackedIDPStoreTestSuite))
}

func (s *CacheBackedIDPStoreTestSuite) SetupTest() {
	s.mockInner = newIdpStoreInterfaceMock(s.T())
	s.idCache = cachemock.NewCacheInterfaceMock[*IDPDTO](s.T())
	s.propertyCache = cachemock.NewCacheInterfaceMock[[]IDPDTO](s.T())
	s.cachedStore = &cacheBackedIDPStore{
		idpByIDCache:       s.idCache,
		idpByPropertyCache: s.propertyCache,
		inner:              s.mockInner,
	}
}

func makeIDPWithIssuer(id, name, issuer string) *IDPDTO {
	prop, _ := cmodels.NewProperty(PropIssuer, issuer, false)
	return &IDPDTO{
		ID:         id,
		Name:       name,
		Type:       IDPTypeOIDC,
		Properties: []cmodels.Property{*prop},
	}
}

// TestGetIdentityProvider_CacheHit tests that an ID cache hit is returned without hitting inner store.
func (s *CacheBackedIDPStoreTestSuite) TestGetIdentityProvider_CacheHit() {
	idp := makeIDPWithIssuer("idp-1", "IDP 1", "https://idp.example.com")
	s.idCache.On("Get", mock.Anything, cache.CacheKey{Key: "idp-1"}).Return(idp, true)

	result, err := s.cachedStore.GetIdentityProvider(context.Background(), "idp-1")

	s.NoError(err)
	s.Equal(idp, result)
	s.mockInner.AssertNotCalled(s.T(), "GetIdentityProvider")
}

// TestGetIdentityProvider_CacheMiss tests a cache miss delegates to inner and populates the ID cache.
func (s *CacheBackedIDPStoreTestSuite) TestGetIdentityProvider_CacheMiss() {
	idp := makeIDPWithIssuer("idp-1", "IDP 1", "https://idp.example.com")
	s.idCache.On("Get", mock.Anything, cache.CacheKey{Key: "idp-1"}).Return((*IDPDTO)(nil), false)
	s.mockInner.On("GetIdentityProvider", mock.Anything, "idp-1").Return(idp, nil)
	s.idCache.On("Set", mock.Anything, cache.CacheKey{Key: "idp-1"}, idp).Return(nil)

	result, err := s.cachedStore.GetIdentityProvider(context.Background(), "idp-1")

	s.NoError(err)
	s.Equal(idp, result)
	s.mockInner.AssertExpectations(s.T())
	s.idCache.AssertExpectations(s.T())
}

// TestGetIdentityProvidersByProperty_CacheHit tests that a property cache hit returns without hitting inner store.
func (s *CacheBackedIDPStoreTestSuite) TestGetIdentityProvidersByProperty_CacheHit() {
	idp := makeIDPWithIssuer("idp-1", "IDP 1", "https://idp.example.com")
	cached := []IDPDTO{*idp}
	s.propertyCache.On("Get", mock.Anything, cache.CacheKey{Key: "issuer:https://idp.example.com"}).
		Return(cached, true)

	result, err := s.cachedStore.GetIdentityProvidersByProperty(
		context.Background(), "issuer", "https://idp.example.com")

	s.NoError(err)
	s.Equal(cached, result)
	s.mockInner.AssertNotCalled(s.T(), "GetIdentityProvidersByProperty")
}

// TestGetIdentityProvidersByProperty_AbsenceCacheHit tests a nil-value cache hit returns ErrIDPNotFound.
func (s *CacheBackedIDPStoreTestSuite) TestGetIdentityProvidersByProperty_AbsenceCacheHit() {
	s.propertyCache.On("Get", mock.Anything, cache.CacheKey{Key: "issuer:https://idp.example.com"}).
		Return([]IDPDTO(nil), true)

	result, err := s.cachedStore.GetIdentityProvidersByProperty(
		context.Background(), "issuer", "https://idp.example.com")

	s.Nil(result)
	s.ErrorIs(err, ErrIDPNotFound)
	s.mockInner.AssertNotCalled(s.T(), "GetIdentityProvidersByProperty")
}

// TestGetIdentityProvidersByProperty_CacheMiss tests a cache miss delegates to inner and populates cache.
func (s *CacheBackedIDPStoreTestSuite) TestGetIdentityProvidersByProperty_CacheMiss() {
	idp := makeIDPWithIssuer("idp-1", "IDP 1", "https://idp.example.com")
	idps := []IDPDTO{*idp}
	s.propertyCache.On("Get", mock.Anything, cache.CacheKey{Key: "issuer:https://idp.example.com"}).
		Return([]IDPDTO(nil), false)
	s.mockInner.On("GetIdentityProvidersByProperty", mock.Anything, "issuer", "https://idp.example.com").
		Return(idps, nil)
	s.propertyCache.On("Set", mock.Anything, cache.CacheKey{Key: "issuer:https://idp.example.com"}, idps).
		Return(nil)

	result, err := s.cachedStore.GetIdentityProvidersByProperty(
		context.Background(), "issuer", "https://idp.example.com")

	s.NoError(err)
	s.Equal(idps, result)
	s.mockInner.AssertExpectations(s.T())
}

// TestGetIdentityProvidersByProperty_CachesAbsenceOnNotFound tests that ErrIDPNotFound is cached as nil.
func (s *CacheBackedIDPStoreTestSuite) TestGetIdentityProvidersByProperty_CachesAbsenceOnNotFound() {
	s.propertyCache.On("Get", mock.Anything, cache.CacheKey{Key: "issuer:https://unknown.example.com"}).
		Return([]IDPDTO(nil), false)
	s.mockInner.On("GetIdentityProvidersByProperty", mock.Anything, "issuer", "https://unknown.example.com").
		Return([]IDPDTO(nil), ErrIDPNotFound)
	s.propertyCache.On("Set", mock.Anything, cache.CacheKey{Key: "issuer:https://unknown.example.com"},
		[]IDPDTO(nil)).Return(nil)

	result, err := s.cachedStore.GetIdentityProvidersByProperty(
		context.Background(), "issuer", "https://unknown.example.com")

	s.Nil(result)
	s.ErrorIs(err, ErrIDPNotFound)
	s.propertyCache.AssertExpectations(s.T())
}

// TestCreateIdentityProvider_CachesResult tests create delegates to inner and caches the result.
func (s *CacheBackedIDPStoreTestSuite) TestCreateIdentityProvider_CachesResult() {
	idp := IDPDTO{ID: "idp-1", Name: "IDP 1", Type: IDPTypeOIDC}
	s.mockInner.On("CreateIdentityProvider", mock.Anything, idp).Return(nil)
	s.idCache.On("Set", mock.Anything, cache.CacheKey{Key: "idp-1"}, &idp).Return(nil)

	err := s.cachedStore.CreateIdentityProvider(context.Background(), idp)

	s.NoError(err)
	s.mockInner.AssertExpectations(s.T())
	s.idCache.AssertExpectations(s.T())
}

// TestUpdateIdentityProvider_InvalidatesOldCacheAndCachesNew tests update invalidates old properties and caches new.
func (s *CacheBackedIDPStoreTestSuite) TestUpdateIdentityProvider_InvalidatesOldCacheAndCachesNew() {
	oldIssuer := "https://old.example.com"
	newIssuer := "https://new.example.com"

	oldProp, _ := cmodels.NewProperty(PropIssuer, oldIssuer, false)
	oldIDP := &IDPDTO{ID: "idp-1", Name: "Old IDP", Type: IDPTypeOIDC,
		Properties: []cmodels.Property{*oldProp}}

	newProp, _ := cmodels.NewProperty(PropIssuer, newIssuer, false)
	newIDP := &IDPDTO{ID: "idp-1", Name: "Updated IDP", Type: IDPTypeOIDC,
		Properties: []cmodels.Property{*newProp}}

	s.mockInner.On("GetIdentityProvider", mock.Anything, "idp-1").Return(oldIDP, nil)
	s.mockInner.On("UpdateIdentityProvider", mock.Anything, newIDP).Return(nil)
	s.idCache.On("Delete", mock.Anything, cache.CacheKey{Key: "idp-1"}).Return(nil)
	s.propertyCache.On("Delete", mock.Anything, cache.CacheKey{Key: "issuer:" + oldIssuer}).Return(nil)
	s.idCache.On("Set", mock.Anything, cache.CacheKey{Key: "idp-1"}, newIDP).Return(nil)

	err := s.cachedStore.UpdateIdentityProvider(context.Background(), newIDP)

	s.NoError(err)
	s.mockInner.AssertExpectations(s.T())
	s.idCache.AssertExpectations(s.T())
	s.propertyCache.AssertExpectations(s.T())
}

// TestDeleteIdentityProvider_InvalidatesCacheEntries tests delete fetches old IDP and invalidates cache.
func (s *CacheBackedIDPStoreTestSuite) TestDeleteIdentityProvider_InvalidatesCacheEntries() {
	issuer := "https://idp.example.com"
	idp := makeIDPWithIssuer("idp-1", "IDP 1", issuer)

	s.mockInner.On("GetIdentityProvider", mock.Anything, "idp-1").Return(idp, nil)
	s.mockInner.On("DeleteIdentityProvider", mock.Anything, "idp-1").Return(nil)
	s.idCache.On("Delete", mock.Anything, cache.CacheKey{Key: "idp-1"}).Return(nil)
	s.propertyCache.On("Delete", mock.Anything, cache.CacheKey{Key: "issuer:" + issuer}).Return(nil)

	err := s.cachedStore.DeleteIdentityProvider(context.Background(), "idp-1")

	s.NoError(err)
	s.mockInner.AssertExpectations(s.T())
	s.idCache.AssertExpectations(s.T())
	s.propertyCache.AssertExpectations(s.T())
}

// TestDeleteIdentityProvider_StillDeletesWhenNotFound tests delete succeeds when inner store has no record.
func (s *CacheBackedIDPStoreTestSuite) TestDeleteIdentityProvider_StillDeletesWhenNotFound() {
	s.mockInner.On("GetIdentityProvider", mock.Anything, "idp-1").Return((*IDPDTO)(nil), ErrIDPNotFound)
	s.mockInner.On("DeleteIdentityProvider", mock.Anything, "idp-1").Return(nil)

	err := s.cachedStore.DeleteIdentityProvider(context.Background(), "idp-1")

	s.NoError(err)
	s.mockInner.AssertExpectations(s.T())
}

// TestGetIdentityProviderByName_PopulatesIDCache tests GetByName populates the ID cache.
func (s *CacheBackedIDPStoreTestSuite) TestGetIdentityProviderByName_PopulatesIDCache() {
	idp := makeIDPWithIssuer("idp-2", "IDP 2", "https://other.example.com")
	s.mockInner.On("GetIdentityProviderByName", mock.Anything, "IDP 2").Return(idp, nil)
	s.idCache.On("Set", mock.Anything, cache.CacheKey{Key: "idp-2"}, idp).Return(nil)

	result, err := s.cachedStore.GetIdentityProviderByName(context.Background(), "IDP 2")

	s.NoError(err)
	s.Equal(idp, result)
	s.mockInner.AssertExpectations(s.T())
	s.idCache.AssertExpectations(s.T())
}

// TestGetIdentityProviderList_NoCaching tests GetIdentityProviderList is a pure delegate.
func (s *CacheBackedIDPStoreTestSuite) TestGetIdentityProviderList_NoCaching() {
	list := []BasicIDPDTO{{ID: "idp-1", Name: "IDP 1"}}
	s.mockInner.On("GetIdentityProviderList", mock.Anything).Return(list, nil)

	result, err := s.cachedStore.GetIdentityProviderList(context.Background())

	s.NoError(err)
	s.Equal(list, result)
	s.idCache.AssertNotCalled(s.T(), "Set")
}

// TestGetIdentityProviderListCount_NoCaching tests GetIdentityProviderListCount is a pure delegate.
func (s *CacheBackedIDPStoreTestSuite) TestGetIdentityProviderListCount_NoCaching() {
	s.mockInner.On("GetIdentityProviderListCount", mock.Anything).Return(5, nil)

	count, err := s.cachedStore.GetIdentityProviderListCount(context.Background())

	s.NoError(err)
	s.Equal(5, count)
	s.idCache.AssertNotCalled(s.T(), "Set")
}

// TestGetIdentityProvidersByProperty_InnerStoreError tests that a non-NotFound error is propagated.
func (s *CacheBackedIDPStoreTestSuite) TestGetIdentityProvidersByProperty_InnerStoreError() {
	s.propertyCache.On("Get", mock.Anything, cache.CacheKey{Key: "issuer:https://idp.example.com"}).
		Return([]IDPDTO(nil), false)
	s.mockInner.On("GetIdentityProvidersByProperty", mock.Anything, "issuer", "https://idp.example.com").
		Return([]IDPDTO(nil), errors.New("db error"))

	result, err := s.cachedStore.GetIdentityProvidersByProperty(
		context.Background(), "issuer", "https://idp.example.com")

	s.Nil(result)
	s.Error(err)
	s.NotErrorIs(err, ErrIDPNotFound)
	s.propertyCache.AssertNotCalled(s.T(), "Set")
}

// TestGetIdentityProvidersByProperty_NonCacheableKeyBypassesCache tests that a non-cacheable property
// key bypasses the cache entirely and delegates directly to the inner store.
func (s *CacheBackedIDPStoreTestSuite) TestGetIdentityProvidersByProperty_NonCacheableKeyBypassesCache() {
	idp := makeIDPWithIssuer("idp-1", "IDP 1", "https://idp.example.com")
	s.mockInner.On("GetIdentityProvidersByProperty", mock.Anything, "client_id", "some-client").
		Return([]IDPDTO{*idp}, nil)

	result, err := s.cachedStore.GetIdentityProvidersByProperty(context.Background(), "client_id", "some-client")

	s.NoError(err)
	s.Len(result, 1)
	s.mockInner.AssertCalled(s.T(), "GetIdentityProvidersByProperty", mock.Anything, "client_id", "some-client")
	s.propertyCache.AssertNotCalled(s.T(), "Get", mock.Anything, mock.Anything)
	s.propertyCache.AssertNotCalled(s.T(), "Set", mock.Anything, mock.Anything, mock.Anything)
}

// --- cacheIDP edge cases ---

func (s *CacheBackedIDPStoreTestSuite) TestCacheIDP_NilIDP() {
	s.cachedStore.cacheIDP(context.Background(), nil)
	s.idCache.AssertNotCalled(s.T(), "Set")
}

func (s *CacheBackedIDPStoreTestSuite) TestCacheIDP_EmptyID() {
	idp := &IDPDTO{ID: "", Name: "No ID", Type: IDPTypeOIDC}
	s.cachedStore.cacheIDP(context.Background(), idp)
	s.idCache.AssertNotCalled(s.T(), "Set")
}

// --- invalidateIDP edge cases ---

func (s *CacheBackedIDPStoreTestSuite) TestInvalidateIDP_NilIDP() {
	s.cachedStore.invalidateIDP(context.Background(), nil)
	s.idCache.AssertNotCalled(s.T(), "Delete")
	s.propertyCache.AssertNotCalled(s.T(), "Delete")
}

func (s *CacheBackedIDPStoreTestSuite) TestInvalidateIDP_PropertyWithEmptyValue() {
	prop, _ := cmodels.NewProperty("empty_prop", "", false)
	idp := &IDPDTO{
		ID:         "idp-empty-prop",
		Name:       "IDP Empty Prop",
		Type:       IDPTypeOIDC,
		Properties: []cmodels.Property{*prop},
	}

	s.idCache.On("Delete", mock.Anything, cache.CacheKey{Key: "idp-empty-prop"}).Return(nil)

	s.cachedStore.invalidateIDP(context.Background(), idp)

	s.idCache.AssertCalled(s.T(), "Delete", mock.Anything, cache.CacheKey{Key: "idp-empty-prop"})
	s.propertyCache.AssertNotCalled(s.T(), "Delete")
}

// --- CreateIdentityProvider error branch ---

func (s *CacheBackedIDPStoreTestSuite) TestCreateIdentityProvider_InnerStoreError() {
	idp := IDPDTO{ID: "idp-fail", Name: "Fail IDP", Type: IDPTypeOIDC}
	s.mockInner.On("CreateIdentityProvider", mock.Anything, idp).Return(errors.New("create failed"))

	err := s.cachedStore.CreateIdentityProvider(context.Background(), idp)

	s.Error(err)
	s.idCache.AssertNotCalled(s.T(), "Set")
}

// --- UpdateIdentityProvider error branches ---

func (s *CacheBackedIDPStoreTestSuite) TestUpdateIdentityProvider_InnerGetError() {
	idp := &IDPDTO{ID: "idp-1", Name: "Updated", Type: IDPTypeOIDC}
	s.mockInner.On("GetIdentityProvider", mock.Anything, "idp-1").
		Return((*IDPDTO)(nil), errors.New("get failed"))

	err := s.cachedStore.UpdateIdentityProvider(context.Background(), idp)

	s.Error(err)
	s.mockInner.AssertNotCalled(s.T(), "UpdateIdentityProvider")
	s.idCache.AssertNotCalled(s.T(), "Set")
	s.idCache.AssertNotCalled(s.T(), "Delete")
}

func (s *CacheBackedIDPStoreTestSuite) TestUpdateIdentityProvider_InnerUpdateError() {
	oldProp, _ := cmodels.NewProperty(PropIssuer, "https://old.example.com", false)
	oldIDP := &IDPDTO{ID: "idp-1", Name: "Old", Type: IDPTypeOIDC,
		Properties: []cmodels.Property{*oldProp}}

	newIDP := &IDPDTO{ID: "idp-1", Name: "New", Type: IDPTypeOIDC}

	s.mockInner.On("GetIdentityProvider", mock.Anything, "idp-1").Return(oldIDP, nil)
	s.mockInner.On("UpdateIdentityProvider", mock.Anything, newIDP).Return(errors.New("update failed"))

	err := s.cachedStore.UpdateIdentityProvider(context.Background(), newIDP)

	s.Error(err)
	s.idCache.AssertNotCalled(s.T(), "Delete")
	s.idCache.AssertNotCalled(s.T(), "Set")
}

// --- DeleteIdentityProvider error branches ---

func (s *CacheBackedIDPStoreTestSuite) TestDeleteIdentityProvider_InnerGetGenericError() {
	s.mockInner.On("GetIdentityProvider", mock.Anything, "idp-1").
		Return((*IDPDTO)(nil), errors.New("connection refused"))

	err := s.cachedStore.DeleteIdentityProvider(context.Background(), "idp-1")

	s.Error(err)
	s.mockInner.AssertNotCalled(s.T(), "DeleteIdentityProvider")
}

func (s *CacheBackedIDPStoreTestSuite) TestDeleteIdentityProvider_InnerDeleteError() {
	idp := makeIDPWithIssuer("idp-1", "IDP 1", "https://idp.example.com")
	s.mockInner.On("GetIdentityProvider", mock.Anything, "idp-1").Return(idp, nil)
	s.mockInner.On("DeleteIdentityProvider", mock.Anything, "idp-1").Return(errors.New("delete failed"))

	err := s.cachedStore.DeleteIdentityProvider(context.Background(), "idp-1")

	s.Error(err)
	s.idCache.AssertNotCalled(s.T(), "Delete")
	s.propertyCache.AssertNotCalled(s.T(), "Delete")
}

func (s *CacheBackedIDPStoreTestSuite) TestCacheIDP_SetError() {
	idp := &IDPDTO{ID: "idp-set-err", Name: "Set Err IDP", Type: IDPTypeOIDC}

	s.idCache.On("Set", mock.Anything, cache.CacheKey{Key: "idp-set-err"}, idp).
		Return(errors.New("cache set failed"))

	// The error is logged and swallowed; caching is best-effort.
	s.cachedStore.cacheIDP(context.Background(), idp)

	s.idCache.AssertCalled(s.T(), "Set", mock.Anything, cache.CacheKey{Key: "idp-set-err"}, idp)
}

func (s *CacheBackedIDPStoreTestSuite) TestInvalidateIDP_DeleteErrors() {
	idp := makeIDPWithIssuer("idp-del-err", "Delete Err IDP", "https://idp.example.com")

	s.idCache.On("Delete", mock.Anything, cache.CacheKey{Key: "idp-del-err"}).
		Return(errors.New("cache delete failed"))
	s.propertyCache.On("Delete", mock.Anything, cache.CacheKey{Key: PropIssuer + ":https://idp.example.com"}).
		Return(errors.New("cache delete failed"))

	// Errors are logged and swallowed; both invalidations are still attempted.
	s.cachedStore.invalidateIDP(context.Background(), idp)

	s.idCache.AssertCalled(s.T(), "Delete", mock.Anything, cache.CacheKey{Key: "idp-del-err"})
	s.propertyCache.AssertCalled(s.T(), "Delete", mock.Anything,
		cache.CacheKey{Key: PropIssuer + ":https://idp.example.com"})
}
