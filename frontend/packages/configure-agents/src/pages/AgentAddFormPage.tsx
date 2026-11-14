// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, FullScreenCreationWizardLayout, QueryErrorNotice} from '@thunderid/components';
import {useGetAgentType, useGetAllAgentTypes, type AgentTypeListItem} from '@thunderid/configure-agent-types';
import {OrganizationUnitTreePicker, useGetOrganizationUnits} from '@thunderid/configure-organization-units';
import {renderSchemaField, useGetUser, useGetUsers} from '@thunderid/configure-users';
import {useProjectScope} from '@thunderid/contexts';
import {useResolveDisplayName} from '@thunderid/hooks';
import {useThunderID} from '@thunderid/react';
import {getErrorMessage, orderAttributes} from '@thunderid/utils';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  FormControl,
  FormControlLabel,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {useCallback, useState, type JSX} from 'react';
import {useForm} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateAgent from '../api/useCreateAgent';
import AgentOwnerField from '../components/AgentOwnerField';
import ClientSecretSuccessDialog from '../components/ClientSecretSuccessDialog';
import useAgentRoutes from '../hooks/useAgentRoutes';
import type {Agent, AgentInboundAuthConfig, AgentOwner, CreateAgentRequest} from '../models/agent';
import {isInvalidExternalOwner} from '../utils/externalOwner';

type AttributeFormData = Record<string, unknown>;

/** The wizard's two screens: the agent's type and placement, then the attributes the type's schema holds. */
type AgentAddStep = 'setup' | 'attributes';

/** The OAuth client an agent is issued: the client credentials grant, authenticated with a secret. */
const AGENT_CREDENTIALS: AgentInboundAuthConfig = {
  type: 'oauth2',
  config: {grantTypes: ['client_credentials'], responseTypes: [], tokenEndpointAuthMethod: 'client_secret_basic'},
};

/**
 * The signed-in user's ID: the `id` of the profile the console reads from `/users/me`, which a caller
 * who is not a user here does not have.
 */
function signedInUserId(user: Record<string, unknown> | null | undefined): string | undefined {
  const id: unknown = user?.['id'];

  return typeof id === 'string' && id !== '' ? id : undefined;
}

/** Whether a read failed because what it asked for does not exist. */
function isNotFound(error: unknown): boolean {
  return (error as {response?: {status?: number}} | null)?.response?.status === 404;
}

/**
 * Creates an agent in two steps, without running an onboarding flow.
 *
 * Used when agent creation runs natively, for example on a deployment that holds configuration
 * only and so has no /flow/execute for the onboarding flow to call. The first step asks for an agent
 * type, an organization unit and an owner; the second asks for what the type's schema holds, the
 * agent's name among it. Both are posted to the agents API. An owner can be chosen from the users that exist here; without one the API makes the
 * caller the owner, so that is offered only when the caller is a user here too. OAuth client
 * credentials are issued with the agent unless switched off: the API generates the client ID and
 * secret, and the secret is shown once here, since the agent's page can only regenerate it afterwards.
 */
