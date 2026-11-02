// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {
  useGetLanguages,
  useGetTranslations,
  useUpdateTranslation,
  NamespaceConstants,
  I18nDefaultConstants,
} from '@thunderid/i18n';
import {isI18nTemplatePattern, I18N_KEY_PATTERN} from '@thunderid/utils';
import {
  Alert,
  Autocomplete,
  type AutocompleteRenderInputParams,
  Box,
  Button,
  Card,
  CardContent,
  CardHeader,
  CircularProgress,
  Divider,
  FormControl,
  FormLabel,
  IconButton,
  InputAdornment,
  Popover,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {PlusIcon, SquareFunction, XIcon} from '@wso2/oxygen-ui-icons-react';
import {type ChangeEvent, type ReactElement, type SyntheticEvent, useCallback, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import PageLoadingAnimation from '../lab/components/PageLoadingAnimation';

/**
 * UI strings displayed by {@link I18nTextInput}. All fields are optional; English fallbacks
 * are used for any string not provided. Callers typically resolve these from their own i18n
 * namespace and pass the resolved strings down so the component stays namespace-agnostic.
 */
export interface I18nTextInputLabels {
  /** Tooltip on the icon button that opens the popover. Default: "Configure translation". */
  triggerTooltip?: string;
  /** Heading inside the popover when selecting an existing key. Default: "Translation". */
  popoverTitle?: string;
  /** Heading inside the popover when creating a new key. Default: "Create new translation". */
  createTitle?: string;
  /** Tooltip on the "create new" button. Default: "Create a new translation key". */
  createTooltip?: string;
  /** Label for the language selector. Default: "Language". */
  languageLabel?: string;
  /** Label for the translation key field. Default: "Translation key". */
  keyLabel?: string;
  /** Placeholder for the key autocomplete in select mode. Default: "Select a translation key". */
  selectKeyPlaceholder?: string;
  /** Label for the translation value field. Default: "Translation value". */
  valueLabel?: string;
  /** Caption above the rendered translation. Default: "Resolved value". */
  resolvedValueLabel?: string;
  /** Error message when the key field is empty on create. Default: "Translation key is required". */
  keyRequiredError?: string;
  /** Error message when the value field is empty on create. Default: "Translation value is required". */
  valueRequiredError?: string;
  /**
   * Error message when the key contains characters outside `[a-zA-Z0-9._-]`. Default: "Key may only
   * contain letters, numbers, dots, hyphens, and underscores".
   */
  invalidKeyFormatError?: string;
  /** Cancel button label. Default: "Cancel". */
  cancelLabel?: string;
  /** Create button label. Default: "Create". */
  createLabel?: string;
  /** Close (X) button aria-label. Default: "Close". */
  closeLabel?: string;
  /** Generic fallback used when the create-translation API returns an unparseable error. Default: "An unknown error occurred". */
  unknownError?: string;
}

const DEFAULT_LABELS: Required<I18nTextInputLabels> = {
  triggerTooltip: 'Configure translation',
  popoverTitle: 'Translation',
  createTitle: 'Create new translation',
  createTooltip: 'Create a new translation key',
  languageLabel: 'Language',
  keyLabel: 'Translation key',
  selectKeyPlaceholder: 'Select a translation key',
  valueLabel: 'Translation value',
  resolvedValueLabel: 'Resolved value',
  keyRequiredError: 'Translation key is required',
  valueRequiredError: 'Translation value is required',
  invalidKeyFormatError: 'Key may only contain letters, numbers, dots, hyphens, and underscores',
  cancelLabel: 'Cancel',
  createLabel: 'Create',
  closeLabel: 'Close',
  unknownError: 'An unknown error occurred',
};

/**
 * Sanitizes a string for use as a translation key.
 * Replaces spaces with underscores, lowercases, and strips invalid characters.
 */
function sanitizeTranslationKey(key: string): string {
  return key
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '_')
    .replace(/[^a-zA-Z0-9._-]/g, '');
}

/**
 * Props interface of {@link I18nTextInput}
 */
export interface I18nTextInputProps {
  label: string;
  value: string;
  onChange: (newValue: string) => void;
  placeholder?: string;
  defaultNewKey?: string;
  /** Optional UI string overrides; English fallbacks are used for any not provided. */
  labels?: I18nTextInputLabels;
  /**
   * Optional callback invoked after a new translation has been successfully persisted via the
   * i18n API. Hosts wire this up to their own translation cache invalidation logic.
   */
  onTranslationCreated?: () => void;
}

/**
 * Props for the i18n content component.
 */
interface I18nContentProps {
  i18nKey: string;
  isActive: boolean;
  isCreateMode: boolean;
  onChange: (key: string) => void;
  onCreateModeChange: (isCreateMode: boolean) => void;
  defaultNewKey?: string;
  labels: Required<I18nTextInputLabels>;
  onTranslationCreated?: () => void;
}

/**
 * Content component for the i18n popover with select and create modes.
 */
function I18nContent({
  i18nKey,
  isActive,
  isCreateMode,
  onChange,
  onCreateModeChange,
  defaultNewKey = undefined,
  labels,
  onTranslationCreated = undefined,
}: I18nContentProps): ReactElement {
  const {i18n} = useTranslation();
  const {data: languagesData} = useGetLanguages();
  const {data: translationsData, isLoading} = useGetTranslations({
    language: I18nDefaultConstants.FALLBACK_LANGUAGE,
    namespace: NamespaceConstants.CUSTOM_NAMESPACE,
    enabled: isActive,
  });
  const updateTranslation = useUpdateTranslation({
    onMutationSuccess: () => {
      onTranslationCreated?.();
    },
  });

  const sanitizedDefaultKey = defaultNewKey ? sanitizeTranslationKey(defaultNewKey) : '';
  const [newKey, setNewKey] = useState(sanitizedDefaultKey);
  const [newValue, setNewValue] = useState('');
  const [selectedLanguage, setSelectedLanguage] = useState(I18nDefaultConstants.FALLBACK_LANGUAGE as string);
  const [error, setError] = useState<string | null>(null);

  const availableKeys = useMemo(() => {
    if (!translationsData?.translations) return [];

    const keys: string[] = [];
    Object.entries(translationsData.translations).forEach(([namespace, translations]) => {
      keys.push(...Object.keys(translations).map((key: string) => `${namespace}:${key}`));
    });
    return keys;
  }, [translationsData]);

  const resolvedValue = useMemo(() => {
    if (!i18nKey || !translationsData?.translations) return '';

    // i18nKey may be namespaced (e.g. "custom:myKey") — split and look up in the correct namespace
    const colonIdx = i18nKey.indexOf(':');
    if (colonIdx !== -1) {
      const ns = i18nKey.slice(0, colonIdx);
      const bareKey = i18nKey.slice(colonIdx + 1);
      return translationsData.translations[ns]?.[bareKey] ?? '';
    }

    // Bare key — search across all namespaces
    let found = '';
    Object.values(translationsData.translations).some((translations) => {
      if (translations[i18nKey]) {
        found = translations[i18nKey];
        return true;
      }
      return false;
    });
    return found;
  }, [i18nKey, translationsData]);

  const availableLanguages = useMemo(() => {
    if (languagesData?.languages && languagesData.languages.length > 0) {
      return languagesData.languages;
    }
    return [I18nDefaultConstants.FALLBACK_LANGUAGE];
  }, [languagesData]);

  const resetCreateForm = useCallback(() => {
    setNewKey(sanitizedDefaultKey);
    setNewValue('');
    setSelectedLanguage(I18nDefaultConstants.FALLBACK_LANGUAGE as string);
    setError(null);
  }, [sanitizedDefaultKey]);

  const handleCreate = useCallback(() => {
    if (!newKey.trim()) {
      setError(labels.keyRequiredError);
      return;
    }
    if (!newValue.trim()) {
      setError(labels.valueRequiredError);
      return;
    }
    if (!/^[a-zA-Z0-9._-]+$/.test(newKey)) {
      setError(labels.invalidKeyFormatError);
      return;
    }

    updateTranslation.mutate(
      {
        language: selectedLanguage,
        namespace: NamespaceConstants.CUSTOM_NAMESPACE,
        key: newKey,
        value: newValue,
      },
      {
        onSuccess: () => {
          // Synchronously add the new translation to i18next so t() can resolve it
          // immediately when the parent re-renders (before the async I18nProvider refresh)
          i18n.addResourceBundle(
            selectedLanguage,
            NamespaceConstants.CUSTOM_NAMESPACE,
            {[newKey]: newValue},
            true,
            true,
          );
          onChange(`${NamespaceConstants.CUSTOM_NAMESPACE}:${newKey}`);
          onCreateModeChange(false);
          resetCreateForm();
        },
        onError: (err: Error) => {
          setError(err.message ?? labels.unknownError);
        },
      },
    );
  }, [
    newKey,
    newValue,
    selectedLanguage,
    updateTranslation,
    onChange,
    onCreateModeChange,
    resetCreateForm,
    labels,
    i18n,
  ]);

  if (isLoading) {
    return <PageLoadingAnimation />;
  }

  if (isCreateMode) {
    return (
      <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
        {error && (
          <Alert severity="error" onClose={() => setError(null)}>
            {error}
          </Alert>
        )}
        <div>
          <Typography variant="subtitle2" gutterBottom>
            {labels.languageLabel}
          </Typography>
          <Autocomplete
            options={availableLanguages}
            value={selectedLanguage}
            onChange={(_e: SyntheticEvent, newLang: string | null) =>
              setSelectedLanguage(newLang ?? I18nDefaultConstants.FALLBACK_LANGUAGE)
            }
            renderInput={(params: AutocompleteRenderInputParams) => <TextField {...params} size="small" />}
            disableClearable
          />
        </div>
        <div>
          <Typography variant="subtitle2" gutterBottom>
            {labels.keyLabel}
          </Typography>
          <TextField
            fullWidth
            size="small"
            value={newKey}
            onChange={(e: ChangeEvent<HTMLInputElement>) => {
              setNewKey(e.target.value);
              if (error) setError(null);
            }}
            placeholder="e.g., user.firstName"
          />
        </div>
        <div>
          <Typography variant="subtitle2" gutterBottom>
            {labels.valueLabel}
          </Typography>
          <TextField
            fullWidth
            size="small"
            multiline
            rows={2}
            value={newValue}
            onChange={(e: ChangeEvent<HTMLInputElement>) => {
              setNewValue(e.target.value);
              if (error) setError(null);
            }}
            placeholder="e.g., First Name"
          />
        </div>
        <Box sx={{display: 'flex', gap: 1, justifyContent: 'flex-end'}}>
          <Button
            variant="text"
            onClick={() => {
              onCreateModeChange(false);
              resetCreateForm();
            }}
          >
            {labels.cancelLabel}
          </Button>
          <Button
            variant="contained"
            onClick={handleCreate}
            disabled={updateTranslation.isPending || !newKey.trim() || !newValue.trim()}
          >
            {updateTranslation.isPending ? <CircularProgress size={16} /> : labels.createLabel}
          </Button>
        </Box>
      </Box>
    );
  }

  return (
    <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
      <div>
        <Typography variant="subtitle2" gutterBottom>
          {labels.keyLabel}
        </Typography>
        <Autocomplete
          options={availableKeys}
          value={i18nKey === '' ? null : i18nKey}
          onChange={(_e: SyntheticEvent, selected: string | null) => onChange(selected ?? '')}
          renderInput={(params: AutocompleteRenderInputParams) => (
            <TextField {...params} placeholder={labels.selectKeyPlaceholder} size="small" />
          )}
          renderOption={({key, ...props}: React.HTMLAttributes<HTMLLIElement> & {key: string}, option: string) => (
            <li key={key} {...props}>
              <span>{option}</span>
            </li>
          )}
        />
      </div>

      {i18nKey && resolvedValue && (
        <Box
          sx={{
            p: 1.5,
            backgroundColor: 'action.hover',
            borderRadius: 1,
            border: '1px solid',
            borderColor: 'divider',
          }}
        >
          <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 0.5}}>
            {labels.resolvedValueLabel}
          </Typography>
          <Typography variant="body2" sx={{wordBreak: 'break-word'}}>
            {resolvedValue}
          </Typography>
        </Box>
      )}

      <Divider />

      <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center'}}>
        <Tooltip title={labels.createTooltip}>
          <Button variant="text" startIcon={<PlusIcon size={16} />} onClick={() => onCreateModeChange(true)}>
            {labels.createTitle}
          </Button>
        </Tooltip>
      </Box>
    </Box>
  );
}

