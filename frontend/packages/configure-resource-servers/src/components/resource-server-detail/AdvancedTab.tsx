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

import {SettingsCard} from '@thunderid/components';
import {FormControl, FormLabel, Stack, TextField} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {ResourceServer} from '../../models/resource-server';

interface AdvancedTabProps {
  resourceServer: ResourceServer;
  identifier: string;
  onIdentifierChange: (value: string) => void;
}

export default function AdvancedTab({resourceServer, identifier, onIdentifierChange}: AdvancedTabProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Stack spacing={3}>
      <SettingsCard
        title={t('resourceServers:edit.advanced.identifier.title', 'Configurations')}
        description={
          resourceServer.type === 'MCP'
            ? t(
                'resourceServers:edit.advanced.identifier.descriptionMcp',
                'Configuration settings for this MCP server.',
              )
            : t(
                'resourceServers:edit.advanced.identifier.description',
                'Configuration settings for this resource server.',
              )
        }
      >
        <FormControl fullWidth>
          <FormLabel htmlFor="resource-server-identifier">
            {t('resourceServers:edit.advanced.identifier.label', 'Identifier (Audience)')}
          </FormLabel>
          <TextField
            id="resource-server-identifier"
            value={identifier}
            onChange={(e) => onIdentifierChange(e.target.value)}
            fullWidth
            size="small"
            placeholder={
              resourceServer.type === 'MCP'
                ? t('resourceServers:edit.advanced.identifier.placeholderMcp', 'https://mcp.example.com')
                : t('resourceServers:edit.advanced.identifier.placeholder', 'https://api.example.com')
            }
            helperText={
              resourceServer.type === 'MCP'
                ? t(
                    'resourceServers:edit.advanced.identifier.hintMcp',
                    'A unique value that identifies this MCP server. When set as an URI, enables RFC 8707 resource indicator support in OAuth2 authorization requests.',
                  )
                : t(
                    'resourceServers:edit.advanced.identifier.hint',
                    'A unique value that identifies this resource server. When set as an URI, enables RFC 8707 resource indicator support in OAuth2 authorization requests.',
                  )
            }
            disabled={resourceServer.isReadOnly}
          />
        </FormControl>
      </SettingsCard>
    </Stack>
  );
}
