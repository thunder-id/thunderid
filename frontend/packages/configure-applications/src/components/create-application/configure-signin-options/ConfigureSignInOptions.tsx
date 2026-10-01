// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {
  AuthenticatorTypes,
  IdentityProviderTypes,
  useIdentityProviders,
  type IdentityProvider,
} from '@thunderid/configure-connections';
import {
  type BasicFlowDefinition,
  findMatchingFlowForIntegrations,
  FlowType,
  useGetFlows,
} from '@thunderid/configure-flows';
import {Typography, Stack, CircularProgress, Alert, Box, Divider, useTheme} from '@wso2/oxygen-ui';
import {Lightbulb} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useEffect, useMemo, useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import FlowsListView from './FlowsListView';
import PasswordlessLoginGroup from './PasswordlessLoginGroup';
import PromptForCredentialsGroup from './PromptForCredentialsGroup';
import SocialLoginGroup from './SocialLoginGroup';
import useApplicationCreateContext from '../../../hooks/useApplicationCreateContext';
import ConfigureMfaSettings from '../configure-security-settings/ConfigureMfaSettings';

/**
 * Props for the {@link ConfigureSignInOptions} component.
 *
 * @public
 */
export interface ConfigureSignInOptionsProps {
  /**
   * Record of enabled authentication integrations
   * Keys are integration IDs, values indicate whether they are enabled
   */
  integrations: Record<string, boolean>;

  /**
   * Callback function when an integration toggle state changes
   */
  onIntegrationToggle: (connectionId: string) => void;

  /**
   * Callback function to broadcast whether this step is ready to proceed
   */
  onReadyChange?: (isReady: boolean) => void;

  /**
   * Whether to render this step's own heading. Set to false when composed inside a larger step
   * (e.g. Sign-in Experience) that already renders its own title.
   * @defaultValue true
   */
  showTitle?: boolean;
}

/**
 * Check if at least one authentication option is selected OR a flow is selected
 *
 * @param integrations - Record of integration states
 * @param selectedFlow - Selected authentication flow
 * @returns True if at least one integration is enabled or a flow is selected
 *
 * @internal
 */
const hasAtLeastOneSelected = (
  integrations: Record<string, boolean>,
  selectedFlow: BasicFlowDefinition | null,
): boolean => Object.values(integrations).some((isEnabled) => isEnabled) || selectedFlow !== null;

/**
 * React component that renders the sign-in options configuration step in the
 * application creation onboarding flow.
 *
 * This component allows users to configure authentication methods for their application
 * by choosing between:
 * 1. Individual authentication integrations (Username & Password, Google, GitHub, etc.)
 * 2. Pre-configured authentication flows that may combine multiple methods
 *
 * Users can either toggle individual integrations OR select a pre-configured flow,
 * but not both simultaneously. When a flow is selected, individual integrations are
 * disabled and vice versa.
 *
 * The component fetches available identity providers and displays them as toggleable
 * list items with appropriate icons. Users can enable/disable multiple authentication
 * methods. The step is marked as ready only when at least one authentication option
 * is selected, ensuring applications have a valid sign-in mechanism.
 *
 * @param props - The component props
 * @param props.integrations - Record of enabled integrations (key: integration ID, value: enabled state)
 * @param props.onIntegrationToggle - Callback invoked when an integration is toggled
 * @param props.onReadyChange - Optional callback to notify parent of step readiness
 *
 * @returns JSX element displaying the sign-in options configuration interface
 *
 * @example
 * ```tsx
 * import ConfigureSignInOptions from './ConfigureSignInOptions';
 *
 * function OnboardingFlow() {
 *   const [integrations, setIntegrations] = useState({
 *     'username-password': true,
 *     'google-idp-id': false,
 *   });
 *
 *   const handleToggle = (id: string) => {
 *     setIntegrations(prev => ({
 *       ...prev,
 *       [id]: !prev[id]
 *     }));
 *   };
 *
 *   return (
 *     <ConfigureSignInOptions
 *       integrations={integrations}
 *       onIntegrationToggle={handleToggle}
 *       onReadyChange={(isReady) => console.log('Ready:', isReady)}
 *     />
 *   );
 * }
 * ```
 *
 * @public
 */
