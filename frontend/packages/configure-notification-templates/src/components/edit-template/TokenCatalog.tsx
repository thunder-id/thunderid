// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {I18nDefaultConstants, useGetTranslations, useUpdateTranslation} from '@thunderid/i18n';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, Button, IconButton, InputAdornment, Stack, TextField, Tooltip, Typography} from '@wso2/oxygen-ui';
import {ChevronLeft, Code, Languages, Palette, Plus, Search} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {CONTEXT_TOKENS, DESIGN_TOKENS, type TemplateToken} from '../../constants/template-tokens';
import type {NotificationChannel} from '../../models/notification-template';

/** Namespace notification-template {{t(key)}} tokens resolve under (matches the backend). */
const NOTIFICATION_NAMESPACE = 'notification';

/** Allowed characters for a translation key, matching the backend token grammar. */
const TOKEN_KEY_PATTERN = /^[A-Za-z0-9_.-]+$/;

/**
 * Props for the {@link TokenCatalog} component.
 *
 * @public
 */
export interface TokenCatalogProps {
  /** The channel being edited; design tokens are shown for email only. */
  channel: NotificationChannel;
  /** Called with the placeholder string to insert into the focused editor. */
  onInsert: (value: string) => void;
  /** Whether the catalog is collapsed to a narrow strip. */
  collapsed: boolean;
  /** Toggles the collapsed state. */
  onToggleCollapse: () => void;
}

function TokenGroup({
  icon,
  name,
  action = undefined,
  tokens,
  accent,
  onInsert,
  emptyLabel,
}: {
  icon: ReactNode;
  name: string;
  action?: ReactNode;
  tokens: readonly TemplateToken[];
  accent: 'primary' | 'warning' | 'success';
  onInsert: (value: string) => void;
  emptyLabel: string;
}): JSX.Element {
  return (
    <Box sx={{p: 1.75, borderTop: '1px solid', borderColor: 'divider', '&:first-of-type': {borderTop: 0}}}>
      <Box sx={{display: 'flex', alignItems: 'center', gap: 1, mb: 1, color: `${accent}.main`}}>
        {icon}
        <Typography variant="caption" sx={{fontWeight: 700, color: 'text.secondary'}}>
          {name}
        </Typography>
        {action && <Box sx={{ml: 'auto'}}>{action}</Box>}
      </Box>
      {tokens.length === 0 ? (
        <Typography variant="caption" sx={{display: 'block', textAlign: 'center', py: 1.5, color: 'text.disabled'}}>
          {emptyLabel}
        </Typography>
      ) : (
        <Box sx={{display: 'grid', gap: 0.5, maxHeight: 180, overflowY: 'auto', pr: 0.5}}>
          {tokens.map((token) => (
            <Button
              key={token.insert}
              onClick={() => onInsert(token.insert)}
              title={token.insert}
              sx={{
                justifyContent: 'flex-start',
                textTransform: 'none',
                fontFamily: 'monospace',
                fontSize: '0.6875rem',
                color: `${accent}.main`,
                px: 1,
                py: 0.75,
                minWidth: 0,
              }}
            >
              {token.label}
            </Button>
          ))}
        </Box>
      )}
    </Box>
  );
}

/**
 * Collapsible catalog of insertable placeholder tokens (translation + design + context), with
 * search. Translation keys are loaded from the i18n API (notification namespace) and new keys can
 * be added inline. Clicking a token inserts it into the focused editor.
 *
 * @public
 */
