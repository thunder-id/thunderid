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

import {Box, Stack, Switch, Tooltip, Typography} from '@wso2/oxygen-ui';
import {ShieldCheck} from '@wso2/oxygen-ui-icons-react';
import {useReactFlow, useUpdateNodeInternals} from '@xyflow/react';
import {useEffect, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import type {SsoFocusRequest, SsoPlacementState} from '../hooks/useSsoToggle';
import type {JoinResolution, SsoState} from '../utils/ssoGraphTransforms';
import useInteractionState from '@/features/flows/hooks/useInteractionState';

export interface SsoTogglePropsInterface {
  ssoState: SsoState;
  joinResolution: JoinResolution;
  placement: SsoPlacementState;
  isReadOnly: boolean;
  focusRequest: SsoFocusRequest | null;
  onFocusHandled: () => void;
  onEnable: () => void;
  onDisableRequest: () => void;
}

/**
 * Resource panel row that enables or disables SSO for a login flow. The toggle
 * state is derived from the graph, so template-created SSO flows read as
 * enabled without any stored flag.
 */
function SsoToggle({
  ssoState,
  joinResolution,
  placement,
  isReadOnly,
  focusRequest,
  onFocusHandled,
  onEnable,
  onDisableRequest,
}: SsoTogglePropsInterface): ReactElement {
  const {t} = useTranslation();
  const {fitView} = useReactFlow();
  const updateNodeInternals = useUpdateNodeInternals();
  const {setLastInteractedResource, setLastInteractedStepId} = useInteractionState();

  // Select the freshly inserted SSO check (opening its properties panel) and
  // focus it once the nodes have rendered.
  useEffect(() => {
    if (!focusRequest) {
      return;
    }
    setLastInteractedStepId(focusRequest.ssoCheckId);
    setLastInteractedResource(focusRequest.resource);
    requestAnimationFrame(() => {
      updateNodeInternals([focusRequest.ssoCheckId, focusRequest.sessionId]);
      fitView({duration: 500, maxZoom: 1.2, nodes: [{id: focusRequest.ssoCheckId}], padding: 0.3}).catch(() => {
        // Focusing is best-effort.
      });
    });
    onFocusHandled();
  }, [focusRequest, fitView, updateNodeInternals, setLastInteractedStepId, setLastInteractedResource, onFocusHandled]);

  let disabledReason: string | null = null;
  if (isReadOnly) {
    disabledReason = t('flows:sso.disabledReadOnly', 'This flow is read-only and cannot be modified.');
  } else if (!ssoState.enabled) {
    if (joinResolution.status === 'no-entry') {
      disabledReason = t('flows:sso.disabledNoEntry', 'Connect the Start step to a view step to enable SSO.');
    } else if (joinResolution.status === 'entry-not-prompt') {
      disabledReason = t(
        'flows:sso.disabledEntryNotPrompt',
        'To enable SSO, the flow must start with a view step. The SSO check needs a login screen to fall back to.',
      );
    } else if (joinResolution.status === 'no-assert') {
      disabledReason = t(
        'flows:sso.disabledNoAssert',
        'Add an authentication completion step to the flow before enabling SSO.',
      );
    }
  }

  const isDisabled = Boolean(disabledReason) || placement.active;
  const tooltip =
    disabledReason ??
    (ssoState.enabled
      ? t('flows:sso.toggleTooltipOn', 'Single sign-on is active for this flow. Turn off to remove the SSO wiring.')
      : '');

  return (
    <Tooltip title={tooltip} placement="right">
      <Stack direction="row" alignItems="center" gap={1} sx={{width: '100%'}}>
        <Box
          component="span"
          display="inline-flex"
          alignItems="center"
          sx={{flexShrink: 0, color: isDisabled ? 'text.disabled' : 'text.primary'}}
        >
          <ShieldCheck size={16} />
        </Box>
        <Stack direction="column" sx={{flex: 1, minWidth: 0}}>
          <Typography variant="subtitle2" fontWeight={600} color={isDisabled ? 'text.disabled' : 'text.primary'}>
            {t('flows:sso.toggleLabel', 'Enable SSO')}
          </Typography>
          <Typography variant="caption" color="text.secondary" sx={{lineHeight: 1.3}}>
            {t('flows:sso.toggleDescription', 'Reuse an active session to skip sign-in')}
          </Typography>
        </Stack>
        <Switch
          size="small"
          checked={ssoState.enabled}
          disabled={isDisabled}
          onChange={() => (ssoState.enabled ? onDisableRequest() : onEnable())}
          data-testid="sso-toggle"
          slotProps={{input: {'aria-label': t('flows:sso.toggleLabel', 'Enable SSO')}}}
        />
      </Stack>
    </Tooltip>
  );
}

export default SsoToggle;
