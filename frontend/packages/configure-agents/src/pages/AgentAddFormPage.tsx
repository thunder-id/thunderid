// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, FullScreenCreationWizardLayout} from '@thunderid/components';
import {useGetAgentType, useGetAgentTypes, type AgentTypeListItem} from '@thunderid/configure-agent-types';
import {OrganizationUnitTreePicker} from '@thunderid/configure-organization-units';
import {renderSchemaField} from '@thunderid/configure-users';
import {useResolveDisplayName} from '@thunderid/hooks';
import {getErrorMessage} from '@thunderid/utils';
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
import {useLocation, useNavigate} from 'react-router';
import useCreateAgent from '../api/useCreateAgent';
import ClientSecretSuccessDialog from '../components/ClientSecretSuccessDialog';
import useAgentRoutes from '../hooks/useAgentRoutes';
import type {Agent, AgentInboundAuthConfig, CreateAgentRequest} from '../models/agent';

type AttributeFormData = Record<string, unknown>;

/** The OAuth client an agent is issued: the client credentials grant, authenticated with a secret. */
const AGENT_CREDENTIALS: AgentInboundAuthConfig = {
  type: 'oauth2',
  config: {grantTypes: ['client_credentials'], responseTypes: [], tokenEndpointAuthMethod: 'client_secret_basic'},
};

/**
 * Creates an agent from a single form, without running an onboarding flow.
 *
 * Used when agent creation runs natively, for example on a deployment that holds configuration
 * only and so has no /flow/execute for the onboarding flow to call. It asks for an agent type, an
 * organization unit and what the type's schema holds, the agent's name among it, and posts them to
 * the agents API. OAuth client credentials are issued with the agent unless switched off: the API
 * generates the client ID and secret, and the secret is shown once here, since the agent's page
 * can only regenerate it afterwards.
 */
export default function AgentAddFormPage(): JSX.Element {
  const routes = useAgentRoutes();
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {pathname} = useLocation();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});
  const createAgent = useCreateAgent();

  const isWelcomeFlow = pathname.startsWith(routes.welcome.getStarted());

  const [description, setDescription] = useState<string>('');
  const [typeId, setTypeId] = useState<string>('');
  const [ouId, setOuId] = useState<string>('');
  const [issueCredentials, setIssueCredentials] = useState<boolean>(true);
  // The created agent whose secret is being shown; the agent's page opens once the secret is closed.
  const [created, setCreated] = useState<{id: string; clientSecret: string} | null>(null);
  const {data: typeList, isLoading: typesLoading} = useGetAgentTypes({limit: 100, offset: 0});
  const {data: agentType} = useGetAgentType(typeId || undefined);

  // A previously chosen type's fields are cleared when the type changes, so they are not posted.
  const {
    control,
    handleSubmit,
    formState: {errors},
    reset,
  } = useForm<AttributeFormData>({mode: 'onBlur', shouldUnregister: true});

  const types: AgentTypeListItem[] = typeList?.types ?? [];
  const selectedType: AgentTypeListItem | undefined = types.find((type) => type.id === typeId);

  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `agents:${key}`, options),
    [t],
  );

  const handleClose = (): void => {
    void navigate(isWelcomeFlow ? routes.welcome.getStarted() : routes.agents.list());
  };

  const onSubmit = handleSubmit((values: AttributeFormData) => {
    // An optional field left empty is omitted rather than posted as an empty string.
    const attributes: AttributeFormData = Object.fromEntries(
      Object.entries(values).filter(([, value]) => value !== '' && value !== undefined && value !== null),
    );
    const trimmedDescription = description.trim();
    const request: CreateAgentRequest = {
      ouId,
      type: selectedType?.handle ?? '',
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
      progress={0}
      breadcrumbItems={[{key: 'add-agent', label: t('agents:addAgent', 'Add agent')}]}
      footer={null}
    >
      <DocumentTitle title={t('agents:addAgent', 'Add agent')} />
      <Stack spacing={3}>
        <Box>
          <Typography variant="h1" gutterBottom>
            {t('agents:addAgent', 'Add agent')}
          </Typography>
          <Typography variant="body1" color="text.secondary">
            {t('agents:addForm.subtitle')}
          </Typography>
        </Box>

        {typesLoading && (
          <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
            <CircularProgress />
          </Box>
        )}

        {!typesLoading && types.length === 0 && <Alert severity="info">{t('agents:addForm.noAgentTypes')}</Alert>}

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

        <form
          noValidate
          onSubmit={(event) => {
            void onSubmit(event);
          }}
        >
          <Stack spacing={3}>
            <FormControl fullWidth>
              <FormLabel htmlFor="agent-description">{t('agents:addForm.description')}</FormLabel>
              <TextField
                id="agent-description"
                value={description}
                multiline
                minRows={2}
                placeholder={t('agents:addForm.descriptionPlaceholder')}
                onChange={(event) => setDescription(event.target.value)}
              />
            </FormControl>

            <FormControl fullWidth required>
              <FormLabel htmlFor="agent-type">{t('agents:addForm.agentType')}</FormLabel>
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
            </FormControl>

            {selectedType && (
              <FormControl fullWidth required>
                <FormLabel htmlFor="agent-ou">{t('agents:addForm.organizationUnit')}</FormLabel>
                {/* An agent of a type belongs to the type's organization unit or one beneath it. */}
                <OrganizationUnitTreePicker
                  key={selectedType.id}
                  id="agent-ou"
                  value={ouId}
                  onChange={setOuId}
                  rootOuId={selectedType.ouId || undefined}
                />
              </FormControl>
            )}

            {Object.entries(agentType?.schema ?? {}).map(([fieldName, fieldDef]) =>
              renderSchemaField(fieldName, fieldDef, control, errors, resolveDisplayName),
            )}

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

            <Stack direction="row" spacing={1}>
              <Button
                type="submit"
                variant="contained"
                disabled={!selectedType || !ouId || !agentType || createAgent.isPending}
              >
                {createAgent.isPending ? t('agents:addForm.creating') : t('agents:addForm.submit')}
              </Button>
              <Button variant="outlined" onClick={handleClose}>
                {t('common:actions.cancel', 'Cancel')}
              </Button>
            </Stack>
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
