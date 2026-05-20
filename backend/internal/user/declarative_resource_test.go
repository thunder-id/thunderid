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

package user

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	entitypkg "github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/cryptolab/hash"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
)

// DeclarativeResourceTestSuite tests user declarative resource parsing and export.
type DeclarativeResourceTestSuite struct {
	suite.Suite
}

// TestDeclarativeResourceTestSuite runs the test suite.
func TestDeclarativeResourceTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeResourceTestSuite))
}

// SetupTest initializes runtime config required for hashing.
func (suite *DeclarativeResourceTestSuite) SetupTest() {
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("test", &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(hash.SHA256),
				SHA256: config.SHA256Config{
					SaltSize: 16,
				},
			},
		},
	})
	suite.Require().NoError(err)
}

func (suite *DeclarativeResourceTestSuite) TestParseCredentials_SimpleFormatHashes() {
	credentials, err := parseCredentials(map[string]interface{}{
		"password": "secret",
	})

	suite.NoError(err)
	suite.Contains(credentials, CredentialType("password"))
	suite.Len(credentials["password"], 1)
	suite.Equal("hash", credentials["password"][0].StorageType)
	suite.NotEqual("secret", credentials["password"][0].Value)
	suite.NotEmpty(credentials["password"][0].StorageAlgo)
}

func (suite *DeclarativeResourceTestSuite) TestParseCredentials_SystemManagedPreserves() {
	credentials, err := parseCredentials(map[string]interface{}{
		string(CredentialTypePasskey): "raw-value",
	})

	suite.NoError(err)
	suite.Contains(credentials, CredentialTypePasskey)
	suite.Len(credentials[CredentialTypePasskey], 1)
	suite.Equal("raw-value", credentials[CredentialTypePasskey][0].Value)
}

func (suite *DeclarativeResourceTestSuite) TestParseCredentials_FullFormatPreserves() {
	credentials, err := parseCredentials(map[string]interface{}{
		"password": []interface{}{
			map[string]interface{}{
				"storageType": "hash",
				"storageAlgo": "argon2",
				"storageAlgoParams": map[string]interface{}{
					"iterations": 1,
					"keySize":    32,
					"salt":       "salt",
				},
				"value": "hashed-value",
			},
		},
	})

	suite.NoError(err)
	suite.Len(credentials["password"], 1)
	suite.Equal("hash", credentials["password"][0].StorageType)
	suite.Equal("hashed-value", credentials["password"][0].Value)
}

func (suite *DeclarativeResourceTestSuite) TestParseCredentials_InvalidFormat() {
	_, err := parseCredentials(map[string]interface{}{
		"password": 123,
	})
	suite.Error(err)
}

func (suite *DeclarativeResourceTestSuite) TestParseCredentialObject_HashesWhenNoStorageType() {
	hashService, err := hash.Initialize(
		hash.HashConfig{Algorithm: hash.PBKDF2, SaltSize: 16, Iterations: 1, KeySize: 32},
	)
	suite.Require().NoError(err)
	cred, err := parseCredentialObject(map[string]interface{}{
		"value": "secret",
	}, hashService, CredentialType("password"))

	suite.NoError(err)
	suite.Equal("hash", cred.StorageType)
	suite.NotEqual("secret", cred.Value)
}

func (suite *DeclarativeResourceTestSuite) TestParseCredentialObject_SystemManagedMarker() {
	hashService, err := hash.Initialize(
		hash.HashConfig{Algorithm: hash.PBKDF2, SaltSize: 16, Iterations: 1, KeySize: 32},
	)
	suite.Require().NoError(err)
	cred, err := parseCredentialObject(map[string]interface{}{
		"value":             "raw",
		"systemManaged":     true,
		"storageType":       "system",
		"storageAlgo":       "",
		"storageAlgoParams": map[string]interface{}{},
	}, hashService, CredentialTypePasskey)

	suite.NoError(err)
	suite.Equal("system", cred.StorageType)
	suite.Equal("raw", cred.Value)
}

func (suite *DeclarativeResourceTestSuite) TestParseToUser_HashesCredentials() {
	yamlData := []byte("" +
		"id: user-1\n" +
		"type: person\n" +
		"ou_id: ou-1\n" +
		"attributes:\n" +
		"  username: alice\n" +
		"  email: alice@example.com\n" +
		"credentials:\n" +
		"  password: \"secret\"\n")

	_, creds, err := parseToUser(yamlData)
	suite.NoError(err)

	passwordCreds := creds["password"]
	suite.Len(passwordCreds, 1)
	suite.NotEqual("secret", passwordCreds[0].Value)
}

func (suite *DeclarativeResourceTestSuite) TestParseToUserWrapper() {
	yamlData := []byte("" +
		"id: user-1\n" +
		"type: person\n" +
		"ou_id: ou-1\n" +
		"attributes:\n" +
		"  username: alice\n" +
		"  email: alice@example.com\n")

	user, _, err := parseToUser(yamlData)
	suite.NoError(err)
	suite.NotEmpty(user.ID)
}