/**
 * Props for the i18n popover component.
 */
interface I18nPopoverProps {
  open: boolean;
  anchorEl: HTMLElement | null;
  onClose: () => void;
  value: string;
  onChange: (newValue: string) => void;
  defaultNewKey?: string;
  labels: Required<I18nTextInputLabels>;
  onTranslationCreated?: () => void;
}

/**
 * Popover with i18n key selection and creation UI.
 */
function I18nPopover({
  open,
  anchorEl,
  onClose,
  value,
  onChange,
  defaultNewKey = undefined,
  labels,
  onTranslationCreated = undefined,
}: I18nPopoverProps): ReactElement {
  const [isCreateMode, setIsCreateMode] = useState(false);

  const handleClose = useCallback(() => {
    setIsCreateMode(false);
    onClose();
  }, [onClose]);

  const i18nKey = useMemo(() => I18N_KEY_PATTERN.exec(value.trim())?.[1] ?? '', [value]);

  const handleChange = useCallback(
    (key: string) => {
      onChange(key ? `{{t(${key})}}` : '');
    },
    [onChange],
  );

  return (
    <Popover
      open={open}
      anchorEl={anchorEl}
      onClose={handleClose}
      anchorOrigin={{vertical: 'top', horizontal: 'right'}}
      transformOrigin={{vertical: 'top', horizontal: 'left'}}
    >
      <Card sx={{width: 400}}>
        <CardHeader
          title={isCreateMode ? labels.createTitle : labels.popoverTitle}
          action={
            <IconButton aria-label={labels.closeLabel} onClick={handleClose} size="small">
              <XIcon />
            </IconButton>
          }
        />
        <CardContent>
          <I18nContent
            i18nKey={i18nKey}
            isActive={open}
            isCreateMode={isCreateMode}
            onChange={handleChange}
            onCreateModeChange={setIsCreateMode}
            defaultNewKey={defaultNewKey}
            labels={labels}
            onTranslationCreated={onTranslationCreated}
          />
        </CardContent>
      </Card>
    </Popover>
  );
}

