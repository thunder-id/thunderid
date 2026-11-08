// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"errors"

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

// GetTemplateByHandle checks the file store first so a declared template wins on a duplicate handle,
// matching the merged list; it then falls back to the database store.
func (c *compositeStore) GetTemplateByHandle(ctx context.Context, channel ChannelType, handle string) (
	templateDAO, error) {
	return declarativeresource.CompositeGetHelper(
		func() (templateDAO, error) { return c.fileStore.GetTemplateByHandle(ctx, channel, handle) },
		func() (templateDAO, error) { return c.dbStore.GetTemplateByHandle(ctx, channel, handle) },
		errTemplateNotFound,
	)
}

// ListTemplates returns a page over the merged set, rejecting a source that exceeds the composite cap.
func (c *compositeStore) ListTemplates(ctx context.Context, channel ChannelType, limit, offset int) (
	[]templateDAO, error) {
	merged, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		cappedCount(ctx, channel, c.fileStore.CountTemplates),
		cappedCount(ctx, channel, c.dbStore.CountTemplates),
		func(count int) ([]templateDAO, error) { return c.fileStore.ListTemplates(ctx, channel, count, 0) },
		func(count int) ([]templateDAO, error) { return c.dbStore.ListTemplates(ctx, channel, count, 0) },
		mergeTemplatesByHandle,
		limit,
		offset,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, errResultLimitExceededInCompositeMode
	}
	return merged, nil
}

// CountTemplates returns the merged count, rejecting a source that exceeds the composite cap.
func (c *compositeStore) CountTemplates(ctx context.Context, channel ChannelType) (int, error) {
	merged, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		cappedCount(ctx, channel, c.fileStore.CountTemplates),
		cappedCount(ctx, channel, c.dbStore.CountTemplates),
		func(count int) ([]templateDAO, error) { return c.fileStore.ListTemplates(ctx, channel, count, 0) },
		func(count int) ([]templateDAO, error) { return c.dbStore.ListTemplates(ctx, channel, count, 0) },
		mergeTemplatesByHandle,
		serverconst.MaxCompositeStoreRecords+1,
		0,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return 0, err
	}
	if limitExceeded {
		return 0, errResultLimitExceededInCompositeMode
	}
	return len(merged), nil
}

// ListTemplatesByOUID returns a page over the merged set of templates an organization unit owns,
// rejecting a source that exceeds the composite cap.
func (c *compositeStore) ListTemplatesByOUID(ctx context.Context, channel ChannelType, ouID string,
	limit, offset int) ([]templateDAO, error) {
	return mergedPage(c.ouSources(ctx, channel, ouID), limit, offset)
}

// CountTemplatesByOUID returns the merged count of templates an organization unit owns, rejecting a
// source that exceeds the composite cap.
func (c *compositeStore) CountTemplatesByOUID(ctx context.Context, channel ChannelType, ouID string) (
	int, error) {
	return mergedCount(c.ouSources(ctx, channel, ouID))
}

// ListTemplatesByProject returns a page over the merged set of templates in a project, rejecting a
// source that exceeds the composite cap.
func (c *compositeStore) ListTemplatesByProject(ctx context.Context, channel ChannelType, projectID string,
	limit, offset int) ([]templateDAO, error) {
	return mergedPage(c.projectSources(ctx, channel, projectID), limit, offset)
}

// CountTemplatesByProject returns the merged count of templates in a project, rejecting a source that
// exceeds the composite cap.
func (c *compositeStore) CountTemplatesByProject(ctx context.Context, channel ChannelType, projectID string) (
	int, error) {
	return mergedCount(c.projectSources(ctx, channel, projectID))
}

// templateSources reads one filtered set of templates from the file and database stores.
type templateSources struct {
	countFile, countDB func() (int, error)
	listFile, listDB   func(count int) ([]templateDAO, error)
}

// ouSources reads the templates of a channel an organization unit owns.
func (c *compositeStore) ouSources(ctx context.Context, channel ChannelType, ouID string) templateSources {
	return templateSources{
		countFile: func() (int, error) { return c.fileStore.CountTemplatesByOUID(ctx, channel, ouID) },
		countDB:   func() (int, error) { return c.dbStore.CountTemplatesByOUID(ctx, channel, ouID) },
		listFile: func(count int) ([]templateDAO, error) {
			return c.fileStore.ListTemplatesByOUID(ctx, channel, ouID, count, 0)
		},
		listDB: func(count int) ([]templateDAO, error) {
			return c.dbStore.ListTemplatesByOUID(ctx, channel, ouID, count, 0)
		},
	}
}

// projectSources reads the templates of a channel in a project.
func (c *compositeStore) projectSources(ctx context.Context, channel ChannelType,
	projectID string) templateSources {
	return templateSources{
		countFile: func() (int, error) { return c.fileStore.CountTemplatesByProject(ctx, channel, projectID) },
		countDB:   func() (int, error) { return c.dbStore.CountTemplatesByProject(ctx, channel, projectID) },
		listFile: func(count int) ([]templateDAO, error) {
			return c.fileStore.ListTemplatesByProject(ctx, channel, projectID, count, 0)
		},
		listDB: func(count int) ([]templateDAO, error) {
			return c.dbStore.ListTemplatesByProject(ctx, channel, projectID, count, 0)
		},
	}
}

// mergedPage returns a page over the merged templates of the sources, rejecting a source that exceeds the
// composite cap.
func mergedPage(sources templateSources, limit, offset int) ([]templateDAO, error) {
	merged, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		sources.countFile, sources.countDB, sources.listFile, sources.listDB,
		mergeTemplatesByHandle, limit, offset, serverconst.MaxCompositeStoreRecords)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, errResultLimitExceededInCompositeMode
	}
	return merged, nil
}

// mergedCount returns the merged count of the templates of the sources, rejecting a source that exceeds the
// composite cap.
func mergedCount(sources templateSources) (int, error) {
	merged, err := mergedPage(sources, serverconst.MaxCompositeStoreRecords+1, 0)
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

// DeleteTemplate refuses a declared template and otherwise deletes from the DB. Only a not-found
// file-store lookup falls through to the DB delete; any other error is returned so a transient
// file-store failure is not misread as "not declared".
func (c *compositeStore) DeleteTemplate(ctx context.Context, channel ChannelType, id string) error {
	_, err := c.fileStore.GetTemplate(ctx, channel, id)
	if err == nil {
		return errDeclarativeTemplate
	}
	if !errors.Is(err, errTemplateNotFound) {
		return err
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

// mergeTemplatesByHandle concatenates declared and DB templates, deduped by handle with the
// declared template winning. It preserves each store's own order (declared first, then DB) without
// re-sorting, so the paged window produced by CompositeMergeListHelperWithLimit stays consistent
// across pages.
func mergeTemplatesByHandle(declared, dbItems []templateDAO) []templateDAO {
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
	return merged
}

// cappedCount wraps a channel count, capping it at MaxCompositeStoreRecords.
func cappedCount(ctx context.Context, channel ChannelType,
	fn func(context.Context, ChannelType) (int, error)) func() (int, error) {
	return func() (int, error) {
		count, err := fn(ctx, channel)
		if err != nil {
			return 0, err
		}
		return min(count, serverconst.MaxCompositeStoreRecords), nil
	}
}
