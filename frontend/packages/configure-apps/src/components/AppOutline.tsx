// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useGetLayouts, useGetThemes, type LayoutListItem, type ThemeListItem} from '@thunderid/design';
import {APP_PAGES, type AppPageDefinition, AppPageConfig, AppPageKey} from '@thunderid/system-apps';
import {Box, ButtonBase, Stack, Switch, Typography} from '@wso2/oxygen-ui';
import {ChevronRight, LayoutTemplate, Palette} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {BuilderAppConfig} from '../models/builder';

/**
 * What the builder has selected: one page, or a part of the app's appearance.
 *
 * @public
 */
export type AppBuilderSelection = AppPageKey | AppearanceSection;

/**
 * The parts of the app's appearance the builder edits, each with its own properties. The Gate
 * parts style the app's sign-in screens; the App parts style the app and its SDK components.
 *
 * @public
 */
export type AppearanceSection = 'gateTheme' | 'appTheme' | 'gateLayout' | 'appLayout';

type AppearanceGroup = 'theme' | 'layout';

/**
 * The appearance groups of the outline, each with its Gate and its App path.
 */
const APPEARANCE_GROUPS: {key: AppearanceGroup; sections: AppearanceSection[]}[] = [
  {key: 'theme', sections: ['gateTheme', 'appTheme']},
  {key: 'layout', sections: ['gateLayout', 'appLayout']},
];

const APPEARANCE_ICONS: Record<AppearanceGroup, JSX.Element> = {
  layout: <LayoutTemplate size={16} />,
  theme: <Palette size={16} />,
};

export interface AppOutlineProps {
  config: BuilderAppConfig;
  onSelect: (selection: AppBuilderSelection) => void;
  onTogglePage: (page: AppPageKey) => void;
  selection: AppBuilderSelection;
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
 * The builder's left panel: the app's pages, each with an on/off switch, and its appearance.
 */
export default function AppOutline({config, onSelect, onTogglePage, selection}: AppOutlineProps): JSX.Element {
  const {t} = useTranslation('apps');
  const {data: themes} = useGetThemes({limit: 100});
  const {data: layouts} = useGetLayouts();
  const gateThemeName: string | undefined = themes?.themes.find(
    (theme: ThemeListItem) => theme.id === config.gate?.themeId,
  )?.displayName;
  const gateLayoutName: string | undefined = layouts?.layouts.find(
    (layout: LayoutListItem) => layout.id === config.gate?.layoutId,
  )?.displayName;
  // What each appearance section is set to, shown under its name.
  const appearanceSummary: Record<AppearanceSection, string> = {
    appLayout: [
      config.navigation.layout === 'SIDE_BAR'
        ? t('builder.appearance.navigation.sideBar', 'Side bar')
        : t('builder.appearance.navigation.topBar', 'Top bar'),
      config.content.width === 'FULL'
        ? t('builder.appearance.width.full', 'Full width')
        : t('builder.appearance.width.contained', 'Contained'),
    ].join(' · '),
    appTheme: config.appTheme
      ? t('builder.appearance.appTheme.custom', 'Customized')
      : t('builder.appearance.appTheme.default', "The app's own look"),
    gateLayout: gateLayoutName ?? t('builder.appearance.defaultLayout', 'Default layout'),
    gateTheme: gateThemeName ?? t('builder.appearance.defaultTheme', 'Default theme'),
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
      {APPEARANCE_GROUPS.map(({key, sections}) => (
        <Stack key={key} spacing={0.75} role="group" aria-label={t(`builder.appearance.groups.${key}`, key)}>
          <Stack direction="row" alignItems="center" spacing={1} sx={{px: 0.5, color: 'text.secondary'}}>
            {APPEARANCE_ICONS[key]}
            <Typography variant="body2" fontWeight={600} color="text.primary">
              {t(`builder.appearance.groups.${key}`, key)}
            </Typography>
          </Stack>
          {sections.map((section: AppearanceSection) => (
            <OutlineItem
              key={section}
              selected={selection === section}
              onClick={() => onSelect(section)}
              label={
                <Stack direction="row" alignItems="center" spacing={1.25} sx={{width: '100%'}}>
                  <Box sx={{flex: 1, minWidth: 0}}>
                    <Typography variant="body2" fontWeight={500}>
                      {t(`builder.appearance.sections.${section}`, section)}
                    </Typography>
                    <Typography variant="caption" color="text.secondary" noWrap component="p">
                      {appearanceSummary[section]}
                    </Typography>
                  </Box>
                  <ChevronRight size={16} />
                </Stack>
              }
            />
          ))}
        </Stack>
      ))}
    </Stack>
  );
}