func (suite *DeclarativeResourceTestSuite) TestUserExporter_GetResourceByID() {
	mockSvc := NewUserServiceInterfaceMock(suite.T())
	exporter := newUserExporter(mockSvc, entitymock.NewEntityServiceInterfaceMock(suite.T()))

	attrs := json.RawMessage(`{"username":"alice"}`)
	mockSvc.On("GetUser", context.Background(), "user-1", false).
		Return(&User{ID: "user-1", Type: "person", OUID: "ou-1", Attributes: attrs}, nil)

	resource, name, err := exporter.GetResourceByID(context.Background(), "user-1")
	suite.Nil(err)
	suite.Equal("alice", name)

	userResource, ok := resource.(*userDeclarativeResource)
	suite.True(ok)
	suite.Empty(userResource.Credentials)
}

func (suite *DeclarativeResourceTestSuite) TestUserExporter_Metadata() {
	exporter := newUserExporter(
		NewUserServiceInterfaceMock(suite.T()), entitymock.NewEntityServiceInterfaceMock(suite.T()))

	suite.Equal(resourceTypeUser, exporter.GetResourceType())
	suite.Equal(paramTypeUser, exporter.GetParameterizerType())
}

func (suite *DeclarativeResourceTestSuite) TestUserExporter_GetAllResourceIDs() {
	ctx := context.Background()
	mockSvc := NewUserServiceInterfaceMock(suite.T())
	entityServiceMock := entitymock.NewEntityServiceInterfaceMock(suite.T())
	exporter := newUserExporter(mockSvc, entityServiceMock)

	users := []User{{ID: "user-1"}, {ID: "user-2"}}
	mockSvc.On("GetUserList", ctx, serverconst.MaxPageSize, 0, mock.Anything, false).
		Return(&UserListResponse{Users: users}, nil)
	entityServiceMock.On("IsEntityDeclarative", ctx, "user-1").Return(true, nil)
	entityServiceMock.On("IsEntityDeclarative", ctx, "user-2").Return(false, nil)
	mockSvc.On("GetUserList", ctx, serverconst.MaxPageSize, 2, mock.Anything, false).
		Return(&UserListResponse{Users: []User{}}, nil)

	ids, err := exporter.GetAllResourceIDs(ctx)
	suite.Nil(err)
	suite.Equal([]string{"user-2"}, ids)
}

func (suite *DeclarativeResourceTestSuite) TestMakeUserParser_ParsesYAMLToEntityWithCredentials() {
	userYAML := []byte("" +
		"id: user-1\n" +
		"type: person\n" +
		"ou_id: ou-1\n" +
		"attributes:\n" +
		"  username: alice\n" +
		"  email: alice@example.com\n" +
		"credentials:\n" +
		"  password: \"secret\"\n")

	parser := makeUserParser()
	e, _, systemCreds, err := parser(userYAML)

	suite.NoError(err)
	suite.NotNil(e)
	suite.Equal("user-1", e.ID)
	suite.Equal("person", e.Type)
	suite.Equal("ou-1", e.OUID)
	suite.Equal(entitypkg.EntityCategoryUser, e.Category)

	// Verify attributes preserved
	var attrs map[string]interface{}
	suite.NoError(json.Unmarshal(e.Attributes, &attrs))
	suite.Equal("alice", attrs["username"])

	// Verify credentials were parsed (password should be hashed)
	suite.NotNil(systemCreds)
	suite.NotEmpty(systemCreds)
}

func (suite *DeclarativeResourceTestSuite) TestGetResourceRules_IncludesCredentials() {
	exporter := newUserExporter(
		NewUserServiceInterfaceMock(suite.T()), entitymock.NewEntityServiceInterfaceMock(suite.T()))

	rules := exporter.GetResourceRules()
	suite.Contains(rules.DynamicPropertyFields, "Credentials")
}

func (suite *DeclarativeResourceTestSuite) TestValidateResource_MissingUsername() {
	exporter := newUserExporter(
		NewUserServiceInterfaceMock(suite.T()), entitymock.NewEntityServiceInterfaceMock(suite.T()))

	resource := &userDeclarativeResource{
		ID:         "user-1",
		Type:       "person",
		OUID:       "ou-1",
		Attributes: map[string]interface{}{},
	}

	_, err := exporter.ValidateResource(resource, "user-1", log.GetLogger())
	suite.NotNil(err)
}

