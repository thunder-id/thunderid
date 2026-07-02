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

export const AUTH_CONFIG = {
  isRedirectBased: import.meta.env.VITE_AUTH_IS_REDIRECT_BASED !== "false",
  isVerbose: import.meta.env.VITE_AUTH_IS_VERBOSE === "true",
};

const AI_FEATURES_ENABLED = import.meta.env.VITE_AI_FEATURES_ENABLED === "true";

export const SCOPES = [
  "openid",
  "profile",
  "email",
  "ou",
  "booking:read",
  "booking:create",
  "booking:cancel",
  ...(AI_FEATURES_ENABLED ? ["agent:access"] : []),
];
