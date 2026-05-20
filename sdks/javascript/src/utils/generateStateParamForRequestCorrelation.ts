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

import PKCEConstants from '../constants/PKCEConstants';

/**
 * Generates a state parameter for request correlation by combining an optional state string with a request index.
 *
 * @param pkceKey - The PKCE key containing the index (format: 'pkce_code_verifier_[index]').
 * @param state - Optional state string to prepend to the request correlation.
 * @returns A state parameter string in the format '[state_]request_[index]'.
 *
 * @example
 * const pkceKey = "pkce_code_verifier_1";
 * const result = generateStateParamForRequestCorrelation(pkceKey, "myState");
 * // Returns: "myState_request_1"
 *
 * const resultNoState = generateStateParamForRequestCorrelation(pkceKey);
 * // Returns: "request_1"
 */
const generateStateParamForRequestCorrelation = (pkceKey: string, state?: string): string => {
  const index: number = parseInt(pkceKey.split(PKCEConstants.Storage.StorageKeys.SEPARATOR)[1], 10);

  return state ? `${state}_request_${index}` : `request_${index}`;
};

export default generateStateParamForRequestCorrelation;
