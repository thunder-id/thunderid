// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Chip, Stack, Typography} from '@wso2/oxygen-ui';
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
   * The currently selected MCP client type, whose OAuth profile the panel previews
   */
  clientType: McpClientType;
}

/**
 * React component that renders the "what you get" preview shown inside the selected client type
 * card on the mcp-client template's Client type step: an eyebrow label followed by outlined
 * profile chips that swap with the selected client type.
 *
 * @param props - The component props
 * @param props.clientType - The currently selected MCP client type
 *
 * @returns JSX element displaying the client type preview
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
    <Stack spacing={1} data-testid="application-mcp-client-type-preview">
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
    </Stack>
  );
}
