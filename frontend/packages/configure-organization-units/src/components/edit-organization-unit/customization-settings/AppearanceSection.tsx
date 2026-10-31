// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {useGetLayouts, useGetThemes} from '@thunderid/design';
import {Box, Typography, TextField, Autocomplete, CircularProgress, Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {OrganizationUnit} from '../../../models/organization-unit';

/**
 * Props for the {@link AppearanceSection} component.
 */
interface AppearanceSectionProps {
  /**
   * The organization unit being edited
   */
  organizationUnit: OrganizationUnit;
  /**
   * Partial organization unit object containing edited fields
   */
  editedOU: Partial<OrganizationUnit>;
  /**
   * Callback function to handle field value changes
   * @param field - The organization unit field being updated
   * @param value - The new value for the field
   */
  onFieldChange: (field: keyof OrganizationUnit, value: unknown) => void;
}

/**
 * Section component for configuring organization unit appearance.
 *
 * Provides an autocomplete dropdown to select a theme from available options.
 * The selected theme affects the look and feel of the organization unit's pages.
 *
 * @param props - Component props
 * @returns Appearance configuration UI within a SettingsCard
 */
export default function AppearanceSection({
  organizationUnit,
  editedOU,
  onFieldChange,
}: AppearanceSectionProps): JSX.Element {
  const {t} = useTranslation();
  const {data: themesData, isLoading: loadingThemes} = useGetThemes();
  const {data: layoutsData, isLoading: loadingLayouts} = useGetLayouts();

  const themeOptions = themesData?.themes ?? [];
  const layoutOptions = layoutsData?.layouts ?? [];

  return (
    <SettingsCard
      title={t('organizationUnits:edit.customization.sections.appearance')}
      description={t('organizationUnits:edit.customization.sections.appearance.description')}
    >
      <Stack spacing={3}>
        <Box>
          <Typography variant="subtitle2" gutterBottom>
            {t('organizationUnits:edit.customization.labels.theme')}
          </Typography>
          <Autocomplete
            fullWidth
            disabled={organizationUnit.isReadOnly === true}
            options={themeOptions}
            getOptionLabel={(option) => (typeof option === 'string' ? option : option.displayName)}
            value={themeOptions.find((theme) => theme.id === (editedOU.themeId ?? organizationUnit.themeId)) ?? null}
            onChange={(_event, newValue) => onFieldChange('themeId', newValue?.id ?? '')}
            loading={loadingThemes}
            renderInput={(params) => (
              <TextField
                {...params}
                placeholder={t('organizationUnits:edit.customization.theme.placeholder')}
                helperText={t('organizationUnits:edit.customization.theme.hint')}
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
            {t('organizationUnits:edit.customization.labels.layout', 'Layout')}
          </Typography>
          <Autocomplete
            fullWidth
            disabled={organizationUnit.isReadOnly === true}
            options={layoutOptions}
            getOptionLabel={(option) => (typeof option === 'string' ? option : option.displayName)}
            value={
              layoutOptions.find((layout) => layout.id === (editedOU.layoutId ?? organizationUnit.layoutId)) ?? null
            }
            onChange={(_event, newValue) => onFieldChange('layoutId', newValue?.id ?? '')}
            loading={loadingLayouts}
            renderInput={(params) => (
              <TextField
                {...params}
                placeholder={t('organizationUnits:edit.customization.layout.placeholder', 'Select a layout')}
                helperText={t(
                  'organizationUnits:edit.customization.layout.hint',
                  'The layout applied to this organization unit.',
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
