// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  LayoutPresetThumbnail,
  LayoutThumbnail,
  ThemeThumbnail,
  type LayoutPresetVariant,
} from '@thunderid/configure-design';
import {useGetLayouts, useGetThemes, type LayoutListItem, type ThemeListItem} from '@thunderid/design';
import {
  APP_PAGES,
  APP_THEME_COLOR,
  DEFAULT_APP_THEME,
  type AppContentWidth,
  type AppNavigationLayout,
  type AppPageConfig,
  type AppPageVisibility,
  type AppTheme,
  type AppThemeColors,
  type AppPageDefinition,
  type AppColorScheme,
} from '@thunderid/system-apps';
import {
  Alert,
  Box,
  Button,
  ButtonBase,
  FormControlLabel,
  Slider,
  Stack,
  Switch,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import {RotateCcw} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {AppBuilderSelection} from './AppOutline';
import ConsoleAttribution from './ConsoleAttribution';
import type {BuilderAppConfig} from '../models/builder';
import type {AppConfigAction} from '../utils/appConfigHistory';

export interface AppPropertiesPanelProps {
  config: BuilderAppConfig;
  dispatch: (action: AppConfigAction) => void;
  /** Called when the App theme switches the color scheme it edits, which the preview follows. */
  onSchemeChange: (scheme: AppColorScheme) => void;
  /** The color scheme the preview shows, and the App theme edits. */
  scheme: AppColorScheme;
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

function DesignOption({
  label,
  onClick,
  selected,
  thumbnail,
}: {
  label: string;
  onClick: () => void;
  selected: boolean;
  thumbnail: ReactNode;
}): JSX.Element {
  return (
    <ButtonBase
      onClick={onClick}
      aria-pressed={selected}
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'stretch',
        borderRadius: 1.5,
        border: 2,
        borderColor: selected ? 'primary.main' : 'divider',
        overflow: 'hidden',
        textAlign: 'start',
        bgcolor: 'background.paper',
        transition: 'border-color 0.15s',
        '&:hover': {borderColor: selected ? 'primary.main' : 'text.disabled'},
      }}
    >
      <Box sx={{aspectRatio: '4 / 3', overflow: 'hidden', pointerEvents: 'none'}}>{thumbnail}</Box>
      <Typography variant="caption" fontWeight={500} noWrap sx={{px: 1, py: 0.75}}>
        {label}
      </Typography>
    </ButtonBase>
  );
}

/** Explains that a Gate setting lives on the application the app signs in with. */
function GateNote({config}: {config: BuilderAppConfig}): JSX.Element {
  const {t} = useTranslation('apps');

  return config.applicationId ? (
    <Typography variant="caption" color="text.secondary">
      {t(
        'builder.appearance.gate.help',
        'Styles the sign-in screens of this app. It is saved on the application the app signs in with.',
      )}
    </Typography>
  ) : (
    <Alert severity="warning">
      {t(
        'builder.appearance.gate.noApplication',
        'The application this app signs in with was not found, so its sign-in screens cannot be styled here.',
      )}
    </Alert>
  );
}

function GateThemeProperties({config, dispatch}: Pick<AppPropertiesPanelProps, 'config' | 'dispatch'>): JSX.Element {
  const {t} = useTranslation('apps');
  const {data: themes} = useGetThemes({limit: 100});
  const defaultLabel: string = t('builder.appearance.defaultTheme', 'Default theme');

  return (
    <Stack spacing={2}>
      <GateNote config={config} />
      {config.applicationId && (
        <Box sx={{display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 1.5}}>
          <DesignOption
            label={defaultLabel}
            selected={!config.gate?.themeId}
            onClick={() => dispatch({themeId: undefined, type: 'setGateTheme'})}
            thumbnail={
              <ThemeThumbnail theme={{displayName: defaultLabel, handle: 'default', id: '', primaryColor: '#3688FF'}} />
            }
          />
          {(themes?.themes ?? []).map((theme: ThemeListItem) => (
            <DesignOption
              key={theme.id}
              label={theme.displayName}
              selected={config.gate?.themeId === theme.id}
              onClick={() => dispatch({themeId: theme.id, type: 'setGateTheme'})}
              thumbnail={<ThemeThumbnail theme={theme} />}
            />
          ))}
        </Box>
      )}
    </Stack>
  );
}

