// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, FullScreenCreationWizardLayout} from '@thunderid/components';
import {useCreateTheme, useGetTheme, useGetThemes, type Theme} from '@thunderid/design';
import {getErrorMessage, kebabCase} from '@thunderid/utils';
import {Alert, Box, Button, CircularProgress} from '@wso2/oxygen-ui';
import {useState, useCallback, useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ConfigureThemeColor from '../components/create-theme/ConfigureThemeColor';
import ConfigureThemeName from '../components/create-theme/ConfigureThemeName';
import GatePreview from '../GatePreview/GatePreview';
import useDesignRoutes from '../hooks/useDesignRoutes';
import buildThemeFromPrimaryColor from '../utils/buildThemeFromPrimaryColor';

type ThemeCreateStep = 'NAME' | 'COLOR';

const STEP_ORDER: ThemeCreateStep[] = ['NAME', 'COLOR'];

/**
 * Minimal theme used for preview and creation when the Classic base theme
 * hasn't loaded yet (or no themes exist). buildThemeFromPrimaryColor will
 * overwrite the primary palette, so only secondary/background need to be set.
 */
const FALLBACK_BASE_THEME: Theme = {
  defaultColorScheme: 'light',
  colorSchemes: {
    light: {
      palette: {
        primary: {main: '#000', contrastText: '#fff', light: '#333', dark: '#000'},
        secondary: {main: '#757575', contrastText: '#ffffff', light: 'rgb(144, 144, 144)', dark: 'rgb(81, 81, 81)'},
        background: {default: '#fafafa', paper: '#ffffff'},
      },
    },
    dark: {
      palette: {
        primary: {main: '#000', contrastText: '#fff', light: '#333', dark: '#000'},
        secondary: {main: '#757575', contrastText: '#ffffff', light: 'rgb(144, 144, 144)', dark: 'rgb(81, 81, 81)'},
        background: {default: '#121212', paper: '#121212'},
      },
    },
  },
} as unknown as Theme;

const DEFAULT_PRIMARY_COLOR = '#4f46e5';

export default function ThemeCreatePage(): JSX.Element {
  const {t} = useTranslation('design');
  const navigate = useNavigate();
  const routes = useDesignRoutes();
  const createTheme = useCreateTheme();
  const {data: themesData} = useGetThemes();

  const STEPS: Record<ThemeCreateStep, {label: string}> = {
    NAME: {label: t('themes.createWizard.steps.name', 'Details')},
    COLOR: {label: t('themes.forms.configure_color.title', 'Primary color')},
  };

  const [currentStep, setCurrentStep] = useState<ThemeCreateStep>('NAME');
  const [themeName, setThemeName] = useState('');
  const [primaryColor, setPrimaryColor] = useState(DEFAULT_PRIMARY_COLOR);
  const [nameReady, setNameReady] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Pick the Classic theme ID from the list; fall back to the first available theme.
  const baseThemeId = useMemo(() => {
    const themes = themesData?.themes ?? [];
    return themes.find((theme) => theme.displayName.toLowerCase() === 'classic')?.id ?? themes[0]?.id ?? null;
  }, [themesData]);

  // Fetch the full theme object (list endpoint returns metadata only, no theme JSON).
  const {data: baseThemeData} = useGetTheme(baseThemeId ?? '');

  const effectiveBaseTheme: Theme = baseThemeData?.theme ?? FALLBACK_BASE_THEME;

  const previewTheme = useMemo(
    () => buildThemeFromPrimaryColor(effectiveBaseTheme, primaryColor),
    [effectiveBaseTheme, primaryColor],
  );

  const stepProgress = ((STEP_ORDER.indexOf(currentStep) + 1) / STEP_ORDER.length) * 100;
  const breadcrumbSteps = STEP_ORDER.slice(0, STEP_ORDER.indexOf(currentStep) + 1);

  const stepReady: Record<ThemeCreateStep, boolean> = {
    NAME: nameReady,
    COLOR: true,
  };

  const handleClose = (): void => {
    void navigate(routes.design.list());
  };

  const handleNext = (): void => {
    if (currentStep === 'NAME') setCurrentStep('COLOR');
  };

  const handleBack = (): void => {
    if (currentStep === 'COLOR') setCurrentStep('NAME');
  };

  // A create failure is stale once the user edits the name or color, so both field-change paths
  // below clear the error before applying the change. Only reset the mutation once it has
  // actually failed: resetting while it's still pending would flip isPending back to false and
  // re-enable the create button before the in-flight request settles.
  const clearCreateError = useCallback((): void => {
    setError(null);
    if (createTheme.isError) {
      createTheme.reset();
    }
  }, [createTheme]);

  const handleThemeNameChange = useCallback(
    (newName: string): void => {
      clearCreateError();
      setThemeName(newName);
    },
    [clearCreateError],
  );

  const handlePrimaryColorChange = useCallback(
    (newColor: string): void => {
      clearCreateError();
      setPrimaryColor(newColor);
    },
    [clearCreateError],
  );

  const handleCreate = (): void => {
    setError(null);
    const handle = kebabCase(themeName);
    createTheme.mutate(
      {
        handle,
        displayName: themeName.trim(),
        theme: buildThemeFromPrimaryColor(effectiveBaseTheme, primaryColor),
      },
      {
        onSuccess: (created) => {
          Promise.resolve(navigate(routes.design.themeDetail(created.id))).catch(() => null);
        },
        onError: (err: Error) => {
          setError(
            getErrorMessage(
              err,
              t,
              'themes.forms.configure_color.errors.create_failed.message',
              'Failed to create theme. Please try again.',
            ),
          );
        },
      },
    );
  };

  const handleNameReadyChange = useCallback((ready: boolean) => setNameReady(ready), []);

  const renderStep = (): JSX.Element | null => {
    switch (currentStep) {
      case 'NAME':
        return (
          <ConfigureThemeName
            themeName={themeName}
            onThemeNameChange={handleThemeNameChange}
            onReadyChange={handleNameReadyChange}
          />
        );
      case 'COLOR':
        return (
          <ConfigureThemeColor
            themeName={themeName}
            primaryColor={primaryColor}
            onPrimaryColorChange={handlePrimaryColorChange}
          />
        );
      default:
        return null;
    }
  };

  return (
    <FullScreenCreationWizardLayout
      onClose={handleClose}
      progress={stepProgress}
      breadcrumbItems={breadcrumbSteps.map((step, index, arr) => ({
        key: step,
        label: STEPS[step].label,
        onClick: index < arr.length - 1 ? () => setCurrentStep(step) : undefined,
      }))}
      preview={
        currentStep === 'NAME' ? undefined : (
          <Box sx={{flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column', p: 5}}>
            <GatePreview theme={previewTheme} displayName={themeName} />
          </Box>
        )
      }
      footer={
        <Box
          sx={{
            display: 'flex',
            justifyContent: currentStep === 'NAME' ? 'flex-end' : 'space-between',
            gap: 2,
          }}
        >
          {currentStep !== 'NAME' && (
            <Button variant="outlined" onClick={handleBack} sx={{minWidth: 100}}>
              {t('themes.forms.configure_color.actions.back.label', 'Back')}
            </Button>
          )}

          {currentStep === 'COLOR' ? (
            <Box sx={{display: 'flex', alignItems: 'center', gap: 2}}>
              {createTheme.isPending && <CircularProgress size={20} />}
              <Button variant="contained" onClick={handleCreate} disabled={createTheme.isPending} sx={{minWidth: 140}}>
                {t('themes.forms.configure_color.actions.create.label', 'Create theme')}
              </Button>
            </Box>
          ) : (
            <Button variant="contained" onClick={handleNext} disabled={!stepReady[currentStep]} sx={{minWidth: 100}}>
              {t('themes.forms.configure_color.actions.continue.label', 'Continue')}
            </Button>
          )}
        </Box>
      }
    >
      <DocumentTitle title={t('themes.forms.configure_color.actions.create.label', 'Create theme')} />
      {error && (
        <Alert severity="error" sx={{mb: 3}} onClose={clearCreateError}>
          {error}
        </Alert>
      )}

      {renderStep()}
    </FullScreenCreationWizardLayout>
  );
}
