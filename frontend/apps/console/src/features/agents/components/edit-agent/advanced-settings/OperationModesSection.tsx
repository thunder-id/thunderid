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
import {
  Alert,
  Box,
  Checkbox,
  Chip,
  FormControl,
  FormLabel,
  ListItemText,
  MenuItem,
  Select,
  Stack,
} from '@wso2/oxygen-ui';
import {useTranslation} from 'react-i18next';
import RedirectURIsSection from './RedirectURIsSection';
import {OAuth2GrantTypes, type OAuth2Config} from '../../../../applications/models/oauth';
import {getGrantTypeLabel} from '../../../../applications/utils/getGrantTypeLabel';
import {applyGrantTypesChange} from '../../../../applications/utils/oauth2Rules';
import {DELEGATED_ONLY_GRANTS} from '../../../constants/delegationGrants';

interface OperationModesSectionProps {
  oauth2Config?: OAuth2Config;
  onOAuth2ConfigChange?: (updates: Partial<OAuth2Config>) => void;
  disabled?: boolean;
}

export default function OperationModesSection({
  oauth2Config = undefined,
  onOAuth2ConfigChange = undefined,
  disabled = false,
}: OperationModesSectionProps) {
  const {t} = useTranslation();

  if (!oauth2Config) return null;

  const isEditable = Boolean(onOAuth2ConfigChange) && !disabled;
  const grantTypes = oauth2Config.grantTypes ?? [];
  const hasDelegatedMode = grantTypes.includes(OAuth2GrantTypes.AUTHORIZATION_CODE);

  const updateGrants = (next: string[]): void => {
    if (!isEditable) return;
    const updates = applyGrantTypesChange(oauth2Config, next);
    // PKCE is fully derived from authorization_code for agents.
    if (next.includes(OAuth2GrantTypes.AUTHORIZATION_CODE)) {
      updates.pkceRequired = true;
    }
    onOAuth2ConfigChange?.(updates);
  };

  // client_credentials and token_exchange apply no matter which modes are active — the former
  // is mandatory for every agent, the latter is shared across both modes. The delegated-only
  // grants stay in the list at all times (see isGrantLocked) rather than disappearing when
  // Delegated mode is off.
  const availableGrantTypes: string[] = [
    OAuth2GrantTypes.CLIENT_CREDENTIALS,
    ...DELEGATED_ONLY_GRANTS,
    OAuth2GrantTypes.TOKEN_EXCHANGE,
  ];

  const isGrantLocked = (grant: string): boolean =>
    grant === OAuth2GrantTypes.CLIENT_CREDENTIALS ||
    (DELEGATED_ONLY_GRANTS.includes(grant) && (!hasDelegatedMode || grant === OAuth2GrantTypes.AUTHORIZATION_CODE));

  const selectedGrantTypes = grantTypes.filter((grant) => availableGrantTypes.includes(grant));

  return (
    <SettingsCard
      title={t('agents:edit.advanced.oauthAccess.title', 'OAuth Configuration')}
      description={t(
        'agents:edit.advanced.oauthAccess.description',
        'The grants and redirect URIs this agent is authorized to use.',
      )}
    >
      <Stack spacing={3}>
        <Box>
          <FormControl fullWidth size="small">
            <FormLabel htmlFor="agent-grant-types">
              {t('agents:edit.advanced.oauthAccess.grantTypes.label', 'Grant Types')}
            </FormLabel>
            <Alert severity="info" sx={{mb: 1.5}}>
              {t(
                'agents:edit.advanced.oauthAccess.grantTypes.hint',
                'The greyed-out grants unlock once you turn on Delegated mode in the Flows tab.',
              )}
            </Alert>
            <Select
              id="agent-grant-types"
              multiple
              displayEmpty
              disabled={!isEditable}
              value={selectedGrantTypes}
              onChange={(e) => updateGrants(e.target.value as string[])}
              renderValue={(selected) => (
                <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                  {selected.map((grant) => (
                    <Chip key={grant} label={getGrantTypeLabel(grant, t)} size="small" />
                  ))}
                </Stack>
              )}
            >
              {availableGrantTypes.map((grant) => (
                <MenuItem key={grant} value={grant} disabled={isGrantLocked(grant)}>
                  <Checkbox checked={grantTypes.includes(grant)} size="small" />
                  <ListItemText primary={getGrantTypeLabel(grant, t)} />
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Box>

        <RedirectURIsSection
          oauth2Config={oauth2Config}
          onOAuth2ConfigChange={onOAuth2ConfigChange}
          disabled={disabled}
        />
      </Stack>
    </SettingsCard>
  );
}