const LAYOUT_PRESETS: readonly LayoutPresetVariant[] = ['centered', 'split', 'fullscreen', 'popup'];

function GateLayoutProperties({config, dispatch}: Pick<AppPropertiesPanelProps, 'config' | 'dispatch'>): JSX.Element {
  const {t} = useTranslation('apps');
  const {data: layouts} = useGetLayouts();

  return (
    <Stack spacing={2}>
      <GateNote config={config} />
      {config.applicationId && (
        <Box sx={{display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 1.5}}>
          <DesignOption
            label={t('builder.appearance.defaultLayout', 'Default layout')}
            selected={!config.gate?.layoutId}
            onClick={() => dispatch({layoutId: undefined, type: 'setGateLayout'})}
            thumbnail={<LayoutPresetThumbnail variant="centered" />}
          />
          {(layouts?.layouts ?? []).map((layout: LayoutListItem) => (
            <DesignOption
              key={layout.id}
              label={layout.displayName}
              selected={config.gate?.layoutId === layout.id}
              onClick={() => dispatch({layoutId: layout.id, type: 'setGateLayout'})}
              thumbnail={
                (LAYOUT_PRESETS as readonly string[]).includes(layout.handle) ? (
                  <LayoutPresetThumbnail variant={layout.handle as LayoutPresetVariant} />
                ) : (
                  <LayoutThumbnail layout={layout} />
                )
              }
            />
          ))}
        </Box>
      )}
    </Stack>
  );
}

const COLOR_FIELDS: {defaultLabel: string; key: keyof AppThemeColors}[] = [
  {defaultLabel: 'Primary', key: 'primary'},
  {defaultLabel: 'Page', key: 'background'},
  {defaultLabel: 'Surfaces', key: 'surface'},
  {defaultLabel: 'Text', key: 'textPrimary'},
  {defaultLabel: 'Secondary text', key: 'textSecondary'},
  {defaultLabel: 'Borders', key: 'border'},
];

/** Font lists the apps can show without loading a font. */
const FONT_OPTIONS: {defaultLabel: string; key: string; value: string | undefined}[] = [
  {defaultLabel: 'Inter', key: 'inter', value: undefined},
  {defaultLabel: 'System', key: 'system', value: "system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif"},
  {defaultLabel: 'Serif', key: 'serif', value: "Georgia, 'Times New Roman', serif"},
  {defaultLabel: 'Monospace', key: 'mono', value: "ui-monospace, 'SF Mono', Menlo, monospace"},
];

const HEX_COLOR = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i;

function ColorField({
  fallback,
  label,
  onChange,
  value,
}: {
  fallback: string;
  label: string;
  onChange: (value: string | undefined) => void;
  value: string | undefined;
}): JSX.Element {
  const {t} = useTranslation('apps');
  const [draft, setDraft] = useState<string>(value ?? '');
  const shown: string = value ?? fallback;

  // Applies typed text once it is a color the server accepts; clearing it goes back to the default.
  const commit = (text: string): void => {
    const trimmed: string = text.trim();
    if (!trimmed) {
      onChange(undefined);
    } else if (APP_THEME_COLOR.test(trimmed)) {
      onChange(trimmed);
    } else {
      setDraft(value ?? '');
    }
  };

  return (
    <Stack direction="row" alignItems="center" spacing={1.25}>
      <Box
        component="label"
        sx={{
          position: 'relative',
          width: 28,
          height: 28,
          flexShrink: 0,
          borderRadius: 1,
          border: 1,
          borderColor: 'divider',
          bgcolor: shown,
          cursor: 'pointer',
          overflow: 'hidden',
        }}
      >
        <input
          type="color"
          aria-label={t('builder.appearance.appTheme.pick', 'Pick {{color}}', {color: label})}
          value={HEX_COLOR.test(shown) ? shown : '#000000'}
          onChange={(event) => onChange(event.target.value)}
          style={{position: 'absolute', inset: 0, opacity: 0, cursor: 'pointer'}}
        />
      </Box>
      <Typography variant="body2" sx={{flex: 1, minWidth: 0}}>
        {label}
      </Typography>
      <TextField
        size="small"
        value={draft}
        placeholder={fallback}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={(event) => commit(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter') commit((event.target as HTMLInputElement).value);
        }}
        slotProps={{htmlInput: {'aria-label': label, spellCheck: false}}}
        sx={{width: 132, '& input': {fontFamily: 'monospace', fontSize: 12}}}
      />
    </Stack>
  );
}

