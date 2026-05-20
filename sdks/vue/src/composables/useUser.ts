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
import {USER_KEY} from '../keys';
import type {UserContextValue} from '../models/contexts';

/**
 * Composable for accessing user profile data.
 *
 * Must be called inside a component that is a descendant of `<ThunderIDProvider>`.
 *
 * @returns {UserContextValue} The user context containing profile, schemas, and update operations.
 * @throws {Error} If called outside of `<ThunderIDProvider>`.
 *
 * @example
 * ```vue
 * <script setup>
 * import { useUser } from '@thunderid/vue';
 *
 * const { profile, flattenedProfile, schemas, updateProfile, revalidateProfile } = useUser();
 * </script>
 *
 * <template>
 *   <div v-if="profile">
 *     <p>Name: {{ flattenedProfile?.name }}</p>
 *     <button @click="revalidateProfile()">Refresh</button>
 *   </div>
 * </template>
 * ```
 */
const useUser = (): UserContextValue => {
  const context: unknown = inject(USER_KEY);

  if (!context) {
    throw new Error(
      '[ThunderID] useUser() was called outside of <ThunderIDProvider>. ' +
        'Make sure to install the ThunderIDPlugin or wrap your app with <ThunderIDProvider>.',
    );
  }

  return context as UserContextValue;
};

export default useUser;
