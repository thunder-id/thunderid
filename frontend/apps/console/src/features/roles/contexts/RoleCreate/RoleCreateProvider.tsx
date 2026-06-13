/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type {PropsWithChildren} from 'react';
import {useState, useMemo, useCallback} from 'react';
import RoleCreateContext, {type RoleCreateContextType} from './RoleCreateContext';
import type {ResourcePermissions} from '../../models/role';
import {RoleCreateFlowStep} from '../../models/role-create-flow';

const INITIAL_STATE = {
  currentStep: RoleCreateFlowStep.BASIC_INFO as RoleCreateFlowStep,
  name: '',
  ouId: '',
  error: null as string | null,
  permissions: [] as ResourcePermissions[],
};

/**
 * React context provider component that provides role creation state
 * to all child components in the wizard flow.
 *
 * @public
 */
export default function RoleCreateProvider({children}: PropsWithChildren) {
  const [currentStep, setCurrentStep] = useState<RoleCreateFlowStep>(INITIAL_STATE.currentStep);
  const [name, setName] = useState<string>(INITIAL_STATE.name);
  const [ouId, setOuId] = useState<string>(INITIAL_STATE.ouId);
  const [error, setError] = useState<string | null>(INITIAL_STATE.error);
  const [permissions, setPermissions] = useState<ResourcePermissions[]>(INITIAL_STATE.permissions);

  const reset = useCallback((): void => {
    setCurrentStep(INITIAL_STATE.currentStep);
    setName(INITIAL_STATE.name);
    setOuId(INITIAL_STATE.ouId);
    setError(INITIAL_STATE.error);
    setPermissions(INITIAL_STATE.permissions);
  }, []);

  const contextValue: RoleCreateContextType = useMemo(
    () => ({
      currentStep,
      setCurrentStep,
      name,
      setName,
      ouId,
      setOuId,
      error,
      setError,
      permissions,
      setPermissions,
      reset,
    }),
    [currentStep, name, ouId, error, permissions, reset],
  );

  return <RoleCreateContext.Provider value={contextValue}>{children}</RoleCreateContext.Provider>;
}
