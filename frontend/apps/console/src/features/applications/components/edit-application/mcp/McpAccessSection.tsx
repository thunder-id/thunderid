/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
import {useGetUserTypes} from '@thunderid/configure-user-types';
import {
  Autocomplete,
  Box,
  Button,
  Chip,
  CircularProgress,
  FormControl,
  FormLabel,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Plus, Trash} from '@wso2/oxygen-ui-icons-react';
import type {ChangeEvent, JSX} from 'react';
import {useEffect, useState} from 'react';
import {Controller, useForm} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import {z} from 'zod';
import type {Application} from '../../../models/application';
import {InboundAuthTypes} from '../../../models/inbound-auth';
import type {OAuth2Config} from '../../../models/oauth';
import validateMcpRedirectUri from '../../../utils/validateMcpRedirectUri';

/**
 * Props for the {@link McpAccessSection} component.
 *
 * @public
 */
export interface McpAccessSectionProps {
  /**
   * The application being edited
   */
  application: Application;

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

  /**
   * Whether the application is read-only, disabling all inputs
   */
  isReadOnly: boolean;

  /**
   * Callback to report whether this section currently has validation errors (feeds the Save bar).
   */
  onValidationChange?: (hasErrors: boolean) => void;
}

/**
 * Section component for an MCP client's access settings, in order: the allowed user
 * types (which user schemas may authorize this client), its optional client URI, and its
 * authorized redirect URIs — matching the React-template General tab's Access card layout.
 *
 * Redirect URIs are validated against the MCP redirect URI rule (loopback or HTTPS) instead
 * of the generic AccessSection rule, and writes update back through
 * `onFieldChange('inboundAuthConfig', …)` by spreading the existing `oauth2Config` so backend
 * fields not modeled on the frontend survive round-trips.
 *
 * Shown only for user-delegated MCP clients — machine-to-machine clients act on
 * their own identity, not a user's, so none of these fields apply.
 *
 * @param props - The component props
 * @param props.application - The application being edited
 * @param props.oauth2Config - OAuth2 configuration for the application
 * @param props.onFieldChange - Callback invoked when the field value changes
 * @param props.isReadOnly - Whether the application is read-only
 * @param props.onValidationChange - Callback invoked with whether this section has validation errors
 *
 * @returns JSX element displaying the access card
 *
 * @example
 * ```tsx
 * <McpAccessSection
 *   application={application}
 *   oauth2Config={oauth2Config}
 *   onFieldChange={handleFieldChange}
 *   isReadOnly={false}
 * />
 * ```
 *
 * @public
 */
