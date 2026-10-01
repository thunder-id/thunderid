// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Divider,
  FormControlLabel,
  Radio,
  RadioGroup,
  Stack,
  Typography,
  useTheme,
} from '@wso2/oxygen-ui';
import {Lightbulb} from '@wso2/oxygen-ui-icons-react';
import type {ChangeEvent, JSX} from 'react';
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
 * React component that renders the MCP client type chooser shown on the Configuration step
 * of the mcp-client template's creation flow, headed "Client Type" — a choice unique to MCP
 * clients, distinct from the "Sign-In Approach" heading used by other templates' Configuration
 * step.
 *
 * Presents the two MCP client types — On behalf of a user and On its own behalf —
 * as a stacked list of radio cards, mirroring the standard flow's Sign-In Approach layout so the
 * two client-obtains-tokens choices in the product read as one consistent pattern.
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
  const theme = useTheme();

  const clientTypeLabel = t('applications:onboarding.mcp.clientType.title');

  // The machine-to-machine type has no redirect-based authorization code flow, so the step is
  // always ready as soon as it's selected — the connection editor below is not rendered and
  // won't fire its own readiness.
  useEffect((): void => {
    if (selectedType === McpClientTypes.M2M) {
      onReadyChange?.(true);
    }
  }, [selectedType, onReadyChange]);

  const handleApproachChange = (event: ChangeEvent<HTMLInputElement>): void => {
    onSelect(event.target.value as McpClientType);
  };

  return (
    <Stack direction="column" spacing={2} data-testid="application-configure-mcp-client-type">
      <Stack direction="column" spacing={0.5}>
        <Typography variant="h1" gutterBottom>
          {clientTypeLabel}
        </Typography>
        <Stack direction="row" alignItems="center" spacing={1}>
          <Lightbulb size={20} color={theme.vars?.palette.warning.main} />
          <Typography variant="body2" color="text.secondary">
            {t('applications:onboarding.mcp.clientType.subtitle')}
          </Typography>
        </Stack>
      </Stack>

      <RadioGroup aria-label={clientTypeLabel} value={selectedType} onChange={handleApproachChange}>
        <Stack direction="column" spacing={2}>
          {McpClientTypeMetadataList.map((option) => {
            const isSelected = selectedType === option.value;

            return (
              <Card key={option.value} variant="outlined" onClick={() => onSelect(option.value)}>
                <CardActionArea
                  sx={{
                    height: '100%',
                    cursor: 'pointer',
                    border: 1,
                    borderColor: isSelected ? 'primary.main' : 'divider',
                    transition: 'all 0.2s ease-in-out',
                    '&:hover': {
                      borderColor: 'primary.main',
                      bgcolor: isSelected ? 'action.selected' : 'action.hover',
                    },
                  }}
                >
                  <CardContent>
                    <Stack direction="row" spacing={2} alignItems="flex-start">
                      <FormControlLabel
                        value={option.value}
                        control={<Radio />}
                        label=""
                        sx={{m: 0}}
                        onClick={(e) => e.stopPropagation()}
                      />
                      <Box sx={{flex: 1}}>
                        <Stack direction="row" spacing={1} alignItems="center" sx={{mb: 1}}>
                          {option.icon}
                          <Typography variant="h6">{t(option.titleKey)}</Typography>
                        </Stack>
                        <Typography variant="body2" color="text.secondary">
                          {t(option.descriptionKey)}
                        </Typography>

                        {isSelected && (
                          <>
                            <Divider sx={{my: 2}} />
                            <McpClientTypePreview clientType={option.value} />
                          </>
                        )}
                      </Box>
                    </Stack>
                  </CardContent>
                </CardActionArea>
              </Card>
            );
          })}
        </Stack>
      </RadioGroup>

      {selectedType === McpClientTypes.USER_DELEGATED && (
        <Stack spacing={3}>
          <Typography variant="h1" gutterBottom>
            {t('applications:onboarding.configure.details.urls.title', 'URLs')}
          </Typography>
          <ConfigureMcpConnection
            compact
            redirectUris={redirectUris}
            onRedirectUrisChange={onRedirectUrisChange}
            onReadyChange={onReadyChange}
          />
        </Stack>
      )}
    </Stack>
  );
}
