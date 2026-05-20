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

import {useThunderID} from '@thunderid/react';
import {useConfig} from '@thunderid/contexts';
import {useEffect, type JSX} from 'react';
import {useLocation, useNavigate} from 'react-router';
import getWelcomeDismissedStorageKey from '../utils/getWelcomeDismissedStorageKey';

export default function WelcomeRedirect(): JSX.Element | null {
  const {isSignedIn} = useThunderID();
  const {config} = useConfig();
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    if (!isSignedIn || location.pathname.startsWith('/welcome')) return;

    const productName = config.brand.product_name;
    const dismissed = sessionStorage.getItem(getWelcomeDismissedStorageKey(productName)) === 'true';

    if (!dismissed) {
      sessionStorage.setItem(getWelcomeDismissedStorageKey(productName), 'true');
      void navigate('/welcome', {replace: true});
    }
  }, [isSignedIn, navigate, config.brand.product_name, location.pathname]);

  return null;
}
