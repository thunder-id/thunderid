// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Checkbox, FormControlLabel, FormHelperText, Stack} from '@wso2/oxygen-ui';
import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import VariableBinding from './VariableBinding';
import {BOOLEAN_VARIABLES} from './variableBindingTypes';

function AttributeUniquenessValidatorProperties({
  resource,
  onChange,
}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  return (
    <Stack gap={2}>
      <VariableBinding
        inline
        resource={resource}
        onChange={onChange}
        propertyKey="allowCrossOUProvisioning"
        accepts={BOOLEAN_VARIABLES}
        label={t('flows:core.executions.federation.allowCrossOUProvisioning.label')}
      >
        {({commit, label, value: stored}) => (
          <div>
            <FormControlLabel
              control={<Checkbox checked={!!stored} onChange={(e) => commit(e.target.checked)} size="small" />}
              label={label}
            />
            <FormHelperText>{t('flows:core.executions.federation.allowCrossOUProvisioning.hint')}</FormHelperText>
          </div>
        )}
      </VariableBinding>
    </Stack>
  );
}

export default AttributeUniquenessValidatorProperties;
