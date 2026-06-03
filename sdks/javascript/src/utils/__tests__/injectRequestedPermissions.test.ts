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

import {describe, expect, it} from 'vitest';
import injectRequestedPermissions from '../../utils/v2/injectRequestedPermissions';

describe('injectRequestedPermissions', (): void => {
  it('joins multiple scopes into a space-separated requested_permissions string', (): void => {
    const result = injectRequestedPermissions({
      applicationId: 'app-1',
      flowType: 'AUTHENTICATION',
      scopes: ['openid', 'profile', 'email'],
    });

    expect(result).toMatchObject({inputs: {requested_permissions: 'openid profile email'}});
    expect(result).not.toHaveProperty('scopes');
  });

  it('handles a single scope', (): void => {
    const result = injectRequestedPermissions({applicationId: 'app-1', flowType: 'AUTHENTICATION', scopes: ['openid']});

    expect(result).toMatchObject({inputs: {requested_permissions: 'openid'}});
  });

  it('removes scopes from the payload when the array is empty', (): void => {
    const result = injectRequestedPermissions({applicationId: 'app-1', flowType: 'AUTHENTICATION', scopes: []});

    expect(result).not.toHaveProperty('scopes');
    expect(result).not.toHaveProperty('inputs');
  });

  it('returns the payload unchanged (minus scopes) when scopes is absent', (): void => {
    const result = injectRequestedPermissions({applicationId: 'app-1', flowType: 'AUTHENTICATION'});

    expect(result).toEqual({applicationId: 'app-1', flowType: 'AUTHENTICATION'});
    expect(result).not.toHaveProperty('inputs');
  });

  it('merges requested_permissions into existing inputs without overwriting other keys', (): void => {
    const result = injectRequestedPermissions({
      applicationId: 'app-1',
      flowType: 'AUTHENTICATION',
      inputs: {someKey: 'someValue'},
      scopes: ['openid', 'profile'],
    });

    expect(result).toMatchObject({inputs: {requested_permissions: 'openid profile', someKey: 'someValue'}});
  });

  it('replaces a non-object inputs value with just requested_permissions', (): void => {
    const result = injectRequestedPermissions({
      applicationId: 'app-1',
      flowType: 'AUTHENTICATION',
      inputs: 'invalid-string',
      scopes: ['openid'],
    });

    expect(result).toMatchObject({inputs: {requested_permissions: 'openid'}});
    expect((result['inputs'] as Record<string, unknown>)['invalid-string']).toBeUndefined();
  });

  it('replaces an array inputs value with just requested_permissions', (): void => {
    const result = injectRequestedPermissions({
      applicationId: 'app-1',
      flowType: 'AUTHENTICATION',
      inputs: ['should', 'be', 'ignored'],
      scopes: ['openid'],
    });

    expect(result).toMatchObject({inputs: {requested_permissions: 'openid'}});
  });

  it('preserves all other payload fields', (): void => {
    const result = injectRequestedPermissions({
      applicationId: 'app-1',
      flowType: 'AUTHENTICATION',
      scopes: ['openid'],
      verbose: true,
    });

    expect(result).toMatchObject({applicationId: 'app-1', flowType: 'AUTHENTICATION', verbose: true});
  });

  it('accepts a space-separated string as scopes', (): void => {
    const result = injectRequestedPermissions({
      applicationId: 'app-1',
      flowType: 'AUTHENTICATION',
      scopes: 'openid profile email',
    });

    expect(result).toMatchObject({inputs: {requested_permissions: 'openid profile email'}});
    expect(result).not.toHaveProperty('scopes');
  });

  it('trims a string scopes value', (): void => {
    const result = injectRequestedPermissions({
      applicationId: 'app-1',
      flowType: 'AUTHENTICATION',
      scopes: '  openid profile  ',
    });

    expect(result).toMatchObject({inputs: {requested_permissions: 'openid profile'}});
  });

  it('treats a whitespace-only string scopes as absent', (): void => {
    const result = injectRequestedPermissions({applicationId: 'app-1', flowType: 'AUTHENTICATION', scopes: '   '});

    expect(result).not.toHaveProperty('scopes');
    expect(result).not.toHaveProperty('inputs');
  });

  it('treats a non-string non-array scopes value as absent', (): void => {
    const result = injectRequestedPermissions({applicationId: 'app-1', flowType: 'AUTHENTICATION', scopes: 42});

    expect(result).not.toHaveProperty('scopes');
    expect(result).not.toHaveProperty('inputs');
  });

  it('trims and filters blank entries in a scopes array', (): void => {
    const result = injectRequestedPermissions({
      applicationId: 'app-1',
      flowType: 'AUTHENTICATION',
      scopes: ['openid', '  ', 'profile'],
    });

    expect(result).toMatchObject({inputs: {requested_permissions: 'openid profile'}});
    expect(result).not.toHaveProperty('scopes');
  });
});