function AppThemeProperties({
  config,
  dispatch,
  onSchemeChange,
  scheme,
}: Pick<AppPropertiesPanelProps, 'config' | 'dispatch'> & {
  onSchemeChange: (scheme: AppColorScheme) => void;
  scheme: AppColorScheme;
}): JSX.Element {
  const {t} = useTranslation('apps');
  const theme: AppTheme = config.appTheme ?? {};
  const colors: AppThemeColors = theme.colorSchemes?.[scheme] ?? {};
  const fallback: Required<AppThemeColors> = DEFAULT_APP_THEME.colorSchemes[scheme];
  const font: string = FONT_OPTIONS.find((option) => option.value === theme.fontFamily)?.key ?? 'inter';

  // Drops values back to the app's own look, so the saved theme only carries what was changed.
  const update = (next: AppTheme): void => {
    const pruned: AppTheme = JSON.parse(JSON.stringify(next)) as AppTheme;
    const isEmpty = (value: unknown): boolean =>
      value === undefined || (typeof value === 'object' && value !== null && Object.keys(value).length === 0);
    (['light', 'dark'] as AppColorScheme[]).forEach((name: AppColorScheme) => {
      if (isEmpty(pruned.colorSchemes?.[name]) && pruned.colorSchemes) delete pruned.colorSchemes[name];
    });
    if (isEmpty(pruned.colorSchemes)) delete pruned.colorSchemes;
    dispatch({appTheme: isEmpty(pruned) ? undefined : pruned, type: 'setAppTheme'});
  };

  const setColor = (key: keyof AppThemeColors, value: string | undefined): void =>
    update({...theme, colorSchemes: {...theme.colorSchemes, [scheme]: {...colors, [key]: value}}});

  return (
    <Stack spacing={3}>
      <Typography variant="caption" color="text.secondary">
        {t(
          'builder.appearance.appTheme.help',
          'Styles the app and the SDK components it is built from. Sign-in screens take the Gate theme.',
        )}
      </Typography>
      <Section title={t('builder.appearance.appTheme.colors', 'Colors')}>
        <ToggleButtonGroup
          exclusive
          fullWidth
          size="small"
          value={scheme}
          onChange={(_event, value: AppColorScheme | null) => {
            if (value) onSchemeChange(value);
          }}
        >
          <ToggleButton value="light">{t('builder.appearance.appTheme.light', 'Light')}</ToggleButton>
          <ToggleButton value="dark">{t('builder.appearance.appTheme.dark', 'Dark')}</ToggleButton>
        </ToggleButtonGroup>
        <Stack spacing={1.25} sx={{pt: 1}}>
          {COLOR_FIELDS.map(({defaultLabel, key}) => (
            <ColorField
              // Starts the typed text over whenever the color changes from elsewhere (picker, undo).
              key={`${scheme}-${key}-${colors[key] ?? ''}`}
              label={t(`builder.appearance.appTheme.color.${key}`, defaultLabel)}
              value={colors[key]}
              fallback={fallback[key]}
              onChange={(value: string | undefined) => setColor(key, value)}
            />
          ))}
        </Stack>
      </Section>
      <Section title={t('builder.appearance.appTheme.shape', 'Corners')}>
        <Stack direction="row" alignItems="center" spacing={2}>
          <Slider
            size="small"
            min={0}
            max={32}
            step={1}
            value={theme.borderRadius ?? DEFAULT_APP_THEME.borderRadius}
            onChange={(_event, value: number | number[]) =>
              update({...theme, borderRadius: value === DEFAULT_APP_THEME.borderRadius ? undefined : (value as number)})
            }
            slotProps={{input: {'aria-label': t('builder.appearance.appTheme.radius', 'Corner radius')}}}
          />
          <Typography variant="body2" sx={{minWidth: 40, textAlign: 'end', fontVariantNumeric: 'tabular-nums'}}>
            {theme.borderRadius ?? DEFAULT_APP_THEME.borderRadius}px
          </Typography>
        </Stack>
      </Section>
      <Section title={t('builder.appearance.appTheme.font', 'Font')}>
        <ToggleButtonGroup
          exclusive
          fullWidth
          size="small"
          value={font}
          onChange={(_event, value: string | null) => {
            if (value) update({...theme, fontFamily: FONT_OPTIONS.find((option) => option.key === value)?.value});
          }}
        >
          {FONT_OPTIONS.map((option) => (
            <ToggleButton key={option.key} value={option.key} sx={{fontFamily: option.value ?? 'inherit'}}>
              {t(`builder.appearance.appTheme.fonts.${option.key}`, option.defaultLabel)}
            </ToggleButton>
          ))}
        </ToggleButtonGroup>
      </Section>
      <Button
        variant="outlined"
        size="small"
        startIcon={<RotateCcw size={14} />}
        disabled={!config.appTheme}
        onClick={() => dispatch({appTheme: undefined, type: 'setAppTheme'})}
        sx={{alignSelf: 'flex-start'}}
      >
        {t('builder.appearance.appTheme.reset', "Back to the app's own look")}
      </Button>
    </Stack>
  );
}

