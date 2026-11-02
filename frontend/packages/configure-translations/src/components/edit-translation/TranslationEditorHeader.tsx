// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getDisplayNameForCode, toFlagEmoji} from '@thunderid/i18n';
import {Box, Button, PageTitle, Typography} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props for the {@link TranslationEditorHeader} component.
 *
 * @public
 */
export interface TranslationEditorHeaderProps {
  /** The currently selected language code, or null if none. */
  selectedLanguage: string | null;
  /** Whether a save or reset operation is in progress. */
  isSaving: boolean;
  /** Whether the selected language is the fallback language (disables Reset to Default). */
  isFallbackLanguage: boolean;
  /** Whether a namespace is selected (required to enable Reset to Default). */
  hasNamespace: boolean;
  /** Called when the user clicks the back button. */
  onBack: () => void;
  /** Called when the user clicks Reset to Default. */
  onResetToDefault: () => void;
}

/**
 * Page title bar for the translations editor. Renders a back button, the
 * current language name with its flag, and the Reset to Default action.
 * Discarding and saving unsaved local edits are handled by the
 * {@link UnsavedChangesBar} shown at the bottom of the page instead.
 *
 * @param props - The component props
 *
 * @returns JSX element rendering the editor header
 *
 * @public
 */
export default function TranslationEditorHeader({
  selectedLanguage,
  isSaving,
  isFallbackLanguage,
  hasNamespace,
  onBack,
  onResetToDefault,
}: TranslationEditorHeaderProps): JSX.Element {
  const {t} = useTranslation('translations');

  return (
    <PageTitle>
      <PageTitle.BackButton onClick={onBack}>{t('editor.back', 'Back to translations')}</PageTitle.BackButton>
      <PageTitle.Header>
        {selectedLanguage ? (
          <Box sx={{display: 'flex', alignItems: 'center', gap: 1}}>
            <Typography component="span" sx={{fontSize: 'inherit', userSelect: 'none'}}>
              {toFlagEmoji(selectedLanguage)}
            </Typography>
            {getDisplayNameForCode(selectedLanguage)}
          </Box>
        ) : (
          t('page.title', 'Translations')
        )}
      </PageTitle.Header>
      {!isFallbackLanguage && (
        <PageTitle.Actions>
          <Button size="small" onClick={onResetToDefault} disabled={!hasNamespace || isSaving}>
            {t('actions.resetToDefault', 'Reset to default')}
          </Button>
        </PageTitle.Actions>
      )}
    </PageTitle>
  );
}
