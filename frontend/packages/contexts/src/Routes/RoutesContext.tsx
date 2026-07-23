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

import {Context, createContext} from 'react';

/**
 * Flat bag of route-building functions, keyed by feature domain (e.g. `organizationUnits`, `users`).
 *
 * The host application owns the concrete shape and URL strings. Feature packages only depend on
 * the slice of this map they need, expressed as their own local interface.
 *
 * @public
 */
export type RoutePaths = object;

/**
 * React context that carries the host application's route configuration.
 *
 * Defaults to an empty object rather than `undefined` so that feature packages can be rendered
 * standalone (e.g. in Storybook or unit tests) without a `RoutesProvider` ancestor, falling back
 * to their own default paths. Consume via `useRoutes` rather than this context directly.
 *
 * @public
 */
const RoutesContext: Context<RoutePaths> = createContext<RoutePaths>({});

export default RoutesContext;
