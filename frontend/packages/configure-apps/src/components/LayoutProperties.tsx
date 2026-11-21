// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  LayoutPresetThumbnail,
  LayoutThumbnail,
  useDesignRoutes,
  type LayoutPresetVariant,
} from '@thunderid/configure-design';
import {useGetLayouts, type LayoutListItem} from '@thunderid/design';
import type {AppContentWidth, AppLanguages, AppNavigationLayout} from '@thunderid/system-apps';
import {
  Box,
  Button,
  Checkbox,
  Divider,
  FormControlLabel,
  Link,
  MenuItem,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
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

export interface LayoutPropertiesProps {
  config: BuilderAppConfig;
  dispatch: (action: AppConfigAction) => void;
  /** Called with the screen a change lays out, which the preview shows. */
  onScreenChange: (screen: AppearanceScreen) => void;
}

const LAYOUT_PRESETS: readonly LayoutPresetVariant[] = ['centered', 'split', 'fullscreen', 'popup'];

const isPreset = (layout: LayoutListItem): boolean => (LAYOUT_PRESETS as readonly string[]).includes(layout.handle);

/** A Gate layout's thumbnail: the preset's own drawing, or one drawn from a layout made in Design. */
function LayoutPicture({layout}: {layout: LayoutListItem | undefined}): JSX.Element {
  if (!layout) {
    return <LayoutPresetThumbnail variant="centered" />;
  }

  return isPreset(layout) ? (
    <LayoutPresetThumbnail variant={layout.handle as LayoutPresetVariant} />
  ) : (
    <LayoutThumbnail layout={layout} />
  );
}

/** A small drawing of an app page, with a top bar or a side bar and its content contained or full width. */
function AppPicture({navigation, width}: {navigation: AppNavigationLayout; width: AppContentWidth}): JSX.Element {
  const isSideBar: boolean = navigation === 'SIDE_BAR';
  const bar: ReactNode = (
    <Box
      sx={{
        bgcolor: 'background.paper',
        borderColor: 'divider',
        display: 'flex',
        flexDirection: isSideBar ? 'column' : 'row',
        gap: 0.5,
        p: 0.75,
        ...(isSideBar ? {borderRight: 1, width: '22%'} : {borderBottom: 1}),
      }}
    >
      <Box sx={{height: 3, width: isSideBar ? '100%' : 14, borderRadius: 1, bgcolor: 'primary.main'}} />
      <Box sx={{height: 3, width: isSideBar ? '70%' : 10, borderRadius: 1, bgcolor: 'divider'}} />
      <Box sx={{height: 3, width: isSideBar ? '70%' : 10, borderRadius: 1, bgcolor: 'divider'}} />
    </Box>
  );

  return (
    <Box sx={{height: '100%', display: 'flex', flexDirection: isSideBar ? 'row' : 'column', bgcolor: 'action.hover'}}>
      {bar}
      <Box sx={{flex: 1, display: 'flex', justifyContent: 'center', p: 1}}>
        <Box
          sx={{
            width: width === 'FULL' ? '100%' : '60%',
            borderRadius: 0.5,
            border: 1,
            borderColor: 'divider',
            bgcolor: 'background.paper',
          }}
        />
      </Box>
    </Box>
  );
}

/** The languages the apps have translations for, as loaded into this Console, plus any already offered. */
const useAvailableLanguages = (offered: string[]): {code: string; name: string}[] => {
  const {i18n} = useTranslation();
  const loaded: string[] = Object.keys((i18n.store?.data as Record<string, unknown> | undefined) ?? {}).filter(
    (code: string) => code !== 'cimode',
  );
  const names = new Intl.DisplayNames([i18n.language || 'en'], {type: 'language'});

  return [...new Set([...loaded, ...offered])].map((code: string) => ({code, name: names.of(code) ?? code}));
};

/** The app's language menu: whether it shows, the languages it lists and the one the app opens in. */
function LanguageMenu({
  dispatch,
  languages,
  onChange,
}: {
  dispatch: LayoutPropertiesProps['dispatch'];
  languages: AppLanguages | undefined;
  onChange: () => void;
}): JSX.Element {
  const {t} = useTranslation('apps');
  const offered: string[] = languages?.offered ?? [];
  const available = useAvailableLanguages(offered);
  const fallback: string = languages?.default ?? offered[0] ?? available[0]?.code ?? 'en-US';

  const set = (next: AppLanguages | undefined): void => {
    dispatch({languages: next, type: 'setLanguages'});
    onChange();
  };

  return (
    <Stack spacing={1.5}>
      <FormControlLabel
        labelPlacement="start"
        sx={{justifyContent: 'space-between', ml: 0, alignItems: 'flex-start'}}
        label={
          <Stack>
            <Typography variant="body2" fontWeight={500}>
              {t('builder.layout.languages', 'Language menu')}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {t('builder.layout.languages.help', "Let admins switch the app's language from the top bar")}
            </Typography>
          </Stack>
        }
        control={
          <Switch
            checked={Boolean(languages?.menu)}
            onChange={() =>
              set(
                languages?.menu
                  ? undefined
                  : {default: fallback, menu: true, offered: offered.length ? offered : [fallback]},
              )
            }
          />
        }
      />
      {languages?.menu && (
        <Stack spacing={1.5} sx={{p: 1.5, border: 1, borderColor: 'divider', borderRadius: 1.5}}>
          <Stack role="group" aria-label={t('builder.layout.languages.offered', 'Languages offered')}>
            <Typography variant="caption" fontWeight={500}>
              {t('builder.layout.languages.offered', 'Languages offered')}
            </Typography>
            {available.map(({code, name}) => {
              const isOffered: boolean = offered.includes(code);

              return (
                <FormControlLabel
                  key={code}
                  label={<Typography variant="body2">{name}</Typography>}
                  control={
                    <Checkbox
                      size="small"
                      checked={isOffered}
                      // The language the app opens in stays offered.
                      disabled={code === languages.default}
                      onChange={() =>
                        set({
                          ...languages,
                          offered: isOffered ? offered.filter((tag: string) => tag !== code) : [...offered, code],
                        })
                      }
                    />
                  }
                />
              );
            })}
          </Stack>
          <TextField
            select
            size="small"
            label={t('builder.layout.languages.default', 'Opens in')}
            value={languages.default ?? ''}
            onChange={(event) => set({...languages, default: event.target.value})}
            helperText={t(
              'builder.layout.languages.defaultHelp',
              "Until an admin picks another. The browser's language wins when it is offered.",
            )}
          >
            {available
              .filter(({code}) => offered.includes(code))
              .map(({code, name}) => (
                <MenuItem key={code} value={code}>
                  {name}
                </MenuItem>
              ))}
          </TextField>
        </Stack>
      )}
      <Typography variant="caption" color="text.secondary">
        {t('builder.layout.languages.gate', 'The sign-in screens show a language menu when their Gate layout has one.')}
      </Typography>
    </Stack>
  );
}

/**
 * The app's layouts. The Gate takes a layout from Design, like it takes a theme; the App's layout is
 * its own: its navigation, page width and language menu.
 */
export default function LayoutProperties({config, dispatch, onScreenChange}: LayoutPropertiesProps): JSX.Element {
  const {t} = useTranslation('apps');
  const routes = useDesignRoutes();
  const {data} = useGetLayouts();
  const layouts: LayoutListItem[] = data?.layouts ?? [];
  const [isChoosing, setIsChoosing] = useState<boolean>(false);
  const gateLayout: LayoutListItem | undefined = layouts.find(
    (layout: LayoutListItem) => layout.id === config.gate?.layoutId,
  );
  // A layout that is gone shows the layouts to choose from, as no layout to show.
  const showPicker: boolean = Boolean(data) && (isChoosing || Boolean(config.gate?.layoutId && !gateLayout));
  const builtIn: string = t('builder.layout.builtIn', 'Built in');

  const pickGateLayout = (layoutId: string | undefined): void => {
    dispatch({layoutId, type: 'setGateLayout'});
    setIsChoosing(false);
    onScreenChange('gate');
  };

  const appChoice = (selected: boolean, label: string, onClick: () => void, picture: ReactNode): JSX.Element => (
    <DesignOption
      label={label}
      selected={selected}
      thumbnail={picture}
      onClick={() => {
        onClick();
        onScreenChange('app');
      }}
    />
  );

  return (
    <Stack spacing={3}>
      <Stack spacing={1.5} component="section" aria-label={t('builder.appearance.screen.gate', 'Gate')}>
        <ScreenHeading
          icon={<LogIn size={16} />}
          title={t('builder.appearance.screen.gate', 'Gate')}
          description={t(
            'builder.layout.gate.help',
            'Where the sign-in card sits. Saved on the application the app signs in with.',
          )}
        />
        <GateNote config={config} />
        {config.applicationId && !showPicker && (
          <Stack spacing={1}>
            <SelectionCard
              thumbnail={<LayoutPicture layout={gateLayout} />}
              title={gateLayout?.displayName ?? t('builder.appearance.defaultLayout', 'Default layout')}
              caption={gateLayout && !isPreset(gateLayout) ? t('builder.layout.fromDesign', 'From Design') : builtIn}
              action={
                <Button size="small" variant="outlined" onClick={() => setIsChoosing(true)}>
                  {t('builder.appearance.theme.change', 'Change')}
                </Button>
              }
            />
            {gateLayout && !isPreset(gateLayout) && (
              <Link
                component={RouterLink}
                to={routes.design.layoutDetail(gateLayout.id)}
                variant="caption"
                fontWeight={500}
              >
                {t('builder.layout.edit', 'Edit {{layout}} in Layouts', {layout: gateLayout.displayName})}
              </Link>
            )}
          </Stack>
        )}
        {config.applicationId && showPicker && (
          <Stack spacing={1}>
            <Box
              role="group"
              aria-label={t('builder.layout.gatePicker', 'Gate layout')}
              sx={{display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 1.5}}
            >
              <DesignOption
                label={t('builder.appearance.defaultLayout', 'Default layout')}
                selected={!config.gate?.layoutId}
                onClick={() => pickGateLayout(undefined)}
                thumbnail={<LayoutPicture layout={undefined} />}
              />
              {layouts.map((layout: LayoutListItem) => (
                <DesignOption
                  key={layout.id}
                  label={layout.displayName}
                  selected={config.gate?.layoutId === layout.id}
                  onClick={() => pickGateLayout(layout.id)}
                  thumbnail={<LayoutPicture layout={layout} />}
                />
              ))}
            </Box>
            {isChoosing && (
              <Button size="small" onClick={() => setIsChoosing(false)} sx={{alignSelf: 'flex-end'}}>
                {t('builder.appearance.theme.cancel', 'Cancel')}
              </Button>
            )}
          </Stack>
        )}
      </Stack>
      <Divider />
      <Stack spacing={2} component="section" aria-label={t('builder.appearance.screen.app', 'App')}>
        <ScreenHeading
          icon={<AppWindow size={16} />}
          title={t('builder.appearance.screen.app', 'App')}
          description={t('builder.layout.app.help', 'How the app arranges its pages. Saved with the app.')}
        />
        <Stack spacing={1} role="group" aria-label={t('builder.appearance.navigation', 'Navigation')}>
          <Typography variant="body2" fontWeight={500}>
            {t('builder.appearance.navigation', 'Navigation')}
          </Typography>
          <Box sx={{display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 1.5}}>
            {(['TOP_BAR', 'SIDE_BAR'] as AppNavigationLayout[]).map((layout: AppNavigationLayout) =>
              appChoice(
                config.navigation.layout === layout,
                layout === 'SIDE_BAR'
                  ? t('builder.appearance.navigation.sideBar', 'Side bar')
                  : t('builder.appearance.navigation.topBar', 'Top bar'),
                () => dispatch({layout, type: 'setLayout'}),
                <AppPicture navigation={layout} width="CONTAINED" />,
              ),
            )}
          </Box>
        </Stack>
        <Stack spacing={1} role="group" aria-label={t('builder.appearance.width', 'Page width')}>
          <Typography variant="body2" fontWeight={500}>
            {t('builder.appearance.width', 'Page width')}
          </Typography>
          <Box sx={{display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 1.5}}>
            {(['CONTAINED', 'FULL'] as AppContentWidth[]).map((width: AppContentWidth) =>
              appChoice(
                config.content.width === width,
                width === 'FULL'
                  ? t('builder.appearance.width.full', 'Full width')
                  : t('builder.appearance.width.contained', 'Contained'),
                () => dispatch({type: 'setContentWidth', width}),
                <AppPicture navigation="TOP_BAR" width={width} />,
              ),
            )}
          </Box>
        </Stack>
        <LanguageMenu dispatch={dispatch} languages={config.languages} onChange={() => onScreenChange('app')} />
      </Stack>
    </Stack>
  );
}
