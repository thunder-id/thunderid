/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {useConfig} from '@thunderid/contexts';
import {useMemo} from 'react';
import elements from '../data/elements.json';
import steps from '../data/steps.json';
import templates from '../data/templates.json';
import widgets from '../data/widgets.json';
import {type Resources} from '../models/resources';
import updateTemplatePlaceholderReferences from '../utils/updateTemplatePlaceholderReferences';

/**
 * Hook to get all the resources supported by the flow builder.
 *
 * Static resource JSON ships with `{{productName}}` placeholders for any
 * branded value that must reflect the deployment's configured product name
 * (currently the WebAuthn relying party name in passkey templates). The
 * placeholder is resolved at load time from `config.brand.product_name` so
 * every consumer sees the correct value without further work.
 *
 * This function calls the GET method of the following endpoint to get the resources.
 * - TODO: Fill this
 * For more details, refer to the documentation:
 * {@link https://TODO:<fillthis>)}
 *
 * @returns SWR response object containing the data, error, isLoading, isValidating, mutate.
 */
const useGetFlowBuilderCoreResources = <Data = Resources>() => {
  const {config} = useConfig();
  const productName = config?.brand?.product_name ?? '';

  const data: unknown = useMemo(() => {
    const [resolved] = updateTemplatePlaceholderReferences(
      {
        elements,
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

export default useGetFlowBuilderCoreResources;
