/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package notification

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/notification/common"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

type notificationFileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// Create implements declarativeresource.Storer interface for resource loader
func (f *notificationFileBasedStore) Create(id string, data interface{}) error {
	sender := data.(*common.NotificationSenderDTO)
	return f.createSender(context.Background(), *sender)
}

// createSender implements notificationStoreInterface.
func (f *notificationFileBasedStore) createSender(ctx context.Context, sender common.NotificationSenderDTO) error {
	return f.GenericFileBasedStore.Create(sender.ID, &sender)
}

// deleteSender implements notificationStoreInterface.
func (f *notificationFileBasedStore) deleteSender(ctx context.Context, id string) error {
	return errors.New("deleteSender is not supported in file-based store")
}

// getSenderByID implements notificationStoreInterface.
func (f *notificationFileBasedStore) getSenderByID(
	ctx context.Context, id string) (*common.NotificationSenderDTO, error) {
	data, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		return nil, err
	}
	sender, ok := data.(*common.NotificationSenderDTO)
	if !ok {
		declarativeresource.LogTypeAssertionError("notification sender", id)
		return nil, errors.New("notification sender data corrupted")
	}
	return sender, nil
}

// getSenderByName implements notificationStoreInterface.
func (f *notificationFileBasedStore) getSenderByName(
	ctx context.Context, name string) (*common.NotificationSenderDTO, error) {
	data, err := f.GenericFileBasedStore.GetByField(name, func(d interface{}) string {
		return d.(*common.NotificationSenderDTO).Name
	})
	if err != nil {
		return nil, nil // Return nil for not found to match original behavior
	}
	return data.(*common.NotificationSenderDTO), nil
}

// listSenders implements notificationStoreInterface.
func (f *notificationFileBasedStore) listSenders(ctx context.Context) ([]common.NotificationSenderDTO, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	senderList := make([]common.NotificationSenderDTO, 0)
	for _, item := range list {
		if sender, ok := item.Data.(*common.NotificationSenderDTO); ok {
			senderList = append(senderList, *sender)
		}
	}
	return senderList, nil
}

// updateSender implements notificationStoreInterface.
func (f *notificationFileBasedStore) updateSender(
	ctx context.Context, id string, sender common.NotificationSenderDTO) error {
	return errors.New("updateSender is not supported in file-based store")
}

// newNotificationFileBasedStore creates a new instance of a file-based store.
func newNotificationFileBasedStore() (notificationStoreInterface, transaction.Transactioner) {
	genericStore := declarativeresource.NewGenericFileBasedStore(entity.KeyTypeNotificationSender)
	return &notificationFileBasedStore{
		GenericFileBasedStore: genericStore,
	}, transaction.NewNoOpTransactioner()
}
