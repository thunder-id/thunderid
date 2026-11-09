// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {zodResolver} from '@hookform/resolvers/zod';
import {DocumentTitle, FullScreenCreationWizardLayout, NameSuggestion} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {generateHandle} from '@thunderid/utils';
import {Alert, Box, Button, FormControl, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useMemo, useRef, useState, type ReactNode} from 'react';
import {Controller, useForm} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import {z} from 'zod';
import useCreateProject from '../api/useCreateProject';
import useProjectRoutes from '../hooks/useProjectRoutes';
import useProjectSelection from '../hooks/useProjectSelection';
import type {CreateProjectRequest} from '../models/requests';
import getProjectErrorMessage from '../utils/getProjectErrorMessage';

/** Limits the project API applies (`api/project.yaml`). */
const NAME_MAX_LENGTH = 100;
const HANDLE_MAX_LENGTH = 100;
const DESCRIPTION_MAX_LENGTH = 255;

const createFormSchema = (t: (key: string, options?: Record<string, unknown>) => string) =>
  z.object({
    displayName: z
      .string()
      .trim()
      .min(1, t('projects:create.name.required', {defaultValue: 'Name is required'}))
      .max(
        NAME_MAX_LENGTH,
        t('projects:create.name.maxLength', {
          max: NAME_MAX_LENGTH,
          defaultValue: `Name cannot exceed ${NAME_MAX_LENGTH} characters`,
        }),
      ),
    handle: z
      .string()
      .trim()
      .min(1, t('projects:create.handle.required', {defaultValue: 'Handle is required'}))
      .max(
        HANDLE_MAX_LENGTH,
        t('projects:create.handle.maxLength', {
          max: HANDLE_MAX_LENGTH,
          defaultValue: `Handle cannot exceed ${HANDLE_MAX_LENGTH} characters`,
        }),
      )
      .regex(
        /^[a-z0-9]([a-z0-9_-]*[a-z0-9])?$/,
        t('projects:create.handle.format', {
          defaultValue:
            'Handle may contain only lowercase letters, numbers, hyphens and underscores, and must start and end with a letter or a number',
        }),
      ),
    description: z
      .string()
      .trim()
      .max(
        DESCRIPTION_MAX_LENGTH,
        t('projects:create.description.maxLength', {
          max: DESCRIPTION_MAX_LENGTH,
          defaultValue: `Description cannot exceed ${DESCRIPTION_MAX_LENGTH} characters`,
        }),
      ),
  });

type FormData = z.infer<ReturnType<typeof createFormSchema>>;

/**
 * Creates a project. The new project is selected and its applications page opens.
 */
export default function ProjectCreatePage(): ReactNode {
  const navigate = useNavigate();
  const routes = useProjectRoutes();
  const {t} = useTranslation();
  const logger = useLogger('ProjectCreatePage');
  const createProject = useCreateProject();
  const {selectProject} = useProjectSelection();

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
    defaultValues: {displayName: '', handle: '', description: ''},
  });

  const goTo = (path: string): void => {
    navigate(path)?.catch((_error: unknown) => {
      logger.error('Failed to navigate from the create project page', {error: _error});
    });
  };

  const goToProjects = (): void => goTo(routes.projects.list());

  // The handle follows the name until the user edits it.
  const handleNameChange = (name: string): void => {
    setError(null); // a create error is stale once the form changes
    setValue('displayName', name, {shouldValidate: true});
    if (!isHandleManuallyEditedRef.current) {
      setValue('handle', generateHandle(name), {shouldValidate: true});
    }
  };

  const handleHandleChange = (handle: string): void => {
    setError(null);
    setValue('handle', handle, {shouldValidate: true});
    isHandleManuallyEditedRef.current = true;
  };

  const onSubmit = (data: FormData): void => {
    setError(null);

    const request: CreateProjectRequest = {
      handle: data.handle,
      displayName: data.displayName,
      ...(data.description ? {description: data.description} : {}),
    };

    createProject.mutate(request, {
      onSuccess: (project) => {
        selectProject(project.handle, routes.applications.list());
      },
      onError: (err: Error) => {
        setError(
          getProjectErrorMessage(err, t, 'projects:create.error', 'Failed to create the project. Please try again.'),
        );
      },
    });
  };

  const pageTitle = t('projects:create.title', 'Create a project');

  return (
    <FullScreenCreationWizardLayout
      onClose={goToProjects}
      progress={100}
      breadcrumbItems={[{key: 'create', label: pageTitle}]}
      footer={
        <Box sx={{display: 'flex', justifyContent: 'flex-end'}}>
          <Button
            type="submit"
            form="create-project-form"
            variant="contained"
            disabled={createProject.isPending || !isValid}
            sx={{minWidth: 100}}
          >
            {createProject.isPending ? t('common:status.saving', 'Saving...') : t('common:actions.create', 'Create')}
          </Button>
        </Box>
      }
    >
      <DocumentTitle title={pageTitle} />
      {error && (
        <Alert severity="error" sx={{mb: 3}} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      <form
        id="create-project-form"
        onSubmit={(e) => {
          e.preventDefault();
          handleSubmit(onSubmit)(e).catch((err: unknown) => {
            logger.error('Form submission error', {error: err});
          });
        }}
      >
        <Stack direction="column" spacing={4}>
          <Typography variant="h1" gutterBottom>
            {t('projects:create.heading', "Let's set up your project")}
          </Typography>

          <FormControl fullWidth required>
            <FormLabel htmlFor="project-name-input">{t('projects:create.name.label', 'Name')}</FormLabel>
            <Controller
              name="displayName"
              control={control}
              render={({field}) => (
                <TextField
                  {...field}
                  fullWidth
                  id="project-name-input"
                  onChange={(e) => handleNameChange(e.target.value)}
                  placeholder={t('projects:create.name.placeholder', 'e.g., Customer support platform')}
                  error={!!errors.displayName}
                  helperText={errors.displayName?.message}
                />
              )}
            />

            <NameSuggestion onSelect={handleNameChange} />
          </FormControl>

          <FormControl fullWidth required>
            <FormLabel htmlFor="project-handle-input">{t('projects:create.handle.label', 'Handle')}</FormLabel>
            <Controller
              name="handle"
              control={control}
              render={({field}) => (
                <TextField
                  {...field}
                  fullWidth
                  id="project-handle-input"
                  onChange={(e) => handleHandleChange(e.target.value)}
                  placeholder={t('projects:create.handle.placeholder', 'e.g., customer-support')}
                  error={!!errors.handle}
                  helperText={
                    errors.handle?.message ?? t('projects:create.handle.hint', 'A unique identifier for this project')
                  }
                />
              )}
            />
          </FormControl>

          <FormControl fullWidth>
            <FormLabel htmlFor="project-description-input">
              {t('projects:create.description.label', 'Description')}
            </FormLabel>
            <Controller
              name="description"
              control={control}
              render={({field}) => (
                <TextField
                  {...field}
                  fullWidth
                  multiline
                  minRows={2}
                  id="project-description-input"
                  onChange={(e) => {
                    setError(null);
                    field.onChange(e);
                  }}
                  placeholder={t('projects:create.description.placeholder', 'What is this project for?')}
                  error={!!errors.description}
                  helperText={errors.description?.message}
                />
              )}
            />
          </FormControl>
        </Stack>
      </form>
    </FullScreenCreationWizardLayout>
  );
}
