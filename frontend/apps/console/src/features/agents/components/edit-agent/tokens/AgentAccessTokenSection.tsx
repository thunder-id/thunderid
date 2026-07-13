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
import {useGetAgentType, useGetAgentTypes} from '@thunderid/configure-agent-types';
import {
  Alert,
  Card,
  CardContent,
  Chip,
  FormControl,
  FormLabel,
  Grid,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {useEffect, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import JwtPreview from '../../../../applications/components/edit-application/token-settings/JwtPreview';
import TokenConstants from '../../../../applications/constants/token-constants';
import type {OAuth2Config} from '../../../../applications/models/oauth';
import type {Agent, AgentInboundAuthConfig, OAuthAgentConfig} from '../../../models/agent';

interface AgentAccessTokenSectionProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
  onValidationChange?: (hasErrors: boolean) => void;
}

// The client_credentials grant has no scopes, so `scope` never appears on this token.
const ACCESS_TOKEN_DEFAULT_CLAIMS = TokenConstants.DEFAULT_TOKEN_ATTRIBUTES.filter((attr) => attr !== 'scope');

/**
 * Access-token settings for the agent acting as its own client (client_credentials grant) —
 * the "clientConfig" half of AccessTokenConfig, as opposed to the "User" tab's userConfig
 * half. Not gated by Delegated mode, since client_credentials is available regardless.
 */
export default function AgentAccessTokenSection({
  agent,
  editedAgent,
  oauth2Config = undefined,
  onFieldChange,
  onValidationChange = undefined,
}: AgentAccessTokenSectionProps): JSX.Element {
  const {t} = useTranslation();
  const disabled = agent.isReadOnly === true;

  const {data: agentTypesData} = useGetAgentTypes();
  const matchedSchema = agentTypesData?.types?.find((s) => s.name === agent.type);
  const {data: schemaDetails, isLoading} = useGetAgentType(matchedSchema?.id);

  const clientConfig = oauth2Config?.token?.accessToken?.clientConfig;
  const currentAttributes = clientConfig?.attributes ?? [];

  const [validityInput, setValidityInput] = useState<string>(String(clientConfig?.validityPeriod ?? 3600));
  const parsedValidity = parseInt(validityInput, 10);
  const isValidityInvalid = validityInput.trim() === '' || Number.isNaN(parsedValidity) || parsedValidity < 1;

  useEffect(() => {
    onValidationChange?.(isValidityInvalid);
  }, [isValidityInvalid, onValidationChange]);

  const handleOAuth2ConfigChange = (updates: Partial<OAuth2Config>): void => {
    if (!oauth2Config || disabled) return;
    const updatedConfig = {...oauth2Config, ...updates} as OAuthAgentConfig;
    const currentInboundAuth: AgentInboundAuthConfig[] = editedAgent.inboundAuthConfig ?? agent.inboundAuthConfig ?? [];
    const updatedInboundAuth = currentInboundAuth.map((auth) =>
      auth.type === 'oauth2' ? {...auth, config: updatedConfig} : auth,
    );
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  const handleAttributeClick = (attr: string): void => {
    const nextAttrs = currentAttributes.includes(attr)
      ? currentAttributes.filter((a) => a !== attr)
      : [...currentAttributes, attr];

    handleOAuth2ConfigChange({
      token: {
        ...oauth2Config?.token,
        accessToken: {
          ...oauth2Config?.token?.accessToken,
          clientConfig: {...clientConfig, attributes: nextAttrs},
        },
      } as OAuth2Config['token'],
    });
  };

  const handleValidityChange = (value: string): void => {
    setValidityInput(value);
    const parsed = parseInt(value, 10);
    if (value.trim() !== '' && !Number.isNaN(parsed) && parsed >= 1) {
      handleOAuth2ConfigChange({
        token: {
          ...oauth2Config?.token,
          accessToken: {
            ...oauth2Config?.token?.accessToken,
            clientConfig: {...clientConfig, validityPeriod: parsed},
          },
        } as OAuth2Config['token'],
      });
    }
  };

  const availableAttributes = (
    schemaDetails?.schema
      ? Object.entries(schemaDetails.schema)
          .filter(
            ([, fieldDef]) => !((fieldDef.type === 'string' || fieldDef.type === 'number') && fieldDef.credential),
          )
          .map(([name]) => name)
      : []
  ).sort();

  const jwtPreview: Record<string, unknown> = {};
  ACCESS_TOKEN_DEFAULT_CLAIMS.forEach((attr) => {
    jwtPreview[attr] = `<${attr}>`;
  });
  currentAttributes.forEach((attr) => {
    jwtPreview[attr] = `<${attr}>`;
  });

  return (
    <Stack spacing={3}>
      <SettingsCard
        title={t('agents:edit.tokens.agent.attributes.title', 'Access Token Attributes')}
        description={t(
          'agents:edit.tokens.agent.attributes.description',
          'Attributes included in the access token this agent receives for its own requests (client_credentials grant).',
        )}
      >
        <Grid container spacing={3}>
          <Grid size={{xs: 12, md: 7}}>
            <Typography variant="body2" sx={{mb: 1}}>
              {t('agents:edit.tokens.agent.attributes.label', 'Add or Remove Attributes')}
            </Typography>
            <Typography variant="body2" color="text.disabled" sx={{mb: 2}}>
              {t(
                'agents:edit.tokens.agent.attributes.hint',
                "Click on this agent's attributes to add them to its access token.",
              )}
            </Typography>
            <Card>
              <CardContent>
                {isLoading && (
                  <Typography variant="body2" color="text.secondary">
                    {t('common:status.loading', 'Loading…')}
                  </Typography>
                )}
                {!isLoading && availableAttributes.length > 0 && (
                  <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                    {availableAttributes.map((attr) => {
                      const isActive = currentAttributes.includes(attr);
                      return (
                        <Chip
                          key={attr}
                          label={attr}
                          size="small"
                          variant={isActive ? 'filled' : 'outlined'}
                          color={isActive ? 'primary' : 'default'}
                          onClick={disabled ? undefined : () => handleAttributeClick(attr)}
                          sx={{cursor: disabled ? 'default' : 'pointer'}}
                        />
                      );
                    })}
                  </Stack>
                )}
                {!isLoading && availableAttributes.length === 0 && (
                  <Alert severity="info">
                    {t(
                      'agents:edit.tokens.agent.attributes.empty',
                      'No attributes available. Configure attributes for this agent in the Attributes tab.',
                    )}
                  </Alert>
                )}
              </CardContent>
            </Card>
          </Grid>
          <Grid size={{xs: 12, md: 5}}>
            <JwtPreview payload={jwtPreview} defaultClaims={ACCESS_TOKEN_DEFAULT_CLAIMS} />
          </Grid>
        </Grid>
      </SettingsCard>

      <SettingsCard
        title={t('agents:edit.tokens.agent.validity.title', 'Token Validity')}
        description={t(
          'agents:edit.tokens.agent.validity.description',
          'How long this access token remains valid before expiration.',
        )}
      >
        <FormControl fullWidth required>
          <FormLabel htmlFor="agent-access-token-validity">
            {t('agents:edit.tokens.agent.validity.label', 'Token Validity')}
          </FormLabel>
          <TextField
            id="agent-access-token-validity"
            type="number"
            fullWidth
            value={validityInput}
            onChange={(e) => handleValidityChange(e.target.value)}
            error={isValidityInvalid}
            helperText={
              isValidityInvalid
                ? t('agents:edit.tokens.agent.validity.error', 'Enter a validity period of at least 1 second.')
                : t(
                    'agents:edit.tokens.agent.validity.hint',
                    'Token validity period in seconds (e.g., 3600 for 1 hour).',
                  )
            }
            inputProps={{min: 1}}
            disabled={disabled}
          />
        </FormControl>
      </SettingsCard>
    </Stack>
  );
}
