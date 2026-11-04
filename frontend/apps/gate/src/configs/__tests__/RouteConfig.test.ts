// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

  it('has device path', () => {
    expect(RouteConfig.device()).toBe('/device');
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
      device: () => '/device',
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
