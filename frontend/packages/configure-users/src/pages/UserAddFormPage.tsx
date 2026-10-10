// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FullScreenCreationWizardLayout} from '@thunderid/components';
import {OrganizationUnitTreePicker} from '@thunderid/configure-organization-units';
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

  const [typeId, setTypeId] = useState<string>('');
  const [ouId, setOuId] = useState<string>('');
  const {data: typeList, isLoading: typesLoading} = useGetUserTypes();
  const {data: userType} = useGetUserType(typeId || undefined);

  // Fields of a previously chosen type are unregistered with it, so they are not posted.
  const {
    control,
    handleSubmit,
    formState: {errors},
  } = useForm<AttributeFormData>({mode: 'onBlur', shouldUnregister: true});

  const types: SchemaInterface[] = typeList?.types ?? [];
  const selectedType: SchemaInterface | undefined = types.find((type) => type.id === typeId);
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
      {ouId, type: selectedType?.handle ?? '', attributes},
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
      breadcrumbItems={[{key: 'add-user', label: t('users:addUser', 'Add User')}]}
      footer={null}
    >
      <Stack spacing={3}>
        <Box>
          <Typography variant="h1" gutterBottom>
            {t('users:addUser', 'Add User')}
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
                  setTypeId(String(event.target.value));
                  setOuId('');
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

            {selectedType && (
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

            {/* The type's own schema decides the rest, so a type with extra attributes needs no
                change here. */}
            {Object.entries(schema).map(([name, definition]: [string, PropertyDefinition]) =>
              renderSchemaField(name, definition, control, errors, resolveDisplayName),
            )}

            <Stack direction="row" spacing={1}>
              <Button
                type="submit"
                variant="contained"
                disabled={!selectedType || !ouId || !userType || createUser.isPending}
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
