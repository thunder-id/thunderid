// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig, useToast} from '@thunderid/contexts';
import {useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import RouteConfig from '../configs/RouteConfig';
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
    void navigate(RouteConfig.home.list());
    showToast(t('common:welcome.dismissed'), 'info');
  }, [navigate, productName, showToast, t]);
}
