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

package revocation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRevokeTokenFamily_WritesTokenFamilyCriterion(t *testing.T) {
	store := newRevocationStoreInterfaceMock(t)
	var captured revocationCriterion
	store.On("insertCriterion", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(revocationCriterion)
		}).
		Return(nil)

	revoker := newCriteriaRevoker(store, time.Hour)
	err := revoker.RevokeTokenFamily(context.Background(), "tfid-abc", RevocationReasonSessionLogout)

	assert.NoError(t, err)
	assert.Equal(t, criterionTypeTokenFamily, captured.Type)
	assert.Equal(t, "tfid-abc", captured.Value)
	assert.Equal(t, RevocationReasonSessionLogout, captured.Reason)
	assert.WithinDuration(t, captured.RevokedAt.Add(time.Hour), captured.ExpiryTime, time.Second)
}

func TestRevokeTokenFamily_EmptyIDIsNoOp(t *testing.T) {
	store := newRevocationStoreInterfaceMock(t)
	// No insertCriterion expectation: an empty tfid must not write.
	revoker := newCriteriaRevoker(store, time.Hour)

	err := revoker.RevokeTokenFamily(context.Background(), "", RevocationReasonSessionLogout)
	assert.NoError(t, err)
	store.AssertNotCalled(t, "insertCriterion", mock.Anything, mock.Anything)
}

func TestRevokeTokenFamily_PropagatesStoreError(t *testing.T) {
	store := newRevocationStoreInterfaceMock(t)
	store.On("insertCriterion", mock.Anything, mock.Anything).Return(errors.New("db down"))

	revoker := newCriteriaRevoker(store, time.Hour)
	err := revoker.RevokeTokenFamily(context.Background(), "tfid-abc", RevocationReasonRefreshReuse)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

func TestNewCriteriaRevoker_NonPositiveTTLFallsBack(t *testing.T) {
	store := newRevocationStoreInterfaceMock(t)
	var captured revocationCriterion
	store.On("insertCriterion", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(revocationCriterion)
		}).
		Return(nil)

	revoker := newCriteriaRevoker(store, 0)
	err := revoker.RevokeTokenFamily(context.Background(), "tfid-abc", RevocationReasonCodeReplay)
	assert.NoError(t, err)
	assert.WithinDuration(t, captured.RevokedAt.Add(defaultTokenFamilyRevocationTTL), captured.ExpiryTime, time.Second)
}
