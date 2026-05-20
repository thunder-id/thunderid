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

import {useConfig} from '@thunderid/contexts';
import {isEmpty} from '@thunderid/utils';
import {useMemo, type PropsWithChildren} from 'react';
import DesignContext, {type DesignContextType} from './DesignContext';
import useGetDesignResolve from '../../api/useGetDesignResolve';
import {DesignResolveType} from '../../models/design';
import type {DesignResolveResponse} from '../../models/responses';

/**
 * Props for the DesignProvider component.
 *
 * @public
 */
export type DesignProviderProps = PropsWithChildren<{
  /**
   * Optional pre-resolved design to use directly, skipping the internal
   * resolve API call. Useful when the host SDK (e.g. ThunderID) already
   * provides design data via its metadata (e.g. `meta.design`).
   */
  design?: DesignResolveResponse;

  /**
   * Flag to indicate that the Design should be resolved internally by the DesignProvider.
   * This is useful if flow metadata does not contain design information and the design needs to be resolved using the client UUID.
   */
  shouldResolveDesignInternally?: boolean;

  /**
   * Signals that an external source (e.g. ThunderID meta) is still loading.
   * When true, isLoading is reported as true so consumers can defer rendering
   * until the design data has actually arrived.
   */
  isLoading?: boolean;
}>;

/**
 * React context provider component that provides design configuration
 * to all child components.
 *
 * This component loads design data from the server using the client UUID
 * and provides it through React context. Theme transformation is handled
 * at the hook level via useDesign().
 *
 * @param props - The component props
 * @param props.children - React children to be wrapped with the design context
 *
 * @returns JSX element that provides design context to children
 *
 * @public
 */
export default function DesignProvider({
  children = null,
  design: externalDesign = undefined,
  shouldResolveDesignInternally = true,
  isLoading: isExternalLoading = false,
}: DesignProviderProps) {
  const {getClientUuid} = useConfig();
  const clientUuid = getClientUuid();

  // Skip internal resolution when no client UUID is available or design is provided externally
  const shouldLoadDesign =
    shouldResolveDesignInternally && !externalDesign && Boolean(clientUuid && clientUuid.trim().length > 0);

  const {
    data: resolvedDesign,
    isLoading,
    error,
  } = useGetDesignResolve(
    {
      id: clientUuid ?? '',
      type: DesignResolveType.APP,
    },
    {
      enabled: shouldLoadDesign,
    },
  );

  const design = externalDesign ?? resolvedDesign;

  const contextValue: DesignContextType = useMemo(
    () => ({
      design,
      isDesignEnabled: Boolean(design) && (!isEmpty(design?.theme) || !isEmpty(design?.layout)),
      isLoading: isExternalLoading || (externalDesign ? false : isLoading),
      error: externalDesign ? null : error,
      theme: undefined,
      layout: isEmpty(design?.layout) ? undefined : design?.layout,
    }),
    [design, externalDesign, isLoading, error, isExternalLoading],
  );

  return <DesignContext.Provider value={contextValue}>{children}</DesignContext.Provider>;
}
