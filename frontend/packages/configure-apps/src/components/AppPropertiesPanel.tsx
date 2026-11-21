// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {APP_PAGES, type AppPageConfig, type AppPageVisibility, type AppPageDefinition} from '@thunderid/system-apps';
import {FormControlLabel, Stack, Switch, ToggleButton, ToggleButtonGroup, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {AppBuilderSelection, AppearanceScreen} from './AppOutline';
import BrandingProperties from './BrandingProperties';
import LayoutProperties from './LayoutProperties';
import ThemeProperties from './ThemeProperties';
import type {BuilderAppConfig} from '../models/builder';
import type {AppConfigAction} from '../utils/appConfigHistory';

export interface AppPropertiesPanelProps {
  /** The logo of the application the app signs in with, which the app shows by default. */
  applicationLogoUrl?: string;
  config: BuilderAppConfig;
  dispatch: (action: AppConfigAction) => void;
  /** Called with the screen a theme or layout change styles, which the preview shows. */
  onScreenChange: (screen: AppearanceScreen) => void;
  selection: AppBuilderSelection;
}

function Section({children, title}: {children: ReactNode; title: string}): JSX.Element {
  return (
    <Stack spacing={1}>
      <Typography variant="overline" color="text.secondary">
        {title}
      </Typography>
      {children}
    </Stack>
  );
}

function PageProperties({
  config,
  definition,
  dispatch,
}: {
  config: BuilderAppConfig;
  definition: AppPageDefinition;
  dispatch: AppPropertiesPanelProps['dispatch'];
}): JSX.Element {
  const {t} = useTranslation('apps');
  const page: AppPageConfig | undefined = config.pages.find(
    (candidate: AppPageConfig) => candidate.key === definition.key,
  );
  const visibility: AppPageVisibility = page?.visibility ?? 'ALL';

  return (
    <Stack spacing={3}>
      {definition.permission && (
        <Section title={t('builder.properties.visibility', 'Who sees this page')}>
          <ToggleButtonGroup
            exclusive
            fullWidth
            size="small"
            value={visibility}
            onChange={(_event, value: AppPageVisibility | null) => {
              if (value) {
                dispatch({page: definition.key, type: 'setVisibility', visibility: value});
              }
            }}
          >
            <ToggleButton value="ALL">{t('builder.properties.visibility.all', 'Every admin')}</ToggleButton>
            <ToggleButton value="BY_PERMISSION">
              {t('builder.properties.visibility.byPermission', 'By permission')}
            </ToggleButton>
          </ToggleButtonGroup>
          <Typography variant="caption" color="text.secondary">
            {visibility === 'ALL'
              ? t('builder.properties.visibility.allHelp', 'Every admin of the organization unit sees this page.')
              : t(
                  'builder.properties.visibility.byPermissionHelp',
                  'Only admins whose token carries {{permission}} see this page.',
                  {
                    permission: definition.permission,
                  },
                )}
          </Typography>
        </Section>
      )}
      <Section title={t('builder.properties.actions', 'Actions')}>
        {definition.actions.map((action: string) => (
          <FormControlLabel
            key={action}
            labelPlacement="start"
            sx={{justifyContent: 'space-between', ml: 0}}
            label={t(`builder.actions.${definition.key}.${action}`, action)}
            control={
              <Switch
                checked={Boolean(page?.actions[action])}
                onChange={() => dispatch({action, page: definition.key, type: 'toggleAction'})}
              />
            }
          />
        ))}
      </Section>
    </Stack>
  );
}

/**
 * The body of the builder's right panel: the settings of whatever is selected in the outline.
 */
export default function AppPropertiesPanel({selection, ...props}: AppPropertiesPanelProps): JSX.Element {
  const {t} = useTranslation('apps');
  const definition: AppPageDefinition | undefined = APP_PAGES[props.config.type].find(
    (candidate: AppPageDefinition) => candidate.key === selection,
  );

  return (
    <Stack spacing={2.5} component="section" aria-label={t('builder.properties.label', 'Properties')}>
      {definition && <PageProperties config={props.config} definition={definition} dispatch={props.dispatch} />}
      {selection === 'theme' && (
        <ThemeProperties config={props.config} dispatch={props.dispatch} onScreenChange={props.onScreenChange} />
      )}
      {selection === 'layout' && (
        <LayoutProperties config={props.config} dispatch={props.dispatch} onScreenChange={props.onScreenChange} />
      )}
      {selection === 'branding' && (
        <BrandingProperties
          config={props.config}
          dispatch={props.dispatch}
          applicationLogoUrl={props.applicationLogoUrl}
        />
      )}
    </Stack>
  );
}
