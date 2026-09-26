// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"context"
	"fmt"

	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ServiceInterface is the variable store.
//
// Secrets and variables are deliberately separate methods rather than one set with a flag. The two
// have different rules about what comes back out, and a flag is a thing a caller can get wrong.
type ServiceInterface interface {
	CreateVariable(ctx context.Context, request VariableRequest) (*Variable, *common.ServiceError)
	GetVariable(ctx context.Context, name string) (*Variable, *common.ServiceError)
	ListVariables(ctx context.Context, q listQuery) (*VariableListResponse, *common.ServiceError)
	// UpdateVariable creates the variable when the name is unused and replaces it when it is used,
	// reporting which happened so the caller can answer 201 or 200.
	UpdateVariable(ctx context.Context, name string, request VariableUpdateRequest) (
		*Variable, bool, *common.ServiceError)
	DeleteVariable(ctx context.Context, name string) *common.ServiceError

	CreateSecret(ctx context.Context, request SecretRequest) (*Secret, *common.ServiceError)
	GetSecret(ctx context.Context, name string) (*Secret, *common.ServiceError)
	ListSecrets(ctx context.Context, q listQuery) (*SecretListResponse, *common.ServiceError)
	// UpdateSecret stores the secret when the name is unused and rotates it when it is used,
	// reporting which happened.
	UpdateSecret(ctx context.Context, name string, request SecretUpdateRequest) (
		*Secret, bool, *common.ServiceError)
	DeleteSecret(ctx context.Context, name string) *common.ServiceError
}

type service struct {
	store  storeInterface
	crypto kmprovider.ConfigCryptoProvider
}

// newService builds the service over a store and the key material secrets are sealed with.
func newService(store storeInterface, crypto kmprovider.ConfigCryptoProvider) ServiceInterface {
	return &service{store: store, crypto: crypto}
}

func (s *service) CreateVariable(ctx context.Context, request VariableRequest) (
	*Variable, *common.ServiceError) {
	if svcErr := validateName(request.Name); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := validateValue(request.Value); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := validateDescription(request.Description); svcErr != nil {
		return nil, svcErr
	}

	existing, err := s.store.GetVariable(ctx, request.Name)
	if err != nil {
		return nil, s.unexpected(ctx, "failed to read variable", err)
	}
	if existing != nil {
		return nil, &ErrorAlreadyExists
	}

	// The insert stores only when the name is free, so two creates arriving together cannot both be
	// told they succeeded. The read above gives the common case its error without a write attempt;
	// this is what makes the answer correct when two requests race.
	variable := Variable{Name: request.Name, Value: request.Value, Description: request.Description}
	inserted, err := s.store.InsertVariable(ctx, variable)
	if err != nil {
		return nil, s.unexpected(ctx, "failed to create variable", err)
	}
	if !inserted {
		return nil, &ErrorAlreadyExists
	}
	return s.readVariable(ctx, request.Name)
}

func (s *service) GetVariable(ctx context.Context, name string) (*Variable, *common.ServiceError) {
	if svcErr := validateName(name); svcErr != nil {
		return nil, svcErr
	}
	variable, err := s.store.GetVariable(ctx, name)
	if err != nil {
		return nil, s.unexpected(ctx, "failed to read variable", err)
	}
	if variable == nil {
		return nil, &ErrorNotFound
	}
	return variable, nil
}

func (s *service) ListVariables(ctx context.Context, q listQuery) (
	*VariableListResponse, *common.ServiceError) {
	variables, total, err := s.store.ListVariables(ctx, q)
	if err != nil {
		return nil, s.unexpected(ctx, "failed to list variables", err)
	}
	return &VariableListResponse{
		TotalResults: total,
		StartIndex:   q.offset + 1,
		Count:        len(variables),
		Variables:    variables,
		Links:        buildLinks(pathVariables, q, total),
	}, nil
}

// UpdateVariable is create-or-replace, which is what makes PUT idempotent: the same request twice
// leaves the same state. A name that is not yet used is not an error here, it is the create case.
func (s *service) UpdateVariable(ctx context.Context, name string, request VariableUpdateRequest) (
	*Variable, bool, *common.ServiceError) {
	if svcErr := validateName(name); svcErr != nil {
		return nil, false, svcErr
	}
	if svcErr := validateValue(request.Value); svcErr != nil {
		return nil, false, svcErr
	}
	if svcErr := validateDescription(request.Description); svcErr != nil {
		return nil, false, svcErr
	}

	created, err := s.store.UpsertVariable(ctx, Variable{
		Name: name, Value: request.Value, Description: request.Description})
	if err != nil {
		return nil, false, s.unexpected(ctx, "failed to write variable", err)
	}

	variable, svcErr := s.readVariable(ctx, name)
	if svcErr != nil {
		return nil, false, svcErr
	}
	return variable, created, nil
}

// DeleteVariable removes a variable, and succeeds whether or not one was there. A caller cleaning up
// should not have to ask first, and the end state it wanted holds either way.
func (s *service) DeleteVariable(ctx context.Context, name string) *common.ServiceError {
	if svcErr := validateName(name); svcErr != nil {
		return svcErr
	}
	if err := s.store.DeleteVariable(ctx, name); err != nil {
		return s.unexpected(ctx, "failed to delete variable", err)
	}
	return nil
}

