// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AgentDetails} from '@thunderid/react';
import {Button, Stack} from '@wso2/oxygen-ui';
import {ChevronLeft} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useManageAgentRoutes from '../hooks/useManageAgentRoutes';

export interface AgentDetailsPageProps {
  deletable?: boolean;
  /**
   * Whether admins may suspend, unsuspend and unlock the agent. Each action is offered only when the
   * server configures its administration flow, and reading that needs permission to read the server
   * configuration.
   */
  lifecycleActions?: boolean;
}

/**
 * Shows the agent named by the `id` route parameter. Deleting the agent returns to the agents list.
 */
export default function AgentDetailsPage({
  deletable = false,
  lifecycleActions = false,
}: AgentDetailsPageProps): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useManageAgentRoutes();
  const {id = ''} = useParams();

  return (
    <Stack spacing={2}>
      <Button
        component={Link}
        to={routes.list()}
        variant="text"
        startIcon={<ChevronLeft size={16} />}
        sx={{alignSelf: 'flex-start'}}
      >
        {t('manage:actions.back', 'Back')}
      </Button>
      <AgentDetails
        agentId={id}
        deletable={deletable}
        lifecycleActions={lifecycleActions}
        onDeleted={() => {
          void navigate(routes.list(), {replace: true});
        }}
      />
    </Stack>
  );
}
