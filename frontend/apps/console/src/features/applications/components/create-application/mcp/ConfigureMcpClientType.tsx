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

import {Box, Card, CardContent, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX, KeyboardEvent} from 'react';
import {useEffect} from 'react';
import {useTranslation} from 'react-i18next';
import ConfigureMcpConnection from './ConfigureMcpConnection';
import McpClientTypePreview from './McpClientTypePreview';
import McpClientTypeMetadataList from '../../../config/McpClientTypeMetadata';
import {McpClientTypes} from '../../../models/mcp-client';
import type {McpClientType} from '../../../models/mcp-client';

/**
 * Props for the {@link ConfigureMcpClientType} component.
 *
 * @public
 */
export interface ConfigureMcpClientTypeProps {
  /**
   * The currently selected MCP client type
   */
  selectedType: McpClientType;

  /**
   * Callback function invoked when a client type card is selected
   */
  onSelect: (type: McpClientType) => void;

  /**
   * The currently configured redirect URIs, collected inline when the user-delegated type is
   * selected
   */
  redirectUris: string[];

  /**
   * Callback function invoked when the redirect URIs change
   */
  onRedirectUrisChange: (uris: string[]) => void;

  /**
   * Callback function to broadcast whether this step is ready to proceed
   */
  onReadyChange?: (isReady: boolean) => void;
}

/**
 * React component that renders the MCP client type chooser shown on the
 * Client type step of the mcp-client template's creation flow.
 *
 * Presents the two MCP client types — On behalf of a user and On its own behalf —
 * as a pair of selectable cards. The pair behaves as a single-select radio
 * group of two options, so it exposes `role="radiogroup"`/`role="radio"`
 * semantics rather than the gallery card's `button`/`aria-pressed` contract.
 * Below the cards, a "what you get" preview panel ({@link McpClientTypePreview}) shows the
 * consequences of the current selection. When the user-delegated type is selected, the
 * redirect URI editor ({@link ConfigureMcpConnection}) is embedded inline beneath the preview
 * panel so the client type and connection details are collected in a single step.
 *
 * @param props - The component props
 * @param props.selectedType - The currently selected MCP client type
 * @param props.onSelect - Callback invoked when a client type card is selected
 * @param props.redirectUris - The currently configured redirect URIs
 * @param props.onRedirectUrisChange - Callback invoked when the redirect URIs change
 * @param props.onReadyChange - Optional callback to notify parent of step readiness
 *
 * @returns JSX element displaying the MCP client type chooser
 *
 * @example
 * ```tsx
 * import ConfigureMcpClientType from './ConfigureMcpClientType';
 *
 * function NameAndTypeStep() {
 *   const [clientType, setClientType] = useState<McpClientType>('userDelegated');
 *   const [redirectUris, setRedirectUris] = useState<string[]>([]);
 *
 *   return (
 *     <ConfigureMcpClientType
 *       selectedType={clientType}
 *       onSelect={setClientType}
 *       redirectUris={redirectUris}
 *       onRedirectUrisChange={setRedirectUris}
 *     />
 *   );
 * }
 * ```
 *
 * @public
 */
export default function ConfigureMcpClientType({
  selectedType,
  onSelect,
  redirectUris,
  onRedirectUrisChange,
  onReadyChange = undefined,
}: ConfigureMcpClientTypeProps): JSX.Element {
  const {t} = useTranslation();

  const clientTypeLabel = t('applications:onboarding.mcp.clientType.title');

  // The machine-to-machine type has no redirect-based authorization code flow, so the step is
  // always ready as soon as it's selected — the connection editor below is not rendered and
  // won't fire its own readiness.
  useEffect((): void => {
    if (selectedType === McpClientTypes.M2M) {
      onReadyChange?.(true);
    }
  }, [selectedType, onReadyChange]);

  return (
    <Stack direction="column" spacing={2} data-testid="application-configure-mcp-client-type">
      <Stack direction="column" spacing={0.5}>
        <Typography variant="h6">{clientTypeLabel}</Typography>
        <Typography variant="body2" color="text.secondary">
          {t('applications:onboarding.mcp.clientType.subtitle')}
        </Typography>
      </Stack>

      <Box
        role="radiogroup"
        aria-label={clientTypeLabel}
        sx={{
          display: 'grid',
          gridTemplateColumns: {xs: '1fr', sm: 'repeat(2, 1fr)'},
          gap: 2,
        }}
      >
        {McpClientTypeMetadataList.map((option) => {
          const isSelected = selectedType === option.value;

          return (
            <Card
              key={option.value}
              variant="outlined"
              role="radio"
              tabIndex={0}
              aria-checked={isSelected}
              onClick={() => onSelect(option.value)}
              onKeyDown={(e: KeyboardEvent<HTMLDivElement>) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  onSelect(option.value);
                }
              }}
              sx={{
                borderRadius: 2,
                borderWidth: isSelected ? 2 : 1,
                borderColor: isSelected ? 'primary.main' : 'divider',
                cursor: 'pointer',
                bgcolor: isSelected ? 'action.selected' : 'background.paper',
                transition: 'border-color 0.15s, box-shadow 0.15s, transform 0.15s',
                '&:hover': {
                  borderColor: 'primary.main',
                  boxShadow: '0 4px 12px rgba(0,0,0,0.1)',
                  transform: 'translateY(-2px)',
                },
                '&:focus-visible': {
                  outline: 'none',
                  borderColor: 'primary.main',
                  boxShadow: '0 4px 12px rgba(0,0,0,0.1)',
                  transform: 'translateY(-2px)',
                },
              }}
            >
              <CardContent sx={{p: 2.5, '&:last-child': {pb: 2.5}}}>
                <Stack direction="column" spacing={2}>
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      width: 48,
                      height: 48,
                    }}
                  >
                    {option.icon}
                  </Box>
                  <Stack direction="column" spacing={0.75}>
                    <Typography variant="subtitle1" sx={{fontWeight: 600, lineHeight: 1.3}}>
                      {t(option.titleKey)}
                    </Typography>
                    <Typography variant="body2" color="text.secondary" sx={{lineHeight: 1.5}}>
                      {t(option.descriptionKey)}
                    </Typography>
                  </Stack>
                </Stack>
              </CardContent>
            </Card>
          );
        })}
      </Box>

      <McpClientTypePreview clientType={selectedType} />

      {selectedType === McpClientTypes.USER_DELEGATED && (
        <ConfigureMcpConnection
          compact
          redirectUris={redirectUris}
          onRedirectUrisChange={onRedirectUrisChange}
          onReadyChange={onReadyChange}
        />
      )}
    </Stack>
  );
}
