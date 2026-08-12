// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {zodResolver} from '@hookform/resolvers/zod';
import {FullScreenCreationWizardLayout, NameSuggestion, OrganizationUnitSummaryChip} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage} from '@thunderid/utils';
import {Box, Stack, Typography, Button, TextField, Alert, FormControl, FormLabel} from '@wso2/oxygen-ui';
import {useState, useMemo, useRef, type JSX} from 'react';
import {useForm, Controller} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {useNavigate, useLocation} from 'react-router';
import {z} from 'zod';
import useCreateOrganizationUnit from '../api/useCreateOrganizationUnit';
import OrganizationUnitConstraints from '../constants/organization-unit-constraints';
import OrganizationUnitTreeConstants from '../constants/organization-unit-tree-constants';
import useOrganizationUnit from '../contexts/useOrganizationUnit';
import useOrganizationUnitRoutes from '../hooks/useOrganizationUnitRoutes';
import type {CreateOrganizationUnitRequest} from '../models/requests';

/**
 * Creates a Zod schema for the create organization unit form with i18n support.
 * Validates name and handle fields.
 */
const createFormSchema = (t: (key: string, options?: Record<string, unknown>) => string) =>
  z.object({
    name: z
      .string()
      .trim()
      .min(
        OrganizationUnitConstraints.NAME_MIN_LENGTH,
        t('organizationUnits:edit.general.name.validations.required', {defaultValue: 'Name is required'}),
      )
      .max(
        OrganizationUnitConstraints.NAME_MAX_LENGTH,
        t('organizationUnits:edit.general.name.validations.maxLength', {
          max: OrganizationUnitConstraints.NAME_MAX_LENGTH,
          defaultValue: `Name cannot exceed ${OrganizationUnitConstraints.NAME_MAX_LENGTH} characters`,
        }),
      ),
    handle: z
      .string()
      .trim()
      .min(
        OrganizationUnitConstraints.HANDLE_MIN_LENGTH,
        t('organizationUnits:edit.general.handle.validations.required', {defaultValue: 'Handle is required'}),
      )
      .max(
        OrganizationUnitConstraints.HANDLE_MAX_LENGTH,
        t('organizationUnits:edit.general.handle.validations.maxLength', {
          max: OrganizationUnitConstraints.HANDLE_MAX_LENGTH,
          defaultValue: `Handle cannot exceed ${OrganizationUnitConstraints.HANDLE_MAX_LENGTH} characters`,
        }),
      )
      .regex(/^[a-z0-9-]+$/, t('organizationUnits:edit.general.handle.validations.format')),
    parentId: z.string().nullable(),
  });

/**
 * Type definition for form data inferred from the Zod schema.
 */
type FormData = z.infer<ReturnType<typeof createFormSchema>>;