func (s *service) CreateSecret(ctx context.Context, request SecretRequest) (
	*Secret, *common.ServiceError) {
	if svcErr := validateName(request.Name); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := validateSecretValue(request.Value); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := validateDescription(request.Description); svcErr != nil {
		return nil, svcErr
	}

	existing, err := s.store.GetSecret(ctx, request.Name)
	if err != nil {
		return nil, s.unexpected(ctx, "failed to read secret", err)
	}
	if existing != nil {
		return nil, &ErrorAlreadyExists
	}

	sealed, svcErr := s.seal(ctx, request.Value)
	if svcErr != nil {
		return nil, svcErr
	}
	inserted, err := s.store.InsertSecret(ctx, request.Name, sealed, request.Description)
	if err != nil {
		return nil, s.unexpected(ctx, "failed to create secret", err)
	}
	if !inserted {
		return nil, &ErrorAlreadyExists
	}
	return s.readSecret(ctx, request.Name)
}

func (s *service) GetSecret(ctx context.Context, name string) (*Secret, *common.ServiceError) {
	if svcErr := validateName(name); svcErr != nil {
		return nil, svcErr
	}
	secret, err := s.store.GetSecret(ctx, name)
	if err != nil {
		return nil, s.unexpected(ctx, "failed to read secret", err)
	}
	if secret == nil {
		return nil, &ErrorNotFound
	}
	return secret, nil
}

func (s *service) ListSecrets(ctx context.Context, q listQuery) (
	*SecretListResponse, *common.ServiceError) {
	secrets, total, err := s.store.ListSecrets(ctx, q)
	if err != nil {
		return nil, s.unexpected(ctx, "failed to list secrets", err)
	}
	return &SecretListResponse{
		TotalResults: total,
		StartIndex:   q.offset + 1,
		Count:        len(secrets),
		Secrets:      secrets,
		Links:        buildLinks(pathSecrets, q, total),
	}, nil
}

// UpdateSecret is store-or-rotate. There is no read-modify-write because the current value is never
// read: a rotation supplies the new value in full.
func (s *service) UpdateSecret(ctx context.Context, name string, request SecretUpdateRequest) (
	*Secret, bool, *common.ServiceError) {
	if svcErr := validateName(name); svcErr != nil {
		return nil, false, svcErr
	}
	if svcErr := validateSecretValue(request.Value); svcErr != nil {
		return nil, false, svcErr
	}
	if svcErr := validateDescription(request.Description); svcErr != nil {
		return nil, false, svcErr
	}

	sealed, svcErr := s.seal(ctx, request.Value)
	if svcErr != nil {
		return nil, false, svcErr
	}

	created, err := s.store.UpsertSecret(ctx, name, sealed, request.Description)
	if err != nil {
		return nil, false, s.unexpected(ctx, "failed to write secret", err)
	}

	secret, svcErr := s.readSecret(ctx, name)
	if svcErr != nil {
		return nil, false, svcErr
	}
	return secret, created, nil
}

// DeleteSecret removes a secret, and succeeds whether or not one was there.
func (s *service) DeleteSecret(ctx context.Context, name string) *common.ServiceError {
	if svcErr := validateName(name); svcErr != nil {
		return svcErr
	}
	if err := s.store.DeleteSecret(ctx, name); err != nil {
		return s.unexpected(ctx, "failed to delete secret", err)
	}
	return nil
}

// seal encrypts a secret value for storage. Nothing outside this file writes to the SECRET table's
// value column, so this is the only door in.
func (s *service) seal(ctx context.Context, value string) (string, *common.ServiceError) {
	if s.crypto == nil {
		return "", s.unexpected(ctx, "secret encryption is unavailable",
			fmt.Errorf("config crypto provider not initialized"))
	}
	sealed, err := s.crypto.Encrypt(ctx, []byte(value))
	if err != nil {
		return "", s.unexpected(ctx, "failed to encrypt secret", err)
	}
	return string(sealed), nil
}

// readVariable re-reads what was just written, so a response carries the timestamps the database set
// rather than ones guessed here.
func (s *service) readVariable(ctx context.Context, name string) (*Variable, *common.ServiceError) {
	variable, err := s.store.GetVariable(ctx, name)
	if err != nil {
		return nil, s.unexpected(ctx, "failed to read variable", err)
	}
	if variable == nil {
		return nil, &ErrorNotFound
	}
	return variable, nil
}

func (s *service) readSecret(ctx context.Context, name string) (*Secret, *common.ServiceError) {
	secret, err := s.store.GetSecret(ctx, name)
	if err != nil {
		return nil, s.unexpected(ctx, "failed to read secret", err)
	}
	if secret == nil {
		return nil, &ErrorNotFound
	}
	return secret, nil
}

// unexpected logs the cause and returns the one error a caller is allowed to see. The message never
// reaches the client, which matters most for secrets: a failure to encrypt must not describe the key.
func (s *service) unexpected(ctx context.Context, message string, err error) *common.ServiceError {
	log.GetLogger().Error(ctx, message, log.Error(err))
	return &ErrorInternalServerError
}
