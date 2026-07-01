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

package definition

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment-id"

type DefinitionStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *definitionStore
}

func TestDefinitionStoreTestSuite(t *testing.T) {
	suite.Run(t, new(DefinitionStoreTestSuite))
}

func (suite *DefinitionStoreTestSuite) SetupTest() {
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.store = &definitionStore{
		dbProvider:   suite.mockDBProvider,
		deploymentID: testDeploymentID,
	}
}

func (suite *DefinitionStoreTestSuite) refreshMocks() {
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.store = &definitionStore{
		dbProvider:   suite.mockDBProvider,
		deploymentID: testDeploymentID,
	}
}

// rowFromDTO simulates a persisted row by marshaling a DTO the way the store
// does, returning the column map buildDefinitionDTOFromRow consumes.
func rowFromDTO(t *testing.T, dto PresentationDefinitionDTO) map[string]interface{} {
	t.Helper()
	claimsJSON, err := marshalClaims(dto)
	require.NoError(t, err)
	authorities, err := marshalTrustedAuthorities(dto.TrustedAuthorities)
	require.NoError(t, err)
	row := map[string]interface{}{
		"id":           dto.ID,
		"handle":       dto.Handle,
		"display_name": dto.DisplayName,
		"vct":          dto.VCT,
		"format":       dto.Format,
		"claims":       claimsJSON,
	}
	if dto.EnforceTrustedIssuer != nil {
		row["enforce_trusted_issuer"] = *dto.EnforceTrustedIssuer
	}
	if authorities != nil {
		row["trusted_authorities"] = authorities
	}
	return row
}

func (suite *DefinitionStoreTestSuite) TestDefinitionStoreRoundTripTrustFields() {
	enforce := true
	original := PresentationDefinitionDTO{
		ID:                   "id-1",
		Handle:               "eudi-pid",
		VCT:                  "urn:eudi:pid:de:1",
		Format:               DefaultCredentialFormat,
		MandatoryClaims:      []string{"given_name"},
		EnforceTrustedIssuer: &enforce,
		TrustedAuthorities:   []string{"root-a", "root-b"},
	}

	got, err := buildDefinitionDTOFromRow(rowFromDTO(suite.T(), original))
	suite.Require().NoError(err)
	suite.Require().NotNil(got.EnforceTrustedIssuer)
	suite.True(*got.EnforceTrustedIssuer)
	suite.Equal([]string{"root-a", "root-b"}, got.TrustedAuthorities)
}

func (suite *DefinitionStoreTestSuite) TestDefinitionStoreRoundTripTrustFieldsNull() {
	// Omitted (nil) enforce + empty authorities persist as NULL and reload as
	// nil/empty, preserving the "inherit / accept any" semantics.
	got, err := buildDefinitionDTOFromRow(rowFromDTO(suite.T(), PresentationDefinitionDTO{
		ID:     "id-2",
		Handle: "h",
		VCT:    "v",
	}))
	suite.Require().NoError(err)
	suite.Nil(got.EnforceTrustedIssuer)
	suite.Empty(got.TrustedAuthorities)
}

func (suite *DefinitionStoreTestSuite) TestColumnNullableBoolFromInt() {
	// SQLite returns INTEGER for BOOLEAN columns.
	got := columnNullableBool(int64(1))
	suite.Require().NotNil(got)
	suite.True(*got)

	got = columnNullableBool(int64(0))
	suite.Require().NotNil(got)
	suite.False(*got)

	suite.Nil(columnNullableBool(nil))
}

