// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import FlowEventsContext, {type FlowEventsContextProps} from '../context/FlowEventsContext';

/**
 * Hook that provides access to the flow events bus for dispatching and subscribing
 * to cross-component events (element added, auto-layout, restore from history).
 */
const useFlowEvents = (): FlowEventsContextProps => {
  const context = useContext(FlowEventsContext);

  if (context === undefined) {
    throw new Error('useFlowEvents must be used within a FlowBuilderCoreProvider');
  }

  return context;
};

export default useFlowEvents;
