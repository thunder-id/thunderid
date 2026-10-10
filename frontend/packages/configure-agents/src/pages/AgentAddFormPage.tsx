// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FullScreenCreationWizardLayout} from '@thunderid/components';
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
  FormLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {useCallback, useState, type JSX} from 'react';
import {useForm} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {useLocation, useNavigate} from 'react-router';
import useCreateAgent from '../api/useCreateAgent';
import useAgentRoutes from '../hooks/useAgentRoutes';
import type {CreateAgentRequest} from '../models/agent';

type AttributeFormData = Record<string, unknown>;

/**
 * Creates an agent from a single form, without running an onboarding flow.
 *
 * Used when agent creation runs natively, for example on a deployment that holds configuration
 * only and so has no /flow/execute for the onboarding flow to call. It asks for a name, an agent
 * type, an organization unit and what the type's schema requires, and posts them to the agents API.
 * The agent is created without inbound auth; that is configured on the agent afterwards.
 */
export default function AgentAddFormPage(): JSX.Element {
  const routes = useAgentRoutes();
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {pathname} = useLocation();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});
  const createAgent = useCreateAgent();

  const isWelcomeFlow = pathname.startsWith('/welcome');

  const [name, setName] = useState<string>('');
  const [description, setDescription] = useState<string>('');
  const [typeId, setTypeId] = useState<string>('');
  const [ouId, setOuId] = useState<string>('');
  const {data: typeList, isLoading: typesLoading} = useGetAgentTypes();
  const {data: agentType} = useGetAgentType(typeId || undefined);

  // Fields of a previously chosen type are unregistered with it, so they are not posted.
  const {
    control,
    handleSubmit,
    formState: {errors},
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
      name: name.trim(),
      ouId,
      type: selectedType?.handle ?? '',
      ...(trimmedDescription ? {description: trimmedDescription} : {}),
      ...(Object.keys(attributes).length > 0 ? {attributes} : {}),
    };

    createAgent.mutate(request, {
      onSuccess: (agent) => {
        void navigate(routes.agents.detail(agent.id));
      },
    });
  });

  return (
    <FullScreenCreationWizardLayout
      onClose={handleClose}
      progress={0}
      breadcrumbItems={[{key: 'add-agent', label: t('agents:addAgent', 'Add Agent')}]}
      footer={null}
    >
      <Stack spacing={3}>
        <Box>
          <Typography variant="h1" gutterBottom>
            {t('agents:addAgent', 'Add Agent')}
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
            {getErrorMessage(createAgent.error, tForErrors, 'create.error', 'Failed to create agent. Please try again.')}
          </Alert>
        )}

        <form
          noValidate
          onSubmit={(event) => {
            void onSubmit(event);
          }}
        >
          <Stack spacing={3}>
            <FormControl fullWidth required>
              <FormLabel htmlFor="agent-name">{t('agents:addForm.name')}</FormLabel>
              <TextField
                id="agent-name"
                value={name}
                required
                inputProps={{maxLength: 100}}
                onChange={(event) => setName(event.target.value)}
              />
            </FormControl>

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

            <Stack direction="row" spacing={1}>
              <Button
                type="submit"
                variant="contained"
                disabled={!name.trim() || !selectedType || !ouId || !agentType || createAgent.isPending}
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
    </FullScreenCreationWizardLayout>
  );
}
