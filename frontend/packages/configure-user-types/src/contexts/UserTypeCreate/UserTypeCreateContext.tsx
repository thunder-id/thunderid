// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Context} from 'react';
import {createContext} from 'react';
import type {UserTypeCreateFlowStep} from '../../models/user-type-create-flow';
import type {SchemaPropertyInput} from '../../types/user-types';

/**
 * User type creation context state interface.
 *
 * Provides centralized state management for the user type creation wizard flow.
 *
 * @public
 */
export interface UserTypeCreateContextType {
  currentStep: UserTypeCreateFlowStep;
  setCurrentStep: (step: UserTypeCreateFlowStep) => void;

  name: string;
  setName: (name: string) => void;

  handle: string;
  setHandle: (handle: string) => void;

  handleEdited: boolean;
  setHandleEdited: (edited: boolean) => void;

  ouId: string;
  setOuId: (ouId: string) => void;

  allowSelfRegistration: boolean;
  setAllowSelfRegistration: (allow: boolean) => void;

  properties: SchemaPropertyInput[];
  setProperties: (properties: SchemaPropertyInput[]) => void;

  enumInput: Record<string, string>;
  setEnumInput: (enumInput: Record<string, string>) => void;

  displayAttribute: string;
  setDisplayAttribute: (displayAttribute: string) => void;

  error: string | null;
  setError: (error: string | null) => void;

  reset: () => void;
}

const UserTypeCreateContext: Context<UserTypeCreateContextType | undefined> = createContext<
  UserTypeCreateContextType | undefined
>(undefined);

export default UserTypeCreateContext;