export default function CreateOrganizationUnitPage(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const routes = useOrganizationUnitRoutes();
  const {t} = useTranslation();
  const logger = useLogger('CreateOrganizationUnitPage');
  const createOrganizationUnit = useCreateOrganizationUnit();
  const {resetTreeState} = useOrganizationUnit();

  const navigationState = location.state as {parentId?: string; parentName?: string; parentHandle?: string} | null;
  const preselectedParentId = navigationState?.parentId ?? null;
  const parentDisplayName = navigationState?.parentName ?? null;
  const parentDisplayHandle = navigationState?.parentHandle ?? null;

  const [error, setError] = useState<string | null>(null);
  const isHandleManuallyEditedRef = useRef<boolean>(false);

  const formSchema = useMemo(() => createFormSchema(t), [t]);

  const {
    control,
    handleSubmit,
    setValue,
    formState: {errors, isValid},
  } = useForm<FormData>({
    resolver: zodResolver(formSchema),
    mode: 'onChange',
    defaultValues: {
      name: '',
      handle: '',
      parentId: preselectedParentId,
    },
  });

  /**
   * Generates a handle from the name by lowercasing and replacing spaces with hyphens.
   */
  const generateHandleFromName = (nameValue: string): string => nameValue.toLowerCase().replace(/\s+/g, '-');

  const listUrl = routes.list();

  const handleClose = (): void => {
    (async (): Promise<void> => {
      await navigate(listUrl);
    })().catch((_error: unknown) => {
      logger.error('Failed to navigate back to organization units list', {error: _error});
    });
  };

  const handleNameChange = (newName: string): void => {
    setError(null); // a create error is stale once the form changes
    setValue('name', newName, {shouldValidate: true});
    // Auto-generate handle if user hasn't manually edited it
    if (!isHandleManuallyEditedRef.current) {
      setValue('handle', generateHandleFromName(newName), {shouldValidate: true});
    }
  };

  const handleHandleChange = (newHandle: string): void => {
    setError(null); // a create error is stale once the form changes
    setValue('handle', newHandle, {shouldValidate: true});
    isHandleManuallyEditedRef.current = true;
  };

  const handleNameSuggestionSelect = (suggestion: string): void => {
    setError(null); // a create error is stale once the form changes
    setValue('name', suggestion, {shouldValidate: true});
    // Auto-generate handle from suggestion if user hasn't manually edited it
    if (!isHandleManuallyEditedRef.current) {
      setValue('handle', generateHandleFromName(suggestion), {shouldValidate: true});
    }
  };

  const onSubmit = (data: FormData): void => {
    setError(null);

    const requestData: CreateOrganizationUnitRequest = {
      handle: data.handle,
      name: data.name,
      parent: data.parentId,
    };

    createOrganizationUnit.mutate(requestData, {
      onSuccess: () => {
        resetTreeState();
        (async (): Promise<void> => {
          await navigate(listUrl);
        })().catch((_error: unknown) => {
          logger.error('Failed to navigate after creating organization unit', {error: _error});
        });
      },
      onError: (err: Error) => {
        setError(
          getErrorMessage(
            err,
            (key, options) => t(key.includes(':') ? key : `organizationUnits:${key}`, options),
            'create.error',
            'Failed to create organization unit. Please try again.',
          ),
        );
      },
    });
  };

  return (
    <FullScreenCreationWizardLayout
      onClose={handleClose}
      progress={100}
      breadcrumbItems={[{key: 'create', label: t('organizationUnits:create.title', 'Create Organization Unit')}]}
      footer={
        <Box sx={{display: 'flex', justifyContent: 'flex-end'}}>
          <Button
            type="submit"
            form="create-organization-unit-form"
            variant="contained"
            disabled={createOrganizationUnit.isPending || !isValid}
            sx={{minWidth: 100}}
          >
            {createOrganizationUnit.isPending
              ? t('common:status.saving', 'Saving...')
              : t('common:actions.create', 'Create')}
          </Button>
        </Box>
      }
    >
      {error && (
        <Alert severity="error" sx={{mb: 3}} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      <form
        id="create-organization-unit-form"
        onSubmit={(e) => {
          e.preventDefault();
          handleSubmit(onSubmit)(e).catch((err: unknown) => {
            logger.error('Form submission error', {error: err});
          });
        }}
      >
        <Stack direction="column" spacing={4}>
          <Typography variant="h1" gutterBottom>
            {t('organizationUnits:create.heading', "Let's set up your organization unit")}
          </Typography>

          <OrganizationUnitSummaryChip
            icon={OrganizationUnitTreeConstants.DEFAULT_AVATAR}
            label={t('organizationUnits:edit.general.parent.label', 'Parent Organization Unit')}
            value={
              parentDisplayName
                ? `${parentDisplayName}${parentDisplayHandle ? ` (${parentDisplayHandle})` : ''}`
                : t('organizationUnits:edit.general.ou.noParent.label', 'Root Organization Unit')
            }
          />

          {/* Name field first */}
          <FormControl fullWidth required>
            <FormLabel htmlFor="ou-name-input">{t('organizationUnits:edit.general.name.label', 'Name')}</FormLabel>
            <Controller
              name="name"
              control={control}
              render={({field}) => (
                <TextField
                  {...field}
                  fullWidth
                  id="ou-name-input"
                  onChange={(e) => handleNameChange(e.target.value)}
                  placeholder={t('organizationUnits:edit.general.name.placeholder', 'e.g., Engineering Department')}
                  error={!!errors.name}
                  helperText={errors.name?.message}
                />
              )}
            />

            <NameSuggestion onSelect={handleNameSuggestionSelect} />
          </FormControl>

          {/* Handle field */}
          <FormControl fullWidth required>
            <FormLabel htmlFor="ou-handle-input">
              {t('organizationUnits:edit.general.handle.label', 'Handle')}
            </FormLabel>
            <Controller
              name="handle"
              control={control}
              render={({field}) => (
                <TextField
                  {...field}
                  fullWidth
                  id="ou-handle-input"
                  onChange={(e) => handleHandleChange(e.target.value)}
                  placeholder={t('organizationUnits:edit.general.handle.placeholder', 'e.g., engineering, sales, hr')}
                  error={!!errors.handle}
                  helperText={
                    errors.handle?.message ??
                    t('organizationUnits:edit.general.handle.hint', 'A unique identifier for this organization unit')
                  }
                />
              )}
            />
          </FormControl>
        </Stack>
      </form>
    </FullScreenCreationWizardLayout>
  );
}
