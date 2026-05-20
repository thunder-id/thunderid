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

import {logger as Logger} from '@thunderid/node';
import express from 'express';
import {SESSION_COOKIE_NAME} from '../constants/CookieConfig';
import ThunderIDExpressClient from '../ThunderIDExpressClient';

/**
 * Returns Express middleware that blocks unauthenticated requests.
 * Requires `thunderID()` to be mounted before this middleware so that
 * `req.thunderIDAuth` is populated.
 *
 * @param onUnauthenticated - Called when the session is missing or invalid.
 *   Defaults to sending a 401 response.
 */
const protect = (
  onUnauthenticated?: (res: express.Response) => void,
): ((req: express.Request, res: express.Response, next: express.NextFunction) => Promise<void>) => {
  return async (req: express.Request, res: express.Response, next: express.NextFunction): Promise<void> => {
    const client: ThunderIDExpressClient | undefined = (req as any).thunderIDAuth;
    const sessionId: string | undefined = req.cookies?.[SESSION_COOKIE_NAME];

    const reject = (): void => {
      if (onUnauthenticated) {
        onUnauthenticated(res);
      } else {
        res.status(401).end();
      }
    };

    if (!client || !sessionId) {
      Logger.error('No session ID found in the request cookies');
      reject();
      return;
    }

    const isValid: boolean = await client.isSignedIn(sessionId);

    if (isValid) {
      return next();
    }

    Logger.error('Invalid session ID found in the request cookies');
    reject();
  };
};

export default protect;
