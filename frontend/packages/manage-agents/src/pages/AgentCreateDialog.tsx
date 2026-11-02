// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AgentCreate} from '@thunderid/react';
import {Dialog, DialogContent} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useManageAgentRoutes from '../hooks/useManageAgentRoutes';

/**
 * Adds a agent in a dialog, then opens it. Closing the dialog returns to the agents list.
 */
export default function AgentCreateDialog(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useManageAgentRoutes();
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
      slotProps={{paper: {'aria-label': t('manage:create.agents', 'Add')}}}
    >
      <DialogContent>
        <AgentCreate onCancel={cancel} onCreated={created} />
      </DialogContent>
    </Dialog>
  );
}
