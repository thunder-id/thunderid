// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Context} from 'react';
import {createContext} from 'react';
import type {AgentTypeCreateFlowStep} from '../../models/agent-type-create-flow';
import type {SchemaPropertyInput} from '../../models/property-definition';

/**
 * Agent type creation context state interface.
 *
 * Provides centralized state management for the agent type creation wizard flow.
 *
 * @public
 */
export interface AgentTypeCreateContextType {
  currentStep: AgentTypeCreateFlowStep;
  setCurrentStep: (step: AgentTypeCreateFlowStep) => void;

  name: string;
  setName: (name: string) => void;

  handle: string;
  setHandle: (handle: string) => void;

  handleEdited: boolean;
  setHandleEdited: (edited: boolean) => void;

  ouId: string;
  setOuId: (ouId: string) => void;

  properties: SchemaPropertyInput[];
  setProperties: (properties: SchemaPropertyInput[]) => void;

  displayAttribute: string;
  setDisplayAttribute: (displayAttribute: string) => void;

  error: string | null;
  setError: (error: string | null) => void;

  reset: () => void;
}

const AgentTypeCreateContext: Context<AgentTypeCreateContextType | undefined> = createContext<
  AgentTypeCreateContextType | undefined
>(undefined);

export default AgentTypeCreateContext;
