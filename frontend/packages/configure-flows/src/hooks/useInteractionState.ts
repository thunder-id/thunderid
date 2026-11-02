// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import InteractionContext, {type InteractionContextProps} from '../context/InteractionContext';

/**
 * Hook that provides access to resource interaction state (last interacted resource, step, attributes).
 *
 * Use this hook when a component only needs to know about or respond to resource interactions.
 * For other context domains, see useUIPanelState, useFlowConfig, useI18nConfig.
 */
const useInteractionState = (): InteractionContextProps => {
  const context = useContext(InteractionContext);

  if (context === undefined) {
    throw new Error('useInteractionState must be used within a FlowBuilderCoreProvider');
  }

  return context;
};

export default useInteractionState;
