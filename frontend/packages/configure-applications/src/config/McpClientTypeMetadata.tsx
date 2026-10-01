// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
    icon: <UserRound size={20} />,
    titleKey: 'applications:onboarding.mcp.clientType.userDelegated.title',
    descriptionKey: 'applications:onboarding.mcp.clientType.userDelegated.description',
  },
  {
    value: McpClientTypes.M2M,
    icon: <Bot size={20} />,
    titleKey: 'applications:onboarding.mcp.clientType.m2m.title',
    descriptionKey: 'applications:onboarding.mcp.clientType.m2m.description',
  },
];

export default McpClientTypeMetadataList;
