// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueryClient} from '@tanstack/react-query';
import {PageLoadingAnimation, QueryErrorNotice, ResourceAvatar} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {useThunderID} from '@thunderid/react';
import {Box, Typography, CircularProgress, TreeView, useTheme} from '@wso2/oxygen-ui';
import {useState, useCallback, useEffect, useRef, useMemo, type JSX, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import fetchChildOrganizationUnits from '../api/fetchChildOrganizationUnits';
import fetchOrganizationUnits from '../api/fetchOrganizationUnits';
import useGetChildOrganizationUnits from '../api/useGetChildOrganizationUnits';
import useGetOrganizationUnit from '../api/useGetOrganizationUnit';
import useGetOrganizationUnits from '../api/useGetOrganizationUnits';
import OrganizationUnitQueryKeys from '../constants/organization-unit-query-keys';
import OrganizationUnitTreeConstants from '../constants/organization-unit-tree-constants';
import type {OrganizationUnitTreeItem} from '../models/organization-unit-tree';
import type {OrganizationUnitListResponse} from '../models/responses';
import appendTreeItemChildren from '../utils/appendTreeItemChildren';
import buildItemMap from '../utils/buildItemMap';
import buildTreeItems from '../utils/buildTreeItems';
import updateTreeItemChildren from '../utils/updateTreeItemChildren';

function PickerLoadingIcon(): JSX.Element {
  return <CircularProgress size={16} />;
}

interface PickerTreeItemProps extends TreeView.TreeItemProps {
  itemId: string;
  itemMap?: Map<string, OrganizationUnitTreeItem>;
  loadingItems?: Set<string>;
  loadMoreLoadingItems?: Set<string>;
  onLoadMore?: (parentId: string) => void;
  onItemActivate?: (itemId: string) => void;
  spacious?: boolean;
}

function PickerTreeItem(allProps: PickerTreeItemProps): JSX.Element {
  const {
    itemMap: itemMapProp,
    loadingItems: loadingItemsProp,
    loadMoreLoadingItems: loadMoreLoadingItemsProp,
    onLoadMore: onLoadMoreProp,
    onItemActivate: onItemActivateProp,
    spacious = false,
    itemId,
    label,
    ...restProps
  } = allProps;
  const treeItemProps = {itemId, label, ...restProps};
  const theme = useTheme();
  const {t} = useTranslation();
  const labelStr = typeof label === 'string' ? label : '';
  const itemData = itemMapProp?.get(itemId);
  const isLoadMoreItem = itemId.endsWith(OrganizationUnitTreeConstants.LOAD_MORE_SUFFIX);
  const isEmptyPlaceholder = itemId.endsWith(OrganizationUnitTreeConstants.EMPTY_SUFFIX);
  const isLoadingPlaceholder =
    !isEmptyPlaceholder &&
    !isLoadMoreItem &&
    (itemData?.isPlaceholder ?? itemId.endsWith(OrganizationUnitTreeConstants.PLACEHOLDER_SUFFIX));
  const isItemLoading = loadingItemsProp?.has(itemId);

  if (isLoadMoreItem) {
    const parentId = itemId.replace(OrganizationUnitTreeConstants.LOAD_MORE_SUFFIX, '');
    const isLoadingMore = loadMoreLoadingItemsProp?.has(parentId);

    return (
      <TreeView.TreeItem
        {...treeItemProps}
        sx={{
          '& > .MuiTreeItem-content': {
            border: '1px dashed',
            borderColor: theme.vars?.palette.divider,
            borderRadius: 0.5,
            backgroundColor: 'transparent !important',
            cursor: isLoadingMore ? 'default' : 'pointer',
            transition: 'all 0.15s ease-in-out',
            '&:hover': {
              borderColor: isLoadingMore ? undefined : theme.vars?.palette.primary.main,
            },
          },
        }}
        label={
          <Box
            role="button"
            tabIndex={0}
            onClick={(e) => {
              e.stopPropagation();
              if (!isLoadingMore) {
                onLoadMoreProp?.(parentId);
              }
            }}
            onKeyDown={(e) => {
              if ((e.key === 'Enter' || e.key === ' ') && !isLoadingMore) {
                e.preventDefault();
                e.stopPropagation();
                onLoadMoreProp?.(parentId);
              }
            }}
            sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 1, py: 0.25}}
          >
            {isLoadingMore ? (
              <>
                <CircularProgress size={14} />
                <Typography variant="caption" color="text.secondary">
                  {t('common:status.loading')}
                </Typography>
              </>
            ) : (
              <Typography variant="caption" color="primary" sx={{fontWeight: 500}}>
                {t('organizationUnits:listing.treeView.loadMore')}
              </Typography>
            )}
          </Box>
        }
      />
    );
  }

  if (isEmptyPlaceholder) {
    return (
      <TreeView.TreeItem
        {...treeItemProps}
        sx={{
          '& > .MuiTreeItem-content': {
            border: 'none !important',
            backgroundColor: 'transparent !important',
          },
        }}
        label={
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{fontStyle: spacious ? 'normal' : 'italic', pl: spacious ? 0 : 1}}
          >
            {labelStr}
          </Typography>
        }
      />
    );
  }

  if (isLoadingPlaceholder) {
    return (
      <TreeView.TreeItem
        {...treeItemProps}
        sx={{
          '& > .MuiTreeItem-content': {
            border: 'none !important',
            backgroundColor: 'transparent !important',
          },
        }}
        label={
          <Box sx={{display: 'flex', alignItems: 'center', gap: 1}}>
            <CircularProgress size={16} />
            <Typography variant="caption" color="text.secondary" sx={{fontStyle: 'italic'}}>
              {t('common:status.loading')}
            </Typography>
          </Box>
        }
      />
    );
  }

  return (
    <TreeView.TreeItem
      {...treeItemProps}
      {...(isItemLoading ? {slots: {collapseIcon: PickerLoadingIcon, expandIcon: PickerLoadingIcon}} : {})}
      label={
        <Box
          role={onItemActivateProp ? 'button' : undefined}
          tabIndex={onItemActivateProp ? 0 : undefined}
          onClick={(event) => {
            if (onItemActivateProp) {
              event.stopPropagation();
              onItemActivateProp(itemId);
            }
          }}
          onKeyDown={(event) => {
            if (onItemActivateProp && (event.key === 'Enter' || event.key === ' ')) {
              event.preventDefault();
              event.stopPropagation();
              onItemActivateProp(itemId);
            }
          }}
          sx={{display: 'flex', alignItems: 'center', gap: spacious ? 2 : 1.5}}
        >
          <ResourceAvatar
            variant="rounded"
            value={itemData?.logoUrl}
            size={spacious ? 40 : 30}
            fallback={OrganizationUnitTreeConstants.DEFAULT_AVATAR}
          />
          <Box sx={{flexGrow: 1, minWidth: 0}}>
            <Typography variant={spacious ? 'body1' : 'body2'} sx={{fontWeight: 600, lineHeight: 1.3}}>
              {labelStr}
            </Typography>
            {itemData?.handle && (
              <Typography variant="caption" color="text.secondary" sx={{lineHeight: 1.2, display: 'block'}}>
                {itemData.handle}
              </Typography>
            )}
          </Box>
        </Box>
      }
    />
  );
}