func (suite *DefinitionStoreTestSuite) TestCreate() {
	testCases := []struct {
		name       string
		dto        PresentationDefinitionDTO
		setupMocks func()
		shouldErr  bool
	}{
		{
			name: "Success",
			dto: PresentationDefinitionDTO{
				ID: "def-1", Handle: "eudi-pid", OUID: "ou-1",
				DisplayName: "EUDI PID", VCT: "urn:eudi:pid:de:1", Format: "dc+sd-jwt",
				MandatoryClaims: []string{"given_name"},
			},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", mock.Anything, queryCreateDefinition,
					"def-1", "eudi-pid", "ou-1", "EUDI PID", "urn:eudi:pid:de:1", "dc+sd-jwt",
					mock.Anything, nil, nil, testDeploymentID,
				).Return(int64(1), nil)
			},
		},
		{
			name: "ExecuteError",
			dto:  PresentationDefinitionDTO{ID: "def-1", Handle: "h", VCT: "v"},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", mock.Anything, queryCreateDefinition,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
				).Return(int64(0), errors.New("insert failed"))
			},
			shouldErr: true,
		},
		{
			name: "DBClientError",
			dto:  PresentationDefinitionDTO{ID: "def-1", Handle: "h", VCT: "v"},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("db error"))
			},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.refreshMocks()
			tc.setupMocks()
			err := suite.store.CreatePresentationDefinition(context.Background(), tc.dto)
			if tc.shouldErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *DefinitionStoreTestSuite) TestGetByID() {
	testCases := []struct {
		name        string
		id          string
		setupMocks  func()
		checkResult func(*PresentationDefinitionDTO)
		shouldErr   bool
		errIs       error
	}{
		{
			name: "Success",
			id:   "def-1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				enforce := true
				suite.mockDBClient.On("QueryContext", mock.Anything, queryGetDefinitionByID,
					"def-1", testDeploymentID,
				).Return([]map[string]interface{}{
					rowFromDTO(suite.T(), PresentationDefinitionDTO{
						ID: "def-1", Handle: "eudi-pid", OUID: "ou-1",
						VCT: "urn:eudi:pid:de:1", Format: "dc+sd-jwt",
						MandatoryClaims:      []string{"given_name"},
						OptionalClaims:       []string{"birthdate"},
						EnforceTrustedIssuer: &enforce,
						TrustedAuthorities:   []string{"root-ca"},
					}),
				}, nil)
			},
			checkResult: func(got *PresentationDefinitionDTO) {
				suite.Equal("def-1", got.ID)
				suite.Equal("eudi-pid", got.Handle)
				suite.Equal([]string{"given_name"}, got.MandatoryClaims)
				suite.Equal([]string{"birthdate"}, got.OptionalClaims)
				suite.Require().NotNil(got.EnforceTrustedIssuer)
				suite.True(*got.EnforceTrustedIssuer)
				suite.Equal([]string{"root-ca"}, got.TrustedAuthorities)
			},
		},
		{
			name: "NotFound",
			id:   "missing",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryGetDefinitionByID,
					"missing", testDeploymentID,
				).Return([]map[string]interface{}{}, nil)
			},
			shouldErr: true,
			errIs:     ErrNotFound,
		},
		{
			name: "QueryError",
			id:   "def-1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryGetDefinitionByID,
					"def-1", testDeploymentID,
				).Return(nil, errors.New("query failed"))
			},
			shouldErr: true,
		},
		{
			name: "DBClientError",
			id:   "def-1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("db error"))
			},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.refreshMocks()
			tc.setupMocks()
			got, err := suite.store.GetPresentationDefinitionByID(context.Background(), tc.id)
			if tc.shouldErr {
				suite.Error(err)
				suite.Nil(got)
				if tc.errIs != nil {
					suite.ErrorIs(err, tc.errIs)
				}
			} else {
				suite.NoError(err)
				suite.Require().NotNil(got)
				if tc.checkResult != nil {
					tc.checkResult(got)
				}
			}
		})
	}
}

func (suite *DefinitionStoreTestSuite) TestGetByHandle() {
	testCases := []struct {
		name       string
		handle     string
		setupMocks func()
		shouldErr  bool
		errIs      error
	}{
		{
			name:   "Success",
			handle: "eudi-pid",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryGetDefinitionByHandle,
					"eudi-pid", testDeploymentID,
				).Return([]map[string]interface{}{
					rowFromDTO(suite.T(), PresentationDefinitionDTO{
						ID: "def-1", Handle: "eudi-pid", VCT: "urn:eudi:pid:de:1", Format: "dc+sd-jwt",
					}),
				}, nil)
			},
		},
		{
			name:   "NotFound",
			handle: "missing",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryGetDefinitionByHandle,
					"missing", testDeploymentID,
				).Return([]map[string]interface{}{}, nil)
			},
			shouldErr: true,
			errIs:     ErrNotFound,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.refreshMocks()
			tc.setupMocks()
			got, err := suite.store.GetPresentationDefinitionByHandle(context.Background(), tc.handle)
			if tc.shouldErr {
				suite.Error(err)
				suite.Nil(got)
				if tc.errIs != nil {
					suite.ErrorIs(err, tc.errIs)
				}
			} else {
				suite.NoError(err)
				suite.NotNil(got)
			}
		})
	}
}

