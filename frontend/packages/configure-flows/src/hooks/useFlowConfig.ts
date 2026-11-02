// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import FlowConfigContext, {type FlowConfigContextProps} from '../context/FlowConfigContext';

/**
 * Hook that provides access to flow configuration (metadata, verbose mode, edge style, node/edge types, factories).
 *
 * Use this hook when a component needs flow-level settings that rarely change after initialization.
 * For other context domains, see useUIPanelState, useInteractionState, useI18nConfig.
 */
const useFlowConfig = (): FlowConfigContextProps => {
  const context = useContext(FlowConfigContext);

  if (context === undefined) {
    throw new Error('useFlowConfig must be used within a FlowBuilderCoreProvider');
  }

  return context;
};

export default useFlowConfig;
