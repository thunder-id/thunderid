/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {FC, useState} from 'react';
import {TokenCallback, TokenCallbackProps} from './TokenCallback';
import {OAuthCallback, OAuthCallbackProps} from './OAuthCallback';

/**
 * Props for the unified Callback component, combining properties for both Token and OAuth callbacks.
 */
export type CallbackProps = OAuthCallbackProps & TokenCallbackProps;

/**
 * A unified Callback component that automatically routes to either OAuthCallback or TokenCallback
 * based on the presence of URL parameters ('code' for OAuth, 'token' for token-based flows).
 */
export const Callback: FC<CallbackProps> = (props: CallbackProps) => {
  // Use state to lock the flow type on initial mount.
  // This prevents the component from swapping to OAuthCallback if TokenCallback
  // removes the '?token=' query parameter from the URL using window.history.
  const [flowType] = useState<'token' | 'oauth'>(() => {
    if (typeof window === 'undefined') {
      return 'oauth';
    }
    const urlParams = new URLSearchParams(window.location.search);
    return urlParams.get('token') ? 'token' : 'oauth';
  });

  if (typeof window === 'undefined') {
    return null;
  }

  if (flowType === 'token') {
    return <TokenCallback {...props} />;
  }

  return <OAuthCallback {...props} />;
};

export default Callback;
