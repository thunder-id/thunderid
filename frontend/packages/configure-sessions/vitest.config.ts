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

import {resolve} from 'path';
import {playwright} from '@vitest/browser-playwright';
import {defineConfig} from 'vitest/config';

export default defineConfig({
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      '@/api': resolve(__dirname, 'src', 'api'),
      '@/components': resolve(__dirname, 'src', 'components'),
      '@/config': resolve(__dirname, 'src', 'config'),
      '@/constants': resolve(__dirname, 'src', 'constants'),
      '@/contexts': resolve(__dirname, 'src', 'contexts'),
      '@/data': resolve(__dirname, 'src', 'data'),
      '@/hooks': resolve(__dirname, 'src', 'hooks'),
      '@/models': resolve(__dirname, 'src', 'models'),
      '@/pages': resolve(__dirname, 'src', 'pages'),
      '@/utils': resolve(__dirname, 'src', 'utils'),
    },
  },
  test: {
    browser: {
      enabled: true,
      headless: true,
      instances: [{browser: 'chromium'}],
      provider: playwright(),
    },
    setupFiles: ['@thunderid/test-utils/setup'],
  },
});
