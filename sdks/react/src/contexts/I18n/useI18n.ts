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

import {useContext} from 'react';
import I18nContext, {I18nContextValue} from './I18nContext';

/**
 * Hook for accessing the I18n context directly.
 * Provides access to the full i18n context including bundles and all utilities.
 *
 * @returns The complete I18n context value
 * @throws Error if used outside of I18nProvider context
 */
const useI18n = (): I18nContextValue => {
  const context: I18nContextValue | null = useContext(I18nContext);

  if (!context) {
    throw new Error(
      'useI18n must be used within an I18nProvider. Make sure your component is wrapped with ThunderIDProvider which includes I18nProvider.',
    );
  }

  return context;
};

export default useI18n;
