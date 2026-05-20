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
 * Nuxt-specific SignedIn control component.
 *
 * Renders its default slot only when the user is authenticated. Renders the
 * `fallback` slot (if provided) when the user is not signed in.
 *
 * Uses `useThunderID()` from the Nuxt auto-import layer so it reads from the
 * THUNDERID_KEY context wired up by the Nuxt plugin — not directly from
 * `@thunderid/vue`.
 *
 * @example
 * ```vue
 * <ThunderIDSignedIn>
 *   <p>Welcome!</p>
 *   <template #fallback><p>Please sign in.</p></template>
 * </ThunderIDSignedIn>
 * ```
 */
const SignedIn: Component = defineComponent({
  name: 'SignedIn',
  setup(_props: Record<string, unknown>, {slots}: {slots: any}): () => VNode | VNode[] | null {
    const {isSignedIn} = useThunderID();

    return (): VNode | VNode[] | null => {
      if (!isSignedIn.value) {
        const fallback: VNode[] | undefined = slots['fallback']?.();
        return fallback ? h(Fragment, {}, fallback) : null;
      }

      const content: VNode[] | undefined = slots['default']?.();
      return content ? h(Fragment, {}, content) : null;
    };
  },
});

export default SignedIn;
