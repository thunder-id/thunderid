/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {useCreateTheme, useGetTheme, useGetThemes, type Theme} from '@thunderid/design';
import {kebabCase} from '@thunderid/utils';
import {Alert, Box, Button, CircularProgress, IconButton, LinearProgress, Stack} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import GatePreview from '../../../components/GatePreview/GatePreview';
import ConfigureThemeColor from '../components/create-theme/ConfigureThemeColor';
import ConfigureThemeName from '../components/create-theme/ConfigureThemeName';
import buildThemeFromPrimaryColor from '../utils/buildThemeFromPrimaryColor';
import AppBreadcrumbs from '@/components/AppBreadcrumbs';

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
  const createTheme = useCreateTheme();
  const {data: themesData} = useGetThemes();

  const STEPS: Record<ThemeCreateStep, {label: string}> = {
    NAME: {label: t('themes.forms.configure_name.title', 'Create a Theme')},
    COLOR: {label: t('themes.forms.configure_color.title', 'Primary Color')},
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
    void navigate('/design');
  };

  const handleNext = (): void => {
    if (currentStep === 'NAME') setCurrentStep('COLOR');
  };

  const handleBack = (): void => {
    if (currentStep === 'COLOR') setCurrentStep('NAME');
  };

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
          Promise.resolve(navigate(`/design/themes/${created.id}`)).catch(() => null);
        },
        onError: (err: Error) => {
          setError(
            err.message ??
              t(
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
            onThemeNameChange={setThemeName}
            onReadyChange={handleNameReadyChange}
          />
        );
      case 'COLOR':
        return (
          <ConfigureThemeColor
            themeName={themeName}
            primaryColor={primaryColor}
            onPrimaryColorChange={setPrimaryColor}
          />
        );
      default:
        return null;
    }
  };

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <LinearProgress variant="determinate" value={stepProgress} sx={{height: 6}} />

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'row'}}>
        {/* Left panel */}
        <Box
          sx={{
            flex: currentStep === 'NAME' ? 1 : '0 0 50%',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          {/* Header */}
          <Box sx={{p: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
            <Stack direction="row" alignItems="center" spacing={2}>
              <IconButton
                onClick={handleClose}
                sx={{bgcolor: 'background.paper', '&:hover': {bgcolor: 'action.hover'}, boxShadow: 1}}
              >
                <X size={24} />
              </IconButton>
              <AppBreadcrumbs
                items={breadcrumbSteps.map((step, index, arr) => ({
                  key: step,
                  label: STEPS[step].label,
                  onClick: index < arr.length - 1 ? () => setCurrentStep(step) : undefined,
                }))}
              />
            </Stack>
          </Box>

          {/* Step content */}
          <Box
            sx={{
              flex: 1,
              display: 'flex',
              flexDirection: 'column',
              py: 8,
              px: 20,
            }}
          >
            <Box sx={{width: '100%', maxWidth: 800}}>
              {error && (
                <Alert severity="error" sx={{mb: 3}} onClose={() => setError(null)}>
                  {error}
                </Alert>
              )}

              {renderStep()}

              {/* Navigation */}
              <Box
                sx={{
                  mt: 5,
                  display: 'flex',
                  justifyContent: currentStep === 'NAME' ? 'flex-start' : 'space-between',
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
                    <Button
                      variant="contained"
                      onClick={handleCreate}
                      disabled={createTheme.isPending}
                      sx={{minWidth: 140}}
                    >
                      {t('themes.forms.configure_color.actions.create.label', 'Create Theme')}
                    </Button>
                  </Box>
                ) : (
                  <Button
                    variant="contained"
                    onClick={handleNext}
                    disabled={!stepReady[currentStep]}
                    sx={{minWidth: 100}}
                  >
                    {t('themes.forms.configure_color.actions.continue.label', 'Continue')}
                  </Button>
                )}
              </Box>
            </Box>
          </Box>
        </Box>

        {/* Right panel — live preview (only on COLOR step) */}
        {currentStep !== 'NAME' && (
          <Box sx={{flex: '0 0 50%', display: 'flex', flexDirection: 'column', p: 5}}>
            <GatePreview theme={previewTheme} displayName={themeName} />
          </Box>
        )}
      </Box>
    </Box>
  );
}