function AppLayoutProperties({config, dispatch}: Pick<AppPropertiesPanelProps, 'config' | 'dispatch'>): JSX.Element {
  const {t} = useTranslation('apps');

  return (
    <Stack spacing={3}>
      <Section title={t('builder.appearance.navigation', 'Navigation')}>
        <ToggleButtonGroup
          exclusive
          fullWidth
          size="small"
          value={config.navigation.layout}
          onChange={(_event, value: AppNavigationLayout | null) => {
            if (value) {
              dispatch({layout: value, type: 'setLayout'});
            }
          }}
        >
          <ToggleButton value="TOP_BAR">{t('builder.appearance.navigation.topBar', 'Top bar')}</ToggleButton>
          <ToggleButton value="SIDE_BAR">{t('builder.appearance.navigation.sideBar', 'Side bar')}</ToggleButton>
        </ToggleButtonGroup>
      </Section>
      <Section title={t('builder.appearance.width', 'Page width')}>
        <ToggleButtonGroup
          exclusive
          fullWidth
          size="small"
          value={config.content.width}
          onChange={(_event, value: AppContentWidth | null) => {
            if (value) {
              dispatch({type: 'setContentWidth', width: value});
            }
          }}
        >
          <ToggleButton value="CONTAINED">{t('builder.appearance.width.contained', 'Contained')}</ToggleButton>
          <ToggleButton value="FULL">{t('builder.appearance.width.full', 'Full width')}</ToggleButton>
        </ToggleButtonGroup>
      </Section>
      <Section title={t('builder.appearance.sections.attribution', 'Attribution')}>
        <FormControlLabel
          labelPlacement="start"
          sx={{justifyContent: 'space-between', ml: 0}}
          label={
            <Stack direction="row" alignItems="center" spacing={1}>
              <Typography variant="body2">{t('builder.appearance.attribution.show', 'Show')}</Typography>
              <ConsoleAttribution />
            </Stack>
          }
          control={
            <Switch
              checked={config.branding.showSecuredBy}
              onChange={() => dispatch({type: 'toggleSecuredBy'})}
              slotProps={{input: {'aria-label': t('builder.appearance.securedBy', 'Show "Secured by ThunderID"')}}}
            />
          }
        />
      </Section>
    </Stack>
  );
}

/**
 * The body of the builder's right panel: the settings of whatever is selected in the outline.
 */
export default function AppPropertiesPanel({
  config,
  dispatch,
  onSchemeChange,
  scheme,
  selection,
}: AppPropertiesPanelProps): JSX.Element {
  const {t} = useTranslation('apps');
  const definition: AppPageDefinition | undefined = APP_PAGES[config.type].find(
    (candidate: AppPageDefinition) => candidate.key === selection,
  );

  return (
    <Stack spacing={2.5} component="section" aria-label={t('builder.properties.label', 'Properties')}>
      {definition && <PageProperties config={config} definition={definition} dispatch={dispatch} />}
      {selection === 'gateTheme' && <GateThemeProperties config={config} dispatch={dispatch} />}
      {selection === 'gateLayout' && <GateLayoutProperties config={config} dispatch={dispatch} />}
      {selection === 'appTheme' && (
        <AppThemeProperties config={config} dispatch={dispatch} scheme={scheme} onSchemeChange={onSchemeChange} />
      )}
      {selection === 'appLayout' && <AppLayoutProperties config={config} dispatch={dispatch} />}
    </Stack>
  );
}
