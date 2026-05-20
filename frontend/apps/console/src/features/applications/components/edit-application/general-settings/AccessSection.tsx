/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
import {SettingsCard} from '@thunderid/components';
import {
  Box,
  Stack,
  TextField,
  Chip,
  Autocomplete,
  CircularProgress,
  FormControl,
  FormLabel,
  Typography,
  Button,
  IconButton,
  Tooltip,
} from '@wso2/oxygen-ui';
import {Trash, Plus} from '@wso2/oxygen-ui-icons-react';
import {useState} from 'react';
import {useForm, Controller} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {z} from 'zod';
import useGetUserTypes from '../../../../user-types/api/useGetUserTypes';
import type {Application} from '../../../models/application';
import type {OAuth2Config} from '../../../models/oauth';

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
   * OAuth2 configuration for the application (optional)
   */
  oauth2Config?: OAuth2Config;
  /**
   * Callback function to handle field value changes
   * @param field - The application field being updated
   * @param value - The new value for the field
   */
  onFieldChange: (field: keyof Application, value: unknown) => void;
}

/**
 * Section component for managing application access settings.
 *
 * Provides configuration for:
 * - Application URL with validation
 * - OAuth2 redirect URIs (add/remove/edit with validation)
 * - Allowed user types selection
 *
 * Includes form validation using Zod schema and react-hook-form.
 *
 * @param props - Component props
 * @returns Access settings UI within a SettingsCard
 */
export default function AccessSection({
  application,
  editedApp,
  oauth2Config = undefined,
  onFieldChange,
}: AccessSectionProps) {
  const {t} = useTranslation();
  const {data: userTypesData, isLoading: loadingUserTypes} = useGetUserTypes();

  const [redirectUris, setRedirectUris] = useState<string[]>(() => oauth2Config?.redirectUris ?? []);
  const [uriErrors, setUriErrors] = useState<Record<number, string>>({});

  const userTypeOptions = userTypesData?.types.map((schema) => schema.name) ?? [];

  const generalSettingsSchema = z.object({
    url: z.string().url('Please enter a valid URL').or(z.literal('')).optional(),
  });

  type GeneralSettingsFormData = z.infer<typeof generalSettingsSchema>;

  const {
    control,
    formState: {errors},
  } = useForm<GeneralSettingsFormData>({
    resolver: zodResolver(generalSettingsSchema),
    mode: 'onChange',
    defaultValues: {
      url: editedApp.url ?? application.url ?? '',
    },
  });

  const validateUri = (uri: string, index: number): boolean => {
    if (!uri || uri.trim() === '') {
      setUriErrors((prev) => ({...prev, [index]: t('applications:edit.general.redirectUris.error.empty')}));
      return false;
    }
    try {
      // eslint-disable-next-line no-new
      new URL(uri);

      setUriErrors((prev) => {
        const newErrors = {...prev};
        delete newErrors[index];

        return newErrors;
      });

      return true;
    } catch {
      setUriErrors((prev) => ({...prev, [index]: t('applications:edit.general.redirectUris.error.invalid')}));

      return false;
    }
  };

  const handleAddUri = () => {
    setRedirectUris((prev) => [...prev, '']);
  };

  const handleRemoveUri = (index: number) => {
    const newUris = redirectUris.filter((_, i) => i !== index);
    setRedirectUris(newUris);
    setUriErrors((prev) => {
      const newErrors = {...prev};
      delete newErrors[index];

      const reindexed: Record<number, string> = {};
      Object.entries(newErrors).forEach(([key, value]) => {
        const oldIndex = parseInt(key, 10);

        if (oldIndex > index) {
          reindexed[oldIndex - 1] = value;
        } else if (oldIndex < index) {
          reindexed[oldIndex] = value;
        }
      });

      return reindexed;
    });

    if (!oauth2Config) return;
    const validUris = newUris.filter((uri) => uri.trim() !== '');
    const updatedConfig = {
      ...oauth2Config,
      redirectUris: validUris,
    };
    const updatedInboundAuth = application.inboundAuthConfig?.map((config) => {
      if (config.type === 'oauth2') {
        return {...config, config: updatedConfig};
      }
      return config;
    });
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  const handleUriChange = (index: number, value: string) => {
    setRedirectUris((prev) => {
      const newUris = [...prev];
      newUris[index] = value;

      return newUris;
    });

    if (value.trim() !== '') {
      setUriErrors((prev) => {
        const newErrors = {...prev};
        delete newErrors[index];

        return newErrors;
      });
    }
  };

  const updateRedirectUris = () => {
    const validUris = redirectUris.filter((uri) => uri.trim() !== '');
    if (!oauth2Config) return;

    const updatedConfig = {
      ...oauth2Config,
      redirectUris: validUris,
    };
    const updatedInboundAuth = application.inboundAuthConfig?.map((config) => {
      if (config.type === 'oauth2') {
        return {...config, config: updatedConfig};
      }
      return config;
    });
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  const handleUriBlur = (index: number) => {
    const uri = redirectUris[index];
    if (validateUri(uri, index) && uri.trim() !== '') {
      updateRedirectUris();
    }
  };

  return (
    <SettingsCard
      title={t('applications:edit.general.sections.access')}
      description={t('applications:edit.general.sections.access.description')}
    >
      <Stack spacing={3}>
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
              />
            )}
          />
        </FormControl>

        {oauth2Config && (
          <FormControl fullWidth>
            <FormLabel htmlFor="redirect-uris-section">{t('applications:edit.general.redirectUris.title')}</FormLabel>
            <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 2}}>
              {t('applications:edit.general.redirectUris.description')}
            </Typography>

            <Stack spacing={2} id="redirect-uris-section">
              {redirectUris.map((uri, index) => (
                // IMPORTANT: Do not remove the suppression since it affects functionality.
                // eslint-disable-next-line react/no-array-index-key
                <Stack key={index} direction="row" spacing={1} alignItems="flex-start">
                  <FormControl fullWidth required sx={{flex: 1}}>
                    <TextField
                      fullWidth
                      id={`redirect-uri-${index}-input`}
                      value={uri}
                      onChange={(e) => handleUriChange(index, e.target.value)}
                      onBlur={() => handleUriBlur(index)}
                      error={!!uriErrors[index]}
                      helperText={uriErrors[index]}
                      placeholder="https://example.com/callback"
                    />
                  </FormControl>
                  <Tooltip title={t('common:actions.delete')}>
                    <IconButton onClick={() => handleRemoveUri(index)} color="error" sx={{mt: 1}}>
                      <Trash size={20} />
                    </IconButton>
                  </Tooltip>
                </Stack>
              ))}

              <Box>
                <Button variant="outlined" startIcon={<Plus />} onClick={handleAddUri} size="small">
                  {t('applications:edit.general.redirectUris.addUri')}
                </Button>
              </Box>
            </Stack>
          </FormControl>
        )}
      </Stack>
    </SettingsCard>
  );
}
