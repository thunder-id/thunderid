// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {useMemo} from 'react';
import elements from '../data/elements.json';
import executors from '../data/executors.json';
import steps from '../data/steps.json';
import templates from '../data/templates.json';
import widgets from '../data/widgets.json';
import {type Resources} from '../models/resources';
import updateTemplatePlaceholderReferences from '../utils/updateTemplatePlaceholderReferences';

/**
 * Hook to get all the resources supported by the flow builder.
 *
 * This resolves `{{productName}}` placeholders for any
 * branded value that must reflect the deployment's configured product name,
 * at load time from `config.brand.product_name`.
 *
 * This function calls the GET method of the following endpoint to get the resources.
 * - TODO: Fill this
 * For more details, refer to the documentation:
 * {@link https://TODO:<fillthis>)}
 *
 * @returns SWR response object containing the data, error, isLoading, isValidating, mutate.
 */
const useGetFlowBuilderResources = <Data = Resources>() => {
  const {config} = useConfig();
  const productName = config?.brand?.product_name ?? '';

  const data: unknown = useMemo(() => {
    const [resolved] = updateTemplatePlaceholderReferences(
      {
        elements,
        executors,
        steps,
        templates,
        widgets,
      },
      [{key: 'productName', value: productName}],
    );

    return resolved;
  }, [productName]);

  return {
    data: data as Data,
    error: null,
    isLoading: false,
    isValidating: false,
    mutate: () => null,
  };
};

export default useGetFlowBuilderResources;
