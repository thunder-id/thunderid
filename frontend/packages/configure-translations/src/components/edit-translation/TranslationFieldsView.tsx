// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, FormControl, FormLabel, IconButton, TextField, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Plus, RotateCcw} from '@wso2/oxygen-ui-icons-react';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * The in-progress "add new key" draft. It is owned by the parent page so it can be cleared
 * centrally on Save, Discard, and namespace change rather than lingering in child state.
 *
 * @public
 */
export interface AddKeyDraft {
  /** Whether the add-key form is open. */
  adding: boolean;
  /** The draft key being entered. */
  key: string;
  /** The draft value being entered. */
  value: string;
}

/**
 * Props for the {@link TranslationFieldsView} component.
 *
 * @public
 */
export interface TranslationFieldsViewProps {
  /** Current display values (server values merged with local edits). */
  localValues: Record<string, string>;
  /** Original server values — used to detect dirtiness and to reset. */
  serverValues: Record<string, string>;
  /** Current search query used to filter visible translation keys. */
  search: string;
  /** Whether the active namespace permits admins to add brand-new keys (e.g. the custom and notification namespaces). */
  allowNewKeys: boolean;
  /** Callback invoked when the user edits a translation field value. */
  onChange: (key: string, value: string) => void;
  /** Callback invoked when the user resets a field back to its saved value. */
  onResetField: (key: string) => void;
  /** The in-progress add-key draft, owned by the parent so it can be cleared centrally. */
  draft: AddKeyDraft;
  /** Callback invoked when the add-key draft changes. */
  onDraftChange: (draft: AddKeyDraft) => void;
}

/**
 * Scrollable list of translation key-value fields with inline dirty-state
 * highlighting and per-field reset controls.
 *
 * Filters the displayed keys by the provided search query. Shows an empty-state
 * message when there are no matching keys or no keys at all. Fields that differ
 * from their server-saved values are highlighted with a warning border, and a
 * reset icon button is shown to restore the saved value.
 *
 * @param props - The component props
 * @param props.localValues - Current display values (server values merged with local edits)
 * @param props.serverValues - Original server values used to detect dirtiness and to reset
 * @param props.search - Current search query used to filter visible translation keys
 * @param props.onChange - Callback invoked when the user edits a field value
 * @param props.onResetField - Callback invoked when the user resets a field to its saved value
 *
 * @returns JSX element rendering the list of translation fields
 *
 * @example
 * ```tsx
 * import TranslationFieldsView from './TranslationFieldsView';
 *
 * function Editor() {
 *   const [changes, setChanges] = useState<Record<string, string>>({});
 *   return (
 *     <TranslationFieldsView
 *       localValues={{'actions.save': 'Enregistrer'}}
 *       serverValues={{'actions.save': 'Save'}}
 *       search=""
 *       onChange={(key, val) => setChanges(prev => ({...prev, [key]: val}))}
 *       onResetField={(key) => setChanges(prev => { const n = {...prev}; delete n[key]; return n; })}
 *     />
 *   );
 * }
 * ```
 *
 * @public
 */
