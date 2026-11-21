// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ThemeThumbnail, useDesignRoutes} from '@thunderid/configure-design';
import {useGetThemes, type ThemeListItem} from '@thunderid/design';
import {Box, Button, Chip, Divider, FormControlLabel, Link, Stack, Switch, Typography} from '@wso2/oxygen-ui';
import {AppWindow, LogIn} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Link as RouterLink} from 'react-router';
import type {AppearanceScreen} from './AppOutline';
import DesignOption from './DesignOption';
import GateNote from './GateNote';
import ScreenHeading from './ScreenHeading';
import SelectionCard from './SelectionCard';
import type {BuilderAppConfig} from '../models/builder';
import type {AppConfigAction} from '../utils/appConfigHistory';

export interface ThemePropertiesProps {
  config: BuilderAppConfig;
  dispatch: (action: AppConfigAction) => void;
  /** Called with the screen a change styles, which the preview shows. */
  onScreenChange: (screen: AppearanceScreen) => void;
}

/** A theme in use, as a thumbnail with its name. A linked theme comes from elsewhere and is not chosen here. */
function ThemeCard({
  action = undefined,
  caption,
  linked = false,
  theme,
}: {
  action?: ReactNode;
  caption: string;
  linked?: boolean;
  theme: ThemeListItem;
}): JSX.Element {
  return (
    <SelectionCard
      action={action}
      caption={caption}
      linked={linked}
      thumbnail={<ThemeThumbnail theme={theme} />}
      title={theme.displayName}
    />
  );
}

/** The themes to choose from, with the current one marked. */
function ThemePicker({
  defaultTheme = undefined,
  label,
  onCancel = undefined,
  onPick,
  selectedId,
  themes,
}: {
  /** A choice that sets no theme, shown first. */
  defaultTheme?: ThemeListItem;
  label: string;
  onCancel?: () => void;
  onPick: (themeId: string | undefined) => void;
  selectedId: string | undefined;
  themes: ThemeListItem[];
}): JSX.Element {
  const {t} = useTranslation('apps');

  return (
    <Stack spacing={1}>
      <Box
        role="group"
        aria-label={label}
        sx={{display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 1.5}}
      >
        {defaultTheme && (
          <DesignOption
            label={defaultTheme.displayName}
            selected={!selectedId}
            onClick={() => onPick(undefined)}
            thumbnail={<ThemeThumbnail theme={defaultTheme} />}
          />
        )}
        {themes.map((theme: ThemeListItem) => (
          <DesignOption
            key={theme.id}
            label={theme.displayName}
            selected={selectedId === theme.id}
            onClick={() => onPick(theme.id)}
            thumbnail={<ThemeThumbnail theme={theme} />}
          />
        ))}
      </Box>
      {onCancel && (
        <Button size="small" onClick={onCancel} sx={{alignSelf: 'flex-end'}}>
          {t('builder.appearance.theme.cancel', 'Cancel')}
        </Button>
      )}
    </Stack>
  );
}

/**
 * The app's themes. The Gate always has its own; the App either follows the Gate or takes a theme
 * of its own from the same Design themes. Themes are picked here and edited in Design.
 */
