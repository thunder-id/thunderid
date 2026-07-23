/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useQueryClient} from '@tanstack/react-query';
import {PageLoadingAnimation, ResourceAvatar} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {useThunderID} from '@thunderid/react';
import {
  Box,
  IconButton,
  Typography,
  CircularProgress,
  TreeView,
  Snackbar,
  Alert,
  useTheme,
  Avatar,
  Tooltip,
} from '@wso2/oxygen-ui';
import {Eye, Pencil, Plus, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useEffect, useRef, useMemo} from 'react';
import type {ReactNode, MouseEvent, KeyboardEvent, SyntheticEvent, JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import OrganizationUnitDeleteDialog from './OrganizationUnitDeleteDialog';
import fetchChildOrganizationUnits from '../api/fetchChildOrganizationUnits';
import fetchOrganizationUnits from '../api/fetchOrganizationUnits';
import useGetOrganizationUnits from '../api/useGetOrganizationUnits';
import OrganizationUnitQueryKeys from '../constants/organization-unit-query-keys';
import OrganizationUnitTreeConstants from '../constants/organization-unit-tree-constants';
import useOrganizationUnit from '../contexts/useOrganizationUnit';
import useOrganizationUnitRoutes from '../hooks/useOrganizationUnitRoutes';
import type {OrganizationUnit} from '../models/organization-unit';
import type {OrganizationUnitTreeItem} from '../models/organization-unit-tree';
import type {OrganizationUnitListResponse} from '../models/responses';
import appendTreeItemChildren from '../utils/appendTreeItemChildren';
import buildItemMap from '../utils/buildItemMap';
import buildTreeItems from '../utils/buildTreeItems';
import findTreeItem from '../utils/findTreeItem';
import updateTreeItemChildren from '../utils/updateTreeItemChildren';

function TreeViewLoadingIcon(): JSX.Element {
  return <CircularProgress size={18} />;
}

function buildAddChildItem(parentId: string, parentName: string, parentHandle: string): OrganizationUnitTreeItem {
  return {
    id: `${parentId}${OrganizationUnitTreeConstants.ADD_CHILD_SUFFIX}`,
    label: parentName,
    handle: parentHandle,
    isPlaceholder: true,
  };
}

interface CustomTreeItemProps extends TreeView.TreeItemProps {
  itemId: string;
  label?: ReactNode;
  onEdit?: (event: MouseEvent<HTMLElement>, ou: {id: string; name: string}) => void;
  onDelete?: (event: MouseEvent<HTMLElement>, ou: {id: string; name: string}) => void;
  onAddChild?: (event: MouseEvent<HTMLElement>, ou: {id: string; name: string; handle: string}) => void;
  onLoadMore?: (parentId: string) => void;
  addChildTooltip?: string;
  addChildButtonText?: string;
  editTooltip?: string;
  deleteTooltip?: string;
  loadingItems?: Set<string>;
  loadMoreLoadingItems?: Set<string>;
  itemMap?: Map<string, OrganizationUnitTreeItem>;
}

function CustomTreeItem(allProps: CustomTreeItemProps): JSX.Element {
  const {
    onEdit,
    onDelete,
    onAddChild,
    onLoadMore: onLoadMoreProp,
    addChildTooltip = '',
    addChildButtonText = '',
    editTooltip = '',
    deleteTooltip = '',
    loadingItems: loadingItemsProp,
    loadMoreLoadingItems: loadMoreLoadingItemsProp,
    itemMap: itemMapProp,
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
  const isAddChildButton = itemId.endsWith(OrganizationUnitTreeConstants.ADD_CHILD_SUFFIX);
  const isPlaceholder =
    !isAddChildButton &&
    !isLoadMoreItem &&
    (itemData?.isPlaceholder ??
      (itemId.endsWith(OrganizationUnitTreeConstants.PLACEHOLDER_SUFFIX) ||
        itemId.endsWith(OrganizationUnitTreeConstants.ERROR_SUFFIX) ||
        itemId.endsWith(OrganizationUnitTreeConstants.EMPTY_SUFFIX)));
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
            borderRadius: 1,
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
            onClick={(e: MouseEvent<HTMLElement>) => {
              e.stopPropagation();
              if (!isLoadingMore) {
                onLoadMoreProp?.(parentId);
              }
            }}
            onKeyDown={(e: KeyboardEvent) => {
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

  if (isAddChildButton) {
    const parentId = itemId.replace(OrganizationUnitTreeConstants.ADD_CHILD_SUFFIX, '');
    const parentItem = itemMapProp?.get(parentId);

    return (
      <TreeView.TreeItem
        {...treeItemProps}
        sx={{
          '& > .MuiTreeItem-content': {
            border: '1px dashed',
            borderColor: theme.vars?.palette.primary.main,
            borderRadius: 1,
            backgroundColor: 'transparent !important',
            cursor: 'pointer',
            transition: 'all 0.15s ease-in-out',
            '&:hover': {
              backgroundColor: `${theme.vars?.palette.primary.main} !important`,
              '& .add-child-avatar': {
                backgroundColor: theme.vars?.palette.primary.contrastText,
                color: theme.vars?.palette.primary.main,
              },
              '& .add-child-text': {
                color: theme.vars?.palette.primary.contrastText,
              },
            },
          },
        }}
        label={
          <Box
            role="button"
            tabIndex={0}
            onClick={(e: MouseEvent<HTMLElement>) => {
              e.stopPropagation();
              onAddChild?.(e, {id: parentId, name: parentItem?.label ?? '', handle: parentItem?.handle ?? ''});
            }}
            onKeyDown={(e: KeyboardEvent) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                e.stopPropagation();
                onAddChild?.(e as unknown as MouseEvent<HTMLElement>, {
                  id: parentId,
                  name: parentItem?.label ?? '',
                  handle: parentItem?.handle ?? '',
                });
              }
            }}
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: 1.5,
            }}
          >
            <Avatar
              className="add-child-avatar"
              sx={{
                p: 0.5,
                backgroundColor: theme.vars?.palette.primary.main,
                color: theme.vars?.palette.primary.contrastText,
                width: 32,
                height: 32,
                fontSize: '0.875rem',
                transition: 'all 0.15s ease-in-out',
              }}
            >
              <Plus size={14} />
            </Avatar>
            <Typography
              className="add-child-text"
              variant="body2"
              sx={{fontWeight: 500, transition: 'color 0.15s ease-in-out'}}
            >
              {addChildButtonText}
            </Typography>
          </Box>
        }
      />
    );
  }

  if (isPlaceholder) {
    const isLoadingPlaceholder = itemId.endsWith(OrganizationUnitTreeConstants.PLACEHOLDER_SUFFIX);
    const isErrorPlaceholder = itemId.endsWith(OrganizationUnitTreeConstants.ERROR_SUFFIX);

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
            {isLoadingPlaceholder ? (
              <>
                <CircularProgress size={16} />
                <Typography variant="caption" color="text.secondary" sx={{fontStyle: 'italic'}}>
                  Loading...
                </Typography>
              </>
            ) : (
              <Typography
                variant="caption"
                color={isErrorPlaceholder ? 'error' : 'text.secondary'}
                sx={{fontStyle: 'italic', pl: 1}}
              >
                {labelStr}
              </Typography>
            )}
          </Box>
        }
      />
    );
  }

  return (
    <TreeView.TreeItem
      {...treeItemProps}
      {...(isItemLoading ? {slots: {collapseIcon: TreeViewLoadingIcon, expandIcon: TreeViewLoadingIcon}} : {})}
      label={
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1.5,
          }}
        >
          <ResourceAvatar
            variant="rounded"
            value={itemData?.logoUrl}
            size={30}
            fallback={OrganizationUnitTreeConstants.DEFAULT_AVATAR}
          />
          <Box sx={{flexGrow: 1, minWidth: 0}}>
            <Typography variant="body2" sx={{fontWeight: 500, lineHeight: 1.3}}>
              {labelStr}
            </Typography>
            {itemData?.handle && (
              <Typography variant="caption" color="text.secondary" sx={{lineHeight: 1.2, display: 'block'}}>
                {itemData.handle}
              </Typography>
            )}
          </Box>
          {itemData?.isReadOnly ? (
            <Tooltip title={t('common:status.readOnly', 'Read Only')}>
              <IconButton size="small" disableRipple sx={{cursor: 'default'}}>
                <Eye size={16} />
              </IconButton>
            </Tooltip>
          ) : (
            <>
              <Tooltip title={addChildTooltip}>
                <IconButton
                  size="small"
                  aria-label={addChildTooltip}
                  onClick={(e: MouseEvent<HTMLButtonElement>) => {
                    e.stopPropagation();
                    onAddChild?.(e as unknown as MouseEvent<HTMLElement>, {
                      id: itemId,
                      name: labelStr,
                      handle: itemData?.handle ?? '',
                    });
                  }}
                >
                  <Plus size={16} />
                </IconButton>
              </Tooltip>
              <Tooltip title={editTooltip}>
                <IconButton
                  size="small"
                  aria-label={editTooltip}
                  onClick={(e: MouseEvent<HTMLButtonElement>) => {
                    e.stopPropagation();
                    onEdit?.(e as unknown as MouseEvent<HTMLElement>, {id: itemId, name: labelStr});
                  }}
                >
                  <Pencil size={16} />
                </IconButton>
              </Tooltip>
              <Tooltip title={deleteTooltip}>
                <IconButton
                  size="small"
                  color="error"
                  aria-label={deleteTooltip}
                  onClick={(e: MouseEvent<HTMLButtonElement>) => {
                    e.stopPropagation();
                    onDelete?.(e as unknown as MouseEvent<HTMLElement>, {id: itemId, name: labelStr});
                  }}
                >
                  <Trash2 size={16} />
                </IconButton>
              </Tooltip>
            </>
          )}
        </Box>
      }
    />
  );
}