export default function ConfigureSignInOptions({
  integrations,
  onIntegrationToggle,
  onReadyChange = undefined,
  showTitle = true,
}: ConfigureSignInOptionsProps): JSX.Element {
  const {t} = useTranslation();
  const theme = useTheme();
  const {selectedAuthFlow, setSelectedAuthFlow, setIntegrations, setIsEmailOtpMfaEnabled, setIsSmsOtpMfaEnabled} =
    useApplicationCreateContext();

  const {data, isLoading, error, refetch} = useIdentityProviders();
  const {
    data: flowsData,
    isLoading: isFlowsLoading,
    error: flowsError,
    refetch: refetchFlows,
  } = useGetFlows({
    flowType: FlowType.AUTHENTICATION,
  });

  // Resolves an error through the code's owning namespace. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with
  // the given namespace, per getErrorMessage's namespace-resolution contract.
  const tForConnectionsErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `connections:${key}`, options),
    [t],
  );
  const tForFlowsErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `flows:${key}`, options),
    [t],
  );

  const availableIntegrations: IdentityProvider[] = useMemo(() => data ?? [], [data]);
  const availableFlows: BasicFlowDefinition[] = useMemo(() => flowsData?.flows ?? [], [flowsData?.flows]);

  /**
   * Map enabled integrations to flow-compatible types and find matching flow
   */
  const getFlowForEnabledIntegrations = useCallback(
    (integrationsState: Record<string, boolean>): BasicFlowDefinition | null => {
      const enabledIntegrations: string[] = Object.entries(integrationsState)
        .filter(([, enabled]) => enabled)
        .map(([integrationId]) => {
          // Handle basic auth
          if (integrationId === AuthenticatorTypes.CREDENTIALS_AUTH) {
            return AuthenticatorTypes.CREDENTIALS_AUTH;
          }

          // Find the provider to get its type
          const provider: IdentityProvider | undefined = availableIntegrations.find((idp) => idp.id === integrationId);
          if (provider) {
            switch (provider.type) {
              case IdentityProviderTypes.GOOGLE:
                return 'google';
              case IdentityProviderTypes.GITHUB:
                return 'github';
              default:
                return integrationId;
            }
          }

          // For other special flow types (like sms-otp)
          return integrationId;
        });

      return findMatchingFlowForIntegrations(enabledIntegrations, availableFlows);
    },
    [availableIntegrations, availableFlows],
  );

  /**
   * Broadcast readiness whenever integrations or selected flow change.
   */
  useEffect((): void => {
    // Mark step as ready if flow is selected OR integrations are enabled (trigger generation)
    const hasEnabledIntegrations = Object.values(integrations).some((enabled) => enabled);
    const isReady: boolean = selectedAuthFlow !== null || hasEnabledIntegrations;
    if (onReadyChange) {
      onReadyChange(isReady);
    }
  }, [integrations, selectedAuthFlow, onReadyChange]);

  /**
   * Auto-select matching flow when integrations change
   */
  useEffect((): void => {
    if (!selectedAuthFlow && availableFlows.length > 0) {
      const matchingFlow: BasicFlowDefinition | null = getFlowForEnabledIntegrations(integrations);
      if (matchingFlow) {
        setSelectedAuthFlow(matchingFlow);
      }
    }
  }, [integrations, availableFlows, selectedAuthFlow, setSelectedAuthFlow, getFlowForEnabledIntegrations]);

  const handleIntegrationToggle = (integrationId: string): void => {
    // Toggle the integration first
    onIntegrationToggle(integrationId);

    // Create the new integrations state
    const newIntegrations: Record<string, boolean> = {
      ...integrations,
      [integrationId]: !integrations[integrationId],
    };

    // Find matching flow for the new integration state
    const matchingFlow: BasicFlowDefinition | null = getFlowForEnabledIntegrations(newIntegrations);

    if (matchingFlow) {
      setSelectedAuthFlow(matchingFlow);
    } else {
      // If no matching flow found, check if we need to generate one
      const hasEnabledIntegrations = Object.values(newIntegrations).some((enabled) => enabled);

      if (hasEnabledIntegrations) {
        // Clear current selection while generating new flow
        setSelectedAuthFlow(null);
      } else {
        setSelectedAuthFlow(null);
      }
    }
  };

  if (isLoading || isFlowsLoading) {
    return (
      <Box sx={{display: 'flex', justifyContent: 'center', alignItems: 'center', py: 8}}>
        <CircularProgress />
      </Box>
    );
  }

  if (error || flowsError) {
    return (
      <Box sx={{mb: 4}}>
        <QueryErrorNotice
          error={error ?? flowsError!}
          t={error ? tForConnectionsErrors : tForFlowsErrors}
          variant="inline"
          fallbackKey="applications:onboarding.configure.SignInOptions.error"
          fallbackDefaultValue="Failed to load authentication methods"
          onRetry={() => {
            void refetch();
            void refetchFlows();
          }}
        />
      </Box>
    );
  }

  const hasAtLeastOneSelectedOption: boolean = hasAtLeastOneSelected(integrations, selectedAuthFlow);

  // Event handlers
  const handleFlowSelect = (flowId: string): void => {
    const selectedFlow: BasicFlowDefinition | null =
      availableFlows?.find((flow: BasicFlowDefinition) => flow.id === flowId) ?? null;

    setSelectedAuthFlow(selectedFlow);

    if (selectedFlow) {
      setIntegrations({});
      setIsEmailOtpMfaEnabled(false);
      setIsSmsOtpMfaEnabled(false);
    }
  };

  const handleClearFlowSelection = (): void => {
    setSelectedAuthFlow(null);
  };

  // The picker should only reveal/fill itself in response to an actual manual pick, not a
  // background auto-match riding along with active toggles (see the effect above) — otherwise
  // flipping a switch makes an unrelated section of the page silently pop open and fill in.
  const hasEnabledIntegrations = Object.values(integrations).some(Boolean);
  const visibleSelectedFlow = hasEnabledIntegrations ? null : selectedAuthFlow;

  // A flow only genuinely blocks the individual toggles when it was manually picked from the
  // pre-configured flows list (an opaque, arbitrary graph) — see handleFlowSelect, which clears
  // `integrations` on pick. Auto-matched flows (integrations still enabled) are regenerated on
  // submit from the toggle state, so they don't need to disable anything here.
  const isDisabledByPreConfiguredFlow = selectedAuthFlow !== null && !hasEnabledIntegrations;

  return (
    <Stack direction="column" spacing={3} data-testid="application-configure-sign-in">
      {showTitle && (
        <Stack direction="column" spacing={1}>
          <Typography variant="h1" gutterBottom>
            {t('applications:onboarding.configure.SignInOptions.title')}
          </Typography>
          <Typography variant="subtitle1" gutterBottom>
            {t('applications:onboarding.configure.SignInOptions.subtitle')}
          </Typography>
        </Stack>
      )}

      <Stack direction="row" alignItems="center" spacing={1} flexWrap="wrap">
        <Stack direction="row" alignItems="center" spacing={1}>
          <Lightbulb size={20} color={theme.vars?.palette.warning.main} />
          <Typography variant="body2" color="text.secondary">
            {t('applications:onboarding.configure.SignInOptions.hint')}
          </Typography>
        </Stack>
      </Stack>

      {/* Validation warning if no options selected */}
      {!hasAtLeastOneSelectedOption && (
        <Alert severity="warning" sx={{mb: 2}}>
          {t('applications:onboarding.configure.SignInOptions.noSelectionWarning')}
        </Alert>
      )}

      <Box sx={{border: '1px solid', borderColor: 'divider', borderRadius: '10px', overflow: 'hidden'}}>
        <PromptForCredentialsGroup
          integrations={integrations}
          onIntegrationToggle={handleIntegrationToggle}
          disabled={isDisabledByPreConfiguredFlow}
        />

        <Divider />

        <PasswordlessLoginGroup
          integrations={integrations}
          onIntegrationToggle={handleIntegrationToggle}
          disabled={isDisabledByPreConfiguredFlow}
        />

        <Divider />

        <SocialLoginGroup
          integrations={integrations}
          availableIntegrations={availableIntegrations}
          onIntegrationToggle={handleIntegrationToggle}
          disabled={isDisabledByPreConfiguredFlow}
        />

        <Divider />

        <ConfigureMfaSettings />
      </Box>

      {/* Pre-configured Authentication Flows — a distinct choice (pick one flow) rather than a
          toggleable method, so it's visually set apart from the boxed groups above rather than
          folded into the same card. */}
      <FlowsListView
        availableFlows={availableFlows}
        selectedAuthFlow={visibleSelectedFlow}
        onFlowSelect={handleFlowSelect}
        onClearSelection={handleClearFlowSelection}
        disabled={false}
      />
    </Stack>
  );
}
