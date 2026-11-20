// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMemo} from 'react';
import elements from '../data/elements.json';
import executors from '../data/executors.json';
import steps from '../data/steps.json';
import templates from '../data/templates.json';
import widgets from '../data/widgets.json';
import {type Resources} from '../models/resources';

/**
 * Hook to get all the resources supported by the flow builder.
 *
 * This function calls the GET method of the following endpoint to get the resources.
 * - TODO: Fill this
 * For more details, refer to the documentation:
 * {@link https://TODO:<fillthis>)}
 *
 * @returns SWR response object containing the data, error, isLoading, isValidating, mutate.
 */
const useGetFlowBuilderResources = <Data = Resources>() => {
  const data: unknown = useMemo(
    () => ({
      elements,
      executors,
      steps,
      templates,
      widgets,
    }),
    [],
  );

  return {
    data: data as Data,
    error: null,
    isLoading: false,
    isValidating: false,
    mutate: () => null,
  };
};

export default useGetFlowBuilderResources;
