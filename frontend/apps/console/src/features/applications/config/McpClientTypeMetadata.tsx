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

import {UserRound, Bot} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {McpClientTypes} from '../models/mcp-client';
import type {McpClientType} from '../models/mcp-client';

export interface McpClientTypeMetadata {
  value: McpClientType;
  icon: JSX.Element;
  titleKey: string;
  descriptionKey: string;
}

const McpClientTypeMetadataList: McpClientTypeMetadata[] = [
  {
    value: McpClientTypes.USER_DELEGATED,
    icon: <UserRound size={32} />,
    titleKey: 'applications:onboarding.mcp.clientType.userDelegated.title',
    descriptionKey: 'applications:onboarding.mcp.clientType.userDelegated.description',
  },
  {
    value: McpClientTypes.M2M,
    icon: <Bot size={32} />,
    titleKey: 'applications:onboarding.mcp.clientType.m2m.title',
    descriptionKey: 'applications:onboarding.mcp.clientType.m2m.description',
  },
];

export default McpClientTypeMetadataList;
