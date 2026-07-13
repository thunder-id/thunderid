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

import {Box, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import {KeyRound, Lock, RefreshCw, ShieldCheck} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {McpClientTypes} from '../../../models/mcp-client';
import type {McpClientType} from '../../../models/mcp-client';

/**
 * Props for the {@link McpClientTypePreview} component.
 *
 * @public
 */
export interface McpClientTypePreviewProps {
  /**
   * The currently selected MCP client type, whose OAuth profile and next-step guidance the
   * panel previews
   */
  clientType: McpClientType;
}

/**
 * React component that renders the "what you get" preview panel shown below the client type
 * cards on the mcp-client template's Client type step. Reuses the OAuth-profile-strip styling
 * from the Connection step: an eyebrow label followed by outlined profile chips, plus a
 * muted next-step line, both of which swap with the selected client type.
 *
 * @param props - The component props
 * @param props.clientType - The currently selected MCP client type
 *
 * @returns JSX element displaying the client type preview panel
 *
 * @example
 * ```tsx
 * import McpClientTypePreview from './McpClientTypePreview';
 *
 * function ClientTypeStep() {
 *   return <McpClientTypePreview clientType="userDelegated" />;
 * }
 * ```
 *
 * @public
 */
export default function McpClientTypePreview({clientType}: McpClientTypePreviewProps): JSX.Element {
  const {t} = useTranslation();
  const isM2m = clientType === McpClientTypes.M2M;

  return (
    <Box
      data-testid="application-mcp-client-type-preview"
      sx={{
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1,
        p: 1.5,
        bgcolor: 'background.default',
      }}
    >
      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap alignItems="center">
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{textTransform: 'uppercase', letterSpacing: 0.5, fontWeight: 600}}
        >
          {t('applications:onboarding.mcp.clientType.preview.label', 'What you get')}
        </Typography>
        {isM2m ? (
          <>
            <Chip
              size="small"
              variant="outlined"
              icon={<KeyRound size={14} />}
              label={t('applications:onboarding.mcp.oauthProfile.clientCredentials', 'Client Credentials')}
            />
            <Chip
              size="small"
              variant="outlined"
              label={t('applications:onboarding.mcp.oauthProfile.confidentialClient', 'Confidential client')}
            />
            <Chip
              size="small"
              variant="outlined"
              icon={<Lock size={14} />}
              label={t('applications:onboarding.mcp.oauthProfile.clientSecretIssued', 'Client secret issued')}
            />
          </>
        ) : (
          <>
            <Chip
              size="small"
              variant="outlined"
              icon={<ShieldCheck size={14} />}
              label={t('applications:onboarding.mcp.oauthProfile.authCodePkce')}
            />
            <Chip size="small" variant="outlined" label={t('applications:onboarding.mcp.oauthProfile.publicClient')} />
            <Chip
              size="small"
              variant="outlined"
              icon={<RefreshCw size={14} />}
              label={t('applications:onboarding.mcp.oauthProfile.refreshTokens')}
            />
          </>
        )}
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{mt: 1}}>
        {isM2m
          ? t(
              'applications:onboarding.mcp.clientType.preview.nextM2m',
              'Next: your client ID and secret are generated.',
            )
          : t('applications:onboarding.mcp.clientType.preview.nextUserDelegated', 'Add your redirect URIs below.')}
      </Typography>
    </Box>
  );
}
