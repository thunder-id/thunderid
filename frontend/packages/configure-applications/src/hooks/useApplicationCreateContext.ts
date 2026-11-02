// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import ApplicationCreateContext, {
  type ApplicationCreateContextType,
} from '../contexts/ApplicationCreate/ApplicationCreateContext';

/**
 * Custom React hook to access the Application Create context.
 *
 * This hook provides access to all state and actions needed for the application
 * creation flow. It must be used within an ApplicationCreateProvider component.
 *
 * @throws {Error} If used outside of an ApplicationCreateProvider
 * @returns The application creation context containing state and actions
 *
 * @example
 * ```tsx
 * import useApplicationCreateContext from '@/hooks/useApplicationCreateContext';
 *
 * function MyComponent() {
 *   const {
 *     appName,
 *     setAppName,
 *     currentStep,
 *     selectedAuthFlow,
 *     setSelectedAuthFlow
 *   } = useApplicationCreateContext();
 *
 *   return (
 *     <div>
 *       <h1>Creating: {appName}</h1>
 *       <p>Step: {currentStep}</p>
 *       {selectedAuthFlow && <p>Selected Flow: {selectedAuthFlow.name}</p>}
 *     </div>
 *   );
 * }
 * ```
 *
 * @public
 */
export default function useApplicationCreateContext(): ApplicationCreateContextType {
  const context = useContext(ApplicationCreateContext);

  if (!context) {
    throw new Error(
      'useApplicationCreateContext must be used within an ApplicationCreateProvider. ' +
        'Make sure your component is wrapped with ApplicationCreateProvider.',
    );
  }

  return context;
}