export default function ThemeProperties({config, dispatch, onScreenChange}: ThemePropertiesProps): JSX.Element {
  const {t} = useTranslation('apps');
  const routes = useDesignRoutes();
  const {data} = useGetThemes({limit: 100});
  const themes: ThemeListItem[] = data?.themes ?? [];
  const [choosing, setChoosing] = useState<AppearanceScreen | null>(null);
  // Own theme was asked for while there was no Gate theme to start from: the picker waits for one.
  const [ownPending, setOwnPending] = useState<boolean>(false);
  const defaultTheme: ThemeListItem = {
    displayName: t('builder.appearance.defaultTheme', 'Default theme'),
    handle: 'default',
    id: '',
    primaryColor: '#3688FF',
  };
  const gateTheme: ThemeListItem | undefined = themes.find((theme: ThemeListItem) => theme.id === config.gate?.themeId);
  const appTheme: ThemeListItem | undefined = themes.find((theme: ThemeListItem) => theme.id === config.themeId);
  const appOwn: boolean = Boolean(config.themeId) || ownPending;
  // Without a theme to show, a screen shows the themes to choose from.
  const showGatePicker: boolean = Boolean(data) && (choosing === 'gate' || !gateTheme);
  const showAppPicker: boolean = Boolean(data) && appOwn && (choosing === 'app' || !appTheme);

  const pickGateTheme = (themeId: string | undefined): void => {
    dispatch({themeId, type: 'setGateTheme'});
    setChoosing(null);
    onScreenChange('gate');
  };

  const pickAppTheme = (themeId: string | undefined): void => {
    dispatch({themeId, type: 'setAppTheme'});
    setChoosing(null);
    setOwnPending(false);
    onScreenChange('app');
  };

  const followGate = (): void => {
    if (config.themeId) {
      dispatch({themeId: undefined, type: 'setAppTheme'});
    }
    setOwnPending(false);
    setChoosing((current) => (current === 'app' ? null : current));
    onScreenChange('app');
  };

  // An own theme starts from the Gate's, so the app looks the same until another is picked.
  const takeOwnTheme = (): void => {
    onScreenChange('app');
    if (config.gate?.themeId) {
      dispatch({themeId: config.gate.themeId, type: 'setAppTheme'});
    } else {
      setOwnPending(true);
      setChoosing('app');
    }
  };

  return (
    <Stack spacing={3}>
      <Stack spacing={1.5} component="section" aria-label={t('builder.appearance.screen.gate', 'Gate')}>
        <ScreenHeading
          icon={<LogIn size={16} />}
          title={t('builder.appearance.screen.gate', 'Gate')}
          description={t(
            'builder.appearance.gate.help',
            'The sign-in screens of this app. They are saved on the application the app signs in with.',
          )}
        />
        <GateNote config={config} />
        {config.applicationId && gateTheme && !showGatePicker && (
          <ThemeCard
            theme={gateTheme}
            caption={
              appOwn
                ? t('builder.appearance.theme.gateOnly', 'Used by the Gate')
                : t('builder.appearance.theme.shared', 'Used by the Gate and the App')
            }
            action={
              <Button size="small" variant="outlined" onClick={() => setChoosing('gate')}>
                {t('builder.appearance.theme.change', 'Change')}
              </Button>
            }
          />
        )}
        {config.applicationId && showGatePicker && (
          <Stack spacing={1}>
            {!gateTheme && (
              <Typography variant="body2">
                {t(
                  'builder.appearance.theme.gateEmpty',
                  'Pick a theme for the sign-in screens. The app uses it too while it follows the Gate.',
                )}
              </Typography>
            )}
            <ThemePicker
              label={t('builder.appearance.theme.gatePicker', 'Gate theme')}
              themes={themes}
              defaultTheme={defaultTheme}
              selectedId={config.gate?.themeId}
              onPick={pickGateTheme}
              onCancel={gateTheme ? () => setChoosing(null) : undefined}
            />
          </Stack>
        )}
      </Stack>
      <Divider />
      <Stack spacing={1.5} component="section" aria-label={t('builder.appearance.screen.app', 'App')}>
        <ScreenHeading
          icon={<AppWindow size={16} />}
          title={t('builder.appearance.screen.app', 'App')}
          description={t('builder.appearance.app.help', 'The app and the SDK components it is built from.')}
        />
        <FormControlLabel
          labelPlacement="start"
          sx={{justifyContent: 'space-between', ml: 0}}
          label={t('builder.appearance.theme.sameAsGate', 'Same theme as the Gate')}
          control={<Switch checked={!appOwn} onChange={() => (appOwn ? followGate() : takeOwnTheme())} />}
        />
        {!appOwn && (
          <Stack spacing={1}>
            <ThemeCard
              linked
              theme={gateTheme ?? defaultTheme}
              caption={
                gateTheme
                  ? t('builder.appearance.theme.follows', 'Follows the Gate theme')
                  : t('builder.appearance.theme.followsDefault', 'Until the Gate has a theme')
              }
              action={
                <Chip
                  size="small"
                  variant="outlined"
                  color="primary"
                  label={t('builder.appearance.theme.linked', 'Linked')}
                />
              }
            />
            <Typography variant="caption" color="text.secondary">
              {t('builder.appearance.theme.linkedHelp', 'Change the Gate theme and the app changes with it.')}
            </Typography>
          </Stack>
        )}
        {appOwn && appTheme && !showAppPicker && (
          <Stack spacing={1}>
            <ThemeCard
              theme={appTheme}
              caption={t('builder.appearance.theme.appOnly', 'Used by the App')}
              action={
                <Button size="small" variant="outlined" onClick={() => setChoosing('app')}>
                  {t('builder.appearance.theme.change', 'Change')}
                </Button>
              }
            />
            <Link component={RouterLink} to={routes.design.themeDetail(appTheme.id)} variant="caption" fontWeight={500}>
              {t('builder.appearance.theme.edit', 'Edit {{theme}} in Themes', {theme: appTheme.displayName})}
            </Link>
          </Stack>
        )}
        {showAppPicker && (
          <ThemePicker
            label={t('builder.appearance.theme.appPicker', 'App theme')}
            themes={themes}
            selectedId={config.themeId}
            onPick={pickAppTheme}
            onCancel={() => (appTheme ? setChoosing(null) : followGate())}
          />
        )}
      </Stack>
      <Divider />
      <Stack direction="row" alignItems="center" justifyContent="space-between" spacing={1}>
        <Typography variant="caption" color="text.secondary">
          {t('builder.appearance.theme.managedIn', 'Themes are created and edited in Design.')}
        </Typography>
        <Link
          component={RouterLink}
          to={routes.design.list()}
          variant="caption"
          fontWeight={500}
          sx={{whiteSpace: 'nowrap'}}
        >
          {t('builder.appearance.theme.open', 'Open Themes')}
        </Link>
      </Stack>
    </Stack>
  );
}
