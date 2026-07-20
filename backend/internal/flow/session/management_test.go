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

package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestManagementListBySubjectReturnsPageWithTotal(t *testing.T) {
	storeMock := newSessionStoreMock(t)
	now := time.Now().UTC()
	storeMock.EXPECT().ListBySubject(mock.Anything, "sub-1", now, 5, 0).
		Return([]Session{{SessionID: "s1"}}, nil)
	storeMock.EXPECT().CountBySubject(mock.Anything, "sub-1", now).Return(7, nil)

	svc := newManagementService(storeMock)
	page, err := svc.ListBySubject(context.Background(), "sub-1", 5, 0, now)

	require.NoError(t, err)
	require.Len(t, page.Sessions, 1)
	require.Equal(t, 7, page.TotalResults)
}

func TestManagementListBySubjectListError(t *testing.T) {
	storeMock := newSessionStoreMock(t)
	now := time.Now().UTC()
	storeMock.EXPECT().ListBySubject(mock.Anything, "sub-1", now, 5, 0).
		Return(nil, errors.New("store down"))

	svc := newManagementService(storeMock)
	page, err := svc.ListBySubject(context.Background(), "sub-1", 5, 0, now)

	require.Error(t, err)
	require.Nil(t, page)
}

func TestManagementListBySubjectCountError(t *testing.T) {
	storeMock := newSessionStoreMock(t)
	now := time.Now().UTC()
	storeMock.EXPECT().ListBySubject(mock.Anything, "sub-1", now, 5, 0).
		Return([]Session{{SessionID: "s1"}}, nil)
	storeMock.EXPECT().CountBySubject(mock.Anything, "sub-1", now).Return(0, errors.New("store down"))

	svc := newManagementService(storeMock)
	page, err := svc.ListBySubject(context.Background(), "sub-1", 5, 0, now)

	require.Error(t, err)
	require.Nil(t, page)
}

func TestManagementListByAppReturnsPageWithTotal(t *testing.T) {
	storeMock := newSessionStoreMock(t)
	now := time.Now().UTC()
	storeMock.EXPECT().ListByApp(mock.Anything, "app-1", now, 5, 0).
		Return([]Session{{SessionID: "s1"}, {SessionID: "s2"}}, nil)
	storeMock.EXPECT().CountByApp(mock.Anything, "app-1", now).Return(2, nil)

	svc := newManagementService(storeMock)
	page, err := svc.ListByApp(context.Background(), "app-1", 5, 0, now)

	require.NoError(t, err)
	require.Len(t, page.Sessions, 2)
	require.Equal(t, 2, page.TotalResults)
}

func TestManagementListByAppListError(t *testing.T) {
	storeMock := newSessionStoreMock(t)
	now := time.Now().UTC()
	storeMock.EXPECT().ListByApp(mock.Anything, "app-1", now, 5, 0).
		Return(nil, errors.New("store down"))

	svc := newManagementService(storeMock)
	page, err := svc.ListByApp(context.Background(), "app-1", 5, 0, now)

	require.Error(t, err)
	require.Nil(t, page)
}

func TestManagementListByAppCountError(t *testing.T) {
	storeMock := newSessionStoreMock(t)
	now := time.Now().UTC()
	storeMock.EXPECT().ListByApp(mock.Anything, "app-1", now, 5, 0).
		Return([]Session{{SessionID: "s1"}}, nil)
	storeMock.EXPECT().CountByApp(mock.Anything, "app-1", now).Return(0, errors.New("store down"))

	svc := newManagementService(storeMock)
	page, err := svc.ListByApp(context.Background(), "app-1", 5, 0, now)

	require.Error(t, err)
	require.Nil(t, page)
}

func TestManagementListParticipantsDelegates(t *testing.T) {
	storeMock := newSessionStoreMock(t)
	storeMock.EXPECT().ListBySessionID(mock.Anything, "s1").
		Return([]Participant{{SessionID: "s1", AppID: "app-1"}}, nil)

	svc := newManagementService(storeMock)
	parts, err := svc.ListParticipants(context.Background(), "s1")

	require.NoError(t, err)
	require.Len(t, parts, 1)
}

func TestManagementListParticipantsError(t *testing.T) {
	storeMock := newSessionStoreMock(t)
	storeMock.EXPECT().ListBySessionID(mock.Anything, "s1").
		Return(nil, errors.New("store down"))

	svc := newManagementService(storeMock)
	parts, err := svc.ListParticipants(context.Background(), "s1")

	require.Error(t, err)
	require.Nil(t, parts)
}
