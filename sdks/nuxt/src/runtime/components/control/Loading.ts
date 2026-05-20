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

import {type Component, type VNode, Fragment, defineComponent, h} from 'vue';
import {useThunderID} from '#imports';

/**
 * Nuxt-specific Loading control component.
 *
 * Renders its default slot while ThunderID is initialising. Renders the
 * `fallback` slot (if provided) once loading is complete.
 *
 * Uses `useThunderID()` from the Nuxt auto-import layer.
 *
 * @example
 * ```vue
 * <ThunderIDLoading>
 *   <Spinner />
 *   <template #fallback><NuxtPage /></template>
 * </ThunderIDLoading>
 * ```
 */
const Loading: Component = defineComponent({
  name: 'Loading',
  setup(_props: Record<string, unknown>, {slots}: {slots: any}): () => VNode | VNode[] | null {
    const {isLoading} = useThunderID();

    return (): VNode | VNode[] | null => {
      if (!isLoading.value) {
        const fallback: VNode[] | undefined = slots['fallback']?.();
        return fallback ? h(Fragment, {}, fallback) : null;
      }

      const content: VNode[] | undefined = slots['default']?.();
      return content ? h(Fragment, {}, content) : null;
    };
  },
});

export default Loading;
