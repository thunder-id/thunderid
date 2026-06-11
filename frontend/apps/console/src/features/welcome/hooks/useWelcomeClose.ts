/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {useConfig, useToast} from '@thunderid/contexts';
import {useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import getWelcomeDismissedStorageKey from '../utils/getWelcomeDismissedStorageKey';

/**
 * Custom hook that provides a function to handle the closing of the welcome page.
 * When invoked, the function sets a flag in session storage to indicate that the welcome page has been dismissed,
 * navigates the user to the home page, and displays a toast notification confirming the dismissal.
 * @returns A function that can be called to close the welcome page.
 */
export default function useWelcomeClose(): () => void {
  const navigate = useNavigate();
  const {showToast} = useToast();
  const {t} = useTranslation(['common']);
  const {config} = useConfig();
  const productName = config.brand.product_name;

  return useCallback((): void => {
    sessionStorage.setItem(getWelcomeDismissedStorageKey(productName), 'true');
    void navigate('/home');
    showToast(t('common:welcome.dismissed'), 'info');
  }, [navigate, productName, showToast, t]);
}
