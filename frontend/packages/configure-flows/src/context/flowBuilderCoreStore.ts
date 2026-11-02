// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useSyncExternalStore} from 'react';
import type {Resource} from '../models/resources';

// Store for lastInteractedResource - frequently changes on clicks
interface LastInteractedResourceStore {
  resource: Resource | null;
  stepId: string;
}

const lastInteractedListeners = new Set<() => void>();
let lastInteractedSnapshot: LastInteractedResourceStore = {
  resource: null,
  stepId: '',
};

/**
 * Updates the last interacted resource store and notifies subscribers.
 * Called by FlowBuilderCoreProvider when lastInteractedResource changes.
 */
export function updateLastInteractedStore(resource: Resource | null, stepId: string): void {
  // Only notify if values actually changed
  if (lastInteractedSnapshot.resource !== resource || lastInteractedSnapshot.stepId !== stepId) {
    lastInteractedSnapshot = {resource, stepId};
    lastInteractedListeners.forEach((listener) => listener());
  }
}

/**
 * Hook to subscribe ONLY to lastInteractedResource changes.
 */
export function useLastInteractedResourceOnly(): LastInteractedResourceStore {
  return useSyncExternalStore(
    (callback) => {
      lastInteractedListeners.add(callback);
      return () => lastInteractedListeners.delete(callback);
    },
    () => lastInteractedSnapshot,
    () => lastInteractedSnapshot,
  );
}

// Store for properties panel state - changes on clicks but only affects certain components
interface PropertiesPanelStore {
  isOpen: boolean;
}

const propertiesPanelListeners = new Set<() => void>();
let propertiesPanelSnapshot: PropertiesPanelStore = {
  isOpen: false,
};

/**
 * Updates the properties panel store and notifies subscribers.
 */
export function updatePropertiesPanelStore(isOpen: boolean): void {
  if (propertiesPanelSnapshot.isOpen !== isOpen) {
    propertiesPanelSnapshot = {isOpen};
    propertiesPanelListeners.forEach((listener) => listener());
  }
}

/**
 * Hook to subscribe ONLY to properties panel open state.
 */
export function usePropertiesPanelStateOnly(): PropertiesPanelStore {
  return useSyncExternalStore(
    (callback) => {
      propertiesPanelListeners.add(callback);
      return () => propertiesPanelListeners.delete(callback);
    },
    () => propertiesPanelSnapshot,
    () => propertiesPanelSnapshot,
  );
}
