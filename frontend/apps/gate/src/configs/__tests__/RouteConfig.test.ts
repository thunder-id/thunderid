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
import RouteConfig, {type RouteConfig as RouteConfigType} from '../RouteConfig';

describe('RouteConfig', () => {
  it('exports RouteConfig object', () => {
    expect(RouteConfig).toBeDefined();
  });

  it('has root path', () => {
    expect(RouteConfig.root()).toBe('/');
  });

  it('has error path', () => {
    expect(RouteConfig.error()).toBe('/error');
  });

  it('has signIn path', () => {
    expect(RouteConfig.signIn()).toBe('/signin');
  });

  it('has signUp path', () => {
    expect(RouteConfig.signUp()).toBe('/signup');
  });

  it('has invite path', () => {
    expect(RouteConfig.invite()).toBe('/invite');
  });

  it('has callback path', () => {
    expect(RouteConfig.callback()).toBe('/callback');
  });

  it('has signout path', () => {
    expect(RouteConfig.signout()).toBe('/signout');
  });

  it('RouteConfig interface has correct structure', () => {
    const routes: RouteConfigType = {
      root: () => '/',
      error: () => '/error',
      signIn: () => '/signin',
      signUp: () => '/signup',
      invite: () => '/invite',
      callback: () => '/callback',
      recovery: () => '/recovery',
      signout: () => '/signout',
    };
    expect(routes.root()).toBe('/');
    expect(routes.error()).toBe('/error');
    expect(routes.signIn()).toBe('/signin');
    expect(routes.signUp()).toBe('/signup');
    expect(routes.invite()).toBe('/invite');
    expect(routes.callback()).toBe('/callback');
  });
});
