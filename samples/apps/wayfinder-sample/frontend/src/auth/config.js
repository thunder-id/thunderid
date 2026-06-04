/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

export const AUTH_CONFIG = {
  isRedirectBased: import.meta.env.VITE_AUTH_IS_REDIRECT_BASED !== 'false',
  isVerbose: import.meta.env.VITE_AUTH_IS_VERBOSE === 'true',
};

export const SCOPES = ["openid", "profile", "email", "ou", "agent:access", "booking:read", "booking:create", "booking:cancel"];
