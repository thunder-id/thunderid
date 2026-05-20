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
import {I18N_KEY} from '../keys';
import type {I18nContextValue} from '../models/contexts';

/**
 * Composable for accessing internationalization utilities.
 *
 * Must be called inside a component that is a descendant of `<ThunderIDProvider>`.
 *
 * @returns {I18nContextValue} The i18n context with translation function, language management, and bundle injection.
 * @throws {Error} If called outside of `<ThunderIDProvider>`.
 *
 * @example
 * ```vue
 * <script setup>
 * import { useI18n } from '@thunderid/vue';
 *
 * const { t, currentLanguage, setLanguage } = useI18n();
 * </script>
 *
 * <template>
 *   <p>{{ t('common.welcome') }}</p>
 *   <select :value="currentLanguage" @change="setLanguage($event.target.value)">
 *     <option value="en-US">English</option>
 *     <option value="fr-FR">Français</option>
 *   </select>
 * </template>
 * ```
 */
const useI18n = (): I18nContextValue => {
  const context: unknown = inject(I18N_KEY);

  if (!context) {
    throw new Error(
      '[ThunderID] useI18n() was called outside of <ThunderIDProvider>. ' +
        'Make sure to install the ThunderIDPlugin or wrap your app with <ThunderIDProvider>.',
    );
  }

  return context as I18nContextValue;
};

export default useI18n;
