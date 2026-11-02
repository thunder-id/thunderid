// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {generateHandle} from '@thunderid/utils';
import {useCallback, useState} from 'react';

/**
 * Props for the useFlowNaming hook.
 */
export interface UseFlowNamingProps {
  /** The existing flow data loaded from the API. */
  existingFlowData?: {
    name?: string;
    handle?: string;
  };
  /** Default name for new flows. */
  defaultName?: string;
  /** Default handle for new flows. */
  defaultHandle?: string;
}

/**
 * Return type for the useFlowNaming hook.
 */
export interface UseFlowNamingReturn {
  /** Current flow name. */
  flowName: string;
  /** Current flow handle (URL-friendly identifier). */
  flowHandle: string;
  /** Whether the loaded flow needs auto-layout. */
  needsAutoLayout: boolean;
  /** Set whether auto-layout is needed. */
  setNeedsAutoLayout: React.Dispatch<React.SetStateAction<boolean>>;
  /** Handler for flow name changes - also updates the handle. */
  handleFlowNameChange: (newName: string) => void;
}

/**
 * Hook to manage flow naming (name and handle) state and logic.
 * Handles synchronization with existing flow data and generates
 * URL-friendly handles from flow names.
 *
 * @param props - Configuration options for the hook.
 * @returns Flow naming state and handlers.
 *
 * @example
 * ```tsx
 * const { flowName, flowHandle, handleFlowNameChange } = useFlowNaming({
 *   existingFlowData,
 *   defaultName: 'Login Flow',
 *   defaultHandle: 'login-flow',
 * });
 * ```
 */
const useFlowNaming = (props?: UseFlowNamingProps): UseFlowNamingReturn => {
  const {existingFlowData, defaultName = 'Login Flow', defaultHandle = 'login-flow'} = props ?? {};

  const [flowName, setFlowName] = useState<string>(() => existingFlowData?.name ?? defaultName);
  const [flowHandle, setFlowHandle] = useState<string>(() => {
    if (existingFlowData?.handle) return existingFlowData.handle;
    if (existingFlowData?.name) return generateHandle(existingFlowData.name);
    return defaultHandle;
  });
  const [needsAutoLayout, setNeedsAutoLayout] = useState<boolean>(false);

  // Sync flowName and flowHandle when existingFlowData changes after initial render
  const [prevExistingFlowData, setPrevExistingFlowData] = useState(existingFlowData);
  if (existingFlowData !== prevExistingFlowData) {
    setPrevExistingFlowData(existingFlowData);
    if (existingFlowData?.name) {
      setFlowName(existingFlowData.name);
    }
    const handle = existingFlowData?.handle;
    if (handle) {
      setFlowHandle(handle);
    } else if (existingFlowData?.name) {
      setFlowHandle(generateHandle(existingFlowData.name));
    }
  }

  /**
   * Handler for flow name changes.
   * Updates both the name and generates a new handle.
   */
  const isExistingFlow = Boolean(existingFlowData?.handle);

  const handleFlowNameChange = useCallback(
    (newName: string) => {
      setFlowName(newName);
      if (!isExistingFlow) {
        setFlowHandle(generateHandle(newName));
      }
    },
    [isExistingFlow],
  );

  return {
    flowName,
    flowHandle,
    needsAutoLayout,
    setNeedsAutoLayout,
    handleFlowNameChange,
  };
};

export default useFlowNaming;