interface OrganizationUnitTreePickerProps {
  id?: string;
  value: string;
  onChange: (ouId: string) => void;
  /**
   * Handles explicit activation of an OU row without treating expansion clicks as selection.
   */
  onItemActivate?: (ouId: string) => void;
  error?: boolean;
  helperText?: string;
  rootOuId?: string;
  /**
   * Hides the configured root OU and renders its direct children as the first visible level.
   */
  hideRoot?: boolean;
  maxHeight?: number;
  /**
   * Renders larger rows with softer, borderless cards and no dashed nesting guide, for standalone
   * full-screen pickers. Defaults to the compact look used inline in forms.
   */
  spacious?: boolean;
  /**
   * Auto-picks the first root organization unit once loaded, if nothing is selected yet. Only
   * applies in global-root mode (i.e. when `rootOuId` isn't set).
   */
  autoSelectFirst?: boolean;
}

export default function OrganizationUnitTreePicker({
  id = undefined,
  value,
  onChange,
  onItemActivate = undefined,
  error = false,
  helperText = '',
  rootOuId = undefined,
  hideRoot = false,
  maxHeight = 300,
  spacious = false,
  autoSelectFirst = false,
}: OrganizationUnitTreePickerProps): JSX.Element {
  const theme = useTheme();
  const {t} = useTranslation();
  const logger = useLogger('OrganizationUnitTreePicker');
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {
    data,
    isLoading,
    error: rootListError,
    refetch: refetchRootList,
  } = useGetOrganizationUnits(undefined, !rootOuId);

  useEffect((): void => {
    if (!autoSelectFirst || rootOuId || value) return;
    const firstRootOuId = data?.organizationUnits[0]?.id;
    if (firstRootOuId) onChange(firstRootOuId);
  }, [autoSelectFirst, rootOuId, value, data, onChange]);
  const {
    data: rootOuData,
    isLoading: isRootOuLoading,
    error: rootOuError,
    refetch: refetchRootOu,
  } = useGetOrganizationUnit(rootOuId, !hideRoot);
  const {
    data: rootOuChildrenData,
    isLoading: isRootOuChildrenLoading,
    error: rootOuChildrenError,
    refetch: refetchRootOuChildren,
  } = useGetChildOrganizationUnits(rootOuId);

  const [treeItems, setTreeItems] = useState<OrganizationUnitTreeItem[]>([]);
  const [expandedItems, setExpandedItems] = useState<string[]>([]);
  const [loadedItems, setLoadedItems] = useState<Set<string>>(new Set());
  const [loadingItems, setLoadingItems] = useState<Set<string>>(new Set());
  const [loadMoreLoadingItems, setLoadMoreLoadingItems] = useState<Set<string>>(new Set());
  const [childOffsets, setChildOffsets] = useState<Map<string, number>>(new Map());
  const [rootOffset, setRootOffset] = useState<number>(0);
  const [rootLoadMoreLoading, setRootLoadMoreLoading] = useState<boolean>(false);
  const rootLoadMoreLoadingRef = useRef<boolean>(false);
  rootLoadMoreLoadingRef.current = rootLoadMoreLoading;
  const loadingItemsRef = useRef<Set<string>>(loadingItems);
  loadingItemsRef.current = loadingItems;

  const itemMap = useMemo(() => buildItemMap(treeItems), [treeItems]);

  // Reset all tree state when rootOuId changes so stale data is never shown.
  useEffect(() => {
    setTreeItems([]);
    setExpandedItems([]);
    setLoadedItems(new Set());
    setLoadingItems(new Set());
    setLoadMoreLoadingItems(new Set());
    setChildOffsets(new Map());
    setRootOffset(0);
    setRootLoadMoreLoading(false);
  }, [rootOuId]);

  // Build root tree when data arrives (global root mode)
  useEffect(() => {
    if (rootOuId) return;
    if (data?.organizationUnits && data.organizationUnits.length > 0 && treeItems.length === 0) {
      const items = buildTreeItems(data.organizationUnits);

      if (data.organizationUnits.length < data.totalResults) {
        items.push({
          id: OrganizationUnitTreeConstants.ROOT_LOAD_MORE_ID,
          label: '',
          handle: '',
          isPlaceholder: true,
        });
      }

      setRootOffset(data.organizationUnits.length);
      setTreeItems(items);
    }
  }, [rootOuId, data, treeItems.length]);

  // Build root tree when data arrives (rooted mode).
  // The treeItems.length === 0 guard prevents re-initialization when dependencies
  // change (e.g. t reference), preserving user-expanded subtrees and loaded-more items.
  // The reset effect on rootOuId change clears treeItems to [], allowing this to rebuild.
  useEffect(() => {
    if (!rootOuId || (!hideRoot && !rootOuData) || !rootOuChildrenData || treeItems.length > 0) return;

    const childItems = buildTreeItems(rootOuChildrenData.organizationUnits);

    if (rootOuChildrenData.organizationUnits.length < rootOuChildrenData.totalResults) {
      childItems.push({
        id: `${rootOuId}${OrganizationUnitTreeConstants.LOAD_MORE_SUFFIX}`,
        label: '',
        handle: '',
        isPlaceholder: true,
      });
    }

    if (hideRoot) {
      const visibleItems: OrganizationUnitTreeItem[] =
        rootOuChildrenData.organizationUnits.length > 0
          ? childItems
          : [
              {
                id: `${rootOuId}${OrganizationUnitTreeConstants.EMPTY_SUFFIX}`,
                label: t('organizationUnits:listing.treeView.noChildren'),
                handle: '',
                isPlaceholder: true,
              },
            ];

      setChildOffsets((prev) => new Map(prev).set(rootOuId, rootOuChildrenData.organizationUnits.length));
      setTreeItems(visibleItems);

      return;
    }

    if (!rootOuData) return;

    // If no children, show the root OU as a leaf node
    const rootChildren: OrganizationUnitTreeItem[] =
      rootOuChildrenData.organizationUnits.length > 0
        ? childItems
        : [
            {
              id: `${rootOuId}${OrganizationUnitTreeConstants.EMPTY_SUFFIX}`,
              label: t('organizationUnits:listing.treeView.noChildren'),
              handle: '',
              isPlaceholder: true,
            },
          ];

    const rootItem: OrganizationUnitTreeItem = {
      id: rootOuData.id,
      label: rootOuData.name,
      handle: rootOuData.handle,
      description: rootOuData.description ?? undefined,
      logoUrl: rootOuData.logoUrl,
      children: rootChildren,
    };

    setChildOffsets((prev) => new Map(prev).set(rootOuId, rootOuChildrenData.organizationUnits.length));
    setLoadedItems((prev) => new Set(prev).add(rootOuId));
    setExpandedItems([rootOuId]);
    setTreeItems([rootItem]);
  }, [rootOuId, hideRoot, rootOuData, rootOuChildrenData, treeItems.length, t]);

  const fetchChildPage = useCallback(
    async (parentId: string, offset: number): Promise<OrganizationUnitListResponse> =>
      queryClient.fetchQuery<OrganizationUnitListResponse>({
        queryKey: [
          OrganizationUnitQueryKeys.CHILD_ORGANIZATION_UNITS,
          parentId,
          {limit: OrganizationUnitTreeConstants.PAGE_SIZE, offset},
        ],
        queryFn: async (): Promise<OrganizationUnitListResponse> =>
          fetchChildOrganizationUnits(http, getServerUrl(), parentId, {
            limit: OrganizationUnitTreeConstants.PAGE_SIZE,
            offset,
          }),
        staleTime: 0,
      }),
    [getServerUrl, queryClient, http],
  );

  const buildChildItems = useCallback(
    (parentId: string, result: OrganizationUnitListResponse, offset: number): OrganizationUnitTreeItem[] => {
      const childOUs = result.organizationUnits;

      if (childOUs.length === 0 && offset === 0) {
        return [
          {
            id: `${parentId}${OrganizationUnitTreeConstants.EMPTY_SUFFIX}`,
            label: t('organizationUnits:listing.treeView.noChildren'),
            handle: '',
            isPlaceholder: true,
          },
        ];
      }

      const items = buildTreeItems(childOUs);
      const loadedSoFar = offset + childOUs.length;

      if (loadedSoFar < result.totalResults) {
        items.push({
          id: `${parentId}${OrganizationUnitTreeConstants.LOAD_MORE_SUFFIX}`,
          label: '',
          handle: '',
          isPlaceholder: true,
        });
      }

      return items;
    },
    [t],
  );

  const fetchChildOUs = useCallback(
    async (parentId: string): Promise<void> => {
      if (loadingItemsRef.current.has(parentId)) return;

      setLoadingItems((prev) => new Set(prev).add(parentId));

      try {
        const result = await fetchChildPage(parentId, 0);
        const childItems = buildChildItems(parentId, result, 0);

        setChildOffsets((prev) => new Map(prev).set(parentId, result.organizationUnits.length));
        setTreeItems((prev) => updateTreeItemChildren(prev, parentId, childItems));
        setLoadedItems((prev) => new Set(prev).add(parentId));
        setExpandedItems((prev) => (prev.includes(parentId) ? prev : [...prev, parentId]));
      } catch (_error: unknown) {
        logger.error('Failed to load child organization units', {error: _error, parentId});
      } finally {
        setLoadingItems((prev) => {
          const next = new Set(prev);
          next.delete(parentId);

          return next;
        });
      }
    },
    [fetchChildPage, buildChildItems, logger],
  );

  const handleRootLoadMore = useCallback(async (): Promise<void> => {
    if (rootLoadMoreLoadingRef.current) return;

    setRootLoadMoreLoading(true);

    try {
      const result = await queryClient.fetchQuery<OrganizationUnitListResponse>({
        queryKey: [
          OrganizationUnitQueryKeys.ORGANIZATION_UNITS,
          {limit: OrganizationUnitTreeConstants.PAGE_SIZE, offset: rootOffset},
        ],
        queryFn: async (): Promise<OrganizationUnitListResponse> =>
          fetchOrganizationUnits(http, getServerUrl(), {
            limit: OrganizationUnitTreeConstants.PAGE_SIZE,
            offset: rootOffset,
          }),
        staleTime: 0,
      });

      const newItems = buildTreeItems(result.organizationUnits);
      const loadedSoFar = rootOffset + result.organizationUnits.length;

      if (loadedSoFar < result.totalResults) {
        newItems.push({
          id: OrganizationUnitTreeConstants.ROOT_LOAD_MORE_ID,
          label: '',
          handle: '',
          isPlaceholder: true,
        });
      }

      setRootOffset(loadedSoFar);
      setTreeItems((prev) => {
        const withoutLoadMore = prev.filter((item) => item.id !== OrganizationUnitTreeConstants.ROOT_LOAD_MORE_ID);

        return [...withoutLoadMore, ...newItems];
      });
    } catch (_error: unknown) {
      logger.error('Failed to load more root organization units', {error: _error});
    } finally {
      setRootLoadMoreLoading(false);
    }
  }, [rootOffset, getServerUrl, queryClient, http, logger]);

  const handleLoadMore = useCallback(
    async (parentId: string): Promise<void> => {
      if (parentId === OrganizationUnitTreeConstants.ROOT_PARENT_ID) {
        await handleRootLoadMore();

        return;
      }

      setLoadMoreLoadingItems((prev) => new Set(prev).add(parentId));

      try {
        const offset = childOffsets.get(parentId) ?? OrganizationUnitTreeConstants.PAGE_SIZE;
        const result = await fetchChildPage(parentId, offset);
        const newItems = buildChildItems(parentId, result, offset);

        setChildOffsets((prev) => new Map(prev).set(parentId, offset + result.organizationUnits.length));
        setTreeItems((prev) => {
          if (hideRoot && parentId === rootOuId) {
            const loadMoreId = `${parentId}${OrganizationUnitTreeConstants.LOAD_MORE_SUFFIX}`;
            const withoutLoadMore = prev.filter((item) => item.id !== loadMoreId);

            return [...withoutLoadMore, ...newItems];
          }

          return appendTreeItemChildren(prev, parentId, newItems);
        });
      } catch (_error: unknown) {
        logger.error('Failed to load more child organization units', {error: _error, parentId});
      } finally {
        setLoadMoreLoadingItems((prev) => {
          const next = new Set(prev);
          next.delete(parentId);

          return next;
        });
      }
    },
    [childOffsets, fetchChildPage, buildChildItems, hideRoot, rootOuId, logger, handleRootLoadMore],
  );

  const combinedLoadMoreLoadingItems = useMemo(() => {
    if (!rootLoadMoreLoading) return loadMoreLoadingItems;
    const combined = new Set(loadMoreLoadingItems);
    combined.add(OrganizationUnitTreeConstants.ROOT_PARENT_ID);

    return combined;
  }, [loadMoreLoadingItems, rootLoadMoreLoading]);

  const handleItemExpansionToggle = useCallback(
    (_event: SyntheticEvent | null, itemId: string, isExpanded: boolean) => {
      if (!isExpanded || loadedItems.has(itemId) || loadingItems.has(itemId)) {
        return;
      }

      fetchChildOUs(itemId).catch((_error: unknown) => {
        logger.error('Failed to load child organization units', {error: _error, parentId: itemId});
      });
    },
    [loadedItems, loadingItems, fetchChildOUs, logger],
  );

  const handleSelectedItemsChange = useCallback(
    (_event: SyntheticEvent | null, itemId: string | null) => {
      if (
        itemId &&
        !itemId.endsWith(OrganizationUnitTreeConstants.PLACEHOLDER_SUFFIX) &&
        !itemId.endsWith(OrganizationUnitTreeConstants.EMPTY_SUFFIX) &&
        !itemId.endsWith(OrganizationUnitTreeConstants.LOAD_MORE_SUFFIX)
      ) {
        onChange(itemId);
      }
    },
    [onChange],
  );

  const handleExpandedItemsChange = useCallback(
    (_event: SyntheticEvent | null, itemIds: string[]) => {
      const prevSet = new Set(expandedItems);
      const filtered = itemIds.filter((itemId) => prevSet.has(itemId) || loadedItems.has(itemId));
      setExpandedItems(filtered);
    },
    [expandedItems, loadedItems],
  );

  const handleLoadMoreWithErrorLogging = useCallback(
    (parentId: string) => {
      handleLoadMore(parentId).catch((_error: unknown) => {
        logger.error('Failed to load more child organization units', {error: _error, parentId});
      });
    },
    [handleLoadMore, logger],
  );

  const isTreeLoading = rootOuId ? (!hideRoot && isRootOuLoading) || isRootOuChildrenLoading : isLoading;
  const activeError = rootOuId ? ((!hideRoot ? rootOuError : null) ?? rootOuChildrenError) : rootListError;

  if (isTreeLoading) {
    return <PageLoadingAnimation />;
  }

  if (activeError) {
    return (
      <QueryErrorNotice
        error={activeError}
        t={(key, options) => t(key.includes(':') ? key : `organizationUnits:${key}`, options)}
        variant="inline"
        fallbackKey="organizationUnits:treePicker.error"
        fallbackDefaultValue="Failed to load organization unit data"
        onRetry={() => {
          if (rootOuId) {
            if (!hideRoot && rootOuError) void refetchRootOu();
            if (rootOuChildrenError) void refetchRootOuChildren();
          } else {
            void refetchRootList();
          }
        }}
      />
    );
  }

  if (!rootOuId && data?.organizationUnits.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        {t('organizationUnits:treePicker.empty')}
      </Typography>
    );
  }

  if (hideRoot && rootOuChildrenData?.organizationUnits.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary" sx={{px: 1, py: 0.75}}>
        {t('organizationUnits:listing.treeView.noChildren')}
      </Typography>
    );
  }

  return (
    <Box>
      <Box
        sx={{
          maxHeight,
          overflow: 'auto',
        }}
      >
        <TreeView.RichTreeView
          id={id}
          items={treeItems}
          expandedItems={expandedItems}
          onExpandedItemsChange={handleExpandedItemsChange}
          onItemExpansionToggle={handleItemExpansionToggle}
          selectedItems={value || null}
          onSelectedItemsChange={handleSelectedItemsChange}
          disableSelection={Boolean(onItemActivate)}
          slots={{item: PickerTreeItem}}
          slotProps={{
            item: {
              itemMap,
              loadingItems,
              loadMoreLoadingItems: combinedLoadMoreLoadingItems,
              onLoadMore: handleLoadMoreWithErrorLogging,
              onItemActivate,
              spacious,
            } as Record<string, unknown>,
          }}
          getItemLabel={(item: OrganizationUnitTreeItem) => item.label}
          sx={
            spacious
              ? {
                  '& .MuiTreeItem-content': {
                    cursor: 'pointer',
                    border: '1.5px solid',
                    borderColor: theme.vars?.palette.divider,
                    borderRadius: '14px',
                    bgcolor: theme.vars?.palette.background.paper,
                    py: '18px',
                    px: '20px',
                    mb: 1.5,
                    transition: 'border-color 0.2s, background-color 0.2s',
                    '&:hover': {
                      borderColor: theme.vars?.palette.text.secondary,
                    },
                  },
                  '& .Mui-selected > .MuiTreeItem-content': {
                    backgroundColor: `${theme.vars?.palette.primary.main}1f`,
                    borderColor: theme.vars?.palette.primary.main,
                  },
                  '& .MuiTreeItem-iconContainer': {
                    color: theme.vars?.palette.text.secondary,
                    mr: 0.75,
                  },
                  '& .MuiTreeItem-groupTransition': {
                    ml: 2.5,
                    pl: 2,
                    borderLeft: '1px dashed',
                    borderColor: theme.vars?.palette.divider,
                  },
                }
              : {
                  '& .MuiTreeItem-content': {
                    cursor: 'pointer',
                    border: '1px solid',
                    borderColor: theme.vars?.palette.divider,
                    borderRadius: 0.5,
                    py: 0.75,
                    px: 1,
                    mb: 0.5,
                    transition: 'all 0.15s ease-in-out',
                    '&:hover': {
                      backgroundColor: theme.vars?.palette.action.hover,
                      borderColor: theme.vars?.palette.primary.main,
                    },
                  },
                  '& .Mui-selected > .MuiTreeItem-content': {
                    backgroundColor: `${theme.vars?.palette.primary.main}14`,
                    borderColor: theme.vars?.palette.primary.main,
                  },
                  '& .MuiTreeItem-iconContainer': {
                    color: theme.vars?.palette.text.secondary,
                    mr: 0.5,
                  },
                  '& .MuiTreeItem-groupTransition': {
                    ml: 2,
                    pl: 2,
                    borderLeft: '1px dashed',
                    borderColor: theme.vars?.palette.divider,
                  },
                }
          }
        />
      </Box>
      {helperText && (
        <Typography variant="caption" color={error ? 'error' : 'text.secondary'} sx={{mt: 0.5, ml: 1.75}}>
          {helperText}
        </Typography>
      )}
    </Box>
  );
}
