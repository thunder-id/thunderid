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

import {defineThunderIDMiddleware} from './defineThunderIDMiddleware';

/**
 * Named route middleware for protecting pages.
 *
 * Registered under the name `'auth'` by the Nuxt module, so pages can
 * opt in by string reference:
 *
 * ```vue
 * <script setup>
 * definePageMeta({ middleware: ['auth'] });
 * </script>
 * ```
 *
 * Equivalent to `defineThunderIDMiddleware()` with no options: redirects
 * unauthenticated users to `/api/auth/signin?returnTo=<path>`. For scope
 * or organization gating, use `defineThunderIDMiddleware({ ... })` directly.
 */
export default defineThunderIDMiddleware();
