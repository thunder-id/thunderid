// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OrganizationUnitTreePicker} from '@thunderid/configure-organization-units';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  FormControl,
  FormLabel,
  MenuItem,
  PageContent,
  PageTitle,
  Select,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {Controller, useForm} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateUser from '../api/useCreateUser';
import useGetUserType from '../api/useGetUserType';
import useGetUserTypes from '../api/useGetUserTypes';
import type {PropertyDefinition, SchemaInterface} from '../models/users';
import renderSchemaField from '../utils/renderSchemaField';

/**
 * Creates a user from a single form, without running an onboarding flow.
 *
 * A Control Plane executes no flows: it holds configuration and has no runtime, so there is no
 * /flow/execute for an embedded onboarding flow to call. Authoring a user there is a plain write, so
 * this asks for what the user type's schema requires and posts it to the users API directly.
 *
 * The flow-driven page remains what a Data Plane uses, where onboarding is a runtime journey that an
 * operator can shape.
 */
export default function UserAddFormPage(): JSX.Element {
  const {t} = useTranslation('users');
  const navigate = useNavigate();
  const createUser = useCreateUser();

  const [typeId, setTypeId] = useState<string>('');
  const {data: typeList, isLoading: typesLoading} = useGetUserTypes();
  const {data: userType} = useGetUserType(typeId || undefined);

  const {control, handleSubmit, formState} = useForm<Record<string, unknown>>({mode: 'onBlur'});

  const types: SchemaInterface[] = typeList?.types ?? [];
  const schema: Record<string, PropertyDefinition> = userType?.schema ?? {};

  const onSubmit = handleSubmit((values: Record<string, unknown>) => {
    const {ouId, ...attributes} = values;
    createUser.mutate(
      {ouId: typeof ouId === 'string' ? ouId : '', type: userType?.name ?? '', attributes},
      {
        onSuccess: () => {
          void navigate('/users');
        },
      },
    );
  });

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{t('add.title', 'Add User')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('add.formSubtitle', 'Create a user directly. Choose a type, then fill in what it requires.')}
        </PageTitle.SubHeader>
      </PageTitle>

      {typesLoading && (
        <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
          <CircularProgress />
        </Box>
      )}

      {!typesLoading && types.length === 0 && (
        <Alert severity="info">
          {t('add.noUserTypes', 'No user types are defined yet. Create one before adding a user.')}
        </Alert>
      )}

      {createUser.isError && <Alert severity="error">{createUser.error.message}</Alert>}

      {/* A refused submit is otherwise invisible: the button appears to do nothing. */}
      {formState.isSubmitted && Object.keys(formState.errors).length > 0 && (
        <Alert severity="warning">
          {t('add.fixErrors', 'Some required values are missing. Check the highlighted fields.')}
        </Alert>
      )}

      <form
        onSubmit={(event) => {
          void onSubmit(event);
        }}
      >
        <Stack spacing={3} sx={{maxWidth: 640}}>
          <FormControl fullWidth>
            <FormLabel htmlFor="type">{t('add.userType', 'User type')}</FormLabel>
            <Select
              id="type"
              value={typeId}
              onChange={(event: {target: {value: unknown}}) => {
                setTypeId(String(event.target.value));
              }}
            >
              {types.map((type: SchemaInterface) => (
                <MenuItem key={type.id} value={type.id}>
                  {type.name}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          <FormControl fullWidth>
            <FormLabel htmlFor="ouId">{t('add.organizationUnit', 'Organization unit')}</FormLabel>
            <Controller
              name="ouId"
              control={control}
              rules={{required: t('add.ouRequired', 'An organization unit is required')}}
              render={({field}) => (
                <OrganizationUnitTreePicker
                  value={(field.value as string) ?? ''}
                  onChange={(ouId: string) => {
                    field.onChange(ouId);
                  }}
                />
              )}
            />
            {formState.errors['ouId'] && (
              <Typography variant="caption" color="error">
                {String(formState.errors['ouId'].message)}
              </Typography>
            )}
          </FormControl>

          {/* The type's own schema decides the rest, so a type with extra attributes needs no change
              here. */}
          {Object.entries(schema).map(([name, definition]: [string, PropertyDefinition]) =>
            renderSchemaField(name, definition, control, formState.errors),
          )}

          <Stack direction="row" spacing={1}>
            <Button type="submit" variant="contained" disabled={!typeId || createUser.isPending}>
              {createUser.isPending ? t('add.creating', 'Creating...') : t('add.submit', 'Create user')}
            </Button>
            <Button
              onClick={() => {
                void navigate('/users');
              }}
            >
              {t('add.cancel', 'Cancel')}
            </Button>
          </Stack>
        </Stack>
      </form>
    </PageContent>
  );
}
