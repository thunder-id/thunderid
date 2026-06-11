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

export interface SampleBundleConfigs {
  declarative?: string;
  env?: string;
}

export interface SampleBundle {
  configs: SampleBundleConfigs;
}

const BUNDLE_ROOT = './sample-bundles/';

export const bundleKeyFromPath = (path: string): string => {
  const tail = path.slice(BUNDLE_ROOT.length);
  const lastSlash = tail.lastIndexOf('/');
  return lastSlash === -1 ? tail : tail.slice(0, lastSlash);
};

export const buildRegistry = (
  yamlModules: Record<string, string>,
  envModules: Record<string, string>,
): Record<string, SampleBundle> => {
  const registry: Record<string, SampleBundle> = {};

  const ensure = (key: string): SampleBundle => {
    if (!registry[key]) {
      registry[key] = {configs: {}};
    }
    return registry[key];
  };

  for (const [path, content] of Object.entries(yamlModules)) {
    ensure(bundleKeyFromPath(path)).configs.declarative = content;
  }

  for (const [path, content] of Object.entries(envModules)) {
    ensure(bundleKeyFromPath(path)).configs.env = content;
  }

  return registry;
};

const yamlModules: Record<string, string> = import.meta.glob('./sample-bundles/**/*.{yaml,yml}', {
  query: '?raw',
  import: 'default',
  eager: true,
});

const envModules: Record<string, string> = import.meta.glob('./sample-bundles/**/*.env', {
  query: '?raw',
  import: 'default',
  eager: true,
});

export const SAMPLE_BUNDLES: Record<string, SampleBundle> = buildRegistry(yamlModules, envModules);
