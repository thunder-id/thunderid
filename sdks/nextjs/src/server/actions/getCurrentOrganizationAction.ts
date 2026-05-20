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

'use server';

import {Organization} from '@thunderid/node';
import getClient from '../getClient';

/**
 * Server action to create an organization.
 */
const getCurrentOrganizationAction = async (
  sessionId: string,
): Promise<{
  data: {organization?: Organization; user?: Record<string, unknown>};
  error: string | null;
  success: boolean;
}> => {
  try {
    const client = getClient();
    const organization: Organization = (await client.getCurrentOrganization(sessionId))!;
    return {data: {organization}, error: null, success: true};
  } catch (error) {
    return {
      data: {
        user: {},
      },
      error: 'Failed to get the current organization',
      success: false,
    };
  }
};

export default getCurrentOrganizationAction;
