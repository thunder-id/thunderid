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

import {Database, Folder, Wrench} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import type {ActionKind} from '../models/resource-server';

/**
 * Returns the icon element for the given action kind. Pass `undefined` to get the Namespace (Folder) icon.
 *
 * @param kind - The action kind ('tool', 'resource', or undefined for namespace).
 * @param size - Icon size in pixels (default 16).
 * @returns The icon JSX element.
 */
export function getActionKindIcon(kind: ActionKind | undefined, size = 16): JSX.Element {
  if (kind === 'tool') return <Wrench size={size} />;
  if (kind === 'resource') return <Database size={size} />;
  return <Folder size={size} />;
}
