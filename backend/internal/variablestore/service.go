// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// VariableServiceInterface is the variables half of the store.
//
// A variable's value is returned as it stands, by a read of one and by a listing alike.
type VariableServiceInterface interface {
	CreateVariable(ctx context.Context, request VariableRequest) (*Variable, *common.ServiceError)
	GetVariable(ctx context.Context, name string) (*Variable, *common.ServiceError)
	ListVariables(ctx context.Context, q listQuery) (*VariableListResponse, *common.ServiceError)
	// UpdateVariable creates the variable when the name is unused and replaces it when it is used,
	// reporting which happened so the caller can answer 201 or 200.
	UpdateVariable(ctx context.Context, name string, request VariableUpdateRequest) (
		*Variable, bool, *common.ServiceError)
	DeleteVariable(ctx context.Context, name string) *common.ServiceError
}

// SecretServiceInterface is the secrets half of the store.
//
// It is a separate interface from the variables half because the two differ in the one way that
// matters: nothing here returns a value. A caller that only writes credentials can depend on this
// alone, and a caller that only reads configuration need not hold a type that can write them.
type SecretServiceInterface interface {
	CreateSecret(ctx context.Context, request SecretRequest) (*Secret, *common.ServiceError)
	GetSecret(ctx context.Context, name string) (*Secret, *common.ServiceError)
	ListSecrets(ctx context.Context, q listQuery) (*SecretListResponse, *common.ServiceError)
	// UpdateSecret stores the secret when the name is unused and rotates it when it is used,
	// reporting which happened.
	UpdateSecret(ctx context.Context, name string, request SecretUpdateRequest) (
		*Secret, bool, *common.ServiceError)
	DeleteSecret(ctx context.Context, name string) *common.ServiceError
}

// ServiceInterface is both halves, which is what the HTTP layer registers: the two collections are
// served by one deployment and wired once.
//
// Secrets and variables are deliberately separate methods rather than one set with a flag. The two
// have different rules about what comes back out, and a flag is a thing a caller can get wrong.
type ServiceInterface interface {
	VariableServiceInterface
	SecretServiceInterface
}

// ReferenceResolverInterface reads back the value held under a name, a secret's included.
//
// It is the one way a secret's value leaves the store, and it is not served over HTTP: an import uses
// it to put a value where a reference stood, on the deployment that holds the value, before the
// resource is written. A secret then takes the path any credential takes, hashed where it is only
// ever verified.
type ReferenceResolverInterface interface {
	// ResolveVariable returns a variable's value and whether one is held under the name.
	ResolveVariable(ctx context.Context, name string) (string, bool, *common.ServiceError)
	// ResolveSecret returns a secret's value and whether one is held under the name.
	ResolveSecret(ctx context.Context, name string) (string, bool, *common.ServiceError)
}

type service struct {
	store  storeInterface
	crypto kmprovider.ConfigCryptoProvider
	bus    eventbus.EventBusInterface
}

// newService builds the service over a store, the key material secrets are encrypted with, and the bus
// it publishes its audit events on.
func newService(store storeInterface, crypto kmprovider.ConfigCryptoProvider,
	bus eventbus.EventBusInterface) ServiceInterface {
	return &service{store: store, crypto: crypto, bus: bus}
}

