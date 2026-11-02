// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Context, Dispatch, ReactNode, SetStateAction} from 'react';
import {createContext} from 'react';

/**
 * Props interface of {@link UIPanelContext}
 */
export interface UIPanelContextProps {
  /**
   * Indicates whether the element panel is open.
   */
  isResourcePanelOpen: boolean;
  /**
   * Indicates whether the element properties panel is open.
   */
  isResourcePropertiesPanelOpen: boolean;
  /**
   * Indicates whether the version history panel is open.
   */
  isVersionHistoryPanelOpen: boolean;
  /**
   * The heading for the element properties panel.
   */
  resourcePropertiesPanelHeading: ReactNode;
  /**
   * Function to set the state of the element panel.
   */
  setIsResourcePanelOpen: Dispatch<SetStateAction<boolean>>;
  /**
   * Function to set the state of the element properties panel.
   */
  setIsOpenResourcePropertiesPanel: (isOpen: boolean) => void;
  /**
   * Function to set the state of the version history panel.
   */
  setIsVersionHistoryPanelOpen: Dispatch<SetStateAction<boolean>>;
  /**
   * Sets the heading for the element properties panel.
   */
  setResourcePropertiesPanelHeading: Dispatch<SetStateAction<ReactNode>>;
  /**
   * Registers a callback to close the validation panel.
   * This is used for mutual exclusion between the resource properties panel and validation panel.
   */
  registerCloseValidationPanel: (callback: () => void) => void;
}

const UIPanelContext: Context<UIPanelContextProps | undefined> = createContext<UIPanelContextProps | undefined>(
  undefined,
);

UIPanelContext.displayName = 'UIPanelContext';

export default UIPanelContext;