export default function TokenCatalog({channel, onInsert, collapsed, onToggleCollapse}: TokenCatalogProps): JSX.Element {
  const {t} = useTranslation('notificationTemplates');
  const [search, setSearch] = useState('');

  // Translation keys are language-neutral identifiers; list and add them against the base locale.
  const baseLocale = I18nDefaultConstants.FALLBACK_LANGUAGE;
  const {data: translationsData} = useGetTranslations({language: baseLocale, namespace: NOTIFICATION_NAMESPACE});
  const updateTranslation = useUpdateTranslation();

  const [adding, setAdding] = useState(false);
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');
  const [addError, setAddError] = useState<string | null>(null);

  const query = search.trim().toLowerCase();
  const match = (tokens: readonly TemplateToken[]): TemplateToken[] =>
    query ? tokens.filter((tok) => tok.label.toLowerCase().includes(query) || tok.insert.toLowerCase().includes(query)) : [...tokens];

  const translationTokens = useMemo(() => {
    const keys = Object.keys(translationsData?.translations?.[NOTIFICATION_NAMESPACE] ?? {});
    return match(keys.map((key) => ({label: key, insert: `{{t(${key})}}`})));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [translationsData, query]);

  const designTokens = useMemo(
    () => (channel === 'email' ? match(DESIGN_TOKENS) : []),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [channel, query],
  );
  const contextTokens = useMemo(
    () => match(CONTEXT_TOKENS),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [query],
  );

  const handleAddKey = (): void => {
    const key = newKey.trim();
    if (!key || !newValue.trim()) return;
    if (!TOKEN_KEY_PATTERN.test(key)) {
      setAddError(t('editor.tokens.add.invalidKey', 'Use only letters, numbers, dots, hyphens and underscores.'));
      return;
    }
    setAddError(null);
    updateTranslation.mutate(
      {language: baseLocale, namespace: NOTIFICATION_NAMESPACE, key, value: newValue},
      {
        onSuccess: () => {
          setNewKey('');
          setNewValue('');
          setAdding(false);
        },
        onError: (error) => {
          setAddError(getErrorMessage(error, t, 'editor.tokens.add.error', 'Failed to add translation key.'));
        },
      },
    );
  };

  if (collapsed) {
    return (
      <Box sx={{borderLeft: '1px solid', borderColor: 'divider', display: 'flex', justifyContent: 'center', p: 1}}>
        <Tooltip title={t('editor.tokens.expand', 'Show available tokens')}>
          <IconButton size="small" onClick={onToggleCollapse} aria-label={t('editor.tokens.expand', 'Show available tokens')}>
            <ChevronLeft size={16} style={{transform: 'rotate(180deg)'}} />
          </IconButton>
        </Tooltip>
      </Box>
    );
  }

  return (
    <Box sx={{borderLeft: '1px solid', borderColor: 'divider', display: 'flex', flexDirection: 'column', minWidth: 0}}>
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 1,
          px: 1.75,
          py: 1.5,
          borderBottom: '1px solid',
          borderColor: 'divider',
        }}
      >
        <Typography variant="caption" sx={{color: 'text.disabled'}}>
          {t('editor.tokens.hint', 'Select a token to insert it into the template')}
        </Typography>
        <Tooltip title={t('editor.tokens.collapse', 'Hide available tokens')}>
          <IconButton size="small" onClick={onToggleCollapse} aria-label={t('editor.tokens.collapse', 'Hide available tokens')}>
            <ChevronLeft size={16} />
          </IconButton>
        </Tooltip>
      </Box>

      <Box sx={{p: 1.75, pb: 0}}>
        <TextField
          size="small"
          fullWidth
          placeholder={t('editor.tokens.searchPlaceholder', 'Search available tokens...')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <Search size={14} />
              </InputAdornment>
            ),
          }}
        />
      </Box>

      <TokenGroup
        icon={<Languages size={16} />}
        name={t('editor.tokens.translation', 'Translation tokens')}
        accent="success"
        tokens={translationTokens}
        onInsert={onInsert}
        emptyLabel={t('editor.tokens.noTranslation', 'No translation keys match your search.')}
        action={
          <Tooltip title={t('editor.tokens.add.title', 'Add translation key')}>
            <IconButton size="small" onClick={() => setAdding((v) => !v)} aria-label={t('editor.tokens.add.title', 'Add translation key')}>
              <Plus size={14} />
            </IconButton>
          </Tooltip>
        }
      />

      {adding && (
        <Box sx={{px: 1.75, pb: 1.75, mt: -1}}>
          <Stack spacing={1}>
            {addError && (
              <Alert severity="error" sx={{py: 0}}>
                {addError}
              </Alert>
            )}
            <TextField
              size="small"
              fullWidth
              placeholder={t('editor.tokens.add.keyPlaceholder', 'Key (e.g. otp.subject)')}
              value={newKey}
              onChange={(e) => setNewKey(e.target.value)}
              inputProps={{sx: {fontFamily: 'monospace', fontSize: '0.75rem'}}}
            />
            <TextField
              size="small"
              fullWidth
              placeholder={t('editor.tokens.add.valuePlaceholder', 'Translation value')}
              value={newValue}
              onChange={(e) => setNewValue(e.target.value)}
            />
            <Stack direction="row" spacing={1} justifyContent="flex-end">
              <Button size="small" onClick={() => setAdding(false)} disabled={updateTranslation.isPending}>
                {t('common:actions.cancel')}
              </Button>
              <Button
                size="small"
                variant="contained"
                onClick={handleAddKey}
                disabled={!newKey.trim() || !newValue.trim() || updateTranslation.isPending}
              >
                {t('editor.tokens.add.submit', 'Add')}
              </Button>
            </Stack>
          </Stack>
        </Box>
      )}

      {channel === 'email' && (
        <TokenGroup
          icon={<Palette size={16} />}
          name={t('editor.tokens.design', 'Design tokens')}
          tokens={designTokens}
          accent="primary"
          onInsert={onInsert}
          emptyLabel={t('editor.tokens.noDesign', 'No design tokens match your search.')}
        />
      )}
      <TokenGroup
        icon={<Code size={16} />}
        name={t('editor.tokens.context', 'Context tokens')}
        tokens={contextTokens}
        accent="warning"
        onInsert={onInsert}
        emptyLabel={t('editor.tokens.noContext', 'No context placeholders match your search.')}
      />
    </Box>
  );
}
