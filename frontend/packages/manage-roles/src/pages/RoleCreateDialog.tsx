// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {RoleCreate} from '@thunderid/react';
import {Dialog, DialogContent} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useManageRoleRoutes from '../hooks/useManageRoleRoutes';

/**
 * Adds a role in a dialog, then opens it. Closing the dialog returns to the roles list.
 */
export default function RoleCreateDialog(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useManageRoleRoutes();
  const cancel = (): void => {
    void navigate(routes.list());
  };
  const created = ({id}: {id: string}): void => {
    void navigate(routes.details(id), {replace: true});
  };

  return (
    <Dialog
      open
      fullWidth
      maxWidth="sm"
      onClose={cancel}
      slotProps={{paper: {'aria-label': t('manage:create.roles', 'Add')}}}
    >
      <DialogContent>
        <RoleCreate onCancel={cancel} onCreated={created} />
      </DialogContent>
    </Dialog>
  );
}
