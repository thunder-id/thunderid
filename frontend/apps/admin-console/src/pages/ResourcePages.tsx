// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AgentCreateDialog, AgentDetailsPage, AgentsListPage} from '@thunderid/manage-agents';
import {GroupCreateDialog, GroupDetailsPage, GroupsListPage} from '@thunderid/manage-groups';
import {RoleCreateDialog, RoleDetailsPage, RolesListPage} from '@thunderid/manage-roles';
import {UserCreateDialog, UserDetailsPage, UsersListPage} from '@thunderid/manage-users';
import {CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Navigate} from 'react-router';
import RouteConfig, {type ManagedResource} from '../configs/RouteConfig';
import useVisibleResources from '../hooks/useVisibleResources';

interface ResourcePageProps {
  resource: ManagedResource;
}

/**
 * Renders a page only when the admin may see it. Any other page sends the admin to the first page
 * they can see, or explains that there is none.
 */
function PageGate({children, resource}: ResourcePageProps & {children: JSX.Element}): JSX.Element {
  const {t} = useTranslation();
  const {isLoading, visible} = useVisibleResources();

  if (isLoading) {
    return <CircularProgress aria-label={t('manage:loading', 'Loading')} />;
  }

  if (visible.includes(resource)) {
    return children;
  }

  if (visible.length > 0) {
    return <Navigate to={RouteConfig[visible[0]].list()} replace />;
  }

  return (
    <Stack spacing={1}>
      <Typography variant="h5" component="h1">
        {t('manage:empty.title', 'Nothing to manage')}
      </Typography>
      <Typography color="text.secondary">
        {t('manage:empty.description', 'You do not have permission to manage anything here.')}
      </Typography>
    </Stack>
  );
}

function ResourceList({resource}: ResourcePageProps): JSX.Element {
  const {t} = useTranslation();
  const listProps = {
    canAdd: true,
    description: t(`manage:${resource}.description`, ''),
    title: t(`manage:nav.${resource}`, resource),
  };

  switch (resource) {
    case 'groups':
      return <GroupsListPage {...listProps} />;
    case 'roles':
      return <RolesListPage {...listProps} />;
    case 'agents':
      return <AgentsListPage {...listProps} />;
    default:
      return <UsersListPage addAction="create" description={listProps.description} title={listProps.title} />;
  }
}

/**
 * Lists one kind of managed resource. Choosing a row opens its details.
 */
export function ResourceListPage({resource}: ResourcePageProps): JSX.Element {
  return (
    <PageGate resource={resource}>
      <ResourceList resource={resource} />
    </PageGate>
  );
}

/**
 * Creates one kind of managed resource in a dialog over its list, then opens it. The dialog has its
 * own route, so it can be linked to and the browser's back button closes it.
 */
export function ResourceCreatePage({resource}: ResourcePageProps): JSX.Element {
  let dialog: JSX.Element;

  switch (resource) {
    case 'groups':
      dialog = <GroupCreateDialog />;
      break;
    case 'roles':
      dialog = <RoleCreateDialog />;
      break;
    case 'agents':
      dialog = <AgentCreateDialog />;
      break;
    default:
      // The user onboarding flow, which lets the admin create the account or invite the person.
      dialog = <UserCreateDialog mode="invite" />;
  }

  return (
    <PageGate resource={resource}>
      <>
        <ResourceList resource={resource} />
        {dialog}
      </>
    </PageGate>
  );
}

function ResourceDetails({resource}: ResourcePageProps): JSX.Element {
  const {canRunLifecycle} = useVisibleResources();

  // Users and agents can be deleted, each after a confirmation; groups and roles stay until the Admin
  // App configuration can turn deleting on per page.
  switch (resource) {
    case 'groups':
      return <GroupDetailsPage editable />;
    case 'roles':
      return <RoleDetailsPage editable />;
    case 'agents':
      return <AgentDetailsPage deletable lifecycleActions={canRunLifecycle} />;
    default:
      return <UserDetailsPage deletable editable lifecycleActions={canRunLifecycle} />;
  }
}

/**
 * Shows one managed resource, identified by the `id` route parameter.
 */
export function ResourceDetailsPage({resource}: ResourcePageProps): JSX.Element {
  return (
    <PageGate resource={resource}>
      <ResourceDetails resource={resource} />
    </PageGate>
  );
}
