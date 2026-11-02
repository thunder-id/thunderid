// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import ApplicationCreateContext, {type ApplicationCreateContextType} from './ApplicationCreateContext';

/**
 * React hook for accessing application creation state throughout the application.
 *
 * This hook provides access to all the state needed for the multi-step application
 * creation flow. It must be used within a component tree wrapped by
 * `ApplicationCreateProvider`, otherwise it will throw an error.
 *
 * The hook returns a context object containing all state variables, setter functions,
 * and utility methods for managing the application creation process.
 *
 * @returns The application creation context containing state data and utility methods
 *
 * @throws {Error} Throws an error if used outside of ApplicationCreateProvider
 *
 * @example
 * Basic usage:
 * ```tsx
 * import useApplicationCreate from './useApplicationCreate';
 *
 * function MyComponent() {
 *   const { appName, setAppName, currentStep, toggleIntegration } = useApplicationCreate();
 *
 *   return (
 *     <div>
 *       <p>Current step: {currentStep}</p>
 *       <input value={appName} onChange={(e) => setAppName(e.target.value)} />
 *     </div>
 *   );
 * }
 * ```
 *
 * @example
 * Using with error boundary:
 * ```tsx
 * import useApplicationCreate from './useApplicationCreate';
 *
 * function StepIndicator() {
 *   try {
 *     const { currentStep } = useApplicationCreate();
 *     return <span>Step: {currentStep}</span>;
 *   } catch (error) {
 *     return <span>Context not available</span>;
 *   }
 * }
 * ```
 *
 * @public
 */
export default function useApplicationCreate(): ApplicationCreateContextType {
  const context = useContext(ApplicationCreateContext);

  if (context === undefined) {
    throw new Error('useApplicationCreate must be used within ApplicationCreateProvider');
  }

  return context;
}
