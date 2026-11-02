// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Resource} from './resources';

/**
 * Interface for drag source data.
 */
export interface DragSourceData {
  dragged?: Resource;
  isReordering?: boolean;
  stepId?: string;
  [key: string]: unknown;
}

/**
 * Interface for drag target data.
 */
export interface DragTargetData {
  stepId?: string;
  droppedOn?: Resource;
  [key: string]: unknown;
}

/**
 * Interface for drag event with optional native event.
 * The nativeEvent may be undefined or a generic Event from dnd-kit,
 * but is validated as MouseEvent before use.
 */
export interface DragEventWithNative {
  nativeEvent?: Event;
  [key: string]: unknown;
}
