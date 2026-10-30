// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {zodResolver} from '@hookform/resolvers/zod';
import {DocumentTitle, FullScreenCreationWizardLayout} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {generateHandle} from '@thunderid/utils';
import {
  Alert,
  Button,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Pencil} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useRef, useState, type ReactNode} from 'react';
import {Controller, useForm} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import {z} from 'zod';
import getProjectErrorMessage from './getProjectErrorMessage';
import type {CreateProjectRequest} from './useCreateProject';
import useCreateProject from './useCreateProject';
import useProjectSelection from './useProjectSelection';
import RouteConfig from '../../configs/RouteConfig';

/** Limits the project API applies (`api/project.yaml`). */
const NAME_MAX_LENGTH = 100;
const HANDLE_MAX_LENGTH = 100;
const DESCRIPTION_MAX_LENGTH = 255;

const createFormSchema = (t: (key: string, options?: Record<string, unknown>) => string) =>
  z.object({
    displayName: z
      .string()
      .trim()
      .min(1, t('common:projects.create.name.required', {defaultValue: 'Name is required'}))
      .max(
        NAME_MAX_LENGTH,
        t('common:projects.create.name.maxLength', {
          max: NAME_MAX_LENGTH,
          defaultValue: `Name cannot exceed ${NAME_MAX_LENGTH} characters`,
        }),
      ),
    handle: z
      .string()
      .trim()
      .min(1, t('common:projects.create.handle.required', {defaultValue: 'Handle is required'}))
      .max(
        HANDLE_MAX_LENGTH,
        t('common:projects.create.handle.maxLength', {
          max: HANDLE_MAX_LENGTH,
          defaultValue: `Handle cannot exceed ${HANDLE_MAX_LENGTH} characters`,
        }),
      )
      .regex(
        /^[a-z0-9-]+$/,
        t('common:projects.create.handle.format', {
          defaultValue: 'Handle must be lowercase alphanumeric with hyphens only',
        }),
      ),
    description: z
      .string()
      .trim()
      .max(
        DESCRIPTION_MAX_LENGTH,
        t('common:projects.create.description.maxLength', {
          max: DESCRIPTION_MAX_LENGTH,
          defaultValue: `Description cannot exceed ${DESCRIPTION_MAX_LENGTH} characters`,
        }),
      ),
  });

type FormData = z.infer<ReturnType<typeof createFormSchema>>;

/**
 * Creates a project, which is backed by a new root organization unit. The new project is selected
 * and its applications page opens.
 */
export default function ProjectCreatePage(): ReactNode {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('ProjectCreatePage');
  const createProject = useCreateProject();
  const {selectProject} = useProjectSelection();

  const [error, setError] = useState<string | null>(null);
  const [isHandleEditable, setIsHandleEditable] = useState<boolean>(false);
  const isHandleManuallyEditedRef = useRef<boolean>(false);
  const handleInputRef = useRef<HTMLInputElement>(null);

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

  const goToProjects = (): void => goTo(RouteConfig.projects.list());

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
  };

  // The handle follows the name until the user unlocks it for editing.
  const handleEditHandle = (): void => {
    isHandleManuallyEditedRef.current = true;
    setIsHandleEditable(true);
    handleInputRef.current?.focus();
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
        selectProject(project.id);
        goTo(RouteConfig.applications.list());
      },
      onError: (err: Error) => {
        setError(
          getProjectErrorMessage(
            err,
            t,
            'common:projects.create.error',
            'Failed to create the project. Please try again.',
          ),
        );
      },
    });
  };

  const pageTitle = t('common:projects.create.title', 'Create a project');

  return (
    <FullScreenCreationWizardLayout
      onClose={goToProjects}
      progress={100}
      breadcrumbItems={[
        {key: 'projects', label: t('common:projects.title', 'All projects'), onClick: goToProjects},
        {key: 'create', label: pageTitle},
      ]}
      footer={
        <Stack direction="row" spacing={2}>
          <Button variant="outlined" onClick={goToProjects}>
            {t('common:actions.cancel', 'Cancel')}
          </Button>
          <Button
            type="submit"
            form="create-project-form"
            variant="contained"
            disabled={createProject.isPending || !isValid}
            sx={{minWidth: 100}}
          >
            {createProject.isPending ? t('common:status.saving', 'Saving...') : t('common:actions.create', 'Create')}
          </Button>
        </Stack>
      }
    >
      <DocumentTitle title={pageTitle} />
      <Typography variant="h1" sx={{mb: 4}}>
        {pageTitle}
      </Typography>

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
        <Grid container spacing={3}>
          <Grid size={{xs: 12, md: 6}}>
            <FormControl fullWidth>
              <FormLabel htmlFor="project-name-input">{t('common:projects.create.name.label', 'Name')}</FormLabel>
              <Controller
                name="displayName"
                control={control}
                render={({field}) => (
                  <TextField
                    {...field}
                    fullWidth
                    id="project-name-input"
                    onChange={(e) => handleNameChange(e.target.value)}
                    placeholder={t('common:projects.create.name.placeholder', 'e.g., Customer support platform')}
                    error={!!errors.displayName}
                    helperText={errors.displayName?.message}
                  />
                )}
              />
            </FormControl>
          </Grid>

          <Grid size={{xs: 12, md: 6}}>
            <FormControl fullWidth>
              <FormLabel htmlFor="project-handle-input">{t('common:projects.create.handle.label', 'Handle')}</FormLabel>
              <Controller
                name="handle"
                control={control}
                render={({field}) => (
                  <TextField
                    {...field}
                    fullWidth
                    id="project-handle-input"
                    inputRef={handleInputRef}
                    onChange={(e) => handleHandleChange(e.target.value)}
                    error={!!errors.handle}
                    helperText={errors.handle?.message}
                    sx={isHandleEditable ? undefined : {'& .MuiInputBase-root': {bgcolor: 'action.hover'}}}
                    slotProps={{
                      input: {
                        readOnly: !isHandleEditable,
                        endAdornment: isHandleEditable ? undefined : (
                          <InputAdornment position="end">
                            <Tooltip title={t('common:projects.create.handle.edit', 'Edit handle')}>
                              <IconButton
                                aria-label={t('common:projects.create.handle.edit', 'Edit handle')}
                                onClick={handleEditHandle}
                                edge="end"
                                size="small"
                              >
                                <Pencil size={16} />
                              </IconButton>
                            </Tooltip>
                          </InputAdornment>
                        ),
                      },
                    }}
                  />
                )}
              />
            </FormControl>
          </Grid>

          <Grid size={12}>
            <FormControl fullWidth>
              <FormLabel htmlFor="project-description-input">
                {t('common:projects.create.description.label', 'Description')}{' '}
                <Typography component="span" variant="inherit" color="text.secondary">
                  {t('common:projects.create.description.optional', '(optional)')}
                </Typography>
              </FormLabel>
              <Controller
                name="description"
                control={control}
                render={({field}) => (
                  <TextField
                    {...field}
                    fullWidth
                    multiline
                    minRows={4}
                    id="project-description-input"
                    onChange={(e) => {
                      setError(null);
                      field.onChange(e);
                    }}
                    placeholder={t('common:projects.create.description.placeholder', 'What is this project for?')}
                    error={!!errors.description}
                    helperText={errors.description?.message}
                  />
                )}
              />
            </FormControl>
          </Grid>
        </Grid>
      </form>
    </FullScreenCreationWizardLayout>
  );
}
