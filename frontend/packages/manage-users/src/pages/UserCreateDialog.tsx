// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {UserCreate} from '@thunderid/react';
import {Button, Dialog, DialogActions, DialogContent} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useManageUserRoutes from '../hooks/useManageUserRoutes';

export interface UserCreateDialogProps {
  /**
   * `api` creates the user directly; `invite` sends them an invitation to finish signing up.
   */
  mode?: 'api' | 'invite';
}

/**
 * Adds a user in a dialog, then opens it. Closing the dialog returns to the users list. The invite
 * flow has no cancel of its own and ends on its own completion step, so the dialog offers Close.
 */
export default function UserCreateDialog({mode = 'invite'}: UserCreateDialogProps): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useManageUserRoutes();
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
      slotProps={{paper: {'aria-label': t('manage:create.users', 'Add')}}}
    >
      <DialogContent>
        <UserCreate mode={mode} onCancel={cancel} onCreated={created} />
      </DialogContent>
      {mode === 'invite' && (
        <DialogActions>
          <Button onClick={cancel}>{t('manage:actions.close', 'Close')}</Button>
        </DialogActions>
      )}
    </Dialog>
  );
}
