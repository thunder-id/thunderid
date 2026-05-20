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

import {TokenExchangeRequestConfig, TokenResponse} from '@thunderid/node';
import getSessionIdAction from './actions/getSessionId';
import {ThunderIDNextConfig} from '../models/config';
import getClient from './getClient';

const thunderid = async (): Promise<{
  exchangeToken: (config: TokenExchangeRequestConfig, sessionId: string) => Promise<TokenResponse | Response>;
  getAccessToken: (sessionId: string) => Promise<string>;
  getSessionId: () => Promise<string | undefined>;
  reInitialize: (config: Partial<ThunderIDNextConfig>) => Promise<boolean>;
}> => {
  const getAccessToken = async (sessionId: string): Promise<string> => {
    const client = getClient();
    return client.getAccessToken(sessionId);
  };

  const getSessionId = async (): Promise<string | undefined> => getSessionIdAction();

  const exchangeToken = async (
    config: TokenExchangeRequestConfig,
    sessionId: string,
  ): Promise<TokenResponse | Response> => {
    const client = getClient();
    return client.exchangeToken(config, sessionId);
  };

  const reInitialize = async (config: Partial<ThunderIDNextConfig>): Promise<boolean> => {
    const client = getClient();
    return client.reInitialize(config);
  };

  return {
    exchangeToken,
    getAccessToken,
    getSessionId,
    reInitialize,
  };
};

export default thunderid;