export default function TranslationFieldsView({
  localValues,
  serverValues,
  search,
  allowNewKeys,
  onChange,
  onResetField,
  draft,
  onDraftChange,
}: TranslationFieldsViewProps): JSX.Element {
  const {t} = useTranslation('translations');

  const allKeys = Object.keys(localValues);

  const filteredKeys = useMemo(() => {
    const q = search.toLowerCase();
    if (!q) return allKeys;
    return allKeys.filter((k) => k.toLowerCase().includes(q) || (localValues[k] ?? '').toLowerCase().includes(q));
  }, [allKeys, localValues, search]);

  const isDuplicateKey = draft.key.trim() !== '' && draft.key.trim() in localValues;

  const handleAddSubmit = () => {
    const key = draft.key.trim();
    if (!key || isDuplicateKey) return;
    onChange(key, draft.value);
    onDraftChange({adding: false, key: '', value: ''});
  };

  const handleAddCancel = () => {
    onDraftChange({adding: false, key: '', value: ''});
  };

  return (
    <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
      {allowNewKeys && (
        <Box>
          {!draft.adding ? (
            <Button
              size="small"
              startIcon={<Plus size={14} />}
              onClick={() => onDraftChange({...draft, adding: true})}
              sx={{textTransform: 'none'}}
            >
              {t('editor.addKey')}
            </Button>
          ) : (
            <Box
              sx={{
                display: 'flex',
                flexDirection: 'column',
                gap: 1,
                p: 1.5,
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 1,
              }}
            >
              <FormControl fullWidth>
                <FormLabel htmlFor="new-translation-key">{t('editor.addKey.keyLabel')}</FormLabel>
                <TextField
                  id="new-translation-key"
                  size="small"
                  placeholder={t('editor.addKey.keyPlaceholder')}
                  value={draft.key}
                  onChange={(e) => onDraftChange({...draft, key: e.target.value})}
                  error={isDuplicateKey}
                  helperText={isDuplicateKey ? t('editor.addKey.duplicateKey') : undefined}
                  sx={{'& .MuiInputBase-input': {fontFamily: 'monospace', fontSize: '0.8rem'}}}
                />
              </FormControl>
              <FormControl fullWidth>
                <FormLabel htmlFor="new-translation-value">{t('editor.addKey.valueLabel')}</FormLabel>
                <TextField
                  id="new-translation-value"
                  size="small"
                  placeholder={t('editor.addKey.valuePlaceholder')}
                  value={draft.value}
                  onChange={(e) => onDraftChange({...draft, value: e.target.value})}
                  multiline
                  minRows={1}
                  maxRows={4}
                />
              </FormControl>
              <Box sx={{display: 'flex', gap: 1}}>
                <Button
                  size="small"
                  variant="contained"
                  onClick={handleAddSubmit}
                  disabled={!draft.key.trim() || isDuplicateKey}
                  sx={{textTransform: 'none'}}
                >
                  {t('editor.addKey.submit')}
                </Button>
                <Button size="small" onClick={handleAddCancel} sx={{textTransform: 'none'}}>
                  {t('editor.addKey.cancel')}
                </Button>
              </Box>
            </Box>
          )}
        </Box>
      )}

      {filteredKeys.length === 0 ? (
        <Box sx={{py: 4, textAlign: 'center', color: 'text.secondary'}}>
          <Typography variant="body2">{t(search ? 'editor.noResults' : 'editor.noKeys')}</Typography>
        </Box>
      ) : (
        filteredKeys.map((key) => {
          const value = localValues[key] ?? '';
          const serverValue = serverValues[key] ?? '';
          const isDirty = value !== serverValue;

          return (
            <FormControl key={key} fullWidth>
              <FormLabel
                htmlFor={`field-${key}`}
                sx={{
                  fontFamily: 'monospace',
                  fontSize: '0.7rem',
                  fontWeight: isDirty ? 600 : 400,
                  color: isDirty ? 'warning.main' : 'text.secondary',
                }}
              >
                {key}
              </FormLabel>
              <Box sx={{display: 'flex', gap: 0.5, alignItems: 'flex-start'}}>
                <TextField
                  id={`field-${key}`}
                  size="small"
                  fullWidth
                  multiline
                  minRows={1}
                  maxRows={5}
                  value={value}
                  onChange={(e) => onChange(key, e.target.value)}
                  sx={{
                    '& .MuiOutlinedInput-root': isDirty
                      ? {
                          '& fieldset': {borderColor: 'warning.main'},
                          '&:hover fieldset': {borderColor: 'warning.dark'},
                          '&.Mui-focused fieldset': {borderColor: 'warning.main'},
                        }
                      : {},
                  }}
                />
                {isDirty && (
                  <Tooltip title={t('editor.resetField')}>
                    <IconButton
                      size="small"
                      aria-label={t('editor.resetField')}
                      onClick={() => onResetField(key)}
                      sx={{mt: 0.25, flexShrink: 0}}
                    >
                      <RotateCcw size={14} />
                    </IconButton>
                  </Tooltip>
                )}
              </Box>
            </FormControl>
          );
        })
      )}
    </Box>
  );
}
