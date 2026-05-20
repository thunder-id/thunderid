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

export {default} from './module';

// ── Types ──────────────────────────────────────────────────────────────────
export type {ThunderIDNuxtConfig, ThunderIDSessionPayload, ThunderIDAuthState} from './runtime/types';

// ── Composables ────────────────────────────────────────────────────────────
// The Nuxt-specific `useThunderID` layers navigation overrides over Vue's
// `useThunderID`. The rest are re-exports of `@thunderid/vue` composables —
// their contexts are mounted by `<ThunderIDRoot>` (see runtime/components).
export {useThunderID} from './runtime/composables/useThunderID';
export {useUser, useOrganization, useFlow, useFlowMeta, useTheme, useBranding} from '@thunderid/vue';
export {useI18n as useThunderIDI18n} from '@thunderid/vue';

// ── Components ─────────────────────────────────────────────────────────────
export {default as ThunderIDRoot} from './runtime/components/ThunderIDRoot';

// ── Middleware ─────────────────────────────────────────────────────────────
export {defineThunderIDMiddleware} from './runtime/middleware/defineThunderIDMiddleware';
export type {ThunderIDMiddlewareOptions} from './runtime/middleware/defineThunderIDMiddleware';

// ── Composable types (re-exported from @thunderid/vue) ─────────────────────
// Only ThunderIDContext is exposed — it is the return type of useThunderID()
// and users may need it to type custom wrappers. The individual *ContextValue
// types are internal implementation details; use ReturnType<typeof useXxx> instead.
export type {ThunderIDContext} from '@thunderid/vue';

// ── Errors ─────────────────────────────────────────────────────────────────
export {ThunderIDError, ErrorCode} from './runtime/errors';
