// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// reversingCrypto stands in for the key manager. It is reversible so a test can prove a stored value
// is not the plaintext, which is the property that matters, without depending on real key material.
type reversingCrypto struct {
	err error
}

func (r reversingCrypto) Encrypt(_ context.Context, content []byte) ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}
	reversed := make([]byte, len(content))
	for i, b := range content {
		reversed[len(content)-1-i] = b
	}
	return reversed, nil
}

func (r reversingCrypto) Decrypt(_ context.Context, content []byte) ([]byte, error) {
	return r.Encrypt(context.Background(), content)
}

type ServiceTestSuite struct {
	suite.Suite
	store   *storeInterfaceMock
	service ServiceInterface
}

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	s.store = newStoreInterfaceMock(s.T())
	s.service = newService(s.store, reversingCrypto{})
}

func (s *ServiceTestSuite) TestCreateVariableStoresAndReadsBack() {
	ctx := context.Background()
	stored := Variable{Name: "API_URL", Value: "https://example.test", CreatedAt: "2026-01-01T00:00:00Z"}

	s.store.On("GetVariable", ctx, "API_URL").Return(nil, nil).Once()
	s.store.On("InsertVariable", ctx, mock.MatchedBy(func(v Variable) bool {
		return v.Name == "API_URL" && v.Value == "https://example.test"
	})).Return(true, nil).Once()
	s.store.On("GetVariable", ctx, "API_URL").Return(&stored, nil).Once()

	created, svcErr := s.service.CreateVariable(ctx, VariableRequest{
		Name: "API_URL", Value: "https://example.test"})

	s.Nil(svcErr)
	s.Require().NotNil(created)
	s.Equal("https://example.test", created.Value)
	// The timestamp comes from the row that was written, not from the request.
	s.Equal("2026-01-01T00:00:00Z", created.CreatedAt)
}