export default function OrganizationUnitsTreeView(): JSX.Element {
  const theme = useTheme();
  const navigate = useNavigate();
  const routes = useOrganizationUnitRoutes();
  const {t} = useTranslation();
  const logger = useLogger('OrganizationUnitsTreeView');
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {data, isLoading, error} = useGetOrganizationUnits();
  const {treeItems, setTreeItems, expandedItems, setExpandedItems, loadedItems, setLoadedItems, resetTreeState} =
    useOrganizationUnit();

  const itemMap = useMemo(() => buildItemMap(treeItems), [treeItems]);

  const [loadingItems, setLoadingItems] = useState<Set<string>>(new Set());
  const [loadMoreLoadingItems, setLoadMoreLoadingItems] = useState<Set<string>>(new Set());
  const [childOffsets, setChildOffsets] = useState<Map<string, number>>(new Map());
  const [rootOffset, setRootOffset] = useState<number>(0);
  const [rootLoadMoreLoading, setRootLoadMoreLoading] = useState<boolean>(false);
  const rootLoadMoreLoadingRef = useRef<boolean>(false);
  rootLoadMoreLoadingRef.current = rootLoadMoreLoading;
  const loadingItemsRef = useRef<Set<string>>(loadingItems);
  loadingItemsRef.current = loadingItems;
  const expandedItemsRef = useRef<string[]>(expandedItems);
  expandedItemsRef.current = expandedItems;
  const treeItemsRef = useRef<OrganizationUnitTreeItem[]>(treeItems);
  treeItemsRef.current = treeItems;
  const rebuildIdRef = useRef(0);
  const builtFromDataRef = useRef<unknown>(null);
  const [selectedOU, setSelectedOU] = useState<{id: string; name: string} | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);
  const [snackbar, setSnackbar] = useState<{open: boolean; message: string; severity: 'success' | 'error'}>({
    open: false,
    message: '',
    severity: 'success',
  });

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

  // Fetch children for a single parent and return the built tree items.
  // Does NOT update React state — caller is responsible for that.
  const fetchChildItems = useCallback(
    async (parentId: string): Promise<OrganizationUnitTreeItem[]> => {
      const result = await fetchChildPage(parentId, 0);
      const childOUs = result.organizationUnits;
      const parentItem = findTreeItem(treeItemsRef.current, parentId);
      const addChildItem = buildAddChildItem(parentId, parentItem?.label ?? '', parentItem?.handle ?? '');
      const items = childOUs.length > 0 ? [addChildItem, ...buildTreeItems(childOUs)] : [addChildItem];

      if (childOUs.length < result.totalResults) {
        items.push({
          id: `${parentId}${OrganizationUnitTreeConstants.LOAD_MORE_SUFFIX}`,
          label: '',
          handle: '',
          isPlaceholder: true,
        });
      }

      return items;
    },
    [fetchChildPage],
  );

  // Fetch and update state for user-triggered node expansion
  const fetchChildOUs = useCallback(
    async (parentId: string): Promise<void> => {
      if (loadingItemsRef.current.has(parentId)) return;

      setLoadingItems((prev) => new Set(prev).add(parentId));

      try {
        const childItems = await fetchChildItems(parentId);

        setChildOffsets((prev) => new Map(prev).set(parentId, childItems.filter((c) => !c.isPlaceholder).length));
        // Update tree items, mark as loaded, then expand in one synchronous block.
        // The node stays collapsed until this point, so it opens directly with real children.
        setTreeItems((prev) => updateTreeItemChildren(prev, parentId, childItems));
        setLoadedItems((prev) => new Set(prev).add(parentId));
        setExpandedItems((prev) => (prev.includes(parentId) ? prev : [...prev, parentId]));
      } catch (_error: unknown) {
        logger.error('Failed to load child organization units', {error: _error, parentId});
        // Replace the loading placeholder with an error placeholder so the user sees feedback.
        // The node is NOT marked as loaded, so collapsing and re-expanding will retry the fetch.
        const errorItem: OrganizationUnitTreeItem = {
          id: `${parentId}${OrganizationUnitTreeConstants.ERROR_SUFFIX}`,
          label: t('organizationUnits:listing.treeView.loadError'),
          handle: '',
          isPlaceholder: true,
        };
        setTreeItems((prev) => updateTreeItemChildren(prev, parentId, [errorItem]));
        setExpandedItems((prev) => (prev.includes(parentId) ? prev : [...prev, parentId]));
      } finally {
        setLoadingItems((prev) => {
          const next = new Set(prev);
          next.delete(parentId);

          return next;
        });
      }
    },
    [fetchChildItems, setTreeItems, setLoadedItems, setExpandedItems, logger, t],
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
  }, [rootOffset, getServerUrl, queryClient, http, setTreeItems, logger]);

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
        const childOUs = result.organizationUnits;
        const newItems = buildTreeItems(childOUs);
        const loadedSoFar = offset + childOUs.length;

        if (loadedSoFar < result.totalResults) {
          newItems.push({
            id: `${parentId}${OrganizationUnitTreeConstants.LOAD_MORE_SUFFIX}`,
            label: '',
            handle: '',
            isPlaceholder: true,
          });
        }

        setChildOffsets((prev) => new Map(prev).set(parentId, loadedSoFar));
        setTreeItems((prev) => appendTreeItemChildren(prev, parentId, newItems));
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
    [childOffsets, fetchChildPage, setTreeItems, logger, handleRootLoadMore],
  );

  // Process one level of the tree: fetch children for the given IDs,
  // insert them into the tree, then recurse for the next deeper level.
  const expandLevel = useCallback(
    (
      tree: OrganizationUnitTreeItem[],
      levelIds: string[],
      expandedSet: Set<string>,
      loaded: Set<string>,
    ): Promise<{tree: OrganizationUnitTreeItem[]; loaded: Set<string>}> => {
      if (levelIds.length === 0) {
        return Promise.resolve({tree, loaded});
      }

      return Promise.all(
        levelIds.map((parentId) =>
          fetchChildItems(parentId)
            .then((children) => ({parentId, children, success: true as const}))
            .catch(() => ({parentId, children: [] as OrganizationUnitTreeItem[], success: false as const})),
        ),
      ).then((results) => {
        // Insert fetched children into the tree and collect next-level IDs
        let updatedTree = tree;
        const nextLoaded = new Set(loaded);
        const nextLevelIds: string[] = [];

        results
          .filter((r) => r.success)
          .forEach((r) => {
            updatedTree = updateTreeItemChildren(updatedTree, r.parentId, r.children);
            nextLoaded.add(r.parentId);

            r.children
              .filter((child) => !child.isPlaceholder && expandedSet.has(child.id))
              .forEach((child) => {
                nextLevelIds.push(child.id);
              });
          });

        // Recurse to the next level
        return expandLevel(updatedTree, nextLevelIds, expandedSet, nextLoaded);
      });
    },
    [fetchChildItems],
  );

  // Build the full tree with all previously expanded nodes restored.
  // Returns the computed tree and loaded set without setting state — the caller
  // is responsible for applying the result so it can guard against stale rebuilds.
  const rebuildTree = useCallback(
    (
      rootOUs: OrganizationUnit[],
      expandedIds: string[],
    ): Promise<{tree: OrganizationUnitTreeItem[]; loaded: Set<string>}> => {
      const rootTree = buildTreeItems(rootOUs);
      const expandedSet = new Set(expandedIds);

      // Start with root-level IDs that are expanded
      const rootLevelIds = rootTree.map((item) => item.id).filter((id) => expandedSet.has(id));

      return expandLevel(rootTree, rootLevelIds, expandedSet, new Set<string>());
    },
    [expandLevel],
  );

  const buildRootTreeItems = useCallback((response: OrganizationUnitListResponse): OrganizationUnitTreeItem[] => {
    const items = buildTreeItems(response.organizationUnits);

    if (response.organizationUnits.length < response.totalResults) {
      items.push({
        id: OrganizationUnitTreeConstants.ROOT_LOAD_MORE_ID,
        label: '',
        handle: '',
        isPlaceholder: true,
      });
    }

    setRootOffset(response.organizationUnits.length);

    return items;
  }, []);

  // Rebuild tree when query data is available and either the tree is empty (after
  // reset) or the data has changed since the last build (fresh fetch after mutation).
  // rebuildIdRef guards against stale rebuilds: if a newer rebuild starts while an
  // older one is in-flight, the older result is silently ignored.
  useEffect(() => {
    if (!data?.organizationUnits || data.organizationUnits.length === 0) return;

    // Skip if tree is already built from this exact data reference
    if (treeItems.length > 0 && builtFromDataRef.current === data) return;

    const currentExpanded = expandedItemsRef.current;
    rebuildIdRef.current += 1;
    const id = rebuildIdRef.current;

    if (currentExpanded.length > 0) {
      // Rebuild with expanded nodes restored
      rebuildTree(data.organizationUnits, currentExpanded)
        .then(({tree, loaded}) => {
          // Only apply if no newer rebuild was triggered
          if (rebuildIdRef.current === id) {
            const items = tree;

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
            setLoadedItems(loaded);
            builtFromDataRef.current = data;
          }
        })
        .catch((_err: unknown) => {
          logger.error('Failed to rebuild tree with expanded items', {error: _err});
          if (rebuildIdRef.current === id) {
            // Fallback: just set root items
            setTreeItems(buildRootTreeItems(data));
            builtFromDataRef.current = data;
          }
        });
    } else {
      setTreeItems(buildRootTreeItems(data));
      builtFromDataRef.current = data;
    }
  }, [data, treeItems.length, rebuildTree, buildRootTreeItems, setTreeItems, setLoadedItems, logger]);

  // Clear builtFromDataRef when tree is reset so the effect rebuilds from current data
  useEffect(() => {
    if (treeItems.length === 0) {
      builtFromDataRef.current = null;
    }
  }, [treeItems.length]);

  const handleItemExpansionToggle = useCallback(
    (_event: SyntheticEvent | null, itemId: string, isExpanded: boolean) => {
      if (!isExpanded || loadedItems.has(itemId) || loadingItems.has(itemId)) {
        return;
      }

      // Don't expand yet — fetchChildOUs will expand after children are loaded.
      fetchChildOUs(itemId).catch((_error: unknown) => {
        logger.error('Failed to load child organization units', {error: _error, parentId: itemId});
      });
    },
    [loadedItems, loadingItems, fetchChildOUs, logger],
  );

  const handleEditClick = useCallback(
    (_event: MouseEvent<HTMLElement>, ou: {id: string; name: string}): void => {
      (async (): Promise<void> => {
        await navigate(routes.detail(ou.id));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to organization unit', {error: _error, ouId: ou.id});
      });
    },
    [navigate, routes, logger],
  );

  const handleDeleteClick = useCallback((_event: MouseEvent<HTMLElement>, ou: {id: string; name: string}): void => {
    setSelectedOU(ou);
    setDeleteDialogOpen(true);
  }, []);

  const handleDeleteDialogClose = (): void => {
    setDeleteDialogOpen(false);
    setSelectedOU(null);
  };

  const handleDeleteSuccess = useCallback((): void => {
    resetTreeState();
    setSnackbar({
      open: true,
      message: t('organizationUnits:edit.general.dangerZone.delete.success'),
      severity: 'success',
    });
  }, [resetTreeState, t]);

  const handleDeleteError = useCallback((message: string): void => {
    setSnackbar({open: true, message, severity: 'error'});
  }, []);

  const handleAddChildClick = useCallback(
    (_event: MouseEvent<HTMLElement>, ou: {id: string; name: string; handle: string}): void => {
      (async (): Promise<void> => {
        await navigate(routes.create(), {
          state: {parentId: ou.id, parentName: ou.name, parentHandle: ou.handle},
        });
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to create child organization unit', {error: _error, parentId: ou.id});
      });
    },
    [navigate, routes, logger],
  );

  const handleAddRootClick = useCallback((): void => {
    (async (): Promise<void> => {
      await navigate(routes.create());
    })().catch((_error: unknown) => {
      logger.error('Failed to navigate to create organization unit page', {error: _error});
    });
  }, [navigate, routes, logger]);

  const combinedLoadMoreLoadingItems = useMemo(() => {
    if (!rootLoadMoreLoading) return loadMoreLoadingItems;
    const combined = new Set(loadMoreLoadingItems);
    combined.add(OrganizationUnitTreeConstants.ROOT_PARENT_ID);

    return combined;
  }, [loadMoreLoadingItems, rootLoadMoreLoading]);

  const handleExpandedItemsChange = useCallback(
    (_event: SyntheticEvent | null, itemIds: string[]) => {
      // Block expansion of items whose children haven't been loaded yet.
      // fetchChildOUs will add them to expandedItems after children are fetched.
      const prevSet = new Set(expandedItems);
      const filtered = itemIds.filter((id) => prevSet.has(id) || loadedItems.has(id));
      setExpandedItems(filtered);
    },
    [expandedItems, loadedItems, setExpandedItems],
  );

  const handleLoadMoreWithErrorLogging = useCallback(
    (parentId: string) => {
      handleLoadMore(parentId).catch((_error: unknown) => {
        logger.error('Failed to load more child organization units', {error: _error, parentId});
      });
    },
    [handleLoadMore, logger],
  );

  if (error) {
    return (
      <Box sx={{textAlign: 'center', py: 8}}>
        <Typography variant="h6" color="error" gutterBottom>
          {t('organizationUnits:listing.error.title')}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {error.message ?? t('organizationUnits:listing.error.unknown')}
        </Typography>
      </Box>
    );
  }

  if (isLoading) {
    return <PageLoadingAnimation />;
  }

  if (!treeItems.length) {
    // Data loaded but no organization units exist — show empty state
    if (data?.organizationUnits.length === 0) {
      return (
        <Box sx={{textAlign: 'center', py: 8}}>
          <Typography variant="body2" color="text.secondary" sx={{mb: 2}}>
            {t('organizationUnits:listing.treeView.empty')}
          </Typography>
          <Box
            role="button"
            tabIndex={0}
            onClick={handleAddRootClick}
            onKeyDown={(e: KeyboardEvent) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                handleAddRootClick();
              }
            }}
            sx={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 1.5,
              border: '1px dashed',
              borderColor: theme.vars?.palette.primary.main,
              borderRadius: 1,
              py: 1,
              px: 2,
              cursor: 'pointer',
              transition: 'all 0.15s ease-in-out',
              '&:hover': {
                backgroundColor: theme.vars?.palette.primary.main,
                '& .add-root-avatar': {
                  backgroundColor: theme.vars?.palette.primary.contrastText,
                  color: theme.vars?.palette.primary.main,
                },
                '& .add-root-text': {
                  color: theme.vars?.palette.primary.contrastText,
                },
              },
            }}
          >
            <Avatar
              className="add-root-avatar"
              sx={{
                p: 0.5,
                backgroundColor: theme.vars?.palette.primary.main,
                color: theme.vars?.palette.primary.contrastText,
                width: 32,
                height: 32,
                fontSize: '0.875rem',
                transition: 'all 0.15s ease-in-out',
              }}
            >
              <Plus size={14} />
            </Avatar>
            <Typography
              className="add-root-text"
              variant="body2"
              sx={{fontWeight: 500, transition: 'color 0.15s ease-in-out'}}
            >
              {t('organizationUnits:listing.addRootOrganizationUnit')}
            </Typography>
          </Box>
        </Box>
      );
    }

    // Still loading tree items
    return <PageLoadingAnimation />;
  }

  return (
    <>
      <Box sx={{width: '100%', minHeight: 400}}>
        <Box
          role="button"
          tabIndex={0}
          onClick={handleAddRootClick}
          onKeyDown={(e: KeyboardEvent) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              handleAddRootClick();
            }
          }}
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1.5,
            border: '1px dashed',
            borderColor: theme.vars?.palette.primary.main,
            borderRadius: 1,
            py: 1,
            pl: 5,
            pr: 1.5,
            mb: 0.75,
            cursor: 'pointer',
            transition: 'all 0.15s ease-in-out',
            '&:hover': {
              backgroundColor: theme.vars?.palette.primary.main,
              '& .add-root-avatar': {
                backgroundColor: theme.vars?.palette.primary.contrastText,
                color: theme.vars?.palette.primary.main,
              },
              '& .add-root-text': {
                color: theme.vars?.palette.primary.contrastText,
              },
            },
          }}
        >
          <Avatar
            className="add-root-avatar"
            sx={{
              p: 0.5,
              backgroundColor: theme.vars?.palette.primary.main,
              color: theme.vars?.palette.primary.contrastText,
              width: 32,
              height: 32,
              fontSize: '0.875rem',
              transition: 'all 0.15s ease-in-out',
            }}
          >
            <Plus size={14} />
          </Avatar>
          <Typography
            className="add-root-text"
            variant="body2"
            sx={{fontWeight: 500, transition: 'color 0.15s ease-in-out'}}
          >
            {t('organizationUnits:listing.addRootOrganizationUnit')}
          </Typography>
        </Box>
        <TreeView.RichTreeView
          items={treeItems}
          expandedItems={expandedItems}
          onExpandedItemsChange={handleExpandedItemsChange}
          onItemExpansionToggle={handleItemExpansionToggle}
          disableSelection
          slots={{item: CustomTreeItem}}
          slotProps={{
            item: {
              onEdit: handleEditClick,
              onDelete: handleDeleteClick,
              onAddChild: handleAddChildClick,
              onLoadMore: handleLoadMoreWithErrorLogging,
              addChildTooltip: t('organizationUnits:listing.treeView.addChild'),
              addChildButtonText: t('organizationUnits:listing.treeView.addChildOrganizationUnit'),
              editTooltip: t('common:actions.edit'),
              deleteTooltip: t('common:actions.delete'),
              loadingItems,
              loadMoreLoadingItems: combinedLoadMoreLoadingItems,
              itemMap,
            } as Record<string, unknown>,
          }}
          getItemLabel={(item: OrganizationUnitTreeItem) => item.label}
          sx={{
            '& .MuiTreeItem-root': {
              position: 'relative',
            },
            '& .MuiTreeItem-content': {
              cursor: 'pointer',
              border: '1px solid',
              borderColor: theme.vars?.palette.divider,
              py: 1,
              px: 1.5,
              mb: 0.75,
              transition: 'all 0.15s ease-in-out',
              '&:hover': {
                backgroundColor: theme.vars?.palette.action.hover,
                borderColor: theme.vars?.palette.primary.main,
              },
            },
            '& .MuiTreeItem-iconContainer': {
              color: theme.vars?.palette.text.secondary,
              mr: 0.5,
            },
            // Hierarchy connector lines
            '& .MuiTreeItem-groupTransition': {
              ml: 3,
              pl: 3,
              borderLeft: '1px dashed',
              borderColor: theme.vars?.palette.divider,
            },
          }}
        />
      </Box>

      <OrganizationUnitDeleteDialog
        open={deleteDialogOpen}
        organizationUnitId={selectedOU?.id ?? null}
        onClose={handleDeleteDialogClose}
        onSuccess={handleDeleteSuccess}
        onError={handleDeleteError}
      />

      <Snackbar
        open={snackbar.open}
        autoHideDuration={6000}
        onClose={() => setSnackbar((prev) => ({...prev, open: false}))}
        anchorOrigin={{vertical: 'bottom', horizontal: 'right'}}
      >
        <Alert
          onClose={() => setSnackbar((prev) => ({...prev, open: false}))}
          severity={snackbar.severity}
          sx={{width: '100%'}}
        >
          {snackbar.message}
        </Alert>
      </Snackbar>
    </>
  );
}
