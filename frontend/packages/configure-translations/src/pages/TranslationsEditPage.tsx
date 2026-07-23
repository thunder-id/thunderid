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

import {useGetTranslations, useUpdateTranslation, NamespaceConstants, I18nDefaultConstants} from '@thunderid/i18n';
import {useLogger} from '@thunderid/logger/react';
import {Alert, PageContent, Snackbar, useColorScheme} from '@wso2/oxygen-ui';
import {useCallback, useMemo, useState, type JSX, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import NamespaceSelector from '@/components/edit-translation/NamespaceSelector';
import TranslationEditorCard from '@/components/edit-translation/TranslationEditorCard';
import TranslationEditorHeader from '@/components/edit-translation/TranslationEditorHeader';
import useTranslationRoutes from '@/hooks/useTranslationRoutes';

interface ToastState {
  open: boolean;
  message: string;
  severity: 'success' | 'error';
}

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
  const {language: languageParam} = useParams<{language: string}>();
  const selectedLanguage = languageParam ?? null;
  const routes = useTranslationRoutes();

  const {mode, systemMode} = useColorScheme();
  const colorMode: 'light' | 'dark' =
    ((mode === 'system' ? systemMode : mode) ?? 'light') === 'dark' ? 'dark' : 'light';

  const [selectedNamespace, setSelectedNamespace] = useState<string | null>(null);
  const [editView, setEditView] = useState<'fields' | 'json'>('fields');
  const [search, setSearch] = useState('');
  const [localChanges, setLocalChanges] = useState<Record<string, string>>({});
  const [isSaving, setIsSaving] = useState(false);
  const [toast, setToast] = useState<ToastState>({open: false, message: '', severity: 'success'});

  const {data: translationsData, isLoading: translationsLoading} = useGetTranslations({
    language: selectedLanguage ?? '',
    enabled: !!selectedLanguage,
  });

  // Fetch the default (en) translations for "Reset to Default"
  const {data: defaultTranslationsData} = useGetTranslations({
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
    setLocalChanges((prev) => ({...prev, [key]: value}));
  }, []);

  const handleResetField = useCallback((key: string) => {
    setLocalChanges((prev) => {
      const next = {...prev};
      delete next[key];
      return next;
    });
  }, []);

  const handleJsonChange = useCallback((changes: Record<string, string>) => {
    setLocalChanges(changes);
  }, []);

  const handleSave = async () => {
    if (!selectedLanguage || !selectedNamespace || dirtyKeys.length === 0) return;
    setIsSaving(true);

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

    const failed = results.filter((r) => r.status === 'rejected').length;
    setIsSaving(false);

    if (failed > 0) {
      setToast({open: true, message: t('editor.jsonSaveError'), severity: 'error'});
    } else {
      setLocalChanges({});
      setToast({open: true, message: t('editor.jsonSaveSuccess'), severity: 'success'});
    }
  };

  const handleDiscard = () => {
    setLocalChanges({});
  };

  const handleResetToDefault = async () => {
    if (!selectedLanguage || !selectedNamespace) return;
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

    const failed = results.filter((r) => r.status === 'rejected').length;
    setIsSaving(false);
    setLocalChanges({});

    if (failed > 0) {
      setToast({open: true, message: t('editor.jsonSaveError'), severity: 'error'});
    } else {
      setToast({open: true, message: t('editor.jsonSaveSuccess'), severity: 'success'});
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
  const isCustomNamespace = selectedNamespace === NamespaceConstants.CUSTOM_NAMESPACE;

  return (
    <PageContent fullWidth sx={{display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0}}>
      <TranslationEditorHeader
        selectedLanguage={selectedLanguage}
        hasDirtyChanges={hasDirtyChanges}
        dirtyCount={dirtyKeys.length}
        isSaving={isSaving}
        isFallbackLanguage={selectedLanguage === I18nDefaultConstants.FALLBACK_LANGUAGE}
        hasNamespace={!!selectedNamespace}
        onBack={handleBack}
        onDiscard={handleDiscard}
        onResetToDefault={() => {
          handleResetToDefault().catch((_error: unknown) =>
            logger.error('Failed to reset to default', {error: _error}),
          );
        }}
        onSave={() => {
          handleSave().catch((_error: unknown) => logger.error('Failed to save translations', {error: _error}));
        }}
      />

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
        isCustomNamespace={isCustomNamespace}
        colorMode={colorMode}
        onTabChange={handleTabChange}
        onSearchChange={setSearch}
        onFieldChange={handleFieldChange}
        onResetField={handleResetField}
        onJsonChange={handleJsonChange}
      />

      <Snackbar
        open={toast.open}
        autoHideDuration={3000}
        onClose={() => setToast((prev) => ({...prev, open: false}))}
        anchorOrigin={{vertical: 'bottom', horizontal: 'center'}}
      >
        <Alert severity={toast.severity} onClose={() => setToast((prev) => ({...prev, open: false}))}>
          {toast.message}
        </Alert>
      </Snackbar>
    </PageContent>
  );
}
