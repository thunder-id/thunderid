// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import AgentTypeCreateContext, {type AgentTypeCreateContextType} from './AgentTypeCreateContext';

/**
 * Hook to access the agent type creation context.
 *
 * @throws Error if used outside of AgentTypeCreateProvider
 * @returns The agent type creation context value
 *
 * @public
 */
export default function useAgentTypeCreate(): AgentTypeCreateContextType {
  const context = useContext(AgentTypeCreateContext);

  if (!context) {
    throw new Error('useAgentTypeCreate must be used within an AgentTypeCreateProvider');
  }

  return context;
}
