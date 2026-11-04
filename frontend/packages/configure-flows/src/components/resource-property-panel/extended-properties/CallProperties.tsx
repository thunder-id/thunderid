// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormControl, FormHelperText, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import {useParams} from 'react-router';
import useGetFlows from '../../../api/useGetFlows';
import useResourceFieldError from '../../../hooks/useResourceFieldError';
import {FlowType} from '../../../models/flows';
import type {BasicFlowDefinition} from '../../../models/responses';
import type {StepData} from '../../../models/steps';
import type {CommonResourcePropertiesPropsInterface} from '../CommonResourceProperties';

export type CallPropertiesPropsInterface = CommonResourcePropertiesPropsInterface;

function CallProperties({resource, onChange}: CallPropertiesPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {flowId} = useParams<{flowId: string}>();
  const {data, isLoading, error} = useGetFlows({limit: 100});

  const flowData = resource?.data as (StepData & {flow?: {ref?: string; filterFlowType?: string}}) | undefined;
  const currentRef = flowData?.flow?.ref ?? '';
  const filterFlowType = flowData?.flow?.filterFlowType;

  const flows: BasicFlowDefinition[] = useMemo(() => {
    const list = data?.flows ?? [];
    return list.filter(
      (flow: BasicFlowDefinition) =>
        flow.id !== flowId &&
        flow.flowType !== FlowType.SIGNOUT &&
        flow.flowType !== FlowType.DEVICE_GRANT &&
        (!filterFlowType || flow.flowType === filterFlowType),
    );
  }, [data, flowId, filterFlowType]);

  const isRefKnown = Boolean(currentRef) && flows.some((flow) => flow.id === currentRef);
  const validatorMessage = useResourceFieldError(resource?.id, 'data.flow.ref');
  const staleRefMessage =
    !validatorMessage && currentRef && !isLoading && !isRefKnown
      ? t('flows:core.call.properties.flow.error.unknown', 'The referenced flow no longer exists. Pick a valid flow.')
      : '';
  const errorMessage = validatorMessage || staleRefMessage;

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
          onChange={(event) => handleChange(String(event.target.value))}
          displayEmpty
          fullWidth
        >
          <MenuItem value="" disabled>
            {isLoading
              ? t('flows:core.call.properties.flow.loading', 'Loading flows…')
              : t('flows:core.call.properties.flow.placeholder', 'Select a flow')}
          </MenuItem>
          {flows.map((flow) => (
            <MenuItem key={flow.id} value={flow.id}>
              {flow.name} ({flow.flowType})
            </MenuItem>
          ))}
        </Select>
        {errorMessage && <FormHelperText data-testid="call-flow-ref-error">{errorMessage}</FormHelperText>}
      </FormControl>
    </Stack>
  );
}

export default CallProperties;
