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

import {useGetTranslations, useCreateTranslations, I18nDefaultConstants} from '@thunderid/i18n';
import {useLogger} from '@thunderid/logger/react';
import {Alert, Box, Breadcrumbs, Button, IconButton, LinearProgress, Typography} from '@wso2/oxygen-ui';
import {ChevronRight, X} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import InitializeLanguage from '@/components/create-translation/InitializeLanguage';
import ReviewLocaleCode from '@/components/create-translation/ReviewLocaleCode';
import SelectCountry from '@/components/create-translation/SelectCountry';
import SelectLanguage from '@/components/create-translation/SelectLanguage';
import useTranslationCreate from '@/contexts/TranslationCreate/useTranslationCreate';
import useTranslationRoutes from '@/hooks/useTranslationRoutes';
import {TranslationCreateFlowStep} from '@/models/translation-create-flow';

const STEPS: TranslationCreateFlowStep[] = [
  TranslationCreateFlowStep.COUNTRY,
  TranslationCreateFlowStep.LANGUAGE,
  TranslationCreateFlowStep.LOCALE_CODE,
  TranslationCreateFlowStep.INITIALIZE,
];

/**
 * Full-page wizard for creating a new translation language.
 *
 * Guides the user through four sequential steps: choosing a country, selecting
 * the language variant, reviewing or overriding the derived BCP 47 locale code,
 * and choosing how to initialize the translation keys. On completion it writes
 * all keys to the server and navigates to the edit page for the new language.
 *
 * @returns JSX element rendering the multi-step language creation page
 *
 * @example
 * ```tsx
 * // Rendered automatically by the router at /translations/create
 * import TranslationCreatePage from './TranslationCreatePage';
 *
 * function App() {
 *   return <TranslationCreatePage />;
 * }
 * ```
 *
 * @public
 */
