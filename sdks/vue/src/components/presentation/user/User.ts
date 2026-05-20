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

import {type Component, type VNode, defineComponent, h, Fragment} from 'vue';
import useThunderID from '../../../composables/useThunderID';

/**
 * User — presentation component that exposes the current user via a scoped slot.
 *
 * Renders the `default` slot with `{ user }` when a user is signed in,
 * or the `fallback` slot when no user is available.
 *
 * @example
 * ```vue
 * <User>
 *   <template #default="{ user }">
 *     <p>Welcome, {{ user.given_name }}!</p>
 *   </template>
 *   <template #fallback>
 *     <p>No user signed in.</p>
 *   </template>
 * </User>
 * ```
 */
const User: Component = defineComponent({
  name: 'User',
  setup(_props: Record<string, unknown>, {slots}: {slots: any}): () => VNode | VNode[] | null {
    const {user} = useThunderID();

    return (): VNode | VNode[] | null => {
      if (!user.value) {
        const fallbackContent: VNode[] | undefined = slots.fallback?.();
        return fallbackContent ? h(Fragment, {}, fallbackContent) : null;
      }

      const defaultContent: VNode[] | undefined = slots.default?.({user: user.value});
      return defaultContent ? h(Fragment, {}, defaultContent) : null;
    };
  },
});

export default User;
