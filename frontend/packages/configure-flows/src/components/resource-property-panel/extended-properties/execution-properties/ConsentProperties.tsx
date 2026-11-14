// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Checkbox, FormControlLabel, FormHelperText, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import DraftTextField from './DraftTextField';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {clampToInteger} from './utils';
import VariableBinding from './VariableBinding';
import {BOOLEAN_VARIABLES, NUMBER_VARIABLES} from './variableBindingTypes';
import type {StepData} from '../../../../models/steps';

interface ConsentPropertyValues {
  timeout?: string;
  failOnDeny?: boolean;
  promptValidityPeriod?: boolean;
}

function ConsentProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo<ConsentPropertyValues>(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.properties as ConsentPropertyValues | undefined) ?? {};
  }, [resource]);

  const currentTimeout = properties.timeout ?? '0';

  const failOnDenyChecked = !!properties.failOnDeny;

  const promptValidityPeriodChecked = !!properties.promptValidityPeriod;

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.consent.description')}
      </Typography>

      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="timeout"
        accepts={NUMBER_VARIABLES}
        label={t('flows:core.executions.consent.timeout.label')}
      >
        {({commit, label}) => (
          <DraftTextField
            labelPlacement="top"
            label={label}
            helperText={t('flows:core.executions.consent.timeout.hint')}
            id="consent-timeout"
            value={currentTimeout}
            // The executor parses this property as a string, so it is stored as one.
            onCommit={commit}
            normalize={(raw) => clampToInteger(raw, 0)}
            placeholder={t('flows:core.executions.consent.timeout.placeholder')}
            fullWidth
            size="small"
            type="number"
            inputProps={{min: 0}}
          />
        )}
      </VariableBinding>

      <VariableBinding
        inline
        resource={resource}
        onChange={onChange}
        propertyKey="failOnDeny"
        accepts={BOOLEAN_VARIABLES}
        label={t('flows:core.executions.consent.failOnDeny.label')}
      >
        {({commit, label}) => (
          <div>
            <FormControlLabel
              control={<Checkbox checked={failOnDenyChecked} onChange={(e) => commit(e.target.checked)} size="small" />}
              label={label}
            />
            <FormHelperText>{t('flows:core.executions.consent.failOnDeny.hint')}</FormHelperText>
          </div>
        )}
      </VariableBinding>

      <VariableBinding
        inline
        resource={resource}
        onChange={onChange}
        propertyKey="promptValidityPeriod"
        accepts={BOOLEAN_VARIABLES}
        label={t(
          'flows:core.executions.consent.promptValidityPeriod.label',
          'Let the user choose the consent validity period',
        )}
      >
        {({commit, label}) => (
          <div>
            <FormControlLabel
              control={
                <Checkbox
                  checked={promptValidityPeriodChecked}
                  onChange={(e) => commit(e.target.checked)}
                  size="small"
                />
              }
              label={label}
            />
            <FormHelperText>
              {t(
                'flows:core.executions.consent.promptValidityPeriod.hint',
                'The user picks a period or types their own, up to the application or agent period.',
              )}
            </FormHelperText>
          </div>
        )}
      </VariableBinding>
    </Stack>
  );
}

export default ConsentProperties;
