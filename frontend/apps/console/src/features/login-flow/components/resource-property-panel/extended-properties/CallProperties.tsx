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

import {FormControl, FormHelperText, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import {useParams} from 'react-router';
import useGetFlows from '@/features/flows/api/useGetFlows';
import type {CommonResourcePropertiesPropsInterface} from '@/features/flows/components/resource-property-panel/ResourceProperties';
import useResourceFieldError from '@/features/flows/hooks/useResourceFieldError';
import {FlowType} from '@/features/flows/models/flows';
import type {BasicFlowDefinition} from '@/features/flows/models/responses';
import type {StepData} from '@/features/flows/models/steps';

export type CallPropertiesPropsInterface = CommonResourcePropertiesPropsInterface;

function CallProperties({resource, onChange}: CallPropertiesPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {flowId} = useParams<{flowId: string}>();
  const {data, isLoading, error} = useGetFlows({limit: 100});

  const currentRef = useMemo<string>(() => {
    const stepData = resource?.data as (StepData & {flow?: {ref?: string}}) | undefined;
    return stepData?.flow?.ref ?? '';
  }, [resource]);

  // Widget-seeded UI hint: when a call step is inserted via a composite widget
  // (e.g. Sign In Link), the widget pins a preferred flow type so the dropdown
  // only lists flows of that type. Purely presentational — the backend still
  // accepts any flow ref, so re-dropping the widget or picking again from a
  // generic Call step lifts the filter.
  const filterFlowType: string | undefined = useMemo<string | undefined>(() => {
    const stepData = resource?.data as (StepData & {flow?: {filterFlowType?: string}}) | undefined;
    return stepData?.flow?.filterFlowType;
  }, [resource]);

  // Sign-out flows are excluded as call targets: terminating an SSO session part-way through another
  // flow (e.g. a login flow) is not a meaningful composition, so they are not offered here.
  const flows: BasicFlowDefinition[] = useMemo<BasicFlowDefinition[]>(() => {
    const list = data?.flows ?? [];
    return list.filter(
      (f: BasicFlowDefinition) =>
        f.id !== flowId && f.flowType !== FlowType.SIGNOUT && (!filterFlowType || f.flowType === filterFlowType),
    );
  }, [data, flowId, filterFlowType]);

  const isRefKnown: boolean = useMemo<boolean>(
    () => Boolean(currentRef) && flows.some((f: BasicFlowDefinition) => f.id === currentRef),
    [currentRef, flows],
  );

  const validatorMessage: string = useResourceFieldError(resource?.id, 'data.flow.ref');

  const staleRefMessage: string =
    !validatorMessage && currentRef && !isLoading && !isRefKnown
      ? t('flows:core.call.properties.flow.error.unknown', 'The referenced flow no longer exists. Pick a valid flow.')
      : '';

  const errorMessage: string = validatorMessage || staleRefMessage;

  const handleChange = (selected: string): void => {
    onChange('data.flow', filterFlowType ? {ref: selected, filterFlowType} : {ref: selected}, resource);
  };

  if (error) {
    return (
      <Typography variant="body2" color="error" data-testid="call-properties-error">
        {t('flows:core.call.properties.loadError', 'Failed to load available flows')}
      </Typography>
    );
  }

  return (
    <Stack gap={2} data-testid="call-properties">
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.call.properties.description', 'Pick the flow to invoke when this node executes.')}
      </Typography>
      <FormControl fullWidth size="small" error={Boolean(errorMessage)}>
        <FormLabel htmlFor="call-flow-ref-select">
          {t('flows:core.call.properties.flow.label', 'Referenced flow')}
        </FormLabel>
        <Select
          id="call-flow-ref-select"
          data-testid="call-flow-ref-select"
          value={isRefKnown ? currentRef : ''}
          disabled={isLoading || flows.length === 0}
          onChange={(e) => handleChange(String(e.target.value))}
          displayEmpty
          fullWidth
        >
          <MenuItem value="" disabled>
            {isLoading
              ? t('flows:core.call.properties.flow.loading', 'Loading flows…')
              : t('flows:core.call.properties.flow.placeholder', 'Select a flow')}
          </MenuItem>
          {flows.map((f: BasicFlowDefinition) => (
            <MenuItem key={f.id} value={f.id}>
              {f.name} ({f.flowType})
            </MenuItem>
          ))}
        </Select>
        {errorMessage && <FormHelperText data-testid="call-flow-ref-error">{errorMessage}</FormHelperText>}
      </FormControl>
    </Stack>
  );
}

export default CallProperties;
