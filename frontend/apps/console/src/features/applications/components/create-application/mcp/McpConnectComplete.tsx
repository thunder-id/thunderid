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

import {useCopyToClipboard} from '@thunderid/hooks';
import {useThunderID} from '@thunderid/react';
import {Alert, Box, Button, Divider, IconButton, InputAdornment, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {AlertTriangle, Check, CheckCircle, Copy, Eye, EyeOff, Info} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';
import {McpClientTypes} from '../../../models/mcp-client';
import type {McpClientType, McpDiscoveryEndpoints} from '../../../models/mcp-client';
import getMcpDiscoveryEndpointRows from '../../../utils/getMcpDiscoveryEndpointRows';
import CopyableField from '../../common/CopyableField';
import CopyableListRow from '../../common/CopyableListRow';

const cardSx = {
  p: 3,
  bgcolor: 'background.paper',
  border: '1px solid',
  borderColor: 'divider',
  borderRadius: 1,
} as const;

const codeSx = {
  fontFamily: 'monospace',
  bgcolor: 'action.hover',
  px: 0.5,
  borderRadius: 0.5,
} as const;

/**
 * Props for the {@link McpConnectComplete} component.
 *
 * @public
 */
export interface McpConnectCompleteProps {
  /**
   * The name of the created MCP client application. Only rendered for the
   * machine-to-machine variant, matching {@link ShowClientSecret}'s app-name row.
   */
  appName?: string;

  /**
   * The OAuth2 client ID of the created MCP client application
   */
  clientId?: string;

  /**
   * The OAuth2 client secret, only present for the machine-to-machine client type
   */
  clientSecret?: string;

  /**
   * The registered redirect URIs, only shown for the user-delegated client type
   */
  redirectUris: string[];

  /**
   * The MCP client type, driving which variant of the screen is rendered
   */
  clientType: McpClientType;

  /**
   * Callback invoked when the user continues to the created application
   */
  onContinue: () => void;
}

/**
 * React component that renders the Connect completion screen shown after creating an
 * mcp-client template application, for both the user-delegated and machine-to-machine
 * client types.
 *
 * Both variants display the pre-registered client ID and the ThunderID OAuth 2.1
 * endpoints (sourced from `useThunderID().discovery.wellKnown`). The user-delegated
 * variant additionally lists the registered redirect URIs; the machine-to-machine
 * variant shows the client secret once (reusing {@link ShowClientSecret}'s
 * mask/copy/warning affordances) alongside a `client_credentials` token-request hint.
 *
 * @param props - The component props
 * @param props.appName - The name of the created application (machine-to-machine only)
 * @param props.clientId - The OAuth2 client ID of the created application
 * @param props.clientSecret - The OAuth2 client secret (machine-to-machine only)
 * @param props.redirectUris - The registered redirect URIs (user-delegated only)
 * @param props.clientType - The MCP client type
 * @param props.onContinue - Callback invoked when the user continues to the created application
 *
 * @returns JSX element displaying the Connect completion screen
 *
 * @example
 * ```tsx
 * import McpConnectComplete from './McpConnectComplete';
 *
 * <McpConnectComplete
 *   clientId="my-client-id"
 *   redirectUris={['http://127.0.0.1:8080/callback']}
 *   clientType="userDelegated"
 *   onContinue={() => navigate(`/applications/${appId}`)}
 * />
 * ```
 *
 * @public
 */
export default function McpConnectComplete({
  appName = undefined,
  clientId = '',
  clientSecret = undefined,
  redirectUris,
  clientType,
  onContinue,
}: McpConnectCompleteProps): JSX.Element {
  const {t} = useTranslation();
  const {discovery} = useThunderID();
  const {copied: secretCopied, copy: copySecret} = useCopyToClipboard({resetDelay: 2000});
  const [showSecret, setShowSecret] = useState(false);

  const isM2m = clientType === McpClientTypes.M2M;
  const hasSecret = isM2m && Boolean(clientSecret);

  const wellKnown = (discovery as {wellKnown?: McpDiscoveryEndpoints | null} | undefined)?.wellKnown;
  const tokenEndpoint = wellKnown?.token_endpoint;

  const copyLabel = t('common:actions.copy');
  const clientIdLabel = t('applications:clientSecret.clientIdLabel');
  const clientSecretLabel = t('applications:clientSecret.clientSecretLabel');
  const tokenEndpointLabel = t('applications:onboarding.mcp.complete.endpoints.token');

  const endpointRows = getMcpDiscoveryEndpointRows(wellKnown, t);

  const handleSecretCopy = (): void => {
    if (!clientSecret) return;
    copySecret(clientSecret).catch(() => {
      // Error already handled in copySecret
    });
  };

  return (
    <Stack spacing={4} sx={{width: '100%', alignItems: 'center'}} data-testid="application-mcp-connect-complete">
      <Box sx={{alignSelf: 'center'}} aria-hidden>
        {isM2m ? (
          <AlertTriangle size={64} color="var(--mui-palette-warning-main)" />
        ) : (
          <CheckCircle size={64} color="var(--mui-palette-success-main)" />
        )}
      </Box>

      <Stack spacing={1} sx={{textAlign: 'center'}}>
        <Typography variant="h3" component="h1">
          {t('applications:onboarding.mcp.complete.title')}
        </Typography>
        <Typography variant="body1" color="text.secondary" data-testid="application-mcp-connect-complete-subtitle">
          {isM2m
            ? t('applications:onboarding.mcp.complete.subtitle.m2m')
            : t('applications:onboarding.mcp.complete.subtitle.userDelegated')}
        </Typography>
      </Stack>

      <Stack spacing={3} sx={{width: '100%', maxWidth: 800}}>
        <Box sx={cardSx}>
          <Typography variant="subtitle1" sx={{fontWeight: 600, mb: 2}}>
            {t('applications:onboarding.mcp.complete.credentials.title')}
          </Typography>
          <Stack spacing={2}>
            {isM2m && appName && (
              <>
                <Box>
                  <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 0.5}}>
                    {t('applications:clientSecret.appNameLabel')}
                  </Typography>
                  <Typography variant="body1">{appName}</Typography>
                </Box>
                <Divider />
              </>
            )}

            <CopyableField
              id="mcp-connect-client-id"
              label={clientIdLabel}
              value={clientId}
              copyAriaLabel={`${copyLabel} ${clientIdLabel}`}
            />

            {hasSecret && (
              <>
                <Divider />
                <Box>
                  <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 0.5}}>
                    {clientSecretLabel}
                  </Typography>
                  <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 1}}>
                    {t('applications:onboarding.mcp.complete.m2m.secretPurpose')}
                  </Typography>
                  <TextField
                    fullWidth
                    data-testid="application-mcp-connect-client-secret-value"
                    type={showSecret ? 'text' : 'password'}
                    value={clientSecret}
                    InputProps={{
                      readOnly: true,
                      endAdornment: (
                        <InputAdornment position="end">
                          <IconButton
                            aria-label={t('applications:regenerateSecret.success.toggleVisibility')}
                            onClick={() => setShowSecret((prev) => !prev)}
                            edge="end"
                            size="small"
                          >
                            {showSecret ? <EyeOff size={16} /> : <Eye size={16} />}
                          </IconButton>
                          <IconButton
                            aria-label={`${copyLabel} ${clientSecretLabel}`}
                            onClick={handleSecretCopy}
                            edge="end"
                            size="small"
                            sx={{ml: 0.5}}
                          >
                            {secretCopied ? <Check size={16} /> : <Copy size={16} />}
                          </IconButton>
                        </InputAdornment>
                      ),
                    }}
                    sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
                  />
                </Box>
              </>
            )}

            {isM2m && tokenEndpoint && (
              <>
                <Divider />
                <CopyableField
                  id="mcp-connect-token-endpoint"
                  label={tokenEndpointLabel}
                  value={tokenEndpoint}
                  copyAriaLabel={`${copyLabel} ${tokenEndpointLabel}`}
                />
              </>
            )}
          </Stack>
        </Box>

        {!isM2m && endpointRows.length > 0 && (
          <Box sx={cardSx}>
            <Typography variant="subtitle1" sx={{fontWeight: 600, mb: 2}}>
              {t('applications:onboarding.mcp.complete.endpoints.title')}
            </Typography>
            <Stack spacing={2}>
              {endpointRows.map((row) => (
                <CopyableField
                  key={row.key}
                  id={`mcp-connect-endpoint-${row.key}`}
                  label={row.label}
                  value={row.value}
                  copyAriaLabel={`${copyLabel} ${row.label}`}
                />
              ))}
            </Stack>
          </Box>
        )}

        {!isM2m && redirectUris.length > 0 && (
          <Box sx={cardSx}>
            <Typography variant="subtitle1" sx={{fontWeight: 600, mb: 2}}>
              {t('applications:onboarding.mcp.complete.redirectUris.title')}
            </Typography>
            <Stack spacing={1.5}>
              {redirectUris.map((uri, index) => (
                <CopyableListRow
                  // IMPORTANT: Do not remove the suppression since it affects functionality.
                  // eslint-disable-next-line react/no-array-index-key
                  key={`${uri}-${index}`}
                  value={uri}
                  copyAriaLabel={`${copyLabel} ${t('applications:onboarding.mcp.complete.redirectUris.title')}`}
                />
              ))}
            </Stack>
          </Box>
        )}

        {isM2m && (
          <Alert severity="warning" icon={<AlertTriangle size={20} />}>
            <Typography variant="body2" sx={{fontWeight: 'medium', mb: 1}}>
              {t('applications:onboarding.mcp.complete.m2m.warning.title')}
            </Typography>
            <Typography variant="body2">{t('applications:onboarding.mcp.complete.m2m.warning.body')}</Typography>
          </Alert>
        )}

        {isM2m && (
          <Alert severity="info" icon={<Info size={20} />}>
            <Typography variant="body2" sx={{mb: 1}}>
              {t('applications:onboarding.mcp.complete.m2m.tokenHint')}
            </Typography>
            <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
              <Typography component="code" sx={codeSx}>
                grant_type=client_credentials
              </Typography>
              <Typography component="code" sx={codeSx}>
                resource
              </Typography>
            </Stack>
          </Alert>
        )}
      </Stack>

      {isM2m ? (
        <Stack direction="row" spacing={2} sx={{width: '100%', maxWidth: 800}}>
          <Button
            variant="contained"
            fullWidth
            startIcon={<Copy size={16} />}
            onClick={handleSecretCopy}
            disabled={secretCopied}
          >
            {secretCopied
              ? t('applications:onboarding.mcp.complete.copied')
              : t('applications:onboarding.mcp.complete.copySecret')}
          </Button>
          <Button variant="outlined" fullWidth onClick={onContinue}>
            {t('applications:onboarding.mcp.complete.goToApp')}
          </Button>
        </Stack>
      ) : (
        <Button variant="contained" sx={{minWidth: 200}} onClick={onContinue}>
          {t('applications:onboarding.mcp.complete.goToApp')}
        </Button>
      )}
    </Stack>
  );
}
