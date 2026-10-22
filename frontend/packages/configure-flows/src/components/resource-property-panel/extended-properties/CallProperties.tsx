// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormControl, FormHelperText, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import useGetFlows from '../../../api/useGetFlows';
import useResourceFieldError from '../../../hooks/useResourceFieldError';
import {FlowType} from '../../../models/flows';
import type {BasicFlowDefinition} from '../../../models/responses';
import type {StepData} from '../../../models/steps';
import type {CommonResourcePropertiesPropsInterface} from '../CommonResourceProperties';

export type CallPropertiesPropsInterface = CommonResourcePropertiesPropsInterface;

const APPLICATION_FLOW_TYPES = [FlowType.AUTHENTICATION, FlowType.REGISTRATION, FlowType.RECOVERY] as const;

interface CallTarget {
  mode?: 'fixed' | 'flow_type';
  ref?: string;
  flowId?: string;
  flowType?: FlowType;
  filterFlowType?: string;
}

function CallProperties({resource, onChange}: CallPropertiesPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {data, isLoading, error} = useGetFlows({limit: 100});
  const target: CallTarget = (resource?.data as StepData & {flow?: CallTarget})?.flow ?? {};
  const mode = target.mode ?? 'fixed';
  const currentFlowId = target.flowId ?? target.ref ?? '';
  const filterFlowType = target.filterFlowType;

  const flows: BasicFlowDefinition[] = useMemo(
    () =>
      (data?.flows ?? []).filter(
        (flow: BasicFlowDefinition) =>
          flow.flowType !== FlowType.SIGNOUT && (!filterFlowType || flow.flowType === filterFlowType),
      ),
    [data, filterFlowType],
  );
  const isFlowKnown = flows.some((flow) => flow.id === currentFlowId);
  const validatorMessage = useResourceFieldError(
    resource?.id,
    mode === 'flow_type' ? 'data.flow.flowType' : target.mode === 'fixed' ? 'data.flow.flowId' : 'data.flow.ref',
  );
  const staleFlowMessage =
    !validatorMessage && mode === 'fixed' && currentFlowId && !isLoading && !isFlowKnown
      ? t('flows:core.call.properties.flow.error.unknown', 'The referenced flow no longer exists. Pick a valid flow.')
      : '';
  const errorMessage = validatorMessage || staleFlowMessage;

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
      <FormControl fullWidth size="small">
        <FormLabel htmlFor="call-flow-source-select">{t('flows:core.call.properties.source.label', 'Mode')}</FormLabel>
        <Select
          id="call-flow-source-select"
          data-testid="call-flow-source-select"
          value={mode}
          onChange={(event) => {
            const nextTarget =
              event.target.value === 'fixed' ? {mode: 'fixed'} : {mode: 'flow_type', flowType: FlowType.AUTHENTICATION};
            onChange('data.flow', nextTarget, resource);
          }}
          fullWidth
        >
          <MenuItem value="fixed">{t('flows:core.call.properties.source.fixed', 'Select a flow')}</MenuItem>
          <MenuItem value="flow_type">{t('flows:core.call.properties.source.flowType', 'Select a flow type')}</MenuItem>
        </Select>
      </FormControl>
      {mode === 'flow_type' ? (
        <FormControl fullWidth size="small">
          <FormLabel htmlFor="call-flow-type-select">
            {t('flows:core.call.properties.flowType.label', 'Flow type')}
          </FormLabel>
          <Select
            id="call-flow-type-select"
            data-testid="call-flow-type-select"
            value={target.flowType ?? ''}
            onChange={(event) => {
              onChange('data.flow', {mode: 'flow_type', flowType: event.target.value as FlowType}, resource);
            }}
            fullWidth
          >
            {APPLICATION_FLOW_TYPES.map((flowType) => (
              <MenuItem key={flowType} value={flowType}>
                {flowType}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      ) : (
        <FormControl fullWidth size="small" error={Boolean(errorMessage)}>
          <FormLabel htmlFor="call-flow-ref-select">
            {t('flows:core.call.properties.flow.label', 'Referenced flow')}
          </FormLabel>
          <Select
            id="call-flow-ref-select"
            data-testid="call-flow-ref-select"
            value={isFlowKnown ? currentFlowId : ''}
            disabled={isLoading || flows.length === 0}
            onChange={(event) =>
              onChange(
                'data.flow',
                filterFlowType
                  ? {mode: 'fixed', flowId: String(event.target.value), filterFlowType}
                  : {mode: 'fixed', flowId: String(event.target.value)},
                resource,
              )
            }
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
      )}
    </Stack>
  );
}

export default CallProperties;
