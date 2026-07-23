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

import {useSyncExternalStore} from 'react';

export type ConnectType = 'app' | 'agent' | 'mcp';

const DEFAULT_TYPE: ConnectType = 'app';

// Shared in-memory state so the sidebar accordion and the docs-home selector
// stay in sync: changing one updates the other live. It is deliberately NOT
// persisted, so there is no storage access that could throw. The choice is kept
// across in-app navigation (the module stays loaded) and resets to the default
// on a full reload, which always starts the page on "Application".
let current: ConnectType = DEFAULT_TYPE;
const listeners = new Set<() => void>();

export function applyConnectType(type: ConnectType): void {
  current = type;
  listeners.forEach(fn => fn());
}

function subscribe(fn: () => void): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

export function useConnectType(): ConnectType {
  return useSyncExternalStore(subscribe, () => current, () => DEFAULT_TYPE);
}
