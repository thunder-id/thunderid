// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import FlowPluginContext, {type FlowPluginContextProps} from '../context/FlowPluginContext';

/**
 * Hook that provides access to the flow plugin registry for subscribing to
 * and emitting interceptor-style events (property changes, element filtering,
 * deletion cascades, etc.).
 */
const useFlowPlugins = (): FlowPluginContextProps => {
  const context = useContext(FlowPluginContext);

  if (context === undefined) {
    throw new Error('useFlowPlugins must be used within a FlowBuilderCoreProvider');
  }

  return context;
};

export default useFlowPlugins;