func (suite *DefinitionStoreTestSuite) TestList() {
	testCases := []struct {
		name          string
		setupMocks    func()
		expectedCount int
		shouldErr     bool
	}{
		{
			name: "ReturnsAllRows",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryListDefinitions, testDeploymentID).
					Return([]map[string]interface{}{
						rowFromDTO(suite.T(), PresentationDefinitionDTO{
							ID: "def-1", Handle: "h1", VCT: "v1", Format: "dc+sd-jwt",
						}),
						rowFromDTO(suite.T(), PresentationDefinitionDTO{
							ID: "def-2", Handle: "h2", VCT: "v2", Format: "dc+sd-jwt",
						}),
					}, nil)
			},
			expectedCount: 2,
		},
		{
			name: "Empty",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryListDefinitions, testDeploymentID).
					Return([]map[string]interface{}{}, nil)
			},
			expectedCount: 0,
		},
		{
			name: "QueryError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryListDefinitions, testDeploymentID).
					Return(nil, errors.New("query failed"))
			},
			shouldErr: true,
		},
		{
			name: "MalformedRow",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryListDefinitions, testDeploymentID).
					Return([]map[string]interface{}{
						{"id": "def-1", "handle": "h", "vct": "v", "claims": "{not valid json"},
					}, nil)
			},
			shouldErr: true,
		},
		{
			name: "DBClientError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("db error"))
			},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.refreshMocks()
			tc.setupMocks()
			defs, err := suite.store.ListPresentationDefinitions(context.Background())
			if tc.shouldErr {
				suite.Error(err)
				suite.Nil(defs)
			} else {
				suite.NoError(err)
				suite.Len(defs, tc.expectedCount)
			}
		})
	}
}

func (suite *DefinitionStoreTestSuite) TestListSummaries() {
	testCases := []struct {
		name          string
		setupMocks    func()
		expectedCount int
		shouldErr     bool
	}{
		{
			name: "ReturnsAllRows",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryListDefinitionSummaries, testDeploymentID).
					Return([]map[string]interface{}{
						{
							"id": "def-1", "handle": "h1", "ou_id": "ou-1",
							"display_name": "D1", "vct": "v1", "format": "dc+sd-jwt",
						},
						{
							"id": "def-2", "handle": "h2", "ou_id": "ou-2",
							"display_name": "D2", "vct": "v2", "format": "dc+sd-jwt",
						},
					}, nil)
			},
			expectedCount: 2,
		},
		{
			name: "QueryError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("QueryContext", mock.Anything, queryListDefinitionSummaries, testDeploymentID).
					Return(nil, errors.New("query failed"))
			},
			shouldErr: true,
		},
		{
			name: "DBClientError",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("db error"))
			},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.refreshMocks()
			tc.setupMocks()
			summaries, err := suite.store.ListPresentationDefinitionSummaries(context.Background())
			if tc.shouldErr {
				suite.Error(err)
				suite.Nil(summaries)
			} else {
				suite.NoError(err)
				suite.Len(summaries, tc.expectedCount)
				suite.Equal("def-1", summaries[0].ID)
				suite.Equal("ou-1", summaries[0].OUID)
			}
		})
	}
}

