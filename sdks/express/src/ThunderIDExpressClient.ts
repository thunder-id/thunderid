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

import {ThunderIDNodeClient, ThunderIDAuthException, Storage, TokenResponse, User} from '@thunderid/node';
import express from 'express';
import {v4 as uuidv4} from 'uuid';
import CookieConfig, {SESSION_COOKIE_NAME} from './constants/CookieConfig';
import {ExpressClientConfig} from './models/config';
import hasErrorInURL from './utils/expressUtils';

class ThunderIDExpressClient<T extends ExpressClientConfig = ExpressClientConfig> extends ThunderIDNodeClient<T> {
  private _expressConfig: ExpressClientConfig | undefined;

  public constructor() {
    super();
  }

  public override async initialize(config: T, storage?: Storage): Promise<boolean> {
    this._expressConfig = config;
    return super.initialize(config, storage);
  }

  public get expressConfig(): ExpressClientConfig | undefined {
    return this._expressConfig;
  }

  public async getUserFromRequest(req: express.Request): Promise<User | undefined> {
    const sessionId: string | undefined = req.cookies?.[SESSION_COOKIE_NAME];
    return this.getUser(sessionId);
  }

  public override async signIn(
    req: express.Request,
    res: express.Response,
    next: express.NextFunction,
    signInConfig?: Record<string, string | boolean>,
  ): Promise<TokenResponse> {
    if (hasErrorInURL(req.originalUrl)) {
      return Promise.reject(
        new ThunderIDAuthException(
          'EXPRESS-CLIENT-SI-IV01',
          'Invalid login request URL',
          'Login request contains an error query parameter in the URL',
        ),
      );
    }

    let userId: string = req.cookies?.[SESSION_COOKIE_NAME];
    if (!userId) {
      userId = uuidv4();
    }

    const sc = this._expressConfig?.sessionCookie;

    const authRedirectCallback = (url: string): void => {
      if (!url) return;

      res.cookie(SESSION_COOKIE_NAME, userId, {
        httpOnly: sc?.httpOnly ?? CookieConfig.defaultHttpOnly,
        maxAge: (sc?.expiryTime ?? CookieConfig.defaultExpirySeconds) * 1000,
        sameSite: (sc?.sameSite ?? CookieConfig.defaultSameSite) as any,
        secure: sc?.secure ?? CookieConfig.defaultSecure,
      });
      res.redirect(url);
      if (typeof next === 'function') next();
    };

    const authResponse: TokenResponse = (await super.signIn(
      authRedirectCallback,
      userId,
      req.query.code as string | undefined,
      req.query.session_state as string | undefined,
      req.query.state as string | undefined,
      signInConfig,
    )) as unknown as TokenResponse;

    if (authResponse.accessToken || authResponse.idToken) {
      return authResponse;
    }

    return {
      accessToken: '',
      createdAt: 0,
      expiresIn: '',
      idToken: '',
      refreshToken: '',
      scope: '',
      tokenType: '',
    };
  }

  public override async signOut(userId?: string): Promise<string> {
    return super.signOut(userId);
  }
}

export default ThunderIDExpressClient;
