// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Switch, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {AuthorizationDetailTypeForm} from '../../hooks/useAuthorizationDetailTypeForm';

export interface AskEveryTimeSettingProps {
  form: Pick<AuthorizationDetailTypeForm, 'askEveryTime' | 'setAskEveryTime'>;
  readOnly?: boolean;
}

/** Whether the user approves each request of the type, or an earlier approval can be reused. */
export default function AskEveryTimeSetting({form, readOnly = false}: AskEveryTimeSettingProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Box sx={{display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 2}}>
      <Box>
        <Typography variant="body2" id="authz-type-ask-every-time" sx={{fontWeight: 500}}>
          {t('authorizationDetailTypes:consent.askEveryTime', 'Ask every time')}
        </Typography>
        <Typography variant="caption" color="text.secondary" component="p">
          {t(
            'authorizationDetailTypes:consent.askEveryTimeHint',
            'On by default, for one-off approvals such as a single payment. Turn off to let an earlier approval of the same version stand in, while it is valid and covers the request.',
          )}
        </Typography>
      </Box>
      <Switch
        checked={form.askEveryTime}
        disabled={readOnly}
        slotProps={{input: {role: 'switch', 'aria-labelledby': 'authz-type-ask-every-time'}}}
        onChange={(e) => form.setAskEveryTime(e.target.checked)}
      />
    </Box>
  );
}
