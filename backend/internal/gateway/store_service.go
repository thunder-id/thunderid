// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/variablestore"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// StoreServiceInterface manages a gateway's variables and secrets through this plane, presenting
// the gateway's key. A call the gateway refuses is answered with its refusal, so the store answers
// the same whether it is called directly or through this plane.
type StoreServiceInterface interface {
	ListVariables(ctx context.Context, gatewayID string,
		query StoreListQuery) (*StoreAnswer[variablestore.VariableListResponse], *tidcommon.ServiceError)
	GetVariable(ctx context.Context, gatewayID, name string) (*StoreAnswer[variablestore.Variable],
		*tidcommon.ServiceError)
	CreateVariable(ctx context.Context, gatewayID string,
		req variablestore.VariableRequest) (*StoreAnswer[variablestore.Variable], *tidcommon.ServiceError)
	SetVariable(ctx context.Context, gatewayID, name string,
		req variablestore.VariableUpdateRequest) (*StoreAnswer[variablestore.Variable], *tidcommon.ServiceError)
	DeleteVariable(ctx context.Context, gatewayID, name string) (*StoreAnswer[struct{}], *tidcommon.ServiceError)

	ListSecrets(ctx context.Context, gatewayID string,
		query StoreListQuery) (*StoreAnswer[variablestore.SecretListResponse], *tidcommon.ServiceError)
	GetSecret(ctx context.Context, gatewayID, name string) (*StoreAnswer[variablestore.Secret],
		*tidcommon.ServiceError)
	CreateSecret(ctx context.Context, gatewayID string,
		req variablestore.SecretRequest) (*StoreAnswer[variablestore.Secret], *tidcommon.ServiceError)
	SetSecret(ctx context.Context, gatewayID, name string,
		req variablestore.SecretUpdateRequest) (*StoreAnswer[variablestore.Secret], *tidcommon.ServiceError)
	DeleteSecret(ctx context.Context, gatewayID, name string) (*StoreAnswer[struct{}], *tidcommon.ServiceError)
}

// StoreAnswer is a gateway's answer to a call to its store: what it returned, or its refusal.
type StoreAnswer[T any] struct {
	Value *T
	// Created is set when a write created the value rather than replacing it.
	Created bool
	Refusal *StoreRefusal
}

type storeService struct {
	gateways storeInterface
	client   gatewayStoreClientInterface
	logger   *log.Logger
}

func newStoreService(gateways storeInterface, client gatewayStoreClientInterface) StoreServiceInterface {
	return &storeService{
		gateways: gateways,
		client:   client,
		logger:   log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GatewayStoreService")),
	}
}

func (s *storeService) ListVariables(ctx context.Context, gatewayID string,
	query StoreListQuery) (*StoreAnswer[variablestore.VariableListResponse], *tidcommon.ServiceError) {
	return callGatewayStore(ctx, s, gatewayID,
		func(gw *Gateway, key string) (*variablestore.VariableListResponse, bool, error) {
			listing, err := s.client.ListVariables(ctx, gw, key, query)
			return listing, false, err
		})
}

func (s *storeService) GetVariable(ctx context.Context, gatewayID,
	name string) (*StoreAnswer[variablestore.Variable], *tidcommon.ServiceError) {
	return callGatewayStore(ctx, s, gatewayID, func(gw *Gateway, key string) (*variablestore.Variable, bool, error) {
		variable, err := s.client.GetVariable(ctx, gw, key, name)
		return variable, false, err
	})
}

func (s *storeService) CreateVariable(ctx context.Context, gatewayID string,
	req variablestore.VariableRequest) (*StoreAnswer[variablestore.Variable], *tidcommon.ServiceError) {
	return callGatewayStore(ctx, s, gatewayID, func(gw *Gateway, key string) (*variablestore.Variable, bool, error) {
		variable, err := s.client.CreateVariable(ctx, gw, key, req)
		return variable, true, err
	})
}

