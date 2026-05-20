/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
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

import isRecognizedBaseUrlPattern from './isRecognizedBaseUrlPattern';
import logger from './logger';
import {Config} from '../models/config';
import {Platform} from '../models/platforms';

/**
 * Identifies the platform based on the given base URL.
 *
 * If the URL is recognized and matches the ThunderID domain, returns Platform.ThunderID.
 * Otherwise, returns Platform.IdentityServer.
 *
 * @param baseUrl - The base URL to check
 * @returns Platform enum value
 */
const identifyPlatform = (config: Config): Platform => {
  const {baseUrl} = config;

  try {
    if (isRecognizedBaseUrlPattern(baseUrl)) {
      try {
        const url: URL = new URL(baseUrl);
        // Check for asgardeo domain (e.g., api.asgardeo.io, etc.)
        if (/\.thunderid\.io$/i.test(url.hostname) || /thunderid\.io$/i.test(url.hostname)) {
          return Platform.ThunderID;
        }
      } catch {
        // Fallback to IdentityServer if URL parsing fails.
        logger.debug(
          `[identifyPlatform] Could not identify platform from the base URL: ${baseUrl}. Defaulting to WSO2 Identity Server as the platform.`,
        );
      }

      return Platform.IdentityServer;
    }

    return Platform.Unknown;
  } catch (error) {
    logger.debug(`[identifyPlatform] Error identifying platform from base URL: ${baseUrl}. Error: ${error.message}`);

    return Platform.Unknown;
  }
};

export default identifyPlatform;
