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

import {SettingsCard} from '@thunderid/components';
import {useGetThemes, useGetLayouts} from '@thunderid/design';
import {Box, Typography, TextField, Autocomplete, CircularProgress, Stack} from '@wso2/oxygen-ui';
import {useTranslation} from 'react-i18next';
import type {Application} from '../../../models/application';

/**
 * Props for the {@link AppearanceSection} component.
 */
interface AppearanceSectionProps {
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
   * Singular noun used to refer to the entity in user-visible copy (default: 'application').
   */
  entityLabel?: string;
}

/**
 * Section component for configuring application appearance.
 *
 * Provides an autocomplete dropdown to select a theme or layout from available options.
 * The selected theme or layout affects the look and feel of the application's login pages.
 *
 * @param props - Component props
 * @returns Appearance configuration UI within a SettingsCard
 */
export default function AppearanceSection({
  application,
  editedApp,
  onFieldChange,
  entityLabel = 'application',
}: AppearanceSectionProps) {
  const {t} = useTranslation();
  const {data: themesData, isLoading: loadingThemes} = useGetThemes();
  const {data: layoutsData, isLoading: loadingLayouts} = useGetLayouts();

  const themeOptions = themesData?.themes ?? [];
  const layoutOptions = layoutsData?.layouts ?? [];

  return (
    <SettingsCard
      title={t('applications:edit.customization.sections.appearance')}
      description={t(
        'applications:edit.customization.sections.appearance.description',
        'Customize the visual appearance of your {{entity}}.',
        {entity: entityLabel},
      )}
    >
      <Stack spacing={3}>
        <Box>
          <Typography variant="subtitle2" gutterBottom>
            {t('applications:edit.customization.labels.theme')}
          </Typography>
          <Autocomplete
            fullWidth
            options={themeOptions}
            getOptionLabel={(option) => (typeof option === 'string' ? option : option.displayName)}
            value={themeOptions.find((theme) => theme.id === (editedApp.themeId! ?? application.themeId!)) ?? null}
            onChange={(_event, newValue) => onFieldChange('themeId' as keyof Application, newValue?.id ?? '')}
            loading={loadingThemes}
            disabled={application.isReadOnly}
            renderInput={(params) => (
              <TextField
                {...params}
                placeholder={t('applications:edit.customization.theme.placeholder')}
                helperText={t('applications:edit.customization.theme.hint')}
                InputProps={{
                  ...params.InputProps,
                  endAdornment: (
                    <>
                      {loadingThemes ? <CircularProgress color="inherit" size={20} /> : null}
                      {params.InputProps.endAdornment}
                    </>
                  ),
                }}
              />
            )}
          />
        </Box>
        <Box>
          <Typography variant="subtitle2" gutterBottom>
            {t('applications:edit.customization.labels.layout', 'Layout')}
          </Typography>
          <Autocomplete
            fullWidth
            options={layoutOptions}
            getOptionLabel={(option) => (typeof option === 'string' ? option : option.displayName)}
            value={layoutOptions.find((layout) => layout.id === (editedApp.layoutId ?? application.layoutId)) ?? null}
            onChange={(_event, newValue) => onFieldChange('layoutId', newValue?.id ?? '')}
            loading={loadingLayouts}
            disabled={application.isReadOnly}
            renderInput={(params) => (
              <TextField
                {...params}
                placeholder={t('applications:edit.customization.layout.placeholder', 'Select a layout')}
                helperText={t(
                  'applications:edit.customization.layout.hint',
                  'Choose a layout to customize the screen structure of login pages.',
                )}
                InputProps={{
                  ...params.InputProps,
                  endAdornment: (
                    <>
                      {loadingLayouts ? <CircularProgress color="inherit" size={20} /> : null}
                      {params.InputProps.endAdornment}
                    </>
                  ),
                }}
              />
            )}
          />
        </Box>
      </Stack>
    </SettingsCard>
  );
}
