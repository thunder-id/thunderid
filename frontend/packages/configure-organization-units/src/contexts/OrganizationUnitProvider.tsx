// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useProjectScope} from '@thunderid/contexts';
import {useState, useMemo, useCallback} from 'react';
import type {PropsWithChildren, JSX} from 'react';
import {Outlet} from 'react-router';
import OrganizationUnitContext from './OrganizationUnitContext';
import type {OrganizationUnitContextType} from './OrganizationUnitContext';
import type {OrganizationUnitTreeItem} from '../models/organization-unit-tree';

export default function OrganizationUnitProvider({children}: PropsWithChildren): JSX.Element {
  const [treeItems, setTreeItems] = useState<OrganizationUnitTreeItem[]>([]);
  const [expandedItems, setExpandedItems] = useState<string[]>([]);
  const [loadedItems, setLoadedItems] = useState<Set<string>>(new Set());

  // The tree of one project is not shown in another, so a switch of project starts the tree again.
  const {projectId} = useProjectScope();
  const [treeProjectId, setTreeProjectId] = useState<string | undefined>(projectId);
  if (treeProjectId !== projectId) {
    setTreeProjectId(projectId);
    setTreeItems([]);
    setExpandedItems([]);
    setLoadedItems(new Set());
  }

  const resetTreeState = useCallback(() => {
    setTreeItems([]);
    setLoadedItems(new Set());
    // expandedItems intentionally preserved so tree re-expands after rebuild
  }, []);

  const contextValue: OrganizationUnitContextType = useMemo(
    () => ({
      treeItems,
      setTreeItems,
      expandedItems,
      setExpandedItems,
      loadedItems,
      setLoadedItems,
      resetTreeState,
    }),
    [treeItems, expandedItems, loadedItems, resetTreeState],
  );

  return (
    <OrganizationUnitContext.Provider value={contextValue}>{children ?? <Outlet />}</OrganizationUnitContext.Provider>
  );
}
