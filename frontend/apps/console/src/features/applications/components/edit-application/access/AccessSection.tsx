// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {zodResolver} from '@hookform/resolvers/zod';
import {SettingsCard} from '@thunderid/components';
import type {Application} from '@thunderid/configure-applications';
import {useGetUserTypes} from '@thunderid/configure-user-types';
import {Stack, TextField, Chip, Autocomplete, CircularProgress, FormControl, FormLabel} from '@wso2/oxygen-ui';
import {useEffect} from 'react';
import {useForm, Controller} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {z} from 'zod';

/**
 * Props for the {@link AccessSection} component.
 */
interface AccessSectionProps {
  /**
   * The application being edited
   */
  application: Application;
  /**
   * Partial application object containing edited fields
   */
  editedApp: Partial<Application>;
  /**
   * Callback function to handle field value changes
   * @param field - The application field being updated
   * @param value - The new value for the field
   */
  onFieldChange: (field: keyof Application, value: unknown) => void;
  /**
   * Callback function to handle validation changes
   * @param hasErrors - Boolean indicating if the access settings have validation errors
   */
  onValidationChange?: (hasErrors: boolean) => void;
  /**
   * Whether to show user-facing access config (allowed user types). Hidden for clients with no
   * user-facing grant.
   */
  showUserAccessConfig?: boolean;
}

/**
 * Section component for managing application access settings.
 *
 * Provides configuration for:
 * - Allowed user types selection
 * - Application access URL, with room for future access-related configuration (e.g. discoverability)
 *
 * Includes form validation using Zod schema and react-hook-form.
 *
 * @param props - Component props
 * @returns Access settings UI within SettingsCards
 */
export default function AccessSection({
  application,
  editedApp,
  onFieldChange,
  onValidationChange = undefined,
  showUserAccessConfig = true,
}: AccessSectionProps) {
  const {t} = useTranslation();
  const {data: userTypesData, isLoading: loadingUserTypes} = useGetUserTypes();

  const userTypeOptions = userTypesData?.types.map((schema) => schema.name) ?? [];

  const accessSchema = z.object({
    url: z.string().url('Please enter a valid URL').or(z.literal('')).optional(),
  });

  type AccessFormData = z.infer<typeof accessSchema>;

  const {
    control,
    trigger,
    formState: {errors},
  } = useForm<AccessFormData>({
    resolver: zodResolver(accessSchema),
    mode: 'onChange',
    defaultValues: {
      url: editedApp.url ?? application.url ?? '',
    },
  });

  // Validate default values on mount so stale validation state doesn't survive a remount.
  useEffect(() => {
    void trigger();
  }, [trigger]);

  // Effect to notify parent component of validation state changes.
  useEffect(() => {
    if (onValidationChange) {
      onValidationChange(!!errors.url);
    }
  }, [errors.url, onValidationChange]);

  return (
    <Stack spacing={3}>
      {showUserAccessConfig && (
        <SettingsCard
          title={t('applications:edit.access.sections.userTypes.title', 'Allowed User Types')}
          description={t(
            'applications:edit.access.sections.userTypes.description',
            'Choose which user types can sign up through this application.',
          )}
        >
          <FormControl fullWidth>
            <FormLabel htmlFor="allowed-user-types-autocomplete">
              {t('applications:edit.general.labels.allowedUserTypes')}
            </FormLabel>
            <Autocomplete
              multiple
              fullWidth
              id="allowed-user-types-autocomplete"
              options={userTypeOptions}
              value={editedApp.allowedUserTypes ?? application.allowedUserTypes ?? []}
              onChange={(_event, newValue) => onFieldChange('allowedUserTypes', newValue)}
              loading={loadingUserTypes}
              disabled={application.isReadOnly}
              renderInput={(params) => (
                <TextField
                  {...params}
                  placeholder={t('applications:edit.general.allowedUserTypes.placeholder')}
                  helperText={t('applications:edit.general.allowedUserTypes.hint')}
                  InputProps={{
                    ...params.InputProps,
                    endAdornment: (
                      <>
                        {loadingUserTypes ? <CircularProgress color="inherit" size={20} /> : null}
                        {params.InputProps.endAdornment}
                      </>
                    ),
                  }}
                />
              )}
              renderTags={(value, getTagProps) =>
                value.map((option, index) => <Chip label={option} {...getTagProps({index})} key={option} />)
              }
              freeSolo={false}
              disableClearable={false}
            />
          </FormControl>
        </SettingsCard>
      )}

      <SettingsCard
        title={t('applications:edit.access.sections.applicationAccess.title', 'Application Access')}
        description={t(
          'applications:edit.access.sections.applicationAccess.description',
          'Configure where this application is accessed from.',
        )}
      >
        <FormControl fullWidth>
          <FormLabel htmlFor="application-url-input">{t('applications:edit.general.labels.applicationUrl')}</FormLabel>
          <Controller
            name="url"
            control={control}
            render={({field}) => (
              <TextField
                {...field}
                onChange={(e) => {
                  field.onChange(e);
                  onFieldChange('url', e.target.value);
                }}
                fullWidth
                id="application-url-input"
                placeholder="https://example.com"
                error={!!errors.url}
                helperText={errors.url?.message ?? t('applications:edit.general.applicationUrl.hint')}
                disabled={application.isReadOnly}
              />
            )}
          />
        </FormControl>
      </SettingsCard>
    </Stack>
  );
}