export default function McpAccessSection({
  application,
  oauth2Config = undefined,
  onFieldChange,
  isReadOnly,
  onValidationChange = undefined,
}: McpAccessSectionProps): JSX.Element {
  const {t} = useTranslation();
  const {data: userTypesData, isLoading: loadingUserTypes} = useGetUserTypes();

  const userTypeOptions = userTypesData?.types.map((schema) => schema.name) ?? [];

  const clientUriSchema = z.object({
    url: z
      .string()
      .url(t('applications:edit.mcp.connect.clientUri.error.invalid', 'Please enter a valid URL'))
      .or(z.literal(''))
      .optional(),
  });

  type ClientUriFormData = z.infer<typeof clientUriSchema>;

  const {
    control,
    formState: {errors},
  } = useForm<ClientUriFormData>({
    resolver: zodResolver(clientUriSchema),
    mode: 'onChange',
    defaultValues: {
      url: application.url ?? '',
    },
  });

  const [redirectUris, setRedirectUris] = useState<string[]>(() => oauth2Config?.redirectUris ?? []);
  const [uriErrors, setUriErrors] = useState<Record<number, string>>({});

  const updateRedirectUris = (uris: string[]): void => {
    if (!oauth2Config) return;

    const validUris = uris.map((uri) => uri.trim()).filter((uri) => uri !== '');
    const updatedConfig = {...oauth2Config, redirectUris: validUris};
    const updatedInboundAuth = application.inboundAuthConfig?.map((config) =>
      config.type === InboundAuthTypes.OAUTH2 ? {...config, config: updatedConfig} : config,
    );
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  const handleUriChange = (index: number, value: string): void => {
    setRedirectUris((prev) => {
      const newUris = [...prev];
      newUris[index] = value;

      return newUris;
    });

    setUriErrors((prev) => {
      if (!(index in prev)) return prev;
      const newErrors = {...prev};
      delete newErrors[index];

      return newErrors;
    });
  };

  const handleUriBlur = (index: number): void => {
    const uri = redirectUris[index];
    const result = validateMcpRedirectUri(uri);

    if (result.valid) {
      setUriErrors((prev) => {
        const newErrors = {...prev};
        delete newErrors[index];

        return newErrors;
      });
      updateRedirectUris(redirectUris);
    } else {
      setUriErrors((prev) => ({
        ...prev,
        [index]: t(result.errorKey ?? 'applications:onboarding.mcp.connection.redirectUris.error.invalid'),
      }));
    }
  };

  const handleAddUri = (): void => {
    setRedirectUris((prev) => [...prev, '']);
  };

  const handleRemoveUri = (index: number): void => {
    const newUris = redirectUris.filter((_, i) => i !== index);
    setRedirectUris(newUris);

    setUriErrors((prev) => {
      const reindexed: Record<number, string> = {};
      Object.entries(prev).forEach(([key, value]) => {
        const oldIndex = parseInt(key, 10);
        if (oldIndex > index) {
          reindexed[oldIndex - 1] = value;
        } else if (oldIndex < index) {
          reindexed[oldIndex] = value;
        }
      });

      return reindexed;
    });

    updateRedirectUris(newUris);
  };

  useEffect(() => {
    const nonEmptyUris = redirectUris.filter((uri) => uri.trim() !== '');
    const redirectUrisValid = nonEmptyUris.length > 0 && nonEmptyUris.every((uri) => validateMcpRedirectUri(uri).valid);
    const clientUriValid = !errors.url;
    onValidationChange?.(!(redirectUrisValid && clientUriValid));
  }, [redirectUris, errors.url, onValidationChange]);

  return (
    <SettingsCard
      title={t('applications:edit.general.sections.access', 'Access')}
      description={t(
        'applications:edit.general.sections.access.description',
        "Configure who can access this application, where it's hosted, etc.",
      )}
    >
      <Stack spacing={3}>
        <FormControl fullWidth>
          <FormLabel htmlFor="mcp-allowed-user-types-autocomplete">
            {t('applications:edit.general.labels.allowedUserTypes', 'Allowed User Types')}
          </FormLabel>
          <Autocomplete
            multiple
            fullWidth
            id="mcp-allowed-user-types-autocomplete"
            options={userTypeOptions}
            value={application.allowedUserTypes ?? []}
            onChange={(_event, newValue) => onFieldChange('allowedUserTypes', newValue)}
            loading={loadingUserTypes}
            disabled={isReadOnly}
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
          <FormLabel htmlFor="mcp-client-uri-input">
            {t('applications:edit.mcp.connect.clientUri.label', 'Client URI')}
          </FormLabel>
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
                id="mcp-client-uri-input"
                placeholder="https://example.com"
                error={!!errors.url}
                helperText={
                  errors.url?.message ??
                  t('applications:edit.mcp.connect.clientUri.hint', 'Public homepage of this client (optional).')
                }
                disabled={isReadOnly}
              />
            )}
          />
        </FormControl>

        <FormControl fullWidth required>
          <FormLabel htmlFor="mcp-redirect-uris-section">
            {t('applications:edit.general.redirectUris.title', 'Authorized redirect URIs')}
          </FormLabel>
          <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 2}}>
            {t(
              'applications:onboarding.mcp.connection.redirectUris.hint',
              'Each URI must be a loopback address (http://localhost or http://127.0.0.1) or use HTTPS. At least one is required.',
            )}
          </Typography>

          <Stack spacing={2} id="mcp-redirect-uris-section">
            {redirectUris.map((uri, index) => (
              // IMPORTANT: Do not remove the suppression since it affects functionality.
              // eslint-disable-next-line react/no-array-index-key
              <Stack key={index} direction="row" spacing={1} alignItems="flex-start">
                <FormControl fullWidth required sx={{flex: 1}}>
                  <TextField
                    fullWidth
                    id={`mcp-redirect-uri-${index}-input`}
                    value={uri}
                    onChange={(e: ChangeEvent<HTMLInputElement>) => handleUriChange(index, e.target.value)}
                    onBlur={() => handleUriBlur(index)}
                    error={!!uriErrors[index]}
                    helperText={uriErrors[index]}
                    placeholder="https://your-app.example.com/callback"
                    disabled={isReadOnly}
                    sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
                  />
                </FormControl>
                <Tooltip title={t('common:actions.delete')}>
                  <IconButton
                    aria-label={t('common:actions.delete')}
                    onClick={() => handleRemoveUri(index)}
                    color="error"
                    sx={{mt: 1}}
                    disabled={isReadOnly}
                  >
                    <Trash size={20} />
                  </IconButton>
                </Tooltip>
              </Stack>
            ))}

            <Box>
              <Button
                variant="outlined"
                size="small"
                startIcon={<Plus size={16} />}
                onClick={handleAddUri}
                disabled={isReadOnly}
              >
                {t('applications:onboarding.mcp.connection.redirectUris.addUri', 'Add redirect URI')}
              </Button>
            </Box>
          </Stack>
        </FormControl>
      </Stack>
    </SettingsCard>
  );
}
