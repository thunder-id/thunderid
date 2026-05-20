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

import {NextRequest} from 'next/server';

/**
 * Creates a route matcher function that tests if a request matches any of the given patterns.
 *
 * @param patterns - Array of route patterns to match. Supports glob-like patterns.
 * @returns Function that tests if a request matches any of the patterns
 *
 * @example
 * ```typescript
 * const isProtectedRoute = createRouteMatcher([
 *   '/dashboard(.*)',
 *   '/admin(.*)',
 *   '/profile'
 * ]);
 *
 * if (isProtectedRoute(req)) {
 *   // Route is protected
 * }
 * ```
 */
const createRouteMatcher = (patterns: string[]): ((req: NextRequest) => boolean) => {
  const regexPatterns: RegExp[] = patterns.map((pattern: string): RegExp => {
    // Convert glob-like patterns to regex
    const regexPattern: string = pattern
      .replace(/\./g, '\\.') // Escape dots
      .replace(/\*/g, '.*') // Convert * to .*
      .replace(/\(\.\*\)/g, '(.*)'); // Handle explicit (.*) patterns

    return new RegExp(`^${regexPattern}$`);
  });

  return (req: NextRequest): boolean => {
    const {pathname} = req.nextUrl;
    return regexPatterns.some((regex: RegExp): boolean => regex.test(pathname));
  };
};

export default createRouteMatcher;
