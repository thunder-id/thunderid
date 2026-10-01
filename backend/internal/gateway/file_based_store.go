// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"strings"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

// errDeclarativeGateway is returned when something tries to write a gateway that came from a file.
//
// A declared gateway is whatever the file says on the next start, so a write that appeared to
// succeed would be undone by a restart. Refusing is the honest answer.
var errDeclarativeGateway = errors.New("a gateway declared in a file cannot be changed through the API")

// gatewayFileStore holds the gateways declared in files, in memory, for the life of the process.
//
// Nothing here reaches the database. The files are read on every start and are the whole truth
// about what they declare, so persisting them would leave rows that outlive the file that made
// them and are indistinguishable from a registration someone made through the API.
type gatewayFileStore struct {
	*declarativeresource.GenericFileBasedStore
}

func newFileStore() *gatewayFileStore {
	return &gatewayFileStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStore(entity.KeyTypeGateway),
	}
}

// put records a declared gateway. It is how the declarative loader fills this store, and is not
// part of storeInterface: a file is the only thing that writes here.
func (f *gatewayFileStore) put(gw Gateway) error {
	stored := gw
	return f.GenericFileBasedStore.Create(gw.ID, &stored)
}

func (f *gatewayFileStore) List(_ context.Context) ([]Gateway, error) {
	entries, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	gateways := make([]Gateway, 0, len(entries))
	for _, item := range entries {
		gw, ok := item.Data.(*Gateway)
		if !ok {
			declarativeresource.LogTypeAssertionError(resourceTypeGateway, "")
			continue
		}
		gateways = append(gateways, *gw)
	}
	return gateways, nil
}

func (f *gatewayFileStore) GetByID(_ context.Context, id string) (*Gateway, error) {
	data, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		// Absent rather than broken: the composite store asks both sources and a miss here is
		// expected for anything registered through the API.
		return nil, nil //nolint:nilnil // absent is not an error, matching the database store
	}
	gw, ok := data.(*Gateway)
	if !ok {
		declarativeresource.LogTypeAssertionError(resourceTypeGateway, id)
		return nil, errors.New("declared gateway data corrupted")
	}
	stored := *gw
	return &stored, nil
}

func (f *gatewayFileStore) GetByName(ctx context.Context, name string) (*Gateway, error) {
	return f.findBy(ctx, func(gw Gateway) bool { return gw.Name == name })
}

// GetByBaseURL matches the way the database store does, on the normalized address, so a declared
// gateway and a registered one cannot both claim the same data plane under different spellings.
func (f *gatewayFileStore) GetByBaseURL(ctx context.Context, baseURL string) (*Gateway, error) {
	wanted, ok := normalizeBaseURL(baseURL)
	if !ok {
		wanted = strings.TrimSpace(baseURL)
	}
	return f.findBy(ctx, func(gw Gateway) bool {
		stored, ok := normalizeBaseURL(gw.BaseURL)
		if !ok {
			stored = strings.TrimSpace(gw.BaseURL)
		}
		return stored == wanted
	})
}

func (f *gatewayFileStore) Count(ctx context.Context) (int, error) {
	gateways, err := f.List(ctx)
	if err != nil {
		return 0, err
	}
	return len(gateways), nil
}

// Create, Update and Delete exist to satisfy storeInterface and always refuse. A declarative store
// is written by its files and by nothing else.
func (f *gatewayFileStore) Create(_ context.Context, _ *Gateway, _ int) (*Gateway, error) {
	return nil, errDeclarativeGateway
}

func (f *gatewayFileStore) Update(_ context.Context, _ *Gateway) error {
	return errDeclarativeGateway
}

func (f *gatewayFileStore) Delete(_ context.Context, _ string) error {
	return errDeclarativeGateway
}

// findBy returns the first declared gateway the predicate accepts, or nil when none does.
func (f *gatewayFileStore) findBy(ctx context.Context, match func(Gateway) bool) (*Gateway, error) {
	gateways, err := f.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range gateways {
		if match(gateways[i]) {
			found := gateways[i]
			return &found, nil
		}
	}
	return nil, nil //nolint:nilnil // absent is not an error, matching the database store
}
