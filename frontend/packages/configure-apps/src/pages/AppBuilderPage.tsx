// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {BuilderLayout, BuilderStaticPanel, QueryErrorNotice} from '@thunderid/components';
import {GatePreview, ElementInspector, PreviewFrame, PreviewToolbar, type Viewport} from '@thunderid/configure-design';
import {useToast} from '@thunderid/contexts';
import {DefaultTheme, useGetTheme, type ColorSchemeOption, type Theme} from '@thunderid/design';
import {APP_PAGES, DEFAULT_APP_CONFIGS, useGetAppConfig} from '@thunderid/system-apps';
import type {AppType, AppColorScheme} from '@thunderid/system-apps';
import {
  Alert,
  Badge,
  Box,
  Button,
  CircularProgress,
  IconButton,
  Stack,
  Tooltip,
  Typography,
  useColorScheme,
} from '@wso2/oxygen-ui';
import {Crosshair, Redo2, Save, Undo2} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useReducer, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetGateDesign from '../api/useGetGateDesign';
import useUpdateAppConfig from '../api/useUpdateAppConfig';
import useUpdateGateDesign from '../api/useUpdateGateDesign';
import AppOutline, {type AppBuilderSelection} from '../components/AppOutline';
import AppPreview from '../components/AppPreview';
import AppPropertiesPanel from '../components/AppPropertiesPanel';
import {PREVIEW_COMPONENT_ATTRIBUTE} from '../constants/app-preview';
import SIGN_IN_APPLICATION_IDS from '../constants/sign-in-applications';
import type {AppGateDesign, BuilderAppConfig} from '../models/builder';
import {
  appConfigHistoryReducer,
  createAppConfigHistory,
  hasUnsavedChanges,
  type AppConfigHistory,
} from '../utils/appConfigHistory';
import propertiesTitle from '../utils/propertiesTitle';

const ZOOM_STEPS: number[] = [25, 50, 75, 100, 125, 150];

/** The least the app needs to lay out like a browser window before the preview shrinks it. */
const MIN_PREVIEW_HEIGHT = 600;

/**
 * The words the builder uses for each app, as translation keys with their English defaults.
 */
const APP_COPY: Record<AppType, Record<'loading' | 'saved' | 'saveError' | 'subtitle' | 'title', [string, string]>> = {
  ACCOUNT: {
    loading: ['builder.account.loading', 'Loading the Account App'],
    saved: ['builder.account.saved', 'Account App saved.'],
    saveError: ['builder.account.saveError', 'Could not save the Account App. Please try again.'],
    subtitle: ['builder.account.subtitle', 'For the people who sign in to your apps. Data planes serve it.'],
    title: ['builder.account.title', 'Account App'],
  },
  ADMIN: {
    loading: ['builder.admin.loading', 'Loading the Admin App'],
    saved: ['builder.admin.saved', 'Admin App saved.'],
    saveError: ['builder.admin.saveError', 'Could not save the Admin App. Please try again.'],
    subtitle: ['builder.admin.subtitle', 'For the admins of each organization unit. Data planes serve it.'],
    title: ['builder.admin.title', 'Admin App'],
  },
};

export interface AppBuilderPageProps {
  type: AppType;
}

/** Whether a Gate design differs from another, treating an unset value like an absent one. */
const isSameGate = (a: AppGateDesign | undefined, b: AppGateDesign | undefined): boolean =>
  (a?.themeId ?? '') === (b?.themeId ?? '') && (a?.layoutId ?? '') === (b?.layoutId ?? '');

/** An app configuration without the Gate design, which is saved on the application instead. */
const withoutGate = ({applicationId, gate, ...config}: BuilderAppConfig): BuilderAppConfig => {
  void applicationId;
  void gate;
  return config;
};

/**
 * Builds an app the data planes serve: choose its pages, what each page lets its users do, and how
 * the app looks, with a live preview. The app's look has two paths: the Gate theme and layout of its
 * sign-in screens, saved on the application it signs in with, and the App theme and layout of the
 * app itself. Saving stores the configuration on the control plane, and data planes pick it up
 * through the usual configuration sync.
 *
 * @public
 */
