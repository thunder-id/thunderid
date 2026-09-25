// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, UnsavedChangesBar} from '@thunderid/components';
import {useToast} from '@thunderid/contexts';
import {useGetTranslations, useUpdateTranslation, NamespaceConstants, I18nDefaultConstants} from '@thunderid/i18n';
import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, PageContent, useColorScheme} from '@wso2/oxygen-ui';
import {useCallback, useMemo, useState, type JSX, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import NamespaceSelector from '@/components/edit-translation/NamespaceSelector';
import TranslationEditorCard from '@/components/edit-translation/TranslationEditorCard';
import TranslationEditorHeader from '@/components/edit-translation/TranslationEditorHeader';
import useTranslationRoutes from '@/hooks/useTranslationRoutes';

/**
 * Page for editing translation key-value pairs for a specific language.
 *
 * Reads the target language from the URL parameter. Displays a namespace
 * selector, a fields/JSON tab editor with local dirty-change tracking, and a
 * live gate preview panel. Supports saving individual field changes,
 * discarding all local edits, and resetting the namespace to the default
 * English values.
 *
 * @returns JSX element rendering the translations edit page
 *
 * @example
 * ```tsx
 * // Rendered automatically by the router at /translations/:language
 * import TranslationsEditPage from './TranslationsEditPage';
 *
 * function App() {
 *   return <TranslationsEditPage />;
 * }
 * ```
 *
 * @public
 */
export default function TranslationsEditPage(): JSX.Element {
  const {t} = useTranslation('translations');
  const navigate = useNavigate();
  const logger = useLogger('TranslationsEditPage');
  const {showToast} = useToast();
  const {language: languageParam} = useParams<{language: string}>();
  const selectedLanguage = languageParam ?? null;
  const routes = useTranslationRoutes();

  const {mode, systemMode} = useColorScheme();
  const colorMode: 'light' | 'dark' =
    ((mode === 'system' ? systemMode : mode) ?? 'light') === 'dark' ? 'dark' : 'light';

  const [selectedNamespace, setSelectedNamespace] = useState<string | null>(null);
  const [editView, setEditView] = useState<'fields' | 'json'>('json');
  const [search, setSearch] = useState('');
  const [localChanges, setLocalChanges] = useState<Record<string, string>>({});
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const {
    data: translationsData,
    isLoading: translationsLoading,
    error: translationsError,
    refetch: refetchTranslations,
  } = useGetTranslations({
    language: selectedLanguage ?? '',
    enabled: !!selectedLanguage,
  });

  // Fetch the default (en) translations for "Reset to Default"
  const {data: defaultTranslationsData, error: defaultTranslationsError} = useGetTranslations({
    language: 'en',
    enabled: !!selectedLanguage && selectedLanguage !== 'en',
  });

  const updateTranslation = useUpdateTranslation();

  const namespaces = useMemo(() => {
    if (!translationsData?.translations) {
      return [];
    }

    const ns = Object.keys(translationsData?.translations ?? {});
    return ns.includes(NamespaceConstants.CUSTOM_NAMESPACE) ? ns : [...ns, NamespaceConstants.CUSTOM_NAMESPACE];
  }, [translationsData]);

  // Reset namespace when language changes
  const [prevLanguage, setPrevLanguage] = useState(selectedLanguage);
  if (prevLanguage !== selectedLanguage) {
    setPrevLanguage(selectedLanguage);
    setSelectedNamespace(null);
    setLocalChanges({});
    setSearch('');
    setSaveError(null);
  }

  // Initialize namespace once API data arrives
  if (namespaces.length > 0 && !selectedNamespace) {
    setSelectedNamespace(namespaces[0]);
  }

  // Reset local changes when namespace switches
  const [prevNamespace, setPrevNamespace] = useState(selectedNamespace);
  if (prevNamespace !== selectedNamespace) {
    setPrevNamespace(selectedNamespace);
    setLocalChanges({});
    setSearch('');
    setSaveError(null);
  }

  const serverValues: Record<string, string> = useMemo(
    () => translationsData?.translations?.[selectedNamespace ?? ''] ?? {},
    [translationsData, selectedNamespace],
  );

  const currentValues: Record<string, string> = useMemo(
    () => ({...serverValues, ...localChanges}),
    [serverValues, localChanges],
  );

  const dirtyKeys = useMemo(
    () => Object.keys(localChanges).filter((k) => localChanges[k] !== serverValues[k]),
    [localChanges, serverValues],
  );
  const hasDirtyChanges = dirtyKeys.length > 0;

  const handleFieldChange = useCallback((key: string, value: string) => {
    setSaveError(null);
    setLocalChanges((prev) => ({...prev, [key]: value}));
  }, []);

  const handleResetField = useCallback((key: string) => {
    setSaveError(null);
    setLocalChanges((prev) => {
      const next = {...prev};
      delete next[key];
      return next;
    });
  }, []);

  const handleJsonChange = useCallback((changes: Record<string, string>) => {
    setSaveError(null);
    setLocalChanges(changes);
  }, []);

  const handleSave = async () => {
    if (!selectedLanguage || !selectedNamespace || dirtyKeys.length === 0) return;
    setIsSaving(true);
    setSaveError(null);

    const results = await Promise.allSettled(
      dirtyKeys.map((key) =>
        updateTranslation.mutateAsync({
          language: selectedLanguage,
          namespace: selectedNamespace,
          key,
          value: localChanges[key],
        }),
      ),
    );

    const firstRejected = results.find((r): r is PromiseRejectedResult => r.status === 'rejected');
    setIsSaving(false);

    if (firstRejected) {
      setSaveError(
        getErrorMessage(firstRejected.reason as Error, t, 'editor.jsonSaveError', 'Failed to save some translations.'),
      );
    } else {
      setLocalChanges({});
      showToast(t('editor.jsonSaveSuccess', 'All translations saved.'), 'success');
    }
  };

  const handleDiscard = () => {
    setSaveError(null);
    setLocalChanges({});
  };

  const handleResetToDefault = async () => {
    if (!selectedLanguage || !selectedNamespace) return;
    setSaveError(null);

    if (defaultTranslationsError) {
      setSaveError(
        getErrorMessage(defaultTranslationsError, t, 'editor.jsonSaveError', 'Failed to save some translations.'),
      );
      return;
    }

    const defaultValues = defaultTranslationsData?.translations?.[selectedNamespace] ?? {};
    const entries = Object.entries(defaultValues);
    if (entries.length === 0) return;

    setIsSaving(true);

    const results = await Promise.allSettled(
      entries.map(([key, value]) =>
        updateTranslation.mutateAsync({
          language: selectedLanguage,
          namespace: selectedNamespace,
          key,
          value,
        }),
      ),
    );

    const firstRejected = results.find((r): r is PromiseRejectedResult => r.status === 'rejected');
    setIsSaving(false);

    if (firstRejected) {
      setSaveError(
        getErrorMessage(firstRejected.reason as Error, t, 'editor.jsonSaveError', 'Failed to save some translations.'),
      );
    } else {
      setLocalChanges({});
      showToast(t('editor.jsonSaveSuccess', 'All translations saved.'), 'success');
    }
  };

  const handleTabChange = (_: SyntheticEvent, v: 'fields' | 'json') => {
    setEditView(v);
    setSearch('');
  };

  const handleBack = () => {
    (async (): Promise<void> => {
      await navigate(routes.list());
    })().catch((_error: unknown) => {
      logger.error('Failed to navigate back to translations list', {error: _error});
    });
  };

  const isLoading = !!selectedLanguage && translationsLoading;
  // Namespaces the admin may add brand-new keys to from the Console. The custom
  // namespace is fully user-defined; the notification namespace is seeded from the
  // server bootstrap resources but is likewise open to admin-authored additions.
  const isKeyCreationAllowedNamespace =
    selectedNamespace === NamespaceConstants.CUSTOM_NAMESPACE ||
    selectedNamespace === NamespaceConstants.NOTIFICATION;

  return (
    <PageContent sx={{display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0}}>
      <TranslationEditorHeader
        selectedLanguage={selectedLanguage}
        isSaving={isSaving}
        isFallbackLanguage={selectedLanguage === I18nDefaultConstants.FALLBACK_LANGUAGE}
        hasNamespace={!!selectedNamespace}
        onBack={handleBack}
        onResetToDefault={() => {
          handleResetToDefault().catch((_error: unknown) =>
            logger.error('Failed to reset to default', {error: _error}),
          );
        }}
      />

      <Box sx={{mb: 2}}>
        {saveError && (
          <Alert severity="error" sx={{py: 0}}>
            {saveError}
          </Alert>
        )}
      </Box>

      {translationsError ? (
        <QueryErrorNotice
          error={translationsError}
          t={t}
          variant="block"
          title={t('page.loadErrorTitle', 'Failed to load translations')}
          fallbackKey="page.loadError"
          fallbackDefaultValue="Failed to load translations"
          onRetry={() => void refetchTranslations()}
        />
      ) : (
        <>
          <NamespaceSelector
            namespaces={namespaces}
            value={selectedNamespace}
            loading={isLoading}
            onChange={setSelectedNamespace}
          />

          <TranslationEditorCard
            selectedLanguage={selectedLanguage}
            isLoading={isLoading}
            editView={editView}
            search={search}
            currentValues={currentValues}
            serverValues={serverValues}
            isKeyCreationAllowedNamespace={isKeyCreationAllowedNamespace}
            colorMode={colorMode}
            onTabChange={handleTabChange}
            onSearchChange={setSearch}
            onFieldChange={handleFieldChange}
            onResetField={handleResetField}
            onJsonChange={handleJsonChange}
          />

          {hasDirtyChanges && (
            <UnsavedChangesBar
              message={t('editor.unsavedCount', {count: dirtyKeys.length, defaultValue: '{{count}} unsaved change'})}
              resetLabel={t('actions.discardChanges', 'Discard Changes')}
              saveLabel={t('actions.saveChanges', 'Save Changes')}
              savingLabel={t('common:status.saving', {ns: 'common', defaultValue: 'Saving...'})}
              isSaving={isSaving}
              onReset={handleDiscard}
              onSave={() => {
                handleSave().catch((_error: unknown) => logger.error('Failed to save translations', {error: _error}));
              }}
            />
          )}
        </>
      )}
    </PageContent>
  );
}
