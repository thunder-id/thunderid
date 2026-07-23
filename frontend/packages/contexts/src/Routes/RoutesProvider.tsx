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

import type {JSX, PropsWithChildren} from 'react';
import RoutesContext, {type RoutePaths} from './RoutesContext';

/**
 * Props for the RoutesProvider component.
 *
 * @public
 */
export interface RoutesProviderProps<T extends RoutePaths = RoutePaths> extends PropsWithChildren {
  /**
   * The application's complete route configuration: a flat object keyed by feature domain,
   * where each value is the set of path-building functions for that domain.
   */
  paths: T;
}

/**
 * React context provider that lets the host application declare the URL structure used by
 * every feature package mounted beneath it.
 *
 * Feature packages never hardcode destination paths. Instead they read the slice of `paths`
 * they need through a package-level hook built on top of `useRoutes`, falling back to their
 * own defaults when a key is absent. This lets the same package be mounted under different
 * URL structures by different host applications.
 *
 * @example
 * Wiring it up once, near the application root:
 * ```tsx
 * import { RoutesProvider } from '@thunderid/contexts';
 * import { appRoutePaths } from './routes/appRoutePaths';
 *
 * function App() {
 *   return (
 *     <RoutesProvider paths={appRoutePaths}>
 *       <Routes />
 *     </RoutesProvider>
 *   );
 * }
 * ```
 *
 * @public
 */
export default function RoutesProvider<T extends RoutePaths = RoutePaths>({
  paths,
  children,
}: RoutesProviderProps<T>): JSX.Element {
  return <RoutesContext.Provider value={paths}>{children}</RoutesContext.Provider>;
}
