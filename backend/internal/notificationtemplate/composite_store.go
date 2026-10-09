// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"sort"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// compositeStore merges file-declared and DB templates. Reads merge; writes hit the DB; declared
// templates are read-only.
type compositeStore struct {
	fileStore *templateFileBasedStore
	dbStore   notificationTemplateStoreInterface
}

// newCompositeStore combines a file store and a DB store.
func newCompositeStore(fileStore *templateFileBasedStore,
	dbStore notificationTemplateStoreInterface) notificationTemplateStoreInterface {
	return &compositeStore{fileStore: fileStore, dbStore: dbStore}
}

// mergedByChannel returns a channel's templates deduped by handle (declared wins), ordered by handle.
func (c *compositeStore) mergedByChannel(ctx context.Context, channel ChannelType) ([]templateDAO, error) {
	declared, err := c.fileStore.ListTemplates(ctx, channel, serverconst.MaxCompositeStoreRecords, 0)
	if err != nil {
		return nil, err
	}
	dbItems, err := c.dbStore.ListTemplates(ctx, channel, serverconst.MaxCompositeStoreRecords, 0)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(declared))
	merged := make([]templateDAO, 0, len(declared)+len(dbItems))
	for _, t := range declared {
		seen[t.Handle] = true
		merged = append(merged, t)
	}
	for _, t := range dbItems {
		if !seen[t.Handle] {
			merged = append(merged, t)
		}
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Handle < merged[j].Handle })
	return merged, nil
}

// CreateTemplate writes to the DB. The service's handle check already ran against both stores.
func (c *compositeStore) CreateTemplate(ctx context.Context, t templateDAO) error {
	return c.dbStore.CreateTemplate(ctx, t)
}

// GetTemplate checks the database store first, then falls back to the file store.
func (c *compositeStore) GetTemplate(ctx context.Context, channel ChannelType, id string) (templateDAO, error) {
	return declarativeresource.CompositeGetHelper(
		func() (templateDAO, error) { return c.dbStore.GetTemplate(ctx, channel, id) },
		func() (templateDAO, error) { return c.fileStore.GetTemplate(ctx, channel, id) },
		errTemplateNotFound,
	)
}

// GetTemplateByHandle checks the database store first, then falls back to the file store.
func (c *compositeStore) GetTemplateByHandle(ctx context.Context, channel ChannelType, handle string) (
	templateDAO, error) {
	return declarativeresource.CompositeGetHelper(
		func() (templateDAO, error) { return c.dbStore.GetTemplateByHandle(ctx, channel, handle) },
		func() (templateDAO, error) { return c.fileStore.GetTemplateByHandle(ctx, channel, handle) },
		errTemplateNotFound,
	)
}

// ListTemplates returns a page over the merged set.
func (c *compositeStore) ListTemplates(ctx context.Context, channel ChannelType, limit, offset int) (
	[]templateDAO, error) {
	merged, err := c.mergedByChannel(ctx, channel)
	if err != nil {
		return nil, err
	}
	if offset >= len(merged) {
		return []templateDAO{}, nil
	}
	end := offset + limit
	if end > len(merged) {
		end = len(merged)
	}
	return merged[offset:end], nil
}

// CountTemplates returns the merged count.
func (c *compositeStore) CountTemplates(ctx context.Context, channel ChannelType) (int, error) {
	merged, err := c.mergedByChannel(ctx, channel)
	if err != nil {
		return 0, err
	}
	return len(merged), nil
}

// UpdateTemplate refuses a declared template and otherwise writes to the DB.
func (c *compositeStore) UpdateTemplate(ctx context.Context, t templateDAO) error {
	declared, err := c.fileStore.IsHandleExists(ctx, t.Channel, t.Handle)
	if err != nil {
		return err
	}
	if declared {
		return errDeclarativeTemplate
	}
	return c.dbStore.UpdateTemplate(ctx, t)
}

// DeleteTemplate refuses a declared template and otherwise deletes from the DB.
func (c *compositeStore) DeleteTemplate(ctx context.Context, channel ChannelType, id string) error {
	if _, err := c.fileStore.GetTemplate(ctx, channel, id); err == nil {
		return errDeclarativeTemplate
	}
	return c.dbStore.DeleteTemplate(ctx, channel, id)
}

// IsHandleExists checks both stores.
func (c *compositeStore) IsHandleExists(ctx context.Context, channel ChannelType, handle string) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) { return c.fileStore.IsHandleExists(ctx, channel, handle) },
		func() (bool, error) { return c.dbStore.IsHandleExists(ctx, channel, handle) },
	)
}
