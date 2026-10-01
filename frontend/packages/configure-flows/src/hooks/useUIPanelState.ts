// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import UIPanelContext, {type UIPanelContextProps} from '../context/UIPanelContext';

/**
 * Hook that provides access to UI panel state (resource panel, properties panel, version history).
 *
 * Use this hook when a component only needs panel open/close state and headings.
 * For other context domains, see useInteractionState, useFlowConfig, useI18nConfig.
 */
const useUIPanelState = (): UIPanelContextProps => {
  const context = useContext(UIPanelContext);

  if (context === undefined) {
    throw new Error('useUIPanelState must be used within a FlowBuilderCoreProvider');
  }

  return context;
};

export default useUIPanelState;
