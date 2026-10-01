// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Context, Dispatch, SetStateAction} from 'react';
import {createContext} from 'react';
import {type Base} from '../models/base';
import type {Claim} from '../models/metadata';
import type {Resource} from '../models/resources';

/**
 * Props interface of {@link InteractionContext}
 */
export interface InteractionContextProps {
  /**
   * The properties of the last interacted resource.
   */
  lastInteractedResource: Base;
  /**
   * The ID of the last user interacted resource node.
   */
  lastInteractedStepId: string;
  /**
   * Sets the latest interacted resource inside the canvas.
   * @param resource - The resource that was interacted with.
   * @param openPanel - Whether to open the properties panel. Defaults to true.
   */
  setLastInteractedResource: (resource: Resource, openPanel?: boolean) => void;
  /**
   * Sets the active element node ID.
   */
  setLastInteractedStepId: (stepId: string) => void;
  /**
   * Function to be called when an element is dropped on the canvas.
   * @param element - The element that was dropped on the canvas.
   * @param nodeId - The ID of the node on which the element was dropped.
   */
  onResourceDropOnCanvas: (element: Base, nodeId: string) => void;
  /**
   * The set of attributes that are selected for the flow that are maintained per node.
   */
  selectedAttributes: Record<string, Claim[]>;
  /**
   * Sets the selected attributes for the flow.
   */
  setSelectedAttributes: Dispatch<SetStateAction<Record<string, Claim[]>>>;
}

const InteractionContext: Context<InteractionContextProps | undefined> = createContext<
  InteractionContextProps | undefined
>(undefined);

InteractionContext.displayName = 'InteractionContext';

export default InteractionContext;
