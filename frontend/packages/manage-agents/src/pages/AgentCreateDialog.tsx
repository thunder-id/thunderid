// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AgentOnboardFlowPage} from '@thunderid/configure-agents';
import {AdministrationModes, useAdministrationOperation} from '@thunderid/contexts';
import {AgentCreate} from '@thunderid/react';
import {Dialog, DialogContent} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useManageAgentRoutes from '../hooks/useManageAgentRoutes';

/**
 * Adds an agent in a dialog. Closing the dialog returns to the agents list.
 *
 * Where agent creation runs natively the SDK form posts to the agents API, then opens the agent.
 * Otherwise the configured agent onboarding flow runs in the dialog, and its provisioning node
 * creates the agent.
 */
export default function AgentCreateDialog(): JSX.Element {
  const {t} = useTranslation();
  const mode = useAdministrationOperation('agents', 'create');
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
        {mode === AdministrationModes.NATIVE ? (
          <AgentCreate onCancel={cancel} onCreated={created} />
        ) : (
          <AgentOnboardFlowPage embedded onClose={cancel} />
        )}
      </DialogContent>
    </Dialog>
  );
}
