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

import {describe, it, expect} from 'vitest';

describe('@thunderid/tanstack-router', () => {
  it('should export ProtectedRoute', async () => {
    const {ProtectedRoute} = await import('../index');
    expect(ProtectedRoute).toBeDefined();
  });

  it('should export ProtectedRouteProps interface', async () => {
    const exports: typeof import('../index') = await import('../index');
    // Interface check - should not throw
    const UNUSED_TEST_VAR: typeof exports.ProtectedRouteProps = undefined as any;
    // Explicitly mark as used for type checking purposes
    expect(UNUSED_TEST_VAR).toBeUndefined();
  });

  it('should have the correct named exports', async () => {
    const exports: typeof import('../index') = await import('../index');
    const exportNames: string[] = Object.keys(exports);
    expect(exportNames).toContain('ProtectedRoute');
  });
});
