// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {PropsWithChildren} from 'react';
import {useState, useMemo, useCallback} from 'react';
import UserTypeCreateContext, {type UserTypeCreateContextType} from './UserTypeCreateContext';
import {UserTypeCreateFlowStep} from '../../models/user-type-create-flow';
import type {SchemaPropertyInput} from '../../types/user-types';

/**
 * Initial state values for user type creation.
 *
 * @internal
 */
const INITIAL_STATE = {
  currentStep: UserTypeCreateFlowStep.ORGANIZATION_UNIT as UserTypeCreateFlowStep,
  name: '',
  handle: '',
  handleEdited: false,
  ouId: '',
  allowSelfRegistration: false,
  properties: [] satisfies SchemaPropertyInput[],
  enumInput: {} as Record<string, string>,
  displayAttribute: '',
  error: null as string | null,
};

/**
 * React context provider component that provides user type creation state
 * to all child components in the wizard flow.
 *
 * @public
 */
export default function UserTypeCreateProvider({children}: PropsWithChildren) {
  const [currentStep, setCurrentStep] = useState<UserTypeCreateFlowStep>(INITIAL_STATE.currentStep);
  const [name, setName] = useState<string>(INITIAL_STATE.name);
  const [handle, setHandle] = useState<string>(INITIAL_STATE.handle);
  const [handleEdited, setHandleEdited] = useState<boolean>(INITIAL_STATE.handleEdited);
  const [ouId, setOuId] = useState<string>(INITIAL_STATE.ouId);
  const [allowSelfRegistration, setAllowSelfRegistration] = useState<boolean>(INITIAL_STATE.allowSelfRegistration);
  const [properties, setProperties] = useState<SchemaPropertyInput[]>(INITIAL_STATE.properties);
  const [enumInput, setEnumInput] = useState<Record<string, string>>(INITIAL_STATE.enumInput);
  const [displayAttribute, setDisplayAttribute] = useState<string>(INITIAL_STATE.displayAttribute);
  const [error, setError] = useState<string | null>(INITIAL_STATE.error);

  const reset = useCallback((): void => {
    setCurrentStep(INITIAL_STATE.currentStep);
    setName(INITIAL_STATE.name);
    setHandle(INITIAL_STATE.handle);
    setHandleEdited(INITIAL_STATE.handleEdited);
    setOuId(INITIAL_STATE.ouId);
    setAllowSelfRegistration(INITIAL_STATE.allowSelfRegistration);
    setProperties(INITIAL_STATE.properties);
    setEnumInput(INITIAL_STATE.enumInput);
    setDisplayAttribute(INITIAL_STATE.displayAttribute);
    setError(INITIAL_STATE.error);
  }, []);

  const contextValue: UserTypeCreateContextType = useMemo(
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
      allowSelfRegistration,
      setAllowSelfRegistration,
      properties,
      setProperties,
      enumInput,
      setEnumInput,
      displayAttribute,
      setDisplayAttribute,
      error,
      setError,
      reset,
    }),
    [
      currentStep,
      name,
      handle,
      handleEdited,
      ouId,
      allowSelfRegistration,
      properties,
      enumInput,
      displayAttribute,
      error,
      reset,
    ],
  );

  return <UserTypeCreateContext.Provider value={contextValue}>{children}</UserTypeCreateContext.Provider>;
}
