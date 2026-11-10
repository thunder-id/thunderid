// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, FullScreenCreationWizardLayout, QueryErrorNotice} from '@thunderid/components';
import {OrganizationUnitTreePicker, useGetOrganizationUnits} from '@thunderid/configure-organization-units';
import {useProjectScope} from '@thunderid/contexts';
import {useResolveDisplayName} from '@thunderid/hooks';
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
  Typography,
} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useForm} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateUser from '../api/useCreateUser';
import useGetUserType from '../api/useGetUserType';
import useGetUserTypes from '../api/useGetUserTypes';
import useUserRoutes from '../hooks/useUserRoutes';
import type {PropertyDefinition, SchemaInterface} from '../models/users';
import getUserErrorMessage from '../utils/getUserErrorMessage';
import orderSchemaAttributes from '../utils/orderSchemaAttributes';
import renderSchemaField from '../utils/renderSchemaField';

type AttributeFormData = Record<string, unknown>;

/**
 * Creates a user from a single form, without running an onboarding flow.
 *
 * Used when user creation runs natively, for example on a deployment that holds configuration only
 * and so has no /flow/execute for an embedded onboarding flow to call. It asks for what the chosen
 * user type's schema requires and posts it to the users API directly.
 */
export default function UserAddFormPage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useUserRoutes();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});
  const createUser = useCreateUser();
  const {projectId} = useProjectScope();
  // Inside a project the user goes to the system organization unit or one of the project's own,
  // picked on the organization unit step. The step is offered only when the project has
  // organization units; without any, the API places the user in the system organization unit.
  const isInProject = Boolean(projectId);
  const {
    data: projectOus,
    isLoading: projectOusLoading,
    error: projectOusError,
    refetch: refetchProjectOus,
  } = useGetOrganizationUnits({limit: 1, offset: 0, projectId}, isInProject);
  const hasProjectOus = (projectOus?.totalResults ?? 0) > 0;

  const [typeId, setTypeId] = useState<string>('');
  const [ouId, setOuId] = useState<string>('');
  const {data: typeList, isLoading: typesLoading} = useGetUserTypes({limit: 100, offset: 0});
  const {data: userType} = useGetUserType(typeId || undefined);

  // A previously chosen type's fields are cleared when the type changes, so they are not posted.
  const {
    control,
    handleSubmit,
    formState: {errors},
    reset,
  } = useForm<AttributeFormData>({mode: 'onBlur', shouldUnregister: true});

  const types: SchemaInterface[] = typeList?.types ?? [];
  const selectedType: SchemaInterface | undefined = types.find((type) => type.id === typeId);
  // A user of a type belongs to the type's organization unit or one beneath it, so inside a project only
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
  // Until both reads succeed it is not known where the user can go, so the user is not created.
  const projectOusReadError = projectOus ? null : projectOusError;
  const typePlacementsReadError = typePlacements ? null : typePlacementsError;
  const placementsReadError = projectOusReadError ?? typePlacementsReadError;
  // The picker selects the first placement once its own listing loads, so the user waits for it rather than being
  // sent without an organization unit, which would place it in the system organization unit.
  const isAwaitingPick = isInProject && hasProjectOus && !ouId;
  const schema: Record<string, PropertyDefinition> = userType?.schema ?? {};

  const handleClose = (): void => {
    void navigate(routes.list());
  };

  const onSubmit = handleSubmit((values: AttributeFormData) => {
    // An optional field left empty is omitted rather than posted as an empty string.
    const attributes: AttributeFormData = Object.fromEntries(
      Object.entries(values).filter(([, value]) => value !== '' && value !== undefined && value !== null),
    );

    createUser.mutate(
      {...(isInProject ? {projectId} : {}), ...(ouId ? {ouId} : {}), type: selectedType?.handle ?? '', attributes},
      {
        onSuccess: () => {
          void navigate(routes.list());
        },
      },
    );
  });

  return (
    <FullScreenCreationWizardLayout
      onClose={handleClose}
      progress={0}
      breadcrumbItems={[{key: 'add-user', label: t('users:addUser', 'Add user')}]}
      footer={null}
    >
      <DocumentTitle title={t('users:addUser', 'Add user')} />
      <Stack spacing={3}>
        <Box>
          <Typography variant="h1" gutterBottom>
            {t('users:addUser', 'Add user')}
          </Typography>
          <Typography variant="body1" color="text.secondary">
            {t('users:addForm.subtitle')}
          </Typography>
        </Box>

        {typesLoading && (
          <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
            <CircularProgress />
          </Box>
        )}

        {!typesLoading && types.length === 0 && <Alert severity="info">{t('users:addForm.noUserTypes')}</Alert>}

        {createUser.isError && (
          <Alert severity="error">
            {getUserErrorMessage(
              createUser.error,
              (key, options) => t(key.includes(':') ? key : `users:${key}`, options),
              'create.error',
              'Failed to create user. Please try again.',
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
            <FormControl fullWidth required>
              <FormLabel htmlFor="user-type">{t('users:addForm.userType')}</FormLabel>
              <Select
                id="user-type"
                value={typeId}
                displayEmpty
                onChange={(event) => {
                  const nextTypeId = String(event.target.value);
                  setTypeId(nextTypeId);
                  // The type's own organization unit is the picker's root, so it starts selected. Inside a
                  // project the picker selects the first placement the type allows instead.
                  setOuId(isInProject ? '' : (types.find((type) => type.id === nextTypeId)?.ouId ?? ''));
                  // Another type's schema can share a field name, so its values are not carried over.
                  reset({});
                }}
              >
                <MenuItem value="">
                  <em>{t('users:addForm.selectUserType')}</em>
                </MenuItem>
                {types.map((type: SchemaInterface) => (
                  <MenuItem key={type.id} value={type.id}>
                    {resolveDisplayName(type.displayName) || type.handle}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            {selectedType && !isInProject && (
              <FormControl fullWidth required>
                <FormLabel htmlFor="user-ou">{t('users:addForm.organizationUnit')}</FormLabel>
                {/* A user of a type belongs to the type's organization unit or one beneath it. */}
                <OrganizationUnitTreePicker
                  key={selectedType.id}
                  id="user-ou"
                  value={ouId}
                  onChange={setOuId}
                  rootOuId={selectedType.ouId || undefined}
                />
              </FormControl>
            )}

            {selectedType && placementsReadError && (
              <QueryErrorNotice
                variant="inline"
                error={placementsReadError}
                t={(key, options) => t(key.includes(':') ? key : `users:${key}`, options)}
                fallbackKey="users:addForm.placementsError"
                fallbackDefaultValue="The organization units this user can be placed in could not be loaded."
                onRetry={() => {
                  if (projectOusReadError) void refetchProjectOus();
                  if (typePlacementsReadError) void refetchTypePlacements();
                }}
              />
            )}

            {selectedType && isTypeOutsideProject && (
              <Alert severity="warning">
                {t(
                  'users:addForm.typeOutsideProject',
                  "Users of this type cannot be created in this project. Choose a type whose organization unit is the system organization unit or one of this project's.",
                )}
              </Alert>
            )}

            {selectedType && isInProject && hasProjectOus && !isTypeOutsideProject && !placementsReadError && (
              <FormControl fullWidth>
                <FormLabel htmlFor="user-ou">{t('users:addForm.organizationUnit')}</FormLabel>
                <OrganizationUnitTreePicker
                  key={selectedType.id}
                  id="user-ou"
                  value={ouId}
                  onChange={setOuId}
                  projectId={projectId}
                  withinOuId={selectedType.ouId || undefined}
                />
              </FormControl>
            )}

            {/* The type's own schema decides the rest, so a type with extra attributes needs no
                change here. */}
            {orderSchemaAttributes(schema).map(([name, definition]: [string, PropertyDefinition]) =>
              renderSchemaField(name, definition, control, errors, resolveDisplayName),
            )}

            <Stack direction="row" spacing={1}>
              <Button
                type="submit"
                variant="contained"
                disabled={
                  !selectedType ||
                  (!isInProject && !ouId) ||
                  (isInProject &&
                    (projectOusLoading ||
                      typePlacementsLoading ||
                      isTypeOutsideProject ||
                      Boolean(placementsReadError) ||
                      isAwaitingPick)) ||
                  !userType ||
                  createUser.isPending
                }
              >
                {createUser.isPending ? t('users:addForm.creating') : t('users:addForm.submit')}
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
