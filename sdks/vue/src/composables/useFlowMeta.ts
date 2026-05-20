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
import {FLOW_META_KEY} from '../keys';
import type {FlowMetaContextValue} from '../models/contexts';

/**
 * Composable for accessing flow metadata.
 *
 * Must be called inside a component that is a descendant of `<ThunderIDProvider>`.
 *
 * @returns {FlowMetaContextValue} The flow meta context with metadata, loading state, and language switching.
 * @throws {Error} If called outside of `<ThunderIDProvider>`.
 *
 * @example
 * ```vue
 * <script setup>
 * import { useFlowMeta } from '@thunderid/vue';
 *
 * const { meta, isLoading, switchLanguage } = useFlowMeta();
 *
 * async function changeLanguage(lang: string) {
 *   await switchLanguage(lang);
 * }
 * </script>
 * ```
 */
const useFlowMeta = (): FlowMetaContextValue => {
  const context: unknown = inject(FLOW_META_KEY);

  if (!context) {
    throw new Error(
      '[ThunderID] useFlowMeta() was called outside of <ThunderIDProvider>. ' +
        'Make sure to install the ThunderIDPlugin or wrap your app with <ThunderIDProvider>.',
    );
  }

  return context as FlowMetaContextValue;
};

export default useFlowMeta;