export default function TranslationCreatePage(): JSX.Element {
  const {t} = useTranslation('translations');
  const navigate = useNavigate();
  const logger = useLogger('TranslationCreatePage');
  const routes = useTranslationRoutes();
  const {refetch: fetchEnTranslations} = useGetTranslations({
    language: I18nDefaultConstants.FALLBACK_LANGUAGE,
    enabled: false,
  });
  const createTranslations = useCreateTranslations();

  const {
    currentStep,
    setCurrentStep,
    selectedCountry,
    setSelectedCountry,
    selectedLocale,
    setSelectedLocale,
    localeCodeOverride,
    setLocaleCodeOverride,
    localeCode,
    populateFromEnglish,
    setPopulateFromEnglish,
    isCreating,
    setIsCreating,
    progress,
    setProgress,
    error,
    setError,
  } = useTranslationCreate();

  const [stepReady, setStepReady] = useState<Record<TranslationCreateFlowStep, boolean>>({
    COUNTRY: false,
    LANGUAGE: false,
    LOCALE_CODE: true,
    INITIALIZE: true,
  });

  // Reset locale when country changes
  const [prevCountry, setPrevCountry] = useState(selectedCountry);
  if (prevCountry !== selectedCountry) {
    setPrevCountry(selectedCountry);
    setSelectedLocale(null);
    setStepReady((prev) => ({...prev, LANGUAGE: false}));
  }

  const stepLabels: Record<TranslationCreateFlowStep, string> = {
    COUNTRY: t('language.create.steps.country'),
    LANGUAGE: t('language.create.steps.language'),
    LOCALE_CODE: t('language.create.steps.localeCode'),
    INITIALIZE: t('language.create.steps.initialize'),
  };

  const stepProgress = ((STEPS.indexOf(currentStep) + 1) / STEPS.length) * 100;

  const getBreadcrumbSteps = (): TranslationCreateFlowStep[] => STEPS.slice(0, STEPS.indexOf(currentStep) + 1);

  const handleCountryReady = useCallback((isReady: boolean): void => {
    setStepReady((prev) => ({...prev, COUNTRY: isReady}));
  }, []);

  const handleLanguageReady = useCallback((isReady: boolean): void => {
    setStepReady((prev) => ({...prev, LANGUAGE: isReady}));
  }, []);

  const handleLocaleCodeReady = useCallback((isReady: boolean): void => {
    setStepReady((prev) => ({...prev, LOCALE_CODE: isReady}));
  }, []);

  const handleClose = (): void => {
    (async () => {
      await navigate(routes.list());
    })().catch((_error: unknown) => {
      logger.error('Failed to navigate to translations page', {error: _error});
    });
  };

  const handleCreate = async (): Promise<void> => {
    if (!localeCode) return;
    setError(null);
    setIsCreating(true);
    setProgress(0);

    const {data: enData, error: enError} = await fetchEnTranslations();
    if (enError || !enData) {
      logger.error('Failed to fetch en-US translations', {error: enError});
      setError(t('language.add.error'));
      setIsCreating(false);
      return;
    }

    const translations: Record<string, Record<string, string>> = {};
    Object.entries(enData.translations).forEach(([ns, nsValues]) => {
      translations[ns] = {};
      Object.entries(nsValues).forEach(([key, val]) => {
        translations[ns][key] = populateFromEnglish ? val : '';
      });
    });

    try {
      await createTranslations.mutateAsync({language: localeCode, translations});
      setProgress(100);
    } catch (_err: unknown) {
      logger.error('Failed to create translations', {error: _err});
      setError(t('language.add.error'));
      setIsCreating(false);
      return;
    }

    try {
      await navigate(routes.detail(localeCode));
    } catch (_err: unknown) {
      logger.error('Translations created but navigation failed', {error: _err, localeCode});
      setIsCreating(false);
    }
  };

  const handleNext = (): void => {
    const idx = STEPS.indexOf(currentStep);
    if (idx < STEPS.length - 1) {
      if (currentStep === TranslationCreateFlowStep.LANGUAGE) {
        setLocaleCodeOverride(selectedLocale?.code ?? '');
      }
      setCurrentStep(STEPS[idx + 1]);
    } else {
      handleCreate().catch((_error: unknown) => {
        logger.error('Failed to create translation', {error: _error});
      });
    }
  };

  const handleBack = (): void => {
    const idx = STEPS.indexOf(currentStep);
    if (idx > 0) setCurrentStep(STEPS[idx - 1]);
  };

  const renderStepContent = (): JSX.Element | null => {
    switch (currentStep) {
      case TranslationCreateFlowStep.COUNTRY:
        return (
          <SelectCountry
            selectedCountry={selectedCountry}
            onCountryChange={setSelectedCountry}
            onReadyChange={handleCountryReady}
          />
        );
      case TranslationCreateFlowStep.LANGUAGE:
        if (!selectedCountry) return null;
        return (
          <SelectLanguage
            selectedCountry={selectedCountry}
            selectedLocale={selectedLocale}
            onLocaleChange={setSelectedLocale}
            onReadyChange={handleLanguageReady}
          />
        );
      case TranslationCreateFlowStep.LOCALE_CODE:
        if (!selectedLocale) return null;
        return (
          <ReviewLocaleCode
            derivedLocale={selectedLocale}
            localeCode={localeCodeOverride}
            onLocaleCodeChange={setLocaleCodeOverride}
            onReadyChange={handleLocaleCodeReady}
          />
        );
      case TranslationCreateFlowStep.INITIALIZE:
        return (
          <InitializeLanguage
            populateFromEnglish={populateFromEnglish}
            onPopulateChange={setPopulateFromEnglish}
            isCreating={isCreating}
            progress={progress}
          />
        );
      default:
        return null;
    }
  };

  const isFirstStep = currentStep === TranslationCreateFlowStep.COUNTRY;

  return (
    <Box sx={{height: '100vh', display: 'flex', flexDirection: 'column', overflow: 'hidden'}}>
      <LinearProgress variant="determinate" value={stepProgress} sx={{height: 6, flexShrink: 0}} />

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0}}>
        {/* Header */}
        <Box sx={{p: 4, display: 'flex', alignItems: 'center', gap: 2, flexShrink: 0}}>
          <IconButton
            onClick={handleClose}
            disabled={isCreating}
            sx={{bgcolor: 'background.paper', '&:hover': {bgcolor: 'action.hover'}, boxShadow: 1}}
          >
            <X size={24} />
          </IconButton>
          <Breadcrumbs separator={<ChevronRight size={16} />} aria-label="breadcrumb">
            {getBreadcrumbSteps().map((step, index, array) => {
              const isLast = index === array.length - 1;
              return isLast ? (
                <Typography key={step} variant="h5" color="text.primary">
                  {stepLabels[step]}
                </Typography>
              ) : (
                <Typography
                  key={step}
                  variant="h5"
                  onClick={() => !isCreating && setCurrentStep(step)}
                  sx={{cursor: isCreating ? 'default' : 'pointer'}}
                >
                  {stepLabels[step]}
                </Typography>
              );
            })}
          </Breadcrumbs>
        </Box>

        {/* Left-aligned form content */}
        <Box
          sx={{
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
            overflowY: 'auto',
            py: 8,
            px: 20,
            alignItems: 'flex-start',
          }}
        >
          <Box sx={{width: '100%', maxWidth: 800, display: 'flex', flexDirection: 'column'}}>
            {error && (
              <Alert severity="error" sx={{mb: 3}} onClose={() => setError(null)}>
                {error}
              </Alert>
            )}

            {renderStepContent()}

            <Box
              sx={{
                mt: 4,
                display: 'flex',
                justifyContent: isFirstStep ? 'flex-end' : 'space-between',
                gap: 2,
              }}
            >
              {!isFirstStep && (
                <Button variant="outlined" onClick={handleBack} sx={{minWidth: 100}} disabled={isCreating}>
                  {t('common:actions.back', {ns: 'common'})}
                </Button>
              )}
              <Button
                variant="contained"
                onClick={handleNext}
                sx={{minWidth: 100}}
                disabled={!stepReady[currentStep] || isCreating}
              >
                {currentStep === TranslationCreateFlowStep.INITIALIZE
                  ? t('language.create.createButton')
                  : t('common:actions.continue', {ns: 'common'})}
              </Button>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
