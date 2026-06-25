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

import {Folder, FolderOpen} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';

/**
 * Returns the namespace icon, toggling between open and closed states.
 *
 * @param expanded - Whether the namespace is expanded.
 * @param size - Icon size in pixels (default 16).
 * @returns The icon JSX element.
 */
export function getNamespaceIcon(expanded: boolean, size = 16): JSX.Element {
  return expanded ? <FolderOpen size={size} /> : <Folder size={size} />;
}