func (s *storeService) SetVariable(ctx context.Context, gatewayID, name string,
	req variablestore.VariableUpdateRequest) (*StoreAnswer[variablestore.Variable], *tidcommon.ServiceError) {
	return callGatewayStore(ctx, s, gatewayID, func(gw *Gateway, key string) (*variablestore.Variable, bool, error) {
		return s.client.SetVariable(ctx, gw, key, name, req)
	})
}

func (s *storeService) DeleteVariable(ctx context.Context, gatewayID,
	name string) (*StoreAnswer[struct{}], *tidcommon.ServiceError) {
	return callGatewayStore(ctx, s, gatewayID, func(gw *Gateway, key string) (*struct{}, bool, error) {
		return nil, false, s.client.DeleteVariable(ctx, gw, key, name)
	})
}

func (s *storeService) ListSecrets(ctx context.Context, gatewayID string,
	query StoreListQuery) (*StoreAnswer[variablestore.SecretListResponse], *tidcommon.ServiceError) {
	return callGatewayStore(ctx, s, gatewayID,
		func(gw *Gateway, key string) (*variablestore.SecretListResponse, bool, error) {
			listing, err := s.client.ListSecrets(ctx, gw, key, query)
			return listing, false, err
		})
}

func (s *storeService) GetSecret(ctx context.Context, gatewayID,
	name string) (*StoreAnswer[variablestore.Secret], *tidcommon.ServiceError) {
	return callGatewayStore(ctx, s, gatewayID, func(gw *Gateway, key string) (*variablestore.Secret, bool, error) {
		secret, err := s.client.GetSecret(ctx, gw, key, name)
		return secret, false, err
	})
}

func (s *storeService) CreateSecret(ctx context.Context, gatewayID string,
	req variablestore.SecretRequest) (*StoreAnswer[variablestore.Secret], *tidcommon.ServiceError) {
	return callGatewayStore(ctx, s, gatewayID, func(gw *Gateway, key string) (*variablestore.Secret, bool, error) {
		secret, err := s.client.CreateSecret(ctx, gw, key, req)
		return secret, true, err
	})
}

func (s *storeService) SetSecret(ctx context.Context, gatewayID, name string,
	req variablestore.SecretUpdateRequest) (*StoreAnswer[variablestore.Secret], *tidcommon.ServiceError) {
	return callGatewayStore(ctx, s, gatewayID, func(gw *Gateway, key string) (*variablestore.Secret, bool, error) {
		return s.client.SetSecret(ctx, gw, key, name, req)
	})
}

func (s *storeService) DeleteSecret(ctx context.Context, gatewayID,
	name string) (*StoreAnswer[struct{}], *tidcommon.ServiceError) {
	return callGatewayStore(ctx, s, gatewayID, func(gw *Gateway, key string) (*struct{}, bool, error) {
		return nil, false, s.client.DeleteSecret(ctx, gw, key, name)
	})
}

// callGatewayStore reads the gateway, opens its key and makes one call to its store. A gateway that
// refused the call with an error of its own is answered with that refusal; one that could not be
// reached, or answered with something other than an error, is reported unreachable.
func callGatewayStore[T any](ctx context.Context, s *storeService, gatewayID string,
	call func(gw *Gateway, key string) (*T, bool, error)) (*StoreAnswer[T], *tidcommon.ServiceError) {
	gw, svcErr := findGateway(ctx, s.gateways, s.logger, gatewayID)
	if svcErr != nil {
		return nil, svcErr
	}
	key, svcErr := reveal(ctx, s.logger, gw.Key)
	if svcErr != nil {
		return nil, svcErr
	}
	value, created, err := call(gw, key)
	if err != nil {
		var refusal *StoreRefusal
		if errors.As(err, &refusal) && json.Valid(refusal.Body) {
			return &StoreAnswer[T]{Refusal: refusal}, nil
		}
		s.logger.Error(ctx, "Failed to reach a gateway's store", log.String("gatewayId", gw.ID), log.Error(err))
		return nil, &ErrorGatewayUnreachable
	}
	return &StoreAnswer[T]{Value: value, Created: created}, nil
}
