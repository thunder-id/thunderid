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

import {describe, expect, it} from 'vitest';
import {createApp, h} from 'vue';
import ThunderIDPlugin from '../../plugins/ThunderIDPlugin';

describe('ThunderIDPlugin', () => {
  it('should be a valid Vue plugin with an install method', () => {
    expect(ThunderIDPlugin).toBeDefined();
    expect(typeof ThunderIDPlugin.install).toBe('function');
  });

  it('should install without errors when given a Vue app', () => {
    const app = createApp({
      render() {
        return h('div', 'test');
      },
    });

    expect(() => {
      app.use(ThunderIDPlugin);
    }).not.toThrow();
  });

  it('should register ThunderIDProvider as a global component', () => {
    const app = createApp({
      render() {
        return h('div', 'test');
      },
    });

    app.use(ThunderIDPlugin);

    // Vue 3 stores global component registrations internally
    // We verify the component was registered by checking the app's component resolution
    expect(app.component('ThunderIDProvider')).toBeDefined();
  });
});