export default function AgentAddFormPage(): JSX.Element {
  const routes = useAgentRoutes();
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});
  const createAgent = useCreateAgent();
  const {projectId} = useProjectScope();
  // Inside a project the agent goes to the system organization unit or one of the project's own. The
  // organization unit is offered only when the project has organization units; without any, the API places
  // the agent in the system organization unit.
  const isInProject = Boolean(projectId);
  const {
    data: projectOus,
    isLoading: projectOusLoading,
    error: projectOusError,
    refetch: refetchProjectOus,
  } = useGetOrganizationUnits({limit: 1, offset: 0, projectId}, isInProject);
  const hasProjectOus = (projectOus?.totalResults ?? 0) > 0;

  const [step, setStep] = useState<AgentAddStep>('setup');
  const [description, setDescription] = useState<string>('');
  const [typeId, setTypeId] = useState<string>('');
  const [ouId, setOuId] = useState<string>('');
  const [ownerId, setOwnerId] = useState<string>('');
  // Set while External is chosen; the user picker's choice applies otherwise.
  const [externalOwner, setExternalOwner] = useState<AgentOwner | undefined>(undefined);
  const [issueCredentials, setIssueCredentials] = useState<boolean>(true);
  // The created agent whose secret is being shown; the agent's page opens once the secret is closed.
  const [created, setCreated] = useState<{id: string; clientSecret: string} | null>(null);
  const {data: typeList, isLoading: typesLoading, error: typesError, refetch: refetchTypes} = useGetAllAgentTypes();
  // A failed refetch of agent types already read leaves them in place.
  const typesReadError = typesError && !typeList ? typesError : null;
  const {data: agentType, error: agentTypeError, refetch: refetchAgentType} = useGetAgentType(typeId || undefined);
  const {
    data: userList,
    isLoading: usersLoading,
    error: usersError,
    refetch: refetchUsers,
  } = useGetUsers({limit: 100, offset: 0});
  // Without an owner the API makes the caller the owner, and a gateway refuses an agent whose owner it
  // cannot find. A caller who signed in through another deployment has no profile here, so they are
  // not offered as the owner, and neither is one this deployment does not find. A lookup refused for
  // another reason, such as no permission to read users, says nothing about the caller being missing.
  const {user: signedInUser} = useThunderID() as {user?: Record<string, unknown> | null};
  const callerId = signedInUserId(signedInUser);
  const {isPending: callerPending, error: callerError} = useGetUser(callerId);
  const callerElsewhere = !callerId || isNotFound(callerError);
  const callerCanOwn = !callerElsewhere && !callerPending;

  // A previously chosen type's fields are cleared when the type changes, so they are not posted.
  const {
    control,
    handleSubmit,
    formState: {errors},
    reset,
  } = useForm<AttributeFormData>({mode: 'onBlur', shouldUnregister: true});

  const types: AgentTypeListItem[] = typeList ?? [];
  const selectedType: AgentTypeListItem | undefined = types.find((type) => type.id === typeId);
  // An agent of a type belongs to the type's organization unit or one beneath it, so inside a project only
  // the placements in that subtree are offered. None means the type cannot be used in this project.
  const {
    data: typePlacements,
    isLoading: typePlacementsLoading,
    error: typePlacementsError,
    refetch: refetchTypePlacements,
  } = useGetOrganizationUnits(
    {limit: 1, offset: 0, projectId, placement: true, within: selectedType?.ouId},
    isInProject && Boolean(selectedType),
  );
  const isTypeOutsideProject = isInProject && typePlacements?.totalResults === 0;
  // Until both reads succeed it is not known where the agent can go, so the setup step stays incomplete.
  const projectOusReadError = projectOus ? null : projectOusError;
  const typePlacementsReadError = typePlacements ? null : typePlacementsError;
  const placementsReadError = projectOusReadError ?? typePlacementsReadError;
  // The picker selects the first placement once its own listing loads, so the agent waits for it rather than being
  // sent without an organization unit, which would place it in the system organization unit.
  const isPlacementKnown =
    !isInProject ||
    (!projectOusLoading &&
      !typePlacementsLoading &&
      !placementsReadError &&
      !isTypeOutsideProject &&
      (!hasProjectOus || Boolean(ouId)));
  const users = userList?.users ?? [];
  // Two users can share a display name, so the option shows the user's ID as well when they do.
  const duplicateDisplays = new Set(
    users.map((user) => user.display).filter((display, index, all) => display && all.indexOf(display) !== index),
  );
  const ownerLabel = (user: (typeof users)[number]): string =>
    user.display && duplicateDisplays.has(user.display) ? `${user.display} (${user.id})` : (user.display ?? user.id);
  // Only the first page of users is offered, which is said when there are more.
  const usersTruncated = (userList?.totalResults ?? 0) > users.length;
  let ownerHint: string = t(
    'agents:addForm.ownerHint',
    'Defaults to you. Choose a user created here if the agent is applied to a gateway, which needs an owner it can find.',
  );
  if (callerElsewhere) {
    ownerHint =
      !usersLoading && !usersError && users.length === 0
        ? t('agents:addForm.ownerNoUsers', 'There are no users here to own the agent. Add a user, then add the agent.')
        : t(
            'agents:addForm.ownerRequiredHint',
            'Choose a user created here. A gateway the agent is applied to needs an owner it can find, and the account you signed in with is not a user here.',
          );
  }

  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `agents:${key}`, options),
    [t],
  );

  const handleClose = (): void => {
    void navigate(routes.agents.list());
  };

  // The attributes come from the chosen type's schema, so the second step opens only once it has loaded.
  const isSetupComplete =
    Boolean(selectedType) &&
    (isInProject || Boolean(ouId)) &&
    isPlacementKnown &&
    Boolean(agentType) &&
    (Boolean(externalOwner) || callerCanOwn || Boolean(ownerId)) &&
    !isInvalidExternalOwner(externalOwner);
  const isAttributesStep = step === 'attributes';
  const selectedTypeName = selectedType ? resolveDisplayName(selectedType.displayName) || selectedType.handle : '';

  const onSubmit = handleSubmit((values: AttributeFormData) => {
    // An optional field left empty is omitted rather than posted as an empty string.
    const attributes: AttributeFormData = Object.fromEntries(
      Object.entries(values).filter(([, value]) => value !== '' && value !== undefined && value !== null),
    );
    const trimmedDescription = description.trim();
    const request: CreateAgentRequest = {
      ...(isInProject ? {projectId} : {}),
      ...(ouId ? {ouId} : {}),
      type: selectedType?.handle ?? '',
      ...(externalOwner ? {owner: externalOwner} : {}),
      ...(!externalOwner && ownerId ? {owner: {id: ownerId, idType: 'uuid', isLocal: true}} : {}),
      ...(trimmedDescription ? {description: trimmedDescription} : {}),
      ...(Object.keys(attributes).length > 0 ? {attributes} : {}),
      // The API generates the client ID and secret for an OAuth config that names none, so only how
      // the agent authenticates is given: the client credentials grant, with the secret.
      ...(issueCredentials ? {inboundAuthConfig: [AGENT_CREDENTIALS]} : {}),
    };

    createAgent.mutate(request, {
      onSuccess: (agent: Agent) => {
        const clientSecret: string =
          agent.inboundAuthConfig?.find((config) => config.type === 'oauth2')?.config?.clientSecret ?? '';
        if (clientSecret) {
          setCreated({id: agent.id, clientSecret});
          return;
        }
        void navigate(routes.agents.detail(agent.id));
      },
    });
  });

  return (
    <FullScreenCreationWizardLayout
      onClose={handleClose}
      progress={isAttributesStep ? 100 : 50}
      breadcrumbItems={[
        {
          key: 'add-agent',
          label: t('agents:addAgent', 'Add agent'),
          // Like Back, it waits for a create in flight, so a failed create keeps what it was given.
          onClick: isAttributesStep && !createAgent.isPending ? () => setStep('setup') : undefined,
        },
        ...(isAttributesStep
          ? [{key: 'attributes', label: t('agents:addForm.attributesStep.breadcrumb', 'Attributes')}]
          : []),
      ]}
      footer={
        <Stack direction="row" justifyContent={isAttributesStep ? 'space-between' : 'flex-end'} spacing={2}>
          {isAttributesStep && (
            <Button
              variant="outlined"
              sx={{minWidth: 100}}
              disabled={createAgent.isPending}
              onClick={() => setStep('setup')}
            >
              {t('common:actions.back', 'Back')}
            </Button>
          )}
          {/* Keyed apart so Continue is never reused as Create: the click that opens the attributes
              would otherwise land on a submit button once React re-renders, and create the agent. */}
          {isAttributesStep ? (
            <Button
              key="create"
              type="submit"
              form="agent-attributes-form"
              variant="contained"
              sx={{minWidth: 140}}
              disabled={!isSetupComplete || createAgent.isPending}
            >
              {createAgent.isPending ? t('agents:addForm.creating') : t('agents:addForm.submit')}
            </Button>
          ) : (
            <Button
              key="continue"
              variant="contained"
              sx={{minWidth: 140}}
              disabled={!isSetupComplete}
              onClick={() => setStep('attributes')}
            >
              {t('common:actions.continue', 'Continue')}
            </Button>
          )}
        </Stack>
      }
    >
      <DocumentTitle title={t('agents:addAgent', 'Add agent')} />
      <Stack spacing={3}>
        <Box>
          <Typography variant="h1" gutterBottom>
            {isAttributesStep
              ? t('agents:addForm.attributesStep.title', 'Agent attributes')
              : t('agents:addAgent', 'Add agent')}
          </Typography>
          <Typography variant="body1" color="text.secondary">
            {isAttributesStep
              ? t('agents:addForm.attributesStep.subtitle', 'Fill in the attributes the {{type}} agent type defines.', {
                  type: selectedTypeName,
                })
              : t('agents:addForm.subtitle')}
          </Typography>
        </Box>

        {!isAttributesStep && typesLoading && (
          <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
            <CircularProgress />
          </Box>
        )}

        {!isAttributesStep && !typesLoading && !typesReadError && types.length === 0 && (
          <Alert severity="info">{t('agents:addForm.noAgentTypes')}</Alert>
        )}

        {createAgent.isError && (
          <Alert severity="error">
            {getErrorMessage(
              createAgent.error,
              tForErrors,
              'create.error',
              'Failed to create agent. Please try again.',
            )}
          </Alert>
        )}

        {/* Both steps stay mounted and only the current one is shown, so going back keeps what was
            entered, the owner field's own choices among it. */}
        <Stack spacing={3} sx={isAttributesStep ? {display: 'none'} : undefined}>
          <TextField
            labelPlacement="top"
            label={t('agents:addForm.description')}
            fullWidth
            id="agent-description"
            value={description}
            multiline
            minRows={2}
            placeholder={t('agents:addForm.descriptionPlaceholder')}
            onChange={(event) => setDescription(event.target.value)}
          />

          <FormControl fullWidth required>
            <FormLabel htmlFor="agent-type">{t('agents:addForm.agentType')}</FormLabel>
            {typesReadError ? (
              // A failed read is shown in place of the picker, not as a list with no agent types.
              <QueryErrorNotice
                variant="inline"
                error={typesReadError}
                t={tForErrors}
                fallbackKey="agents:addForm.agentTypesError"
                fallbackDefaultValue="The agent types could not be loaded."
                onRetry={() => {
                  void refetchTypes();
                }}
              />
            ) : (
              <Select
                id="agent-type"
                value={typeId}
                displayEmpty
                onChange={(event) => {
                  setTypeId(String(event.target.value));
                  setOuId('');
                  // Another type's schema can share a field name, so its values are not carried over.
                  reset({});
                }}
              >
                <MenuItem value="">
                  <em>{t('agents:addForm.selectAgentType')}</em>
                </MenuItem>
                {types.map((type: AgentTypeListItem) => (
                  <MenuItem key={type.id} value={type.id}>
                    {resolveDisplayName(type.displayName) || type.handle}
                  </MenuItem>
                ))}
              </Select>
            )}
          </FormControl>

          {selectedType && !isInProject && (
            <FormControl fullWidth required>
              <FormLabel htmlFor="agent-ou">{t('agents:addForm.organizationUnit')}</FormLabel>
              {/* An agent of a type belongs to the type's organization unit or one beneath it. */}
              <OrganizationUnitTreePicker
                key={selectedType.id}
                id="agent-ou"
                value={ouId}
                onChange={setOuId}
                rootOuId={selectedType.ouId || undefined}
                helperText={t(
                  'agents:addForm.organizationUnitHint',
                  'Agents of this agent type can be created in its organization unit, shown at the top, or in any unit below it.',
                )}
              />
            </FormControl>
          )}

          {selectedType && placementsReadError && (
            <QueryErrorNotice
              variant="inline"
              error={placementsReadError}
              t={tForErrors}
              fallbackKey="agents:addForm.placementsError"
              fallbackDefaultValue="The organization units this agent can be placed in could not be loaded."
              onRetry={() => {
                if (projectOusReadError) void refetchProjectOus();
                if (typePlacementsReadError) void refetchTypePlacements();
              }}
            />
          )}

          {selectedType && isTypeOutsideProject && (
            <Alert severity="warning">
              {t(
                'agents:addForm.typeOutsideProject',
                "Agents of this type cannot be created in this project. Choose a type whose organization unit is the system organization unit or one of this project's.",
              )}
            </Alert>
          )}

          {selectedType && isInProject && hasProjectOus && !isTypeOutsideProject && !placementsReadError && (
            <FormControl fullWidth>
              <FormLabel htmlFor="agent-ou">{t('agents:addForm.organizationUnit')}</FormLabel>
              <OrganizationUnitTreePicker
                key={selectedType.id}
                id="agent-ou"
                value={ouId}
                onChange={setOuId}
                projectId={projectId}
                withinOuId={selectedType.ouId || undefined}
              />
            </FormControl>
          )}

          {agentTypeError && !agentType && (
            // Without the type's schema there are no attribute fields to fill in, so Continue stays off.
            <QueryErrorNotice
              variant="inline"
              error={agentTypeError}
              t={tForErrors}
              fallbackKey="agents:addForm.agentTypeSchemaError"
              fallbackDefaultValue="The attributes of the selected agent type could not be loaded."
              onRetry={() => {
                void refetchAgentType();
              }}
            />
          )}

          <AgentOwnerField
            id="agent-owner"
            required={callerElsewhere}
            onExternalChange={setExternalOwner}
            internalPicker={
              <>
                {usersError ? (
                  // A failed read is shown in place of the picker, not as an empty one: the agent would
                  // otherwise be owned by the caller with nothing said about the owners that did not load.
                  <QueryErrorNotice
                    variant="inline"
                    error={usersError}
                    t={tForErrors}
                    fallbackKey="agents:addForm.ownersError"
                    fallbackDefaultValue="The users to choose an owner from could not be loaded."
                    onRetry={() => {
                      void refetchUsers();
                    }}
                  />
                ) : (
                  <Select
                    id="agent-owner-select"
                    labelId="agent-owner-label"
                    value={ownerId}
                    displayEmpty
                    disabled={usersLoading}
                    onChange={(event) => setOwnerId(String(event.target.value))}
                  >
                    <MenuItem value="">
                      <em>
                        {callerElsewhere
                          ? t('agents:addForm.selectOwner', 'Select an owner')
                          : t('agents:addForm.ownerDefault', 'Me (default)')}
                      </em>
                    </MenuItem>
                    {users.map((user) => (
                      <MenuItem key={user.id} value={user.id}>
                        {ownerLabel(user)}
                      </MenuItem>
                    ))}
                  </Select>
                )}
                {usersTruncated && (
                  <Typography variant="caption" color="text.secondary">
                    {t('agents:addForm.ownerTruncated', 'Showing the first {{shown}} of {{total}} users.', {
                      shown: users.length,
                      total: userList?.totalResults,
                    })}
                  </Typography>
                )}
                <Typography variant="caption" color="text.secondary">
                  {ownerHint}
                </Typography>
              </>
            }
          />

          <FormControl fullWidth>
            <FormControlLabel
              control={
                <Switch
                  checked={issueCredentials}
                  onChange={(event) => setIssueCredentials(event.target.checked)}
                  inputProps={{'aria-label': t('agents:addForm.issueCredentials')}}
                />
              }
              label={t('agents:addForm.issueCredentials')}
            />
            <Typography variant="caption" color="text.secondary">
              {t('agents:addForm.issueCredentialsHint')}
            </Typography>
          </FormControl>
        </Stack>

        <form
          id="agent-attributes-form"
          noValidate
          onSubmit={(event) => {
            void onSubmit(event);
          }}
        >
          <Stack spacing={3} sx={isAttributesStep ? undefined : {display: 'none'}}>
            {orderAttributes(agentType?.schema ?? {}, {
              onCreate: true,
              leading: agentType?.systemAttributes?.display,
            }).map(([fieldName, fieldDef]) =>
              renderSchemaField(fieldName, fieldDef, control, errors, resolveDisplayName),
            )}
          </Stack>
        </form>
      </Stack>

      <ClientSecretSuccessDialog
        open={created !== null}
        clientSecret={created?.clientSecret ?? ''}
        title={t('agents:addForm.secretIssued.title')}
        subtitle={t('agents:addForm.secretIssued.subtitle')}
        onClose={() => {
          const agentId: string = created?.id ?? '';
          setCreated(null);
          void navigate(routes.agents.detail(agentId));
        }}
      />
    </FullScreenCreationWizardLayout>
  );
}
