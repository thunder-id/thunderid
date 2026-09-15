// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Checkbox, FormControlLabel, FormHelperText, FormLabel, Stack, Typography} from '@wso2/oxygen-ui';
import {useCallback, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import DraftTextField from './DraftTextField';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {clampToInteger} from './utils';
import type {StepData} from '@/features/flows/models/steps';

interface ConsentPropertyValues {
  timeout?: string;
  failOnDeny?: boolean;
}

function ConsentProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo<ConsentPropertyValues>(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.properties as ConsentPropertyValues | undefined) ?? {};
  }, [resource]);

  const currentTimeout = properties.timeout ?? '0';

  const failOnDenyChecked = !!properties.failOnDeny;

  const handleFailOnDenyChange = useCallback(
    (checked: boolean): void => {
      onChange('data.properties.failOnDeny', checked, resource);
    },
    [resource, onChange],
  );

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.consent.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="consent-timeout">{t('flows:core.executions.consent.timeout.label')}</FormLabel>
        <DraftTextField
          id="consent-timeout"
          value={currentTimeout}
          // The executor parses this property as a string, so it is stored as one.
          onCommit={(value) => onChange('data.properties.timeout', value, resource)}
          normalize={(raw) => clampToInteger(raw, 0)}
          placeholder={t('flows:core.executions.consent.timeout.placeholder')}
          fullWidth
          size="small"
          type="number"
          inputProps={{min: 0}}
        />
        <FormHelperText>{t('flows:core.executions.consent.timeout.hint')}</FormHelperText>
      </div>

      <div>
        <FormControlLabel
          control={
            <Checkbox
              checked={failOnDenyChecked}
              onChange={(e) => handleFailOnDenyChange(e.target.checked)}
              size="small"
            />
          }
          label={t('flows:core.executions.consent.failOnDeny.label')}
        />
        <FormHelperText>{t('flows:core.executions.consent.failOnDeny.hint')}</FormHelperText>
      </div>
    </Stack>
  );
}

export default ConsentProperties;
