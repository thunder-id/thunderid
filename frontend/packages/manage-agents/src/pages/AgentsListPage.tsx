// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AgentList} from '@thunderid/react';
import {Button} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate} from 'react-router';
import useManageAgentRoutes from '../hooks/useManageAgentRoutes';

export interface AgentsListPageProps {
  /**
   * Whether admins may add agents from this page.
   */
  canAdd?: boolean;
  description?: string;
  title?: string;
}

/**
 * Lists the agents the signed-in admin can manage. Choosing one opens its details.
 */
export default function AgentsListPage({
  canAdd = false,
  description = undefined,
  title = undefined,
}: AgentsListPageProps): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useManageAgentRoutes();

  return (
    <AgentList
      title={title ?? t('manage:nav.agents', 'Agents')}
      description={description}
      actions={
        canAdd ? (
          <Button component={Link} to={routes.create()} variant="contained" startIcon={<Plus size={16} />}>
            {t('manage:actions.add.agents', t('manage:actions.add', 'Add'))}
          </Button>
        ) : undefined
      }
      onSelect={({id}: {id: string}) => {
        void navigate(routes.details(id));
      }}
    />
  );
}