// CreateVariable creates a variable. Its audit event records the write, not the read that builds the response.
func (s *service) CreateVariable(ctx context.Context, request VariableRequest) (
	*Variable, *common.ServiceError) {
	svcErr := s.createVariable(ctx, request)
	s.publishEvent(ctx, variableCreate, request.Name, svcErr)
	if svcErr != nil {
		return nil, svcErr
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
//
// Its audit event is named for the update the caller attempted, whichever of the two happened, and
// records the write, not the read that builds the response.
func (s *service) UpdateVariable(ctx context.Context, name string, request VariableUpdateRequest) (
	*Variable, bool, *common.ServiceError) {
	created, svcErr := s.updateVariable(ctx, name, request)
	s.publishEvent(ctx, variableUpdate, name, svcErr)
	if svcErr != nil {
		return nil, false, svcErr
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
	deleted, svcErr := s.deleteVariable(ctx, name)
	// A delete that found nothing to remove succeeds without an event, so no caller can record a
	// deletion of a name it made up.
	if svcErr == nil && !deleted {
		return nil
	}
	s.publishEvent(ctx, variableDelete, name, svcErr)
	return svcErr
}

// CreateSecret stores a secret. Its audit event records the write, not the read that builds the response.
func (s *service) CreateSecret(ctx context.Context, request SecretRequest) (
	*Secret, *common.ServiceError) {
	svcErr := s.createSecret(ctx, request)
	s.publishEvent(ctx, secretCreate, request.Name, svcErr)
	if svcErr != nil {
		return nil, svcErr
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
//
// Its audit event is named for the update the caller attempted, whichever of the two happened, and
// records the write, not the read that builds the response.
func (s *service) UpdateSecret(ctx context.Context, name string, request SecretUpdateRequest) (
	*Secret, bool, *common.ServiceError) {
	created, svcErr := s.updateSecret(ctx, name, request)
	s.publishEvent(ctx, secretUpdate, name, svcErr)
	if svcErr != nil {
		return nil, false, svcErr
	}
	secret, svcErr := s.readSecret(ctx, name)
	if svcErr != nil {
		return nil, false, svcErr
	}
	return secret, created, nil
}

// DeleteSecret removes a secret, and succeeds whether or not one was there.
func (s *service) DeleteSecret(ctx context.Context, name string) *common.ServiceError {
	deleted, svcErr := s.deleteSecret(ctx, name)
	// A delete that found nothing to remove succeeds without an event, so no caller can record a
	// deletion of a name it made up.
	if svcErr == nil && !deleted {
		return nil
	}
	s.publishEvent(ctx, secretDelete, name, svcErr)
	return svcErr
}

func (s *service) ResolveVariable(ctx context.Context, name string) (string, bool, *common.ServiceError) {
	variable, err := s.store.GetVariable(ctx, name)
	if err != nil {
		return "", false, s.unexpected(ctx, "failed to read variable", err)
	}
	if variable == nil {
		return "", false, nil
	}
	return variable.Value, true, nil
}

func (s *service) ResolveSecret(ctx context.Context, name string) (string, bool, *common.ServiceError) {
	encrypted, held, err := s.store.GetSecretValue(ctx, name)
	if err != nil {
		return "", false, s.unexpected(ctx, "failed to read secret value", err)
	}
	if !held {
		return "", false, nil
	}
	value, err := s.crypto.Decrypt(ctx, []byte(encrypted))
	if err != nil {
		return "", false, s.unexpected(ctx, "failed to decrypt secret", err)
	}
	return string(value), true, nil
}

// createVariable validates and inserts a variable.
func (s *service) createVariable(ctx context.Context, request VariableRequest) *common.ServiceError {
	if svcErr := validateName(request.Name); svcErr != nil {
		return svcErr
	}
	if svcErr := validateValue(request.Value); svcErr != nil {
		return svcErr
	}
	if svcErr := validateDescription(request.Description); svcErr != nil {
		return svcErr
	}

	existing, err := s.store.GetVariable(ctx, request.Name)
	if err != nil {
		return s.unexpected(ctx, "failed to read variable", err)
	}
	if existing != nil {
		return &ErrorAlreadyExists
	}

	// The insert stores only when the name is free, so two creates arriving together cannot both be
	// told they succeeded. The read above gives the common case its error without a write attempt;
	// this is what makes the answer correct when two requests race.
	variable := Variable{Name: request.Name, Value: request.Value, Description: request.Description}
	inserted, err := s.store.InsertVariable(ctx, variable)
	if err != nil {
		return s.unexpected(ctx, "failed to create variable", err)
	}
	if !inserted {
		return &ErrorAlreadyExists
	}
	return nil
}

// updateVariable validates and writes a variable, reporting whether the name was unused.
func (s *service) updateVariable(ctx context.Context, name string, request VariableUpdateRequest) (
	bool, *common.ServiceError) {
	if svcErr := validateName(name); svcErr != nil {
		return false, svcErr
	}
	if svcErr := validateValue(request.Value); svcErr != nil {
		return false, svcErr
	}
	if svcErr := validateDescription(request.Description); svcErr != nil {
		return false, svcErr
	}

	created, err := s.store.UpsertVariable(ctx, Variable{
		Name: name, Value: request.Value, Description: request.Description})
	if err != nil {
		return false, s.unexpected(ctx, "failed to write variable", err)
	}
	return created, nil
}

// deleteVariable removes a variable, reporting whether there was one.
func (s *service) deleteVariable(ctx context.Context, name string) (bool, *common.ServiceError) {
	if svcErr := validateName(name); svcErr != nil {
		return false, svcErr
	}
	deleted, err := s.store.DeleteVariable(ctx, name)
	if err != nil {
		return false, s.unexpected(ctx, "failed to delete variable", err)
	}
	return deleted, nil
}

// createSecret validates, seals and inserts a secret.
func (s *service) createSecret(ctx context.Context, request SecretRequest) *common.ServiceError {
	if svcErr := validateName(request.Name); svcErr != nil {
		return svcErr
	}
	if svcErr := validateSecretValue(request.Value); svcErr != nil {
		return svcErr
	}
	if svcErr := validateDescription(request.Description); svcErr != nil {
		return svcErr
	}

	existing, err := s.store.GetSecret(ctx, request.Name)
	if err != nil {
		return s.unexpected(ctx, "failed to read secret", err)
	}
	if existing != nil {
		return &ErrorAlreadyExists
	}

	encrypted, svcErr := s.encrypt(ctx, request.Value)
	if svcErr != nil {
		return svcErr
	}
	inserted, err := s.store.InsertSecret(ctx, request.Name, encrypted, request.Description)
	if err != nil {
		return s.unexpected(ctx, "failed to create secret", err)
	}
	if !inserted {
		return &ErrorAlreadyExists
	}
	return nil
}

// updateSecret validates, seals and writes a secret, reporting whether the name was unused.
func (s *service) updateSecret(ctx context.Context, name string, request SecretUpdateRequest) (
	bool, *common.ServiceError) {
	if svcErr := validateName(name); svcErr != nil {
		return false, svcErr
	}
	if svcErr := validateSecretValue(request.Value); svcErr != nil {
		return false, svcErr
	}
	if svcErr := validateDescription(request.Description); svcErr != nil {
		return false, svcErr
	}

	encrypted, svcErr := s.encrypt(ctx, request.Value)
	if svcErr != nil {
		return false, svcErr
	}

	created, err := s.store.UpsertSecret(ctx, name, encrypted, request.Description)
	if err != nil {
		return false, s.unexpected(ctx, "failed to write secret", err)
	}
	return created, nil
}

// deleteSecret removes a secret, reporting whether there was one.
func (s *service) deleteSecret(ctx context.Context, name string) (bool, *common.ServiceError) {
	if svcErr := validateName(name); svcErr != nil {
		return false, svcErr
	}
	deleted, err := s.store.DeleteSecret(ctx, name)
	if err != nil {
		return false, s.unexpected(ctx, "failed to delete secret", err)
	}
	return deleted, nil
}

// publishEvent publishes the audit event of an attempt on the variable or secret named name. The name is
// its only ID, and the payload keeps the first 128 characters of an ID, so two names sharing them share a
// target ID in the log. Its value and description are never recorded.
func (s *service) publishEvent(ctx context.Context, change auditedChange, name string,
	svcErr *common.ServiceError) {
	// A failed attempt keeps the name the caller sent, even one that matches nothing or is not a valid
	// name, as forensic evidence of what was probed; the payload caps it and the sink escapes it.
	target := audit.Ref{Type: change.targetType, ID: name}
	eventbus.TryPublish(ctx, s.bus,
		eventbus.NewEvent(ctx, change.event).With(audit.ChangePayload(ctx, target, change.action, svcErr)))
}

// encrypt seals a secret value for storage. Nothing outside this file writes to the SECRET table's
// value column, so this is the only door in.
func (s *service) encrypt(ctx context.Context, value string) (string, *common.ServiceError) {
	if s.crypto == nil {
		return "", s.unexpected(ctx, "secret encryption is unavailable",
			fmt.Errorf("config crypto provider not initialized"))
	}
	encrypted, err := s.crypto.Encrypt(ctx, []byte(value))
	if err != nil {
		return "", s.unexpected(ctx, "failed to encrypt secret", err)
	}
	return string(encrypted), nil
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
