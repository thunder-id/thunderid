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

package template

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

// newTemplateFileBasedStoreForTest creates an isolated store for testing
func newTemplateFileBasedStoreForTest() *templateFileBasedStore {
	genericStore := declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeTemplate)
	return &templateFileBasedStore{
		GenericFileBasedStore: genericStore,
	}
}

type FileBasedStoreTestSuite struct {
	suite.Suite
	store *templateFileBasedStore
}

func TestFileBasedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileBasedStoreTestSuite))
}

func (suite *FileBasedStoreTestSuite) SetupTest() {
	suite.store = newTemplateFileBasedStoreForTest()
}

func (suite *FileBasedStoreTestSuite) TestFileBasedStore() {
	dto := &TemplateDTO{
		ID:       "t1",
		Scenario: ScenarioUserInvite,
		Type:     TemplateTypeEmail,
	}
	err := suite.store.Create("t1", dto)
	suite.NoError(err)

	res, err := suite.store.GetTemplate(context.Background(), "t1")
	suite.NoError(err)
	suite.NotNil(res)
	suite.Equal("t1", res.ID)

	resScen, err := suite.store.GetTemplateByScenario(context.Background(), ScenarioUserInvite, TemplateTypeEmail)
	suite.NoError(err)
	suite.NotNil(resScen)
	suite.Equal("t1", resScen.ID)

	list, err := suite.store.ListTemplates(context.Background())
	suite.NoError(err)
	suite.Len(list, 1)
	suite.Equal("t1", list[0].ID)

	resNotFound, err := suite.store.GetTemplate(context.Background(), "non-existent")
	suite.Error(err)
	suite.ErrorIs(err, errTemplateNotFound)
	suite.Nil(resNotFound)

	resScenNotFound, err := suite.store.GetTemplateByScenario(
		context.Background(), ScenarioType("UNKNOWN"), TemplateTypeEmail)
	suite.Error(err)
	suite.ErrorIs(err, errTemplateNotFound)
	suite.Nil(resScenNotFound)
}

func (suite *FileBasedStoreTestSuite) TestFileBasedStore_CorruptedDataType() {
	// Create with wrong type by using the internal GenericFileBasedStore
	err := suite.store.GenericFileBasedStore.Create("t1", "not-a-dto")
	suite.NoError(err)

	res, err := suite.store.GetTemplate(context.Background(), "t1")
	suite.Error(err)
	suite.Contains(err.Error(), "template data corrupted")
	suite.Nil(res)
}

func (suite *FileBasedStoreTestSuite) TestFileBasedStore_Create_InvalidType() {
	// Test Create with invalid data type
	err := suite.store.Create("t1", "invalid-type")
	suite.Error(err)
	suite.Contains(err.Error(), "invalid data type")
}

func (suite *FileBasedStoreTestSuite) TestFileBasedStore_GetTemplateByScenario_CorruptedData() {
	// First create a valid template so the store knows where to look by scenario
	// Then corrupt it by directly modifying the store
	dto := &TemplateDTO{
		ID:       "t1",
		Scenario: ScenarioUserInvite,
		Type:     TemplateTypeEmail,
	}
	err := suite.store.Create("t1", dto)
	suite.NoError(err)

	// Now replace it with corrupted data using the GenericFileBasedStore
	err = suite.store.GenericFileBasedStore.Create("t1", "not-a-dto")
	suite.NoError(err)

	// Try to get a template by ID - should fail with corrupted data error
	res, err := suite.store.GetTemplate(context.Background(), "t1")
	suite.Error(err)
	suite.Contains(err.Error(), "template data corrupted")
	suite.Nil(res)
}

func (suite *FileBasedStoreTestSuite) TestFileBasedStore_SameScenarioDifferentType() {
	emailDTO := &TemplateDTO{
		ID:       "otp-email",
		Scenario: ScenarioOTP,
		Type:     TemplateTypeEmail,
	}
	smsDTO := &TemplateDTO{
		ID:       "otp-sms",
		Scenario: ScenarioOTP,
		Type:     TemplateTypeSMS,
	}
	suite.NoError(suite.store.Create("otp-email", emailDTO))
	suite.NoError(suite.store.Create("otp-sms", smsDTO))

	resEmail, err := suite.store.GetTemplateByScenario(context.Background(), ScenarioOTP, TemplateTypeEmail)
	suite.NoError(err)
	suite.NotNil(resEmail)
	suite.Equal("otp-email", resEmail.ID)
	suite.Equal(TemplateTypeEmail, resEmail.Type)

	resSMS, err := suite.store.GetTemplateByScenario(context.Background(), ScenarioOTP, TemplateTypeSMS)
	suite.NoError(err)
	suite.NotNil(resSMS)
	suite.Equal("otp-sms", resSMS.ID)
	suite.Equal(TemplateTypeSMS, resSMS.Type)
}

func (suite *FileBasedStoreTestSuite) TestFileBasedStore_ListTemplates_WithCorruptedData() {
	// Create a valid template
	dto := &TemplateDTO{
		ID:       "t1",
		Scenario: ScenarioUserInvite,
	}
	err := suite.store.Create("t1", dto)
	suite.NoError(err)

	// Create with the wrong type by using the internal GenericFileBasedStore
	err = suite.store.GenericFileBasedStore.Create("t2", "not-a-dto")
	suite.NoError(err)

	// List should only return valid templates, skipping corrupted ones
	list, err := suite.store.ListTemplates(context.Background())
	suite.NoError(err)
	suite.Len(list, 1)
	suite.Equal("t1", list[0].ID)
}
