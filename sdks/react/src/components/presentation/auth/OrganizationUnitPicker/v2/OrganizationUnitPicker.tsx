/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {cx} from '@emotion/css';
import {OrganizationUnit, OrganizationUnitListResponse} from '@thunderid/browser';
import React, {useCallback, useEffect, useState} from 'react';
import useStyles from './OrganizationUnitPicker.styles';
import useTheme from '../../../../../contexts/Theme/useTheme';

interface NodeState {
  children: OrganizationUnit[];
  expanded: boolean;
  hasMore: boolean;
  loading: boolean;
  offset: number;
  totalResults: number;
}

export interface OrganizationUnitPickerProps {
  className?: string;
  fetchChildren: (parentId: string, limit: number, offset: number) => Promise<OrganizationUnitListResponse>;
  onSelect: (ouId: string) => void;
  pageSize?: number;
  rootOuId: string;
  selectedOuId?: string | null;
}

const OrganizationUnitPicker = ({
  rootOuId,
  selectedOuId,
  onSelect,
  fetchChildren,
  pageSize = 10,
  className,
}: OrganizationUnitPickerProps): React.ReactElement => {
  const {theme} = useTheme();
  const styles: Record<string, string> = useStyles(theme);

  const [nodeStates, setNodeStates] = useState<Record<string, NodeState>>({});

  const loadChildren: (parentId: string, offset?: number) => Promise<void> = useCallback(
    async (parentId: string, offset = 0) => {
      setNodeStates((prev: Record<string, NodeState>) => ({
        ...prev,
        [parentId]: {
          ...(prev[parentId] || {children: [], expanded: true, hasMore: false, offset: 0, totalResults: 0}),
          loading: true,
        },
      }));

      try {
        const response: OrganizationUnitListResponse = await fetchChildren(parentId, pageSize, offset);
        const newChildren: OrganizationUnit[] = response.organizationUnits || [];

        setNodeStates((prev: Record<string, NodeState>) => {
          const existing: NodeState = prev[parentId] || {
            children: [],
            expanded: true,
            hasMore: false,
            loading: false,
            offset: 0,
            totalResults: 0,
          };
          const mergedChildren: OrganizationUnit[] =
            offset === 0 ? newChildren : [...existing.children, ...newChildren];
          const newOffset: number = offset + newChildren.length;

          return {
            ...prev,
            [parentId]: {
              children: mergedChildren,
              expanded: true,
              hasMore: newOffset < response.totalResults,
              loading: false,
              offset: newOffset,
              totalResults: response.totalResults,
            },
          };
        });
      } catch {
        setNodeStates((prev: Record<string, NodeState>) => ({
          ...prev,
          [parentId]: {
            ...(prev[parentId] || {children: [], expanded: true, hasMore: false, offset: 0, totalResults: 0}),
            loading: false,
          },
        }));
      }
    },
    [fetchChildren, pageSize],
  );

  // Auto-load root children on mount
  useEffect(() => {
    if (rootOuId && !nodeStates[rootOuId]) {
      loadChildren(rootOuId);
    }
  }, [rootOuId, loadChildren, nodeStates]);

  const handleToggle: (ouId: string) => void = useCallback(
    (ouId: string) => {
      const state: NodeState | undefined = nodeStates[ouId];

      if (state?.expanded) {
        setNodeStates((prev: Record<string, NodeState>) => ({
          ...prev,
          [ouId]: {...prev[ouId], expanded: false},
        }));
      } else if (state?.children.length) {
        setNodeStates((prev: Record<string, NodeState>) => ({
          ...prev,
          [ouId]: {...prev[ouId], expanded: true},
        }));
      } else {
        loadChildren(ouId);
      }
    },
    [nodeStates, loadChildren],
  );

  const handleLoadMore: (parentId: string) => void = useCallback(
    (parentId: string) => {
      const state: NodeState | undefined = nodeStates[parentId];

      if (state) {
        loadChildren(parentId, state.offset);
      }
    },
    [nodeStates, loadChildren],
  );

  const renderLoadingPlaceholders = (depth: number): React.ReactElement => (
    <>
      {[0, 1, 2].map((i: number) => (
        <div
          key={`skeleton-${i}`}
          className={styles['loadingPlaceholder']}
          style={{paddingLeft: `${(depth + 1) * 20}px`}}
        >
          <div className={styles['skeleton']} style={{width: `${100 - i * 20}px`}} />
        </div>
      ))}
    </>
  );

  const renderNode = (ou: OrganizationUnit, depth = 0): React.ReactElement => {
    const state: NodeState | undefined = nodeStates[ou.id];
    const isSelected: boolean = selectedOuId === ou.id;
    const isExpanded: boolean = state?.expanded || false;
    const isLoading: boolean = state?.loading || false;
    const hasChildren: boolean = !state || state.totalResults > 0 || state.children.length > 0;

    return (
      <React.Fragment key={ou.id}>
        <div
          className={cx(styles['node'], isSelected && styles['nodeSelected'])}
          style={{paddingLeft: `${depth * 20 + 12}px`}}
          role="treeitem"
          aria-selected={isSelected}
          aria-expanded={hasChildren ? isExpanded : undefined}
          onClick={() => onSelect(ou.id)}
          onKeyDown={(e: React.KeyboardEvent) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              onSelect(ou.id);
            }
          }}
          tabIndex={0}
        >
          {hasChildren ? (
            <button
              className={styles['toggleButton']}
              onClick={(e: React.MouseEvent) => {
                e.stopPropagation();
                handleToggle(ou.id);
              }}
              aria-label={isExpanded ? 'Collapse' : 'Expand'}
              type="button"
            >
              {isExpanded ? '\u25BE' : '\u25B8'}
            </button>
          ) : (
            <span className={styles['togglePlaceholder']} />
          )}
          <span className={styles['nodeName']}>{ou.name}</span>
        </div>

        {isExpanded && isLoading && !state?.children.length && renderLoadingPlaceholders(depth)}

        {isExpanded && state?.children.map((child: OrganizationUnit) => renderNode(child, depth + 1))}

        {isExpanded && state?.hasMore && (
          <button
            className={styles['loadMoreButton']}
            style={{paddingLeft: `${(depth + 1) * 20 + 12}px`}}
            onClick={() => handleLoadMore(ou.id)}
            disabled={isLoading}
            type="button"
          >
            {isLoading ? 'Loading...' : 'Load more'}
          </button>
        )}
      </React.Fragment>
    );
  };

  const rootState: NodeState | undefined = nodeStates[rootOuId];
  const isRootLoading: boolean = rootState?.loading && !rootState?.children.length;

  return (
    <div className={cx(styles['container'], className)} role="tree" aria-label="Organization unit picker">
      {isRootLoading && renderLoadingPlaceholders(0)}

      {rootState?.children.map((ou: OrganizationUnit) => renderNode(ou, 0))}

      {rootState?.hasMore && (
        <button
          className={styles['loadMoreButton']}
          onClick={() => handleLoadMore(rootOuId)}
          disabled={rootState?.loading}
          type="button"
        >
          {rootState?.loading ? 'Loading...' : 'Load more'}
        </button>
      )}
    </div>
  );
};

export default OrganizationUnitPicker;
