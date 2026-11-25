// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {PropsWithChildren} from 'react';
import {useState, useMemo, useCallback} from 'react';
import AgentTypeCreateContext, {type AgentTypeCreateContextType} from './AgentTypeCreateContext';
import {AgentTypeCreateFlowStep} from '../../models/agent-type-create-flow';
import type {SchemaPropertyInput} from '../../models/property-definition';

/**
 * Initial state values for agent type creation.
 *
 * @internal
 */
const INITIAL_STATE = {
  currentStep: AgentTypeCreateFlowStep.ORGANIZATION_UNIT as AgentTypeCreateFlowStep,
  name: '',
  handle: '',
  handleEdited: false,
  ouId: '',
  properties: [] satisfies SchemaPropertyInput[],
  displayAttribute: '',
  error: null as string | null,
};

/**
 * React context provider component that provides agent type creation state
 * to all child components in the wizard flow.
 *
 * @public
 */
export default function AgentTypeCreateProvider({children}: PropsWithChildren) {
  const [currentStep, setCurrentStep] = useState<AgentTypeCreateFlowStep>(INITIAL_STATE.currentStep);
  const [name, setName] = useState<string>(INITIAL_STATE.name);
  const [handle, setHandle] = useState<string>(INITIAL_STATE.handle);
  const [handleEdited, setHandleEdited] = useState<boolean>(INITIAL_STATE.handleEdited);
  const [ouId, setOuId] = useState<string>(INITIAL_STATE.ouId);
  const [properties, setProperties] = useState<SchemaPropertyInput[]>(INITIAL_STATE.properties);
  const [displayAttribute, setDisplayAttribute] = useState<string>(INITIAL_STATE.displayAttribute);
  const [error, setError] = useState<string | null>(INITIAL_STATE.error);

  const reset = useCallback((): void => {
    setCurrentStep(INITIAL_STATE.currentStep);
    setName(INITIAL_STATE.name);
    setHandle(INITIAL_STATE.handle);
    setHandleEdited(INITIAL_STATE.handleEdited);
    setOuId(INITIAL_STATE.ouId);
    setProperties(INITIAL_STATE.properties);
    setDisplayAttribute(INITIAL_STATE.displayAttribute);
    setError(INITIAL_STATE.error);
  }, []);

  const contextValue: AgentTypeCreateContextType = useMemo(
    () => ({
      currentStep,
      setCurrentStep,
      name,
      setName,
      handle,
      setHandle,
      handleEdited,
      setHandleEdited,
      ouId,
      setOuId,
      properties,
      setProperties,
      displayAttribute,
      setDisplayAttribute,
      error,
      setError,
      reset,
    }),
    [currentStep, name, handle, handleEdited, ouId, properties, displayAttribute, error, reset],
  );

  return <AgentTypeCreateContext.Provider value={contextValue}>{children}</AgentTypeCreateContext.Provider>;
}
