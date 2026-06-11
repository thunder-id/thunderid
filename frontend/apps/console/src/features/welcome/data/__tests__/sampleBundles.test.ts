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
import {buildRegistry, bundleKeyFromPath} from '../sampleBundles';

describe('bundleKeyFromPath', () => {
  it('returns the bundle directory for a single-level bundle', () => {
    expect(bundleKeyFromPath('./sample-bundles/wayfinder/thunderid-config.yaml')).toBe('wayfinder');
  });

  it('preserves nested grouping in the bundle key', () => {
    expect(bundleKeyFromPath('./sample-bundles/wayfinder/redirect-based/thunderid-config.yaml')).toBe(
      'wayfinder/redirect-based',
    );
  });

  it('returns the file stem when the file sits directly under the bundle root', () => {
    expect(bundleKeyFromPath('./sample-bundles/loose.yaml')).toBe('loose.yaml');
  });
});

describe('buildRegistry', () => {
  it('returns an empty registry when no modules are passed', () => {
    expect(buildRegistry({}, {})).toEqual({});
  });

  it('builds a registry entry from a yaml-only bundle', () => {
    const registry = buildRegistry({'./sample-bundles/wayfinder/thunderid-config.yaml': 'yaml body'}, {});
    expect(registry).toEqual({
      wayfinder: {configs: {declarative: 'yaml body'}},
    });
  });

  it('merges yaml + env files into one bundle entry', () => {
    const registry = buildRegistry(
      {'./sample-bundles/wayfinder/thunderid-config.yaml': 'yaml body'},
      {'./sample-bundles/wayfinder/thunderid.env': 'KEY=value'},
    );
    expect(registry).toEqual({
      wayfinder: {configs: {declarative: 'yaml body', env: 'KEY=value'}},
    });
  });

  it('keys nested bundles by their directory path so variants stay distinct', () => {
    const registry = buildRegistry(
      {
        './sample-bundles/wayfinder/redirect-based/thunderid-config.yaml': 'redirect yaml',
        './sample-bundles/wayfinder/app-native/thunderid-config.yaml': 'native yaml',
      },
      {
        './sample-bundles/wayfinder/redirect-based/thunderid.env': 'REDIRECT=1',
      },
    );
    expect(registry).toEqual({
      'wayfinder/redirect-based': {configs: {declarative: 'redirect yaml', env: 'REDIRECT=1'}},
      'wayfinder/app-native': {configs: {declarative: 'native yaml'}},
    });
  });

  it('exposes only env when a bundle has env but no yaml', () => {
    const registry = buildRegistry({}, {'./sample-bundles/orphan/thunderid.env': 'X=1'});
    expect(registry).toEqual({
      orphan: {configs: {env: 'X=1'}},
    });
  });
});
