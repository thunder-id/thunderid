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

import {inject} from 'vue';
import {FLOW_KEY} from '../keys';
import type {FlowContextValue} from '../models/contexts';

/**
 * Composable for managing authentication flow UI state.
 *
 * Must be called inside a component that is a descendant of `<ThunderIDProvider>`.
 *
 * @returns {FlowContextValue} The flow context with step navigation, messages, and loading state.
 * @throws {Error} If called outside of `<ThunderIDProvider>`.
 *
 * @example
 * ```vue
 * <script setup>
 * import { useFlow } from '@thunderid/vue';
 *
 * const { currentStep, isLoading, messages, navigateToFlow, reset } = useFlow();
 * </script>
 *
 * <template>
 *   <div>
 *     <p v-if="isLoading">Loading...</p>
 *     <component :is="currentStep?.component" v-else />
 *     <p v-for="msg in messages" :key="msg.id">{{ msg.content }}</p>
 *   </div>
 * </template>
 * ```
 */
const useFlow = (): FlowContextValue => {
  const context: unknown = inject(FLOW_KEY);

  if (!context) {
    throw new Error(
      '[ThunderID] useFlow() was called outside of <ThunderIDProvider>. ' +
        'Make sure to install the ThunderIDPlugin or wrap your app with <ThunderIDProvider>.',
    );
  }

  return context as FlowContextValue;
};

export default useFlow;
