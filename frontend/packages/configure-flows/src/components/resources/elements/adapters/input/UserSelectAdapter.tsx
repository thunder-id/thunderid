// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Stack, Typography} from '@wso2/oxygen-ui';
import {Users} from '@wso2/oxygen-ui-icons-react';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import SelectAdapter, {type SelectAdapterPropsInterface} from './SelectAdapter';

/**
 * Props interface of {@link UserSelectAdapter}
 */
export type UserSelectAdapterPropsInterface = SelectAdapterPropsInterface;

/**
 * Adapter for USER_SELECT input elements in the flow builder.
 * Renders the same dropdown preview as a SELECT, plus a caption saying that its entries are the system
 * users, loaded page by page at runtime, so it is not mistaken for a SELECT whose options the flow carries.
 *
 * @param props - Props injected to the component.
 * @returns The UserSelectAdapter component.
 */
function UserSelectAdapter({resource}: UserSelectAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();

  return (
    <Stack>
      <SelectAdapter resource={resource} />
      <Stack direction="row" alignItems="center" spacing={0.5} sx={{mt: 0.5}}>
        <Users size={12} />
        <Typography variant="caption" color="text.secondary">
          {t('flows:core.elements.userSelect.caption')}
        </Typography>
      </Stack>
    </Stack>
  );
}

export default UserSelectAdapter;
