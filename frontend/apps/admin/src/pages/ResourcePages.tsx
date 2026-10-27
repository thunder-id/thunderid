// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AgentCreateDialog, AgentDetailsPage, AgentsListPage} from '@thunderid/manage-agents';
import {GroupCreateDialog, GroupDetailsPage, GroupsListPage} from '@thunderid/manage-groups';
import {RoleCreateDialog, RoleDetailsPage, RolesListPage} from '@thunderid/manage-roles';
import {UserCreateDialog, UserDetailsPage, UsersListPage} from '@thunderid/manage-users';
import {useUser} from '@thunderid/react';
import {CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Navigate} from 'react-router';
import RouteConfig, {type ManagedResource} from '../configs/RouteConfig';
import useAdminAppConfig from '../hooks/useAdminAppConfig';

interface ResourcePageProps {
  resource: ManagedResource;
}

/**
 * Renders a page only when it is on and the admin may see it. Any other page sends
 * the admin to the first page they can see, or explains that there is none.
 */
function PageGate({children, resource}: ResourcePageProps & {children: JSX.Element}): JSX.Element {
  const {t} = useTranslation();
  const {isLoading, visibleResources} = useAdminAppConfig();

  if (isLoading) {
    return <CircularProgress aria-label={t('manage:loading', 'Loading')} />;
  }

  if (visibleResources.includes(resource)) {
    return children;
  }

  if (visibleResources.length > 0) {
    return <Navigate to={RouteConfig[visibleResources[0]].list()} replace />;
  }

  return (
    <Stack spacing={1}>
      <Typography variant="h5" component="h1">
        {t('manage:empty.title', 'Nothing to manage yet')}
      </Typography>
      <Typography color="text.secondary">
        {t('manage:empty.description', 'Your administrator has not turned on any pages for this app.')}
      </Typography>
    </Stack>
  );
}

/**
 * Whether a page lets admins add a new resource: users by creating or inviting them, everything
 * else by creating it.
 */
const canAdd = (actions: Record<string, boolean> | undefined): boolean =>
  actions?.create === true || actions?.invite === true;

/**
 * How admins add people: directly when the app configuration lets them create users, otherwise by
 * invitation.
 */
const userAddAction = (actions: Record<string, boolean> | undefined): 'create' | 'invite' | undefined => {
  if (actions?.create === true) {
    return 'create';
  }

  return actions?.invite === true ? 'invite' : undefined;
};

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
 * The organization unit the admin manages, by its friendliest name, for page titles.
 */
function useOrganizationName(): string | undefined {
  const {flattenedProfile} = useUser();
  const profile = (flattenedProfile ?? {}) as Record<string, unknown>;

  return [profile.ouName, profile.ouHandle].find(
    (value: unknown): value is string => typeof value === 'string' && value.length > 0,
  );
}

function ResourceList({resource}: ResourcePageProps): JSX.Element {
  const {t} = useTranslation();
  const {page} = useAdminAppConfig();
  const organization: string | undefined = useOrganizationName();
  const actions: Record<string, boolean> | undefined = page(resource)?.actions;
  const description: string = t(`manage:${resource}.description`, '');
  const title: string = organization
    ? t(`manage:${resource}.titleIn`, {defaultValue: t(`manage:nav.${resource}`, resource), organization})
    : t(`manage:nav.${resource}`, resource);
  const listProps = {canAdd: canAdd(actions), description, title};

  switch (resource) {
    case 'groups':
      return <GroupsListPage {...listProps} />;
    case 'roles':
      return <RolesListPage {...listProps} />;
    case 'agents':
      return <AgentsListPage {...listProps} />;
    default:
      return <UsersListPage addAction={userAddAction(actions)} description={description} title={title} />;
  }
}

/**
 * Creates one kind of managed resource in a dialog over its list, then opens it. The dialog has its
 * own route, so it can be linked to and the browser's back button closes it.
 */
export function ResourceCreatePage({resource}: ResourcePageProps): JSX.Element {
  return (
    <PageGate resource={resource}>
      <>
        <ResourceList resource={resource} />
        <ResourceCreate resource={resource} />
      </>
    </PageGate>
  );
}

function ResourceCreate({resource}: ResourcePageProps): JSX.Element {
  const {page} = useAdminAppConfig();
  const actions: Record<string, boolean> | undefined = page(resource)?.actions;

  if (!canAdd(actions)) {
    return <Navigate to={RouteConfig[resource].list()} replace />;
  }

  switch (resource) {
    case 'groups':
      return <GroupCreateDialog />;
    case 'roles':
      return <RoleCreateDialog />;
    case 'agents':
      return <AgentCreateDialog />;
    default:
      return <UserCreateDialog mode={actions?.create === true ? 'api' : 'invite'} />;
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

function ResourceDetails({resource}: ResourcePageProps): JSX.Element {
  const {page} = useAdminAppConfig();
  const actions: Record<string, boolean> | undefined = page(resource)?.actions;

  switch (resource) {
    case 'groups':
      return <GroupDetailsPage editable={actions?.members === true} />;
    case 'roles':
      return <RoleDetailsPage editable={actions?.assign === true} />;
    case 'agents':
      return <AgentDetailsPage deletable={actions?.delete === true} />;
    default:
      return <UserDetailsPage deletable={actions?.delete === true} editable={actions?.edit === true} />;
  }
}
