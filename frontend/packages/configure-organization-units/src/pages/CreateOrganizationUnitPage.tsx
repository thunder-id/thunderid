/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {zodResolver} from '@hookform/resolvers/zod';
import {useLogger} from '@thunderid/logger/react';
import {generateRandomHumanReadableIdentifiers} from '@thunderid/utils';
import {
  Box,
  Stack,
  Typography,
  Button,
  TextField,
  Alert,
  IconButton,
  LinearProgress,
  FormControl,
  FormLabel,
  Chip,
  useTheme,
} from '@wso2/oxygen-ui';
import {X, Lightbulb} from '@wso2/oxygen-ui-icons-react';
import {useState, useMemo, useRef, type JSX} from 'react';
import {useForm, Controller} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {useNavigate, useLocation} from 'react-router';
import {z} from 'zod';
import useCreateOrganizationUnit from '../api/useCreateOrganizationUnit';
import useOrganizationUnit from '../contexts/useOrganizationUnit';
import useOrganizationUnitRoutes from '../hooks/useOrganizationUnitRoutes';
import type {CreateOrganizationUnitRequest} from '../models/requests';

/**
 * Creates a Zod schema for the create organization unit form with i18n support.
 * Validates name, handle, description, and parent fields.
 */
const createFormSchema = (t: (key: string) => string) =>
  z.object({
    name: z.string().trim().min(1, t('organizationUnits:edit.general.name.validations.required')),
    handle: z
      .string()
      .trim()
      .min(1, t('organizationUnits:edit.general.handle.validations.required'))
      .regex(/^[a-z0-9-]+$/, t('organizationUnits:edit.general.handle.validations.format')),
    description: z.string().optional(),
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
  const theme = useTheme();
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
      description: '',
      parentId: preselectedParentId,
    },
  });

  const nameSuggestions: string[] = useMemo((): string[] => generateRandomHumanReadableIdentifiers(), []);

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
    setValue('name', newName, {shouldValidate: true});
    // Auto-generate handle if user hasn't manually edited it
    if (!isHandleManuallyEditedRef.current) {
      setValue('handle', generateHandleFromName(newName), {shouldValidate: true});
    }
  };

  const handleHandleChange = (newHandle: string): void => {
    setValue('handle', newHandle, {shouldValidate: true});
    isHandleManuallyEditedRef.current = true;
  };

  const handleNameSuggestionClick = (suggestion: string): void => {
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
      description: data.description?.trim() ? data.description.trim() : null,
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
        setError(err.message ?? t('organizationUnits:create.error'));
      },
    });
  };

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      {/* Progress bar at the very top - single step so 100% */}
      <LinearProgress variant="determinate" value={100} sx={{height: 6}} />

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'row'}}>
        <Box
          sx={{
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          {/* Header with close button */}
          <Box sx={{p: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
            <Stack direction="row" alignItems="center" spacing={2}>
              <IconButton
                onClick={handleClose}
                sx={{
                  bgcolor: 'background.paper',
                  '&:hover': {bgcolor: 'action.hover'},
                  boxShadow: 1,
                }}
              >
                <X size={24} />
              </IconButton>
              <Typography variant="h5">{t('organizationUnits:create.title')}</Typography>
            </Stack>
          </Box>

          {/* Main content */}
          <Box sx={{flex: 1, display: 'flex', minHeight: 0}}>
            {/* Left side - Form content */}
            <Box
              sx={{
                flex: 1,
                display: 'flex',
                flexDirection: 'column',
                py: 8,
                px: 20,
              }}
            >
              <Box
                sx={{
                  width: '100%',
                  maxWidth: 800,
                  display: 'flex',
                  flexDirection: 'column',
                }}
              >
                {/* Error Alert */}
                {error && (
                  <Alert severity="error" sx={{my: 3}} onClose={() => setError(null)}>
                    {error}
                  </Alert>
                )}

                <form
                  onSubmit={(e) => {
                    e.preventDefault();
                    handleSubmit(onSubmit)(e).catch((err: unknown) => {
                      logger.error('Form submission error', {error: err});
                    });
                  }}
                >
                  <Stack direction="column" spacing={4}>
                    {/* Large heading - matching application create style */}
                    <Typography variant="h1" gutterBottom>
                      {t('organizationUnits:create.heading')}
                    </Typography>

                    {/* Name field first */}
                    <FormControl fullWidth required>
                      <FormLabel htmlFor="ou-name-input">{t('organizationUnits:edit.general.name.label')}</FormLabel>
                      <Controller
                        name="name"
                        control={control}
                        render={({field}) => (
                          <TextField
                            {...field}
                            fullWidth
                            id="ou-name-input"
                            onChange={(e) => handleNameChange(e.target.value)}
                            placeholder={t('organizationUnits:edit.general.name.placeholder')}
                            error={!!errors.name}
                            helperText={errors.name?.message}
                          />
                        )}
                      />
                    </FormControl>

                    {/* Name suggestions */}
                    <Stack direction="column" spacing={2}>
                      <Stack direction="row" alignItems="center" spacing={1}>
                        <Lightbulb size={20} color={theme.vars?.palette.warning.main} />
                        <Typography variant="body2" color="text.secondary">
                          {t('organizationUnits:create.suggestions.label')}
                        </Typography>
                      </Stack>
                      <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 1}}>
                        {nameSuggestions.map(
                          (suggestion: string): JSX.Element => (
                            <Chip
                              key={suggestion}
                              label={suggestion}
                              onClick={(): void => handleNameSuggestionClick(suggestion)}
                              variant="outlined"
                              clickable
                              sx={{
                                '&:hover': {
                                  bgcolor: 'primary.main',
                                  color: 'text.primary',
                                  borderColor: 'primary.main',
                                },
                              }}
                            />
                          ),
                        )}
                      </Box>
                    </Stack>

                    {/* Handle field */}
                    <FormControl fullWidth required>
                      <FormLabel htmlFor="ou-handle-input">
                        {t('organizationUnits:edit.general.handle.label')}
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
                            placeholder={t('organizationUnits:edit.general.handle.placeholder')}
                            error={!!errors.handle}
                            helperText={errors.handle?.message ?? t('organizationUnits:edit.general.handle.hint')}
                          />
                        )}
                      />
                    </FormControl>

                    {/* Description field */}
                    <FormControl fullWidth>
                      <FormLabel htmlFor="ou-description-input">
                        {t('organizationUnits:edit.general.description.label')}
                      </FormLabel>
                      <Controller
                        name="description"
                        control={control}
                        render={({field}) => (
                          <TextField
                            {...field}
                            fullWidth
                            id="ou-description-input"
                            placeholder={t('organizationUnits:edit.general.description.placeholder')}
                            multiline
                            rows={3}
                          />
                        )}
                      />
                    </FormControl>

                    {/* Parent OU field - read-only */}
                    <FormControl fullWidth>
                      <FormLabel htmlFor="ou-parent-input">
                        {t('organizationUnits:edit.general.parent.label')}
                      </FormLabel>
                      <TextField
                        id="ou-parent-input"
                        fullWidth
                        value={
                          parentDisplayName
                            ? `${parentDisplayName}${parentDisplayHandle ? ` (${parentDisplayHandle})` : ''}`
                            : t('organizationUnits:edit.general.ou.noParent.label')
                        }
                        slotProps={{input: {readOnly: true}}}
                        helperText={t('organizationUnits:edit.general.parent.hint')}
                      />
                    </FormControl>

                    {/* Navigation buttons */}
                    <Box
                      sx={{
                        mt: 4,
                        display: 'flex',
                        justifyContent: 'flex-start',
                        gap: 2,
                      }}
                    >
                      <Button
                        type="submit"
                        variant="contained"
                        disabled={createOrganizationUnit.isPending || !isValid}
                        sx={{minWidth: 100}}
                      >
                        {createOrganizationUnit.isPending ? t('common:status.saving') : t('common:actions.create')}
                      </Button>
                    </Box>
                  </Stack>
                </form>
              </Box>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