func (suite *DeclarativeResourceTestSuite) TestMakeUserValidator_Success() {
	attrs, err := json.Marshal(map[string]interface{}{"username": "alice"})
	suite.Require().NoError(err)

	e := &entitypkg.Entity{
		ID:         "user-1",
		Type:       "person",
		OUID:       "ou-1",
		Attributes: attrs,
	}

	svcMock := entitymock.NewEntityServiceInterfaceMock(suite.T())
	svcMock.On("GetEntity", context.Background(), "user-1").
		Return((*entitypkg.Entity)(nil), entitypkg.ErrEntityNotFound)

	validator := makeUserValidator()
	err = validator(e, svcMock)
	suite.NoError(err)
}

func (suite *DeclarativeResourceTestSuite) TestMakeUserValidator_DuplicateEntity() {
	attrs, err := json.Marshal(map[string]interface{}{"username": "alice"})
	suite.Require().NoError(err)

	e := &entitypkg.Entity{
		ID:         "user-1",
		Type:       "person",
		OUID:       "ou-1",
		Attributes: attrs,
	}

	svcMock := entitymock.NewEntityServiceInterfaceMock(suite.T())
	svcMock.On("GetEntity", context.Background(), "user-1").
		Return(&entitypkg.Entity{Category: entitypkg.EntityCategoryUser, ID: "user-1"}, nil)

	validator := makeUserValidator()
	err = validator(e, svcMock)
	suite.Error(err)
	suite.Contains(err.Error(), "duplicate user ID")
}

func (suite *DeclarativeResourceTestSuite) TestMakeUserValidator_DBError() {
	attrs, err := json.Marshal(map[string]interface{}{"username": "alice"})
	suite.Require().NoError(err)

	e := &entitypkg.Entity{
		ID:         "user-1",
		Type:       "person",
		OUID:       "ou-1",
		Attributes: attrs,
	}

	svcMock := entitymock.NewEntityServiceInterfaceMock(suite.T())
	svcMock.On("GetEntity", context.Background(), "user-1").
		Return((*entitypkg.Entity)(nil), errors.New("db error"))

	validator := makeUserValidator()
	err = validator(e, svcMock)
	suite.Error(err)
	suite.Contains(err.Error(), "checking user existence")
}

func (suite *DeclarativeResourceTestSuite) TestParseCredentials_YAMLMapInterfaceFormat() {
	// Simulate YAML map[interface{}]interface{} for credentials
	credMap := map[interface{}]interface{}{
		"storageType": "hash",
		"storageAlgo": "argon2",
		"storageAlgoParams": map[interface{}]interface{}{
			"iterations": 2,
			"keySize":    64,
			"salt":       "salty",
		},
		"value": "hashed-value",
	}
	creds := map[string]interface{}{
		"password": []interface{}{credMap},
	}
	parsed, err := parseCredentials(creds)
	suite.NoError(err)
	suite.Len(parsed["password"], 1)
	suite.Equal("hash", parsed["password"][0].StorageType)
	suite.Equal("hashed-value", parsed["password"][0].Value)
	suite.Equal("argon2", string(parsed["password"][0].StorageAlgo))
	suite.Equal(2, parsed["password"][0].StorageAlgoParams.Iterations)
	suite.Equal(64, parsed["password"][0].StorageAlgoParams.KeySize)
	suite.Equal("salty", parsed["password"][0].StorageAlgoParams.Salt)
}

func (suite *DeclarativeResourceTestSuite) TestParseCredentialObject_YAMLMapInterfaceParams() {
	hashService, err := hash.Initialize(
		hash.HashConfig{Algorithm: hash.PBKDF2, SaltSize: 16, Iterations: 1, KeySize: 32},
	)
	suite.Require().NoError(err)
	credMap := map[string]interface{}{
		"value":       "hashed-value",
		"storageType": "hash",
		"storageAlgo": "argon2",
		"storageAlgoParams": map[interface{}]interface{}{
			"iterations": 3,
			"keySize":    128,
			"salt":       "pepper",
		},
	}
	cred, err := parseCredentialObject(credMap, hashService, CredentialType("password"))
	suite.NoError(err)
	suite.Equal("hash", cred.StorageType)
	suite.Equal("hashed-value", cred.Value)
	suite.Equal("argon2", string(cred.StorageAlgo))
	suite.Equal(3, cred.StorageAlgoParams.Iterations)
	suite.Equal(128, cred.StorageAlgoParams.KeySize)
	suite.Equal("pepper", cred.StorageAlgoParams.Salt)
}

func (suite *DeclarativeResourceTestSuite) TestParseCredentials_InvalidCredentialMapType() {
	creds := map[string]interface{}{
		"password": []interface{}{123}, // not a map
	}
	_, err := parseCredentials(creds)
	suite.Error(err)
}

func (suite *DeclarativeResourceTestSuite) TestBuildHashCfgForUser_UnrecognizedAlgorithmErrors() {
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("test", &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: "BCRYPT",
			},
		},
	})
	suite.Require().NoError(err)

	_, err = buildHashCfgForUser()
	suite.Error(err)
	suite.Contains(err.Error(), "BCRYPT")
}
