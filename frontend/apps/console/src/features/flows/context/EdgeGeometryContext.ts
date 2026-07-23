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

import {createContext, type Context} from 'react';
import type {EdgeInput} from '../utils/calculateEdgePath';

/**
 * Registry where each rendered edge reports the exact endpoint geometry React
 * Flow computed for it, so {@link EdgePathsProvider} can route all edges
 * together (with overlap separation) from the same coordinates the edges
 * would use individually.
 */
export interface EdgeGeometryRegistry {
  register: (input: EdgeInput) => void;
  unregister: (id: string) => void;
}

const EdgeGeometryContext: Context<EdgeGeometryRegistry | null> = createContext<EdgeGeometryRegistry | null>(null);

EdgeGeometryContext.displayName = 'EdgeGeometryContext';

export default EdgeGeometryContext;
