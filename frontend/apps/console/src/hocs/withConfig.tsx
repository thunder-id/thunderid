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

import {useConfig} from '@thunderid/contexts';
import {ThunderIDProvider} from '@thunderid/react';
import type {ThunderIDProviderProps} from '@thunderid/react';
import {merge} from '@thunderid/utils';
import type {JSX, ComponentType} from 'react';

export default function withConfig<P extends object>(WrappedComponent: ComponentType<P>) {
  return function WithConfig(props: P): JSX.Element {
    const {getClientUrl, config} = useConfig();

    // Behavioral defaults. config.sdk values are deep-merged on top, so operators
    // can override any of these via the sdk block in config.js.
    const sdkDefaults: Partial<ThunderIDProviderProps> = {
      discovery: {wellKnown: {enabled: true}},
    };

    const sdkProps = merge({}, sdkDefaults, config.sdk ?? {}) as Partial<ThunderIDProviderProps>;

    return (
      <ThunderIDProvider
        baseUrl={import.meta.env.VITE_THUNDER_BASE_URL as string}
        clientId={import.meta.env.VITE_THUNDER_CLIENT_ID as string}
        afterSignInUrl={getClientUrl() ?? (import.meta.env.VITE_THUNDER_AFTER_SIGN_IN_URL as string)}
        {...sdkProps}
      >
        <WrappedComponent {...props} />
      </ThunderIDProvider>
    );
  };
}
