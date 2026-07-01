/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {Box, Typography, Stack, Autocomplete, TextField, Divider} from '@wso2/oxygen-ui';
import {Workflow} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useMemo} from 'react';
import {useTranslation} from 'react-i18next';
import {type BasicFlowDefinition} from '../../../../flows/models/responses';

/**
 * Props for the FlowsListView component
 */
export interface FlowsListViewProps {
  /**
   * Available authentication flows
   */
  availableFlows: BasicFlowDefinition[];

  /**
   * Currently selected authentication flow
   */
  selectedAuthFlow: BasicFlowDefinition | null;

  /**
   * Callback when a flow is selected
   */
  onFlowSelect: (flowId: string) => void;

  /**
   * Callback when clearing flow selection
   */
  onClearSelection: () => void;

  /**
   * Whether the flows list should be disabled
   */
  disabled?: boolean;
}

/**
 * Component that renders the flows list view with either radio buttons or autocomplete
 */
export default function FlowsListView({
  availableFlows,
  selectedAuthFlow,
  onFlowSelect,
  onClearSelection,
  disabled = false,
}: FlowsListViewProps): JSX.Element | null {
  const {t} = useTranslation();

  // Exclude the console-app flows which is reserved for the system management console itself & default flows.
  const selectableFlows: BasicFlowDefinition[] = useMemo(
    () => availableFlows.filter((flow) => !flow.handle.includes('console-app-') && !flow.handle.startsWith('default-')),
    [availableFlows],
  );

  if (selectableFlows.length === 0) {
    return null;
  }

  return (
    <>
      <Divider sx={{my: 2}}>
        <Typography variant="body2" color="text.secondary" sx={{px: 2}}>
          {t('common:or')}
        </Typography>
      </Divider>
      <Stack direction="column" spacing={2}>
        <Stack direction="column" spacing={2}>
          <Autocomplete
            disabled={disabled}
            options={selectableFlows}
            getOptionLabel={(option) => option.name}
            value={selectableFlows.find((flow) => flow.id === selectedAuthFlow?.id) ?? null}
            onChange={(_, newValue) => {
              if (newValue?.id) {
                onFlowSelect(newValue.id);
              } else {
                onClearSelection();
              }
            }}
            renderInput={(params) => (
              <TextField
                {...params}
                label={t('applications:onboarding.configure.SignInOptions.preConfiguredFlows.selectFlow')}
                placeholder={t('applications:onboarding.configure.SignInOptions.preConfiguredFlows.searchFlows')}
              />
            )}
            renderOption={(props, option) => (
              <Box component="li" {...props}>
                <Box sx={{display: 'flex', alignItems: 'center', gap: 1, width: '100%'}}>
                  <Workflow size={20} />
                  <Box>
                    <Typography variant="body2" fontWeight="medium">
                      {option.name}
                    </Typography>
                  </Box>
                </Box>
              </Box>
            )}
          />
        </Stack>
      </Stack>
    </>
  );
}
