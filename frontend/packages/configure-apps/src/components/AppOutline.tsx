// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {APP_PAGES, type AppPageDefinition, AppPageConfig, AppPageKey} from '@thunderid/system-apps';
import {Box, ButtonBase, Stack, Switch, Typography} from '@wso2/oxygen-ui';
import {ChevronRight, Image, LayoutTemplate, Palette} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {BuilderAppConfig} from '../models/builder';

/**
 * What the builder has selected: one page, or the app's theme or its layout.
 *
 * @public
 */
export type AppBuilderSelection = AppPageKey | AppearanceSection;

/**
 * The parts of the app's appearance the builder edits, each for the Gate and for the App.
 *
 * @public
 */
export type AppearanceSection = 'theme' | 'layout' | 'branding';

const APPEARANCE_SECTIONS: {defaultLabel: string; icon: JSX.Element; key: AppearanceSection}[] = [
  {defaultLabel: 'Theme', icon: <Palette size={16} />, key: 'theme'},
  {defaultLabel: 'Layout', icon: <LayoutTemplate size={16} />, key: 'layout'},
  {defaultLabel: 'Branding', icon: <Image size={16} />, key: 'branding'},
];

/**
 * The screens whose theme and layout the builder edits: the Gate's sign-in screens, or the app
 * itself and the SDK components it is built from.
 *
 * @public
 */
export type AppearanceScreen = 'gate' | 'app';

export interface AppOutlineProps {
  config: BuilderAppConfig;
  onSelect: (selection: AppBuilderSelection) => void;
  onTogglePage: (page: AppPageKey) => void;
  selection: AppBuilderSelection;
  /** Whether the app takes the Gate theme instead of a theme of its own. */
  themeShared: boolean;
}

function OutlineItem({
  children = undefined,
  label,
  onClick,
  selected,
}: {
  children?: ReactNode;
  label: ReactNode;
  onClick: () => void;
  selected: boolean;
}): JSX.Element {
  return (
    <Stack
      direction="row"
      alignItems="center"
      spacing={1}
      sx={{
        px: 1.5,
        py: 1,
        borderRadius: 1,
        border: 1,
        borderColor: selected ? 'primary.main' : 'divider',
        bgcolor: selected ? 'action.selected' : 'background.paper',
      }}
    >
      <ButtonBase
        onClick={onClick}
        aria-pressed={selected}
        sx={{flex: 1, justifyContent: 'flex-start', textAlign: 'start', borderRadius: 1}}
      >
        {label}
      </ButtonBase>
      {children}
    </Stack>
  );
}

/**
 * The builder's left panel: the app's pages, each with an on/off switch, then its theme and its layout.
 */
export default function AppOutline({
  config,
  onSelect,
  onTogglePage,
  selection,
  themeShared,
}: AppOutlineProps): JSX.Element {
  const {t} = useTranslation('apps');
  // What each part of the appearance is set to, shown under its name.
  const appearanceSummary: Record<AppearanceSection, string> = {
    branding: [
      config.branding.logo
        ? t('builder.branding.summary.ownLogo', 'Own logo')
        : t('builder.branding.summary.applicationLogo', 'Application logo'),
      config.branding.favicon
        ? t('builder.branding.summary.ownIcon', 'own icon')
        : t('builder.branding.summary.appIcon', "app's own icon"),
    ].join(' · '),
    layout: [
      config.navigation.layout === 'SIDE_BAR'
        ? t('builder.appearance.navigation.sideBar', 'Side bar')
        : t('builder.appearance.navigation.topBar', 'Top bar'),
      config.content.width === 'FULL'
        ? t('builder.appearance.width.full', 'Full width')
        : t('builder.appearance.width.contained', 'Contained'),
      ...(config.languages?.menu ? [t('builder.layout.languages.summary', 'Language menu')] : []),
    ].join(' · '),
    theme: themeShared
      ? t('builder.appearance.summary.shared', 'One theme for Gate and App')
      : t('builder.appearance.summary.separate', 'Separate Gate and App themes'),
  };

  return (
    <Stack spacing={1.5}>
      <Typography variant="overline" color="text.secondary">
        {t('builder.outline.pages', 'Pages')}
      </Typography>
      {APP_PAGES[config.type].map((definition: AppPageDefinition) => {
        const page: AppPageConfig | undefined = config.pages.find(
          (candidate: AppPageConfig) => candidate.key === definition.key,
        );
        const enabled: boolean = page?.enabled ?? false;
        let status: string = t('builder.outline.hidden', 'Hidden from the app');

        if (enabled && !definition.permission) {
          status = t('builder.outline.everyUser', 'Every user');
        } else if (enabled) {
          status =
            page?.visibility === 'BY_PERMISSION'
              ? t('builder.outline.byPermission', 'Needs {{permission}}', {permission: definition.permission})
              : t('builder.outline.everyAdmin', 'Every admin');
        }

        return (
          <OutlineItem
            key={definition.key}
            selected={selection === definition.key}
            onClick={() => onSelect(definition.key)}
            label={
              <Box sx={{opacity: enabled ? 1 : 0.6}}>
                <Typography variant="body2" fontWeight={500}>
                  {t(`builder.pages.${definition.key}`, definition.key)}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {status}
                </Typography>
              </Box>
            }
          >
            <Switch
              checked={enabled}
              onChange={() => onTogglePage(definition.key)}
              slotProps={{
                input: {
                  'aria-label': t('builder.outline.toggle', 'Show {{page}} in the app', {
                    page: t(`builder.pages.${definition.key}`, definition.key),
                  }),
                },
              }}
            />
          </OutlineItem>
        );
      })}
      <Typography variant="overline" color="text.secondary" sx={{pt: 1}}>
        {t('builder.outline.customize', 'Customize')}
      </Typography>
      {APPEARANCE_SECTIONS.map(({defaultLabel, icon, key}) => (
        <OutlineItem
          key={key}
          selected={selection === key}
          onClick={() => onSelect(key)}
          label={
            <Stack direction="row" alignItems="center" spacing={1.25} sx={{width: '100%'}}>
              {icon}
              <Box sx={{flex: 1, minWidth: 0}}>
                <Typography variant="body2" fontWeight={500}>
                  {t(`builder.appearance.sections.${key}`, defaultLabel)}
                </Typography>
                <Typography variant="caption" color="text.secondary" noWrap component="p">
                  {appearanceSummary[key]}
                </Typography>
              </Box>
              <ChevronRight size={16} />
            </Stack>
          }
        />
      ))}
    </Stack>
  );
}
