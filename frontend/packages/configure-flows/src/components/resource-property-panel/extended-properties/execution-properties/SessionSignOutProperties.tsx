// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Checkbox, FormControlLabel, FormHelperText, Stack, Typography} from '@wso2/oxygen-ui';
import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import VariableBinding from './VariableBinding';
import {BOOLEAN_VARIABLES} from './variableBindingTypes';

function SessionSignOutProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.sessionSignOut.description')}
      </Typography>

      <VariableBinding
        inline
        resource={resource}
        onChange={onChange}
        propertyKey="promptOnSignOut"
        accepts={BOOLEAN_VARIABLES}
        label={t('flows:core.executions.sessionSignOut.promptOnSignOut.label')}
      >
        {({commit, label, value: stored}) => (
          <div>
            <FormControlLabel
              control={<Checkbox checked={!!stored} onChange={(e) => commit(e.target.checked)} size="small" />}
              label={label}
            />
            <FormHelperText>{t('flows:core.executions.sessionSignOut.promptOnSignOut.hint')}</FormHelperText>
          </div>
        )}
      </VariableBinding>
    </Stack>
  );
}

export default SessionSignOutProperties;
