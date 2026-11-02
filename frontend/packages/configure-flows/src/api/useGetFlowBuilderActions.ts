// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import actions from '../data/actions.json';
import {type Actions} from '../models/actions';

/**
 * Hook to get the actions supported by the flow builder.
 *
 * This function calls the GET method of the following endpoint to get the elements.
 * - TODO: Fill this
 * For more details, refer to the documentation:
 * {@link https://TODO:<fillthis>)}
 *
 * @returns SWR response object containing the data, error, isLoading, isValidating, mutate.
 */
const useGetFlowBuilderActions = <Data = Actions>() => ({
  data: actions as unknown as Data,
  error: null,
  isLoading: false,
  isValidating: false,
  mutate: () => null,
});

export default useGetFlowBuilderActions;
