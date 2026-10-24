// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, FullScreenCreationWizardLayout} from '@thunderid/components';
import {useGetTranslations, useCreateTranslations, I18nDefaultConstants} from '@thunderid/i18n';
import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, Button, CircularProgress} from '@wso2/oxygen-ui';
import {useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
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
];

/**
 * Full-page wizard for creating a new translation language.
 *
 * Guides the user through three sequential steps: choosing a country, selecting
 * the language variant, and reviewing or overriding the derived BCP 47 locale
 * code. On completion it seeds every key from English (en-US) and navigates to
 * the edit page for the new language.
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
    isCreating,
    setIsCreating,
    error,
    setError,
  } = useTranslationCreate();

  const [stepReady, setStepReady] = useState<Record<TranslationCreateFlowStep, boolean>>({
    COUNTRY: false,
    LANGUAGE: false,
    LOCALE_CODE: true,
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
  };

  const stepProgress = ((STEPS.indexOf(currentStep) + 1) / STEPS.length) * 100;

  const getBreadcrumbItems = (): {key: string; label: string; onClick?: () => void}[] =>
    STEPS.slice(0, STEPS.indexOf(currentStep) + 1).map((step, index, array) => {
      const isLast = index === array.length - 1;
      return {
        key: step,
        label: stepLabels[step],
        ...(isLast || isCreating ? {} : {onClick: () => setCurrentStep(step)}),
      };
    });

  const handleCountryReady = useCallback((isReady: boolean): void => {
    setStepReady((prev) => ({...prev, COUNTRY: isReady}));
  }, []);

  const handleLanguageReady = useCallback((isReady: boolean): void => {
    setStepReady((prev) => ({...prev, LANGUAGE: isReady}));
  }, []);

  const handleLocaleCodeReady = useCallback((isReady: boolean): void => {
    setStepReady((prev) => ({...prev, LOCALE_CODE: isReady}));
  }, []);

  // A create failure is stale once the wizard's data changes.
  const handleCountryChange: typeof setSelectedCountry = useCallback(
    (value) => {
      setError(null);
      setSelectedCountry(value);
    },
    [setError, setSelectedCountry],
  );

  const handleLocaleChange: typeof setSelectedLocale = useCallback(
    (value) => {
      setError(null);
      setSelectedLocale(value);
    },
    [setError, setSelectedLocale],
  );

  const handleLocaleCodeOverrideChange: typeof setLocaleCodeOverride = useCallback(
    (value) => {
      setError(null);
      setLocaleCodeOverride(value);
    },
    [setError, setLocaleCodeOverride],
  );

  const handleClose = (): void => {
    if (isCreating) return;
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

    const {data: enData, error: enError} = await fetchEnTranslations();
    if (enError || !enData) {
      logger.error('Failed to fetch en-US translations', {error: enError});
      setError(
        enError
          ? getErrorMessage(enError, t, 'language.add.error', 'Failed to add language. Please try again.')
          : t('language.add.error'),
      );
      setIsCreating(false);
      return;
    }

    // The server rejects empty override values (a language is only known to exist once it has at
    // least one translation row, so there's no way to represent a genuinely empty language), so
    // every key is seeded from English; the admin can overwrite each value afterwards.
    const translations: Record<string, Record<string, string>> = {};
    Object.entries(enData.translations).forEach(([ns, nsValues]) => {
      translations[ns] = {};
      Object.entries(nsValues).forEach(([key, val]) => {
        translations[ns][key] = val;
      });
    });

    try {
      await createTranslations.mutateAsync({language: localeCode, translations});
    } catch (_err: unknown) {
      logger.error('Failed to create translations', {error: _err});
      setError(getErrorMessage(_err as Error, t, 'language.add.error', 'Failed to add language. Please try again.'));
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
            onCountryChange={handleCountryChange}
            onReadyChange={handleCountryReady}
          />
        );
      case TranslationCreateFlowStep.LANGUAGE:
        if (!selectedCountry) return null;
        return (
          <SelectLanguage
            selectedCountry={selectedCountry}
            selectedLocale={selectedLocale}
            onLocaleChange={handleLocaleChange}
            onReadyChange={handleLanguageReady}
          />
        );
      case TranslationCreateFlowStep.LOCALE_CODE:
        if (!selectedLocale) return null;
        return (
          <ReviewLocaleCode
            derivedLocale={selectedLocale}
            localeCode={localeCodeOverride}
            onLocaleCodeChange={handleLocaleCodeOverrideChange}
            onReadyChange={handleLocaleCodeReady}
          />
        );
      default:
        return null;
    }
  };

  const isFirstStep = currentStep === TranslationCreateFlowStep.COUNTRY;
  const isLastStep = currentStep === TranslationCreateFlowStep.LOCALE_CODE;

  const footer = (
    <Box sx={{display: 'flex', justifyContent: isFirstStep ? 'flex-end' : 'space-between', gap: 2}}>
      {!isFirstStep && (
        <Button variant="outlined" onClick={handleBack} sx={{minWidth: 100}} disabled={isCreating}>
          {t('common:actions.back', {ns: 'common', defaultValue: 'Back'})}
        </Button>
      )}
      <Box sx={{display: 'flex', alignItems: 'center', gap: 2}}>
        {isCreating && <CircularProgress size={20} />}
        <Button
          variant="contained"
          onClick={handleNext}
          sx={{minWidth: 100}}
          disabled={!stepReady[currentStep] || isCreating}
        >
          {isLastStep
            ? t('language.create.createButton', {defaultValue: 'Create'})
            : t('common:actions.continue', {ns: 'common', defaultValue: 'Continue'})}
        </Button>
      </Box>
    </Box>
  );

  return (
    <FullScreenCreationWizardLayout
      onClose={handleClose}
      progress={stepProgress}
      breadcrumbItems={getBreadcrumbItems()}
      footer={footer}
    >
      <DocumentTitle title={t('listing.addLanguage', 'Add language')} />
      {error && (
        <Alert severity="error" sx={{mb: 3}} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {renderStepContent()}
    </FullScreenCreationWizardLayout>
  );
}
