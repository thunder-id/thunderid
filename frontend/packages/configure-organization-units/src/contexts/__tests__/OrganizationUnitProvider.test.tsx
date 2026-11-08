// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, act} from '@testing-library/react';
import {ProjectScopeProvider} from '@thunderid/contexts';
import {useContext} from 'react';
import type {ReactNode} from 'react';
import {describe, it, expect} from 'vitest';
import OrganizationUnitContext from '../OrganizationUnitContext';
import OrganizationUnitProvider from '../OrganizationUnitProvider';

function useOrganizationUnitContext() {
  const context = useContext(OrganizationUnitContext);
  if (!context) {
    throw new Error('useOrganizationUnitContext must be used within OrganizationUnitProvider');
  }
  return context;
}

describe('OrganizationUnitProvider', () => {
  it('should provide default context values', () => {
    const {result} = renderHook(() => useOrganizationUnitContext(), {
      wrapper: ({children}: {children: ReactNode}) => <OrganizationUnitProvider>{children}</OrganizationUnitProvider>,
    });

    expect(result.current.treeItems).toEqual([]);
    expect(result.current.expandedItems).toEqual([]);
    expect(result.current.loadedItems).toEqual(new Set());
    expect(typeof result.current.setTreeItems).toBe('function');
    expect(typeof result.current.setExpandedItems).toBe('function');
    expect(typeof result.current.setLoadedItems).toBe('function');
    expect(typeof result.current.resetTreeState).toBe('function');
  });

  it('should allow setting treeItems', () => {
    const {result} = renderHook(() => useOrganizationUnitContext(), {
      wrapper: ({children}: {children: ReactNode}) => <OrganizationUnitProvider>{children}</OrganizationUnitProvider>,
    });

    const items = [{id: 'ou-1', label: 'Test OU', handle: 'test'}];

    act(() => {
      result.current.setTreeItems(items);
    });

    expect(result.current.treeItems).toEqual(items);
  });

  it('should allow setting expandedItems', () => {
    const {result} = renderHook(() => useOrganizationUnitContext(), {
      wrapper: ({children}: {children: ReactNode}) => <OrganizationUnitProvider>{children}</OrganizationUnitProvider>,
    });

    act(() => {
      result.current.setExpandedItems(['ou-1', 'ou-2']);
    });

    expect(result.current.expandedItems).toEqual(['ou-1', 'ou-2']);
  });

  it('should allow setting loadedItems', () => {
    const {result} = renderHook(() => useOrganizationUnitContext(), {
      wrapper: ({children}: {children: ReactNode}) => <OrganizationUnitProvider>{children}</OrganizationUnitProvider>,
    });

    act(() => {
      result.current.setLoadedItems(new Set(['ou-1']));
    });

    expect(result.current.loadedItems).toEqual(new Set(['ou-1']));
  });

  it('should reset treeItems and loadedItems but preserve expandedItems on resetTreeState', () => {
    const {result} = renderHook(() => useOrganizationUnitContext(), {
      wrapper: ({children}: {children: ReactNode}) => <OrganizationUnitProvider>{children}</OrganizationUnitProvider>,
    });

    // Set some state first
    act(() => {
      result.current.setTreeItems([{id: 'ou-1', label: 'Test', handle: 'test'}]);
      result.current.setExpandedItems(['ou-1']);
      result.current.setLoadedItems(new Set(['ou-1']));
    });

    expect(result.current.treeItems).toHaveLength(1);
    expect(result.current.expandedItems).toEqual(['ou-1']);
    expect(result.current.loadedItems.size).toBe(1);

    // Reset tree state
    act(() => {
      result.current.resetTreeState();
    });

    expect(result.current.treeItems).toEqual([]);
    expect(result.current.loadedItems).toEqual(new Set());
    // expandedItems should be preserved
    expect(result.current.expandedItems).toEqual(['ou-1']);
  });

  it('starts the tree again when the selected project changes', () => {
    let projectId = 'project-1';
    const {result, rerender} = renderHook(() => useOrganizationUnitContext(), {
      wrapper: ({children}: {children: ReactNode}) => (
        <ProjectScopeProvider project={{id: projectId}}>
          <OrganizationUnitProvider>{children}</OrganizationUnitProvider>
        </ProjectScopeProvider>
      ),
    });

    act(() => {
      result.current.setTreeItems([{id: 'ou-1', label: 'Test', handle: 'test'}]);
      result.current.setExpandedItems(['ou-1']);
      result.current.setLoadedItems(new Set(['ou-1']));
    });

    rerender();
    expect(result.current.treeItems).toHaveLength(1);

    projectId = 'project-2';
    rerender();

    expect(result.current.treeItems).toEqual([]);
    expect(result.current.expandedItems).toEqual([]);
    expect(result.current.loadedItems).toEqual(new Set());
  });
});
