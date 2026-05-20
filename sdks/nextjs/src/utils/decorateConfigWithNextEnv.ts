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

import {ThunderIDNextConfig} from '../models/config';

const decorateConfigWithNextEnv = (config: ThunderIDNextConfig): ThunderIDNextConfig => {
  const {
    organizationHandle,
    scopes,
    applicationId,
    baseUrl,
    clientId,
    clientSecret,
    signInUrl,
    signUpUrl,
    afterSignInUrl,
    afterSignOutUrl,
    ...rest
  } = config;

  const envExpiryTime = process.env['THUNDERID_SESSION_COOKIE_EXPIRY_TIME']
    ? parseInt(process.env['THUNDERID_SESSION_COOKIE_EXPIRY_TIME'], 10)
    : undefined;

  return {
    ...rest,
    afterSignInUrl: afterSignInUrl || process.env['NEXT_PUBLIC_THUNDERID_AFTER_SIGN_IN_URL']!,
    afterSignOutUrl: afterSignOutUrl || process.env['NEXT_PUBLIC_THUNDERID_AFTER_SIGN_OUT_URL']!,
    applicationId: applicationId || process.env['NEXT_PUBLIC_THUNDERID_APPLICATION_ID']!,
    baseUrl: baseUrl || process.env['NEXT_PUBLIC_THUNDERID_BASE_URL']!,
    clientId: clientId || process.env['NEXT_PUBLIC_THUNDERID_CLIENT_ID']!,
    clientSecret: clientSecret || process.env['THUNDERID_CLIENT_SECRET']!,
    organizationHandle: organizationHandle || process.env['NEXT_PUBLIC_THUNDERID_ORGANIZATION_HANDLE']!,
    scopes: scopes || process.env['NEXT_PUBLIC_THUNDERID_SCOPES']!,
    sessionCookie: {
      ...rest.sessionCookie,
      expiryTime: rest.sessionCookie?.expiryTime || envExpiryTime,
    },
    signInUrl: signInUrl || process.env['NEXT_PUBLIC_THUNDERID_SIGN_IN_URL']!,
    signUpUrl: signUpUrl || process.env['NEXT_PUBLIC_THUNDERID_SIGN_UP_URL']!,
  };
};

export default decorateConfigWithNextEnv;