export default function AppBuilderPage({type}: AppBuilderPageProps): JSX.Element {
  const {t} = useTranslation('apps');
  const copy = APP_COPY[type];
  const applicationId: string = SIGN_IN_APPLICATION_IDS[type];
  const {data: savedConfig, error, isLoading, refetch} = useGetAppConfig(type);
  const {data: savedGate, isError: gateFailed} = useGetGateDesign(applicationId);
  const save = useUpdateAppConfig();
  const saveGate = useUpdateGateDesign();
  const {showToast} = useToast();
  const [history, dispatch] = useReducer(
    appConfigHistoryReducer,
    DEFAULT_APP_CONFIGS[type],
    (config): AppConfigHistory => createAppConfigHistory(config),
  );
  const [selection, setSelection] = useState<AppBuilderSelection>(APP_PAGES[type][0].key);
  const [viewport, setViewport] = useState<Viewport>('desktop');
  const [zoom, setZoom] = useState<number>(75);
  const [inspectorEnabled, setInspectorEnabled] = useState<boolean>(true);
  const stageRef = useRef<HTMLDivElement>(null);
  const {mode, systemMode} = useColorScheme();
  const isDark: boolean = (mode === 'system' ? systemMode : mode) === 'dark';
  const [previewScheme, setPreviewScheme] = useState<ColorSchemeOption>('system');
  const [effectiveScheme, setEffectiveScheme] = useState<AppColorScheme>(isDark ? 'dark' : 'light');
  const isLoaded = useRef<boolean>(false);

  // Starts editing once the configuration, and the Gate design of its application, have loaded.
  // Later refetches do not reset the working copy; a save resets it with what was saved.
  useEffect(() => {
    if (isLoaded.current || !savedConfig || (!savedGate && !gateFailed)) {
      return;
    }
    isLoaded.current = true;
    // When the application no longer exists, its Gate design cannot be edited here.
    dispatch({
      config: {...savedConfig, applicationId: gateFailed ? undefined : applicationId, gate: savedGate ?? {}},
      type: 'reset',
    });
  }, [applicationId, savedConfig, savedGate, gateFailed]);

  const isDirty: boolean = hasUnsavedChanges(history);
  const config = history.present;
  const isGateSelection: boolean = selection === 'gateTheme' || selection === 'gateLayout';
  const {data: gateTheme} = useGetTheme(isGateSelection ? (config.gate?.themeId ?? '') : '');

  const handleSave = async (): Promise<void> => {
    const appChanged: boolean = JSON.stringify(withoutGate(config)) !== JSON.stringify(withoutGate(history.saved));
    const gateChanged: boolean = Boolean(config.applicationId) && !isSameGate(config.gate, history.saved.gate);

    try {
      const [savedApp, savedGateDesign] = await Promise.all([
        appChanged ? save.mutateAsync(config) : Promise.resolve(withoutGate(history.saved)),
        gateChanged
          ? saveGate.mutateAsync({applicationId: config.applicationId!, gate: config.gate ?? {}})
          : Promise.resolve(history.saved.gate),
      ]);
      dispatch({
        config: {...savedApp, applicationId: config.applicationId, gate: savedGateDesign ?? {}},
        type: 'reset',
      });
      if (!appChanged) {
        showToast(t(...copy.saved), 'success');
      }
    } catch {
      // The failed save shows its own error below the top bar.
    }
  };

  if (error) {
    return <QueryErrorNotice error={error} t={t} onRetry={() => void refetch()} />;
  }

  if (isLoading) {
    return (
      <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
        <CircularProgress aria-label={t(...copy.loading)} />
      </Box>
    );
  }

  return (
    <Box
      sx={{
        width: '100%',
        height: 'inherit',
        display: 'flex',
        flexDirection: 'column',
        bgcolor: 'var(--flow-builder-background-color)',
        '[data-color-scheme="dark"] &': {bgcolor: 'var(--flow-builder-background-color-dark)'},
      }}
    >
      {/* Top bar: the app on the left, builder toolbar in the middle, save on the right. */}
      <Box sx={{display: 'flex', alignItems: 'center', px: 2, py: 1, flexShrink: 0}}>
        <Box sx={{flex: 1, minWidth: 0}}>
          <Typography variant="h6" component="h1">
            {t(...copy.title)}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {t(...copy.subtitle)}
          </Typography>
        </Box>
        <Box role="toolbar" aria-label={t('builder.toolbar.label', 'Builder')} sx={{display: 'flex'}}>
          <PreviewToolbar
            viewport={viewport}
            setViewport={setViewport}
            zoom={zoom}
            setZoom={setZoom}
            zoomIdx={ZOOM_STEPS.indexOf(zoom)}
            previewColorScheme={previewScheme}
            setPreviewColorScheme={setPreviewScheme}
            onEffectiveSchemeChange={setEffectiveScheme}
            startContent={
              <Stack direction="row" spacing={0.25}>
                <Tooltip title={t('builder.toolbar.undo', 'Undo')}>
                  <span>
                    <IconButton
                      size="small"
                      aria-label={t('builder.toolbar.undo', 'Undo')}
                      disabled={history.past.length === 0}
                      onClick={() => dispatch({type: 'undo'})}
                    >
                      <Undo2 size={14} />
                    </IconButton>
                  </span>
                </Tooltip>
                <Tooltip title={t('builder.toolbar.redo', 'Redo')}>
                  <span>
                    <IconButton
                      size="small"
                      aria-label={t('builder.toolbar.redo', 'Redo')}
                      disabled={history.future.length === 0}
                      onClick={() => dispatch({type: 'redo'})}
                    >
                      <Redo2 size={14} />
                    </IconButton>
                  </span>
                </Tooltip>
              </Stack>
            }
            extraContent={
              <Tooltip
                title={
                  inspectorEnabled
                    ? t('builder.toolbar.inspector.disable', 'Stop showing components')
                    : t('builder.toolbar.inspector.enable', 'Show components on hover')
                }
              >
                <IconButton
                  size="small"
                  aria-label={t('builder.toolbar.inspector.label', 'Component inspector')}
                  aria-pressed={inspectorEnabled}
                  onClick={() => setInspectorEnabled((enabled: boolean) => !enabled)}
                  sx={{
                    bgcolor: inspectorEnabled ? 'primary.main' : 'transparent',
                    color: inspectorEnabled ? 'primary.contrastText' : 'text.secondary',
                    '&:hover': {bgcolor: inspectorEnabled ? 'primary.dark' : 'action.hover'},
                  }}
                >
                  <Crosshair size={14} />
                </IconButton>
              </Tooltip>
            }
          />
        </Box>
        <Box sx={{flex: 1, display: 'flex', justifyContent: 'flex-end'}}>
          <Tooltip title={isDirty ? t('builder.save.unsaved', 'You have unsaved changes') : ''}>
            <span>
              <Badge color="primary" variant="dot" invisible={!isDirty} data-testid="app-builder-dirty-indicator">
                <Button
                  variant="contained"
                  startIcon={<Save size={18} />}
                  disabled={!isDirty || save.isPending || saveGate.isPending}
                  onClick={() => void handleSave()}
                >
                  {t('builder.save.action', 'Save')}
                </Button>
              </Badge>
            </span>
          </Tooltip>
        </Box>
      </Box>
      {save.error && (
        <Alert severity="error" sx={{mx: 2, mb: 1}} onClose={() => save.reset()}>
          {t(...copy.saveError)}
        </Alert>
      )}
      {saveGate.error && (
        <Alert severity="error" sx={{mx: 2, mb: 1}} onClose={() => saveGate.reset()}>
          {t(
            'builder.save.gateError',
            'Could not save the sign-in theme and layout on the application. Please try again.',
          )}
        </Alert>
      )}
      {/* Three-column builder area, shared with the flow and design builders. */}
      <Box sx={{flex: 1, overflow: 'hidden', p: 1, pt: 0}}>
        <BuilderLayout
          // The outline always stays open: it is how pages are turned on and chosen.
          open
          onPanelToggle={() => undefined}
          panelWidth={300}
          panelPaperSx={{borderRight: '1px solid', borderColor: 'divider'}}
          panelContent={
            <AppOutline
              config={config}
              selection={selection}
              onSelect={setSelection}
              onTogglePage={(page) => dispatch({page, type: 'togglePage'})}
            />
          }
          rightPanel={
            <BuilderStaticPanel width={340} header={propertiesTitle(t, config, selection)}>
              <AppPropertiesPanel
                config={config}
                dispatch={dispatch}
                selection={selection}
                scheme={effectiveScheme}
                onSchemeChange={setPreviewScheme}
              />
            </BuilderStaticPanel>
          }
        >
          {/* Center: the app in a device frame on a dotted canvas, filling the space it has. */}
          <Box
            sx={{
              height: '100%',
              display: 'flex',
              flexDirection: 'column',
              overflow: 'hidden',
              borderRadius: 1,
              bgcolor: isDark ? '#141414' : '#f6f7f9',
              backgroundImage: 'radial-gradient(circle, rgba(127, 127, 127, 0.22) 1px, transparent 1px)',
              backgroundSize: '18px 18px',
            }}
          >
            {isGateSelection ? (
              // The sign-in screens, as the Gate shows them with the application's theme.
              <GatePreview
                theme={config.gate?.themeId ? (gateTheme?.theme ?? null) : undefined}
                baseTheme={DefaultTheme as Theme}
                themelessBranding
                displayName={t('builder.preview.signIn', '{{app}} sign-in', {app: t(...copy.title)})}
                showToolbar={false}
                viewportPreset={viewport}
                zoom={zoom}
                colorScheme={effectiveScheme}
              />
            ) : (
              <PreviewFrame
                label={t('builder.preview.frame', '{{app}} preview', {app: t(...copy.title)})}
                viewport={viewport}
                zoom={zoom}
                stageRef={stageRef}
                minContentHeight={MIN_PREVIEW_HEIGHT}
              >
                <Box ref={stageRef} sx={{position: 'absolute', top: 0, left: 0, transformOrigin: 'top left'}}>
                  <ElementInspector enabled={inspectorEnabled} labelAttribute={PREVIEW_COMPONENT_ATTRIBUTE}>
                    <AppPreview
                      config={config}
                      selection={selection}
                      onSelect={setSelection}
                      scheme={effectiveScheme}
                    />
                  </ElementInspector>
                </Box>
              </PreviewFrame>
            )}
          </Box>
        </BuilderLayout>
      </Box>
    </Box>
  );
}