func (suite *DefinitionStoreTestSuite) TestUpdate() {
	testCases := []struct {
		name       string
		dto        PresentationDefinitionDTO
		setupMocks func()
		shouldErr  bool
	}{
		{
			name: "Success",
			dto: PresentationDefinitionDTO{
				ID: "def-1", Handle: "new-handle", OUID: "ou-1",
				VCT: "urn:eudi:pid:de:1", Format: "dc+sd-jwt",
			},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateDefinition,
					"def-1", "new-handle", "ou-1", "", "urn:eudi:pid:de:1", "dc+sd-jwt",
					mock.Anything, nil, nil, testDeploymentID,
				).Return(int64(1), nil)
			},
		},
		{
			name: "ExecuteError",
			dto:  PresentationDefinitionDTO{ID: "def-1", Handle: "h", VCT: "v"},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateDefinition,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
				).Return(int64(0), errors.New("update failed"))
			},
			shouldErr: true,
		},
		{
			name: "DBClientError",
			dto:  PresentationDefinitionDTO{ID: "def-1", Handle: "h", VCT: "v"},
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("db error"))
			},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.refreshMocks()
			tc.setupMocks()
			err := suite.store.UpdatePresentationDefinition(context.Background(), tc.dto)
			if tc.shouldErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *DefinitionStoreTestSuite) TestDelete() {
	testCases := []struct {
		name       string
		id         string
		setupMocks func()
		shouldErr  bool
	}{
		{
			name: "Success",
			id:   "def-1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteDefinition,
					"def-1", testDeploymentID,
				).Return(int64(1), nil)
			},
		},
		{
			name: "ExecuteError",
			id:   "def-1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
				suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteDefinition,
					"def-1", testDeploymentID,
				).Return(int64(0), errors.New("delete failed"))
			},
			shouldErr: true,
		},
		{
			name: "DBClientError",
			id:   "def-1",
			setupMocks: func() {
				suite.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("db error"))
			},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.refreshMocks()
			tc.setupMocks()
			err := suite.store.DeletePresentationDefinition(context.Background(), tc.id)
			if tc.shouldErr {
				suite.Error(err)
			} else {
				suite.NoError(err)
			}
		})
	}
}

func (suite *DefinitionStoreTestSuite) TestIsDeclarative() {
	isDeclarative, err := suite.store.IsPresentationDefinitionDeclarative(context.Background(), "any-id")
	suite.NoError(err)
	suite.False(isDeclarative)
}

func (suite *DefinitionStoreTestSuite) TestNullableBool() {
	suite.Nil(nullableBool(nil))

	enforce := true
	suite.Equal(true, nullableBool(&enforce))

	disable := false
	suite.Equal(false, nullableBool(&disable))
}

func (suite *DefinitionStoreTestSuite) TestColumnNullableBoolFromBool() {
	got := columnNullableBool(true)
	suite.Require().NotNil(got)
	suite.True(*got)

	// An unsupported type yields a nil pointer (inherit the engine default).
	suite.Nil(columnNullableBool("not-a-bool"))
}

func (suite *DefinitionStoreTestSuite) TestColumnStringFromBytesAndDefault() {
	suite.Equal("from-bytes", columnString([]byte("from-bytes")))
	suite.Equal("from-string", columnString("from-string"))
	// An unsupported type yields the empty string.
	suite.Empty(columnString(42))
}

func (suite *DefinitionStoreTestSuite) TestColumnBytesFromStringAndDefault() {
	suite.Equal([]byte("from-string"), columnBytes("from-string"))
	suite.Equal([]byte("from-bytes"), columnBytes([]byte("from-bytes")))
	// An unsupported type yields nil.
	suite.Nil(columnBytes(42))
}

func (suite *DefinitionStoreTestSuite) TestBuildDefinitionDTOFromRowMalformedClaims() {
	row := map[string]interface{}{
		"id":     "def-1",
		"handle": "h",
		"vct":    "v",
		"claims": "{not valid json",
	}
	got, err := buildDefinitionDTOFromRow(row)
	suite.Error(err)
	suite.Nil(got)
}

func (suite *DefinitionStoreTestSuite) TestBuildDefinitionDTOFromRowMalformedTrustedAuthorities() {
	row := map[string]interface{}{
		"id":                  "def-1",
		"handle":              "h",
		"vct":                 "v",
		"trusted_authorities": "{not valid json",
	}
	got, err := buildDefinitionDTOFromRow(row)
	suite.Error(err)
	suite.Nil(got)
}