func (s *ServiceTestSuite) TestCreateVariableRefusesADuplicate() {
	ctx := context.Background()
	s.store.On("GetVariable", ctx, "API_URL").Return(&Variable{Name: "API_URL"}, nil).Once()

	_, svcErr := s.service.CreateVariable(ctx, VariableRequest{Name: "API_URL", Value: "x"})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorAlreadyExists.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestCreateVariableAcceptsAnEmptyValue() {
	ctx := context.Background()
	s.store.On("GetVariable", ctx, "EMPTY").Return(nil, nil).Once()
	s.store.On("InsertVariable", ctx, mock.Anything).Return(true, nil).Once()
	s.store.On("GetVariable", ctx, "EMPTY").Return(&Variable{Name: "EMPTY"}, nil).Once()

	_, svcErr := s.service.CreateVariable(ctx, VariableRequest{Name: "EMPTY", Value: ""})

	s.Nil(svcErr)
}

func (s *ServiceTestSuite) TestGetVariableReportsAMissingOne() {
	ctx := context.Background()
	s.store.On("GetVariable", ctx, "ABSENT").Return(nil, nil).Once()

	_, svcErr := s.service.GetVariable(ctx, "ABSENT")

	s.Require().NotNil(svcErr)
	s.Equal(ErrorNotFound.Code, svcErr.Code)
}

// PUT is create-or-replace, so an unused name creates rather than failing.
func (s *ServiceTestSuite) TestUpdateVariableCreatesWhenTheNameIsUnused() {
	ctx := context.Background()
	s.store.On("UpsertVariable", ctx, Variable{Name: "NEW", Value: "x"}).Return(true, nil).Once()
	s.store.On("GetVariable", ctx, "NEW").Return(&Variable{Name: "NEW", Value: "x"}, nil).Once()

	variable, created, svcErr := s.service.UpdateVariable(ctx, "NEW", VariableUpdateRequest{Value: "x"})

	s.Nil(svcErr)
	s.True(created, "an unused name must report that it created")
	s.Require().NotNil(variable)
	s.Equal("x", variable.Value)
}

// Deleting is idempotent: a caller cleaning up should not have to ask first.
func (s *ServiceTestSuite) TestDeleteVariableSucceedsWhenThereWasNone() {
	ctx := context.Background()
	s.store.On("DeleteVariable", ctx, "ABSENT").Return(nil).Once()

	s.Nil(s.service.DeleteVariable(ctx, "ABSENT"))
}

// A store failure must not reach the caller as itself: the caller gets one opaque error, and the
// cause is logged. This matters most for secrets, where a message could describe key material.
func (s *ServiceTestSuite) TestStoreFailureIsNotDisclosed() {
	ctx := context.Background()
	s.store.On("GetVariable", ctx, "API_URL").Return(nil, errors.New("dial tcp 10.0.0.1:5432: refused")).Once()

	_, svcErr := s.service.GetVariable(ctx, "API_URL")

	s.Require().NotNil(svcErr)
	s.Equal(ErrorInternalServerError.Code, svcErr.Code)
	s.NotContains(svcErr.ErrorDescription.DefaultValue, "5432")
}

func (s *ServiceTestSuite) TestCreateSecretSealsTheValueBeforeItIsStored() {
	ctx := context.Background()
	plaintext := "correct horse battery staple"

	s.store.On("GetSecret", ctx, "DB_PASSWORD").Return(nil, nil).Once()
	s.store.On("InsertSecret", ctx, "DB_PASSWORD", mock.MatchedBy(func(sealed string) bool {
		// The exact ciphertext is the crypto provider's business; that it is not the plaintext is
		// this service's.
		return sealed != plaintext && sealed != ""
	}), "").Return(true, nil).Once()
	s.store.On("GetSecret", ctx, "DB_PASSWORD").Return(&Secret{Name: "DB_PASSWORD", Exists: true}, nil).Once()

	created, svcErr := s.service.CreateSecret(ctx, SecretRequest{Name: "DB_PASSWORD", Value: plaintext})

	s.Nil(svcErr)
	s.Require().NotNil(created)
	s.True(created.Exists)
}

// The type carries no value field, so a response cannot disclose one. This asserts the shape rather
// than trusting that no handler ever adds it.
func (s *ServiceTestSuite) TestASecretResponseHasNowhereToPutAValue() {
	ctx := context.Background()
	s.store.On("GetSecret", ctx, "DB_PASSWORD").Return(&Secret{Name: "DB_PASSWORD", Exists: true}, nil).Once()

	secret, svcErr := s.service.GetSecret(ctx, "DB_PASSWORD")

	s.Nil(svcErr)
	s.Require().NotNil(secret)
	marshaled := mustMarshal(s.T(), secret)
	s.NotContains(strings.ToLower(marshaled), `"value"`)
}

func (s *ServiceTestSuite) TestCreateSecretRefusesAnEmptyValue() {
	ctx := context.Background()

	_, svcErr := s.service.CreateSecret(ctx, SecretRequest{Name: "DB_PASSWORD", Value: ""})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidValue.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestSecretIsNotStoredWhenSealingFails() {
	ctx := context.Background()
	s.service = newService(s.store, reversingCrypto{err: errors.New("key unavailable")})
	s.store.On("GetSecret", ctx, "DB_PASSWORD").Return(nil, nil).Once()

	_, svcErr := s.service.CreateSecret(ctx, SecretRequest{Name: "DB_PASSWORD", Value: "hunter2"})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorInternalServerError.Code, svcErr.Code)
	// No InsertSecret expectation was set: a plaintext write here would fail the mock.
	s.store.AssertNotCalled(s.T(), "InsertSecret", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestUpdateSecretRotatesWithoutReadingTheOldValue() {
	ctx := context.Background()
	s.store.On("UpsertSecret", ctx, "DB_PASSWORD", mock.MatchedBy(func(sealed string) bool {
		return sealed != "new-value"
	}), "").Return(false, nil).Once()
	s.store.On("GetSecret", ctx, "DB_PASSWORD").Return(&Secret{Name: "DB_PASSWORD", Exists: true}, nil).Once()

	_, created, svcErr := s.service.UpdateSecret(ctx, "DB_PASSWORD", SecretUpdateRequest{Value: "new-value"})

	s.Nil(svcErr)
	s.False(created)
	// No GetSecret was expected before the write: a rotation never reads the value it replaces.
}

func (s *ServiceTestSuite) TestListVariablesReportsTheTotalBeyondThePage() {
	ctx := context.Background()
	page := []Variable{{Name: "A"}, {Name: "B"}}
	s.store.On("ListVariables", ctx, listQuery{limit: 2, offset: 0}).Return(page, 7, nil).Once()

	response, svcErr := s.service.ListVariables(ctx, listQuery{limit: 2, offset: 0})

	s.Nil(svcErr)
	s.Require().NotNil(response)
	s.Equal(7, response.TotalResults)
	s.Equal(2, response.Count)
	s.Equal(1, response.StartIndex)
	s.Len(response.Links, 1)
	s.Equal("next", response.Links[0].Rel)
}

func (s *ServiceTestSuite) TestListSecretsOnTheLastPageOffersNoNextLink() {
	ctx := context.Background()
	q := listQuery{limit: 2, offset: 6}
	s.store.On("ListSecrets", ctx, q).Return([]Secret{{Name: "G", Exists: true}}, 7, nil).Once()

	response, svcErr := s.service.ListSecrets(ctx, q)

	s.Nil(svcErr)
	s.Require().NotNil(response)
	s.Equal(7, response.StartIndex)
	s.Len(response.Links, 1)
	s.Equal("previous", response.Links[0].Rel)
}

func (s *ServiceTestSuite) TestAnInvalidNameIsRefusedBeforeTheStoreIsTouched() {
	ctx := context.Background()

	for _, name := range []string{"", "has space", "1leading", "dash-ed", strings.Repeat("A", maxNameLength+1)} {
		_, svcErr := s.service.GetVariable(ctx, name)
		s.Require().NotNil(svcErr, "expected %q to be refused", name)
		s.Equal(ErrorInvalidName.Code, svcErr.Code)
	}
	// Every assertion above returned before reaching the store, which has no expectations set.
}

func mustMarshal(t require.TestingT, value interface{}) string {
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}

func TestValidateValueBounds(t *testing.T) {
	assert.Nil(t, validateValue(strings.Repeat("v", maxValueLength)))
	assert.NotNil(t, validateValue(strings.Repeat("v", maxValueLength+1)))
	assert.Nil(t, validateDescription(strings.Repeat("d", maxDescriptionLength)))
	assert.NotNil(t, validateDescription(strings.Repeat("d", maxDescriptionLength+1)))
}

func (s *ServiceTestSuite) TestDeleteSecretRemovesAStoredOne() {
	ctx := context.Background()
	s.store.On("DeleteSecret", ctx, "DB_PASSWORD").Return(nil).Once()

	s.Nil(s.service.DeleteSecret(ctx, "DB_PASSWORD"))
}

func (s *ServiceTestSuite) TestDeleteSecretSucceedsWhenThereWasNone() {
	ctx := context.Background()
	s.store.On("DeleteSecret", ctx, "ABSENT").Return(nil).Once()

	s.Nil(s.service.DeleteSecret(ctx, "ABSENT"))
}

func (s *ServiceTestSuite) TestDeleteVariableRemovesAStoredOne() {
	ctx := context.Background()
	s.store.On("DeleteVariable", ctx, "API_URL").Return(nil).Once()

	s.Nil(s.service.DeleteVariable(ctx, "API_URL"))
}

func (s *ServiceTestSuite) TestUpdateVariableReplacesTheValue() {
	ctx := context.Background()
	s.store.On("UpsertVariable", ctx, Variable{Name: "API_URL", Value: "new"}).Return(false, nil).Once()
	s.store.On("GetVariable", ctx, "API_URL").Return(&Variable{Name: "API_URL", Value: "new"}, nil).Once()

	updated, created, svcErr := s.service.UpdateVariable(ctx, "API_URL", VariableUpdateRequest{Value: "new"})

	s.Nil(svcErr)
	s.False(created, "a name already in use must report that it replaced")
	s.Require().NotNil(updated)
	s.Equal("new", updated.Value)
}

func (s *ServiceTestSuite) TestUpdateSecretStoresWhenTheNameIsUnused() {
	ctx := context.Background()
	s.store.On("UpsertSecret", ctx, "NEW", mock.Anything, "").Return(true, nil).Once()
	s.store.On("GetSecret", ctx, "NEW").Return(&Secret{Name: "NEW", Exists: true}, nil).Once()

	_, created, svcErr := s.service.UpdateSecret(ctx, "NEW", SecretUpdateRequest{Value: "x"})

	s.Nil(svcErr)
	s.True(created)
}

func (s *ServiceTestSuite) TestAnOverlongDescriptionIsRefused() {
	ctx := context.Background()

	_, svcErr := s.service.CreateVariable(ctx, VariableRequest{
		Name: "API_URL", Value: "v", Description: strings.Repeat("d", maxDescriptionLength+1)})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorInvalidDescription.Code, svcErr.Code)
}

// Two creates arriving together both pass the pre-read; the insert stores only for one, and the
// other must be told the name is taken rather than that something went wrong.
func (s *ServiceTestSuite) TestCreateVariableLosingTheRaceIsAConflict() {
	ctx := context.Background()
	s.store.On("GetVariable", ctx, "API_URL").Return(nil, nil).Once()
	s.store.On("InsertVariable", ctx, mock.Anything).Return(false, nil).Once()

	_, svcErr := s.service.CreateVariable(ctx, VariableRequest{Name: "API_URL", Value: "v"})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorAlreadyExists.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestCreateSecretLosingTheRaceIsAConflict() {
	ctx := context.Background()
	s.store.On("GetSecret", ctx, "DB_PASSWORD").Return(nil, nil).Once()
	s.store.On("InsertSecret", ctx, "DB_PASSWORD", mock.Anything, "").Return(false, nil).Once()

	_, svcErr := s.service.CreateSecret(ctx, SecretRequest{Name: "DB_PASSWORD", Value: "hunter2"})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorAlreadyExists.Code, svcErr.Code)
}
