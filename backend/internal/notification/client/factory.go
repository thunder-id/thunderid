// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/notification/common"
)

// ClientFactoryInterface defines the interface for obtaining notification clients.
type ClientFactoryInterface interface {
	GetClient(ctx context.Context,
		sender common.NotificationSenderDTO) (NotificationClientInterface, *tidcommon.ServiceError)
}

// clientFactory is the implementation of ClientFactoryInterface.
type clientFactory struct{}

// newClientFactory returns a new instance of ClientFactoryInterface.
func newClientFactory() ClientFactoryInterface {
	return &clientFactory{}
}

// GetClient returns the notification client for the given sender.
func (p *clientFactory) GetClient(ctx context.Context, sender common.NotificationSenderDTO) (
	NotificationClientInterface, *tidcommon.ServiceError) {
	var _client NotificationClientInterface
	var err error
	// Dispatch on the sender type first, so a provider name can never resolve to a client
	// of the wrong channel.
	switch sender.Type {
	case common.NotificationSenderTypeMessage:
		switch sender.Provider {
		case common.NotificationProviderTypeVonage:
			_client, err = newVonageClient(ctx, sender)
		case common.NotificationProviderTypeTwilio:
			_client, err = newTwilioClient(ctx, sender)
		case common.NotificationProviderTypeCustom:
			_client, err = newCustomClient(ctx, sender)
		default:
			return nil, &ErrorInvalidProvider
		}
	case common.NotificationSenderTypeEmail:
		switch sender.Provider {
		case common.NotificationProviderTypeSMTP:
			_client, err = newSMTPEmailClient(ctx, sender)
		default:
			return nil, &ErrorInvalidProvider
		}
	default:
		return nil, &ErrorInvalidProvider
	}

	if err != nil {
		return nil, &tidcommon.InternalServerError
	}

	return _client, nil
}