/**
 * A text input with an i18n button that opens a popover for selecting or creating translation
 * keys. The component is i18n-namespace-agnostic — callers pass their own UI strings via the
 * optional `labels` prop and wire up post-create cache invalidation via `onTranslationCreated`.
 */
export default function I18nTextInput({
  label,
  value,
  onChange,
  placeholder = undefined,
  defaultNewKey = undefined,
  labels: labelsProp = undefined,
  onTranslationCreated = undefined,
}: I18nTextInputProps): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();
  const [iconButtonEl, setIconButtonEl] = useState<HTMLButtonElement | null>(null);
  const [isPopoverOpen, setIsPopoverOpen] = useState(false);

  const labels = useMemo<Required<I18nTextInputLabels>>(() => ({...DEFAULT_LABELS, ...labelsProp}), [labelsProp]);

  const isDynamic = isI18nTemplatePattern(value);
  const resolvedValue = isDynamic ? (resolve(value, {t}) ?? '') : '';

  return (
    <FormControl fullWidth>
      <FormLabel>{label}</FormLabel>
      <TextField
        fullWidth
        value={value}
        onChange={(e: ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
        placeholder={placeholder}
        size="small"
        sx={
          isDynamic
            ? {
                '& .MuiOutlinedInput-root': {
                  backgroundColor: 'rgba(var(--mui-palette-primary-mainChannel) / 0.1)',
                  '& fieldset': {borderColor: 'primary.main'},
                  '&:hover fieldset': {borderColor: 'primary.dark'},
                  '&.Mui-focused fieldset': {borderColor: 'primary.main'},
                },
              }
            : undefined
        }
        InputProps={{
          endAdornment: (
            <InputAdornment position="end">
              <Tooltip title={labels.triggerTooltip}>
                <IconButton
                  ref={setIconButtonEl}
                  onClick={() => setIsPopoverOpen(!isPopoverOpen)}
                  size="small"
                  edge="end"
                  color={isDynamic ? 'primary' : 'default'}
                >
                  <SquareFunction size={16} />
                </IconButton>
              </Tooltip>
            </InputAdornment>
          ),
        }}
      />
      {isDynamic && resolvedValue && (
        <Box
          sx={{
            mt: 1,
            p: 1.5,
            backgroundColor: 'action.hover',
            borderRadius: 1,
            border: '1px solid',
            borderColor: 'divider',
          }}
        >
          <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 0.5}}>
            {labels.resolvedValueLabel}
          </Typography>
          <Typography variant="body2" sx={{wordBreak: 'break-word'}}>
            {resolvedValue}
          </Typography>
        </Box>
      )}
      <I18nPopover
        open={isPopoverOpen}
        anchorEl={iconButtonEl}
        onClose={() => setIsPopoverOpen(false)}
        value={value}
        onChange={onChange}
        defaultNewKey={defaultNewKey}
        labels={labels}
        onTranslationCreated={onTranslationCreated}
      />
    </FormControl>
  );
}
