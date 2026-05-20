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

import {describe, it, expect} from 'vitest';
import ROUTES, {type Routes} from '../routes';

describe('ROUTES', () => {
  it('exports ROUTES object', () => {
    expect(ROUTES).toBeDefined();
  });

  it('has ROOT path', () => {
    expect(ROUTES.ROOT).toBe('/');
  });

  it('has AUTH object', () => {
    expect(ROUTES.AUTH).toBeDefined();
  });

  it('has AUTH.ERROR path', () => {
    expect(ROUTES.AUTH.ERROR).toBe('/error');
  });

  it('has AUTH.SIGN_IN path', () => {
    expect(ROUTES.AUTH.SIGN_IN).toBe('/signin');
  });

  it('has AUTH.SIGN_UP path', () => {
    expect(ROUTES.AUTH.SIGN_UP).toBe('/signup');
  });

  it('has AUTH.INVITE path', () => {
    expect(ROUTES.AUTH.INVITE).toBe('/invite');
  });

  it('has AUTH.CALLBACK path', () => {
    expect(ROUTES.AUTH.CALLBACK).toBe('/callback');
  });

  it('Routes interface has correct structure', () => {
    const routes: Routes = {
      ROOT: '/',
      AUTH: {
        ERROR: '/error',
        SIGN_IN: '/signin',
        SIGN_UP: '/signup',
        INVITE: '/invite',
        CALLBACK: '/callback',
        RECOVERY: '/recovery',
      },
    };
    expect(routes.ROOT).toBe('/');
    expect(routes.AUTH.ERROR).toBe('/error');
    expect(routes.AUTH.SIGN_IN).toBe('/signin');
    expect(routes.AUTH.SIGN_UP).toBe('/signup');
    expect(routes.AUTH.INVITE).toBe('/invite');
    expect(routes.AUTH.CALLBACK).toBe('/callback');
  });
});
