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

import {Google, GitHub} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {IdentityProviderTypes} from '../models/identity-provider';

/**
 * Get the identity provider icon component for a given provider type.
 *
 * Returns the appropriate icon component based on the identity provider type.
 * Supports common social login providers like Google and GitHub.
 *
 * @param type - The identity provider type (e.g., 'GOOGLE', 'GITHUB')
 * @returns The corresponding JSX icon component, or `null` if the type is not supported
 *
 * @public
 * @example
 * ```tsx
 * const icon = getIcon(IdentityProviderTypes.GOOGLE); // Returns <Google />
 * const unknownIcon = getIcon('UNKNOWN'); // Returns null
 * ```
 */
const getConnectionIcon = (type: string): JSX.Element | null => {
  if (type === IdentityProviderTypes.GOOGLE) return <Google />;
  if (type === IdentityProviderTypes.GITHUB) return <GitHub />;

  return null;
};

export default getConnectionIcon;
