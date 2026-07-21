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
import {useThunderID} from '@thunderid/react';
import {FormControl, FormLabel, MenuItem, Select, Typography} from '@wso2/oxygen-ui';
import {useTranslation} from 'react-i18next';
import {TokenEndpointAuthMethods, type OAuth2Config} from '../../../../applications/models/oauth';
import {applyTokenEndpointAuthMethodChange, deriveOAuth2Flags} from '../../../../applications/utils/oauth2Rules';

interface OidcDiscovery {
  token_endpoint_auth_methods_supported?: string[];
}

interface TokenEndpointAuthMethodSectionProps {
  oauth2Config?: OAuth2Config;
  onOAuth2ConfigChange?: (updates: Partial<OAuth2Config>) => void;
  disabled?: boolean;
}

export default function TokenEndpointAuthMethodSection({
  oauth2Config = undefined,
  onOAuth2ConfigChange = undefined,
  disabled = false,
}: TokenEndpointAuthMethodSectionProps) {
  const {t} = useTranslation();
  const {discovery} = useThunderID();

  if (!oauth2Config) return null;

  const wellKnown = (discovery as {wellKnown?: OidcDiscovery} | undefined)?.wellKnown;
  const availableTokenEndpointAuthMethods: string[] = wellKnown?.token_endpoint_auth_methods_supported ?? [];
  const isEditable = Boolean(onOAuth2ConfigChange) && !disabled;

  const flags = deriveOAuth2Flags(oauth2Config);
  const isTokenMethodLocked = oauth2Config.publicClient === true;
  const effectiveTokenMethod = oauth2Config.publicClient ? 'none' : (oauth2Config.tokenEndpointAuthMethod ?? '');

  return (
    <SettingsCard
      title={t('agents:edit.credentials.tokenEndpointAuthMethod.title', 'Token Endpoint Auth Method')}
      description={t(
        'agents:edit.credentials.tokenEndpointAuthMethod.description',
        'Defines how this agent authenticates when requesting tokens.',
      )}
    >
      <FormControl fullWidth size="small">
        <FormLabel htmlFor="agent_token_endpoint_auth_method">
          {t('agents:edit.credentials.tokenEndpointAuthMethod.title', 'Token Endpoint Auth Method')}
        </FormLabel>
        <Select
          id="agent_token_endpoint_auth_method"
          displayEmpty
          disabled={!isEditable || isTokenMethodLocked}
          value={effectiveTokenMethod}
          onChange={(e) => onOAuth2ConfigChange?.(applyTokenEndpointAuthMethodChange(oauth2Config, e.target.value))}
          renderValue={(selected) =>
            !selected ? (
              <Typography color="text.secondary" variant="body2">
                {t('agents:edit.credentials.tokenEndpointAuthMethod.placeholder', 'Select an auth method')}
              </Typography>
            ) : (
              selected
            )
          }
        >
          {availableTokenEndpointAuthMethods.map((method) => (
            <MenuItem
              key={method}
              value={method}
              disabled={method === TokenEndpointAuthMethods.NONE && flags.isPublicClientDisabledByGrants}
            >
              {method}
            </MenuItem>
          ))}
        </Select>
        <Typography variant="caption" color="text.secondary" sx={{mt: 0.5}}>
          {isTokenMethodLocked
            ? t(
                'agents:edit.credentials.tokenEndpointAuthMethod.lockedHint',
                'Set to "none" because this agent is a public client.',
              )
            : t(
                'agents:edit.credentials.tokenEndpointAuthMethod.hint',
                'How this agent proves its identity when it calls the token endpoint.',
              )}
        </Typography>
      </FormControl>
    </SettingsCard>
  );
}
