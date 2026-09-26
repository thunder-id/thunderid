// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Stack, Typography, Chip, Divider, IconButton, Tooltip, TextField, Button} from '@wso2/oxygen-ui';
import {Trash2} from '@wso2/oxygen-ui-icons-react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';
import TokenConstants from '../../../constants/token-constants';
import type {ScopeClaims} from '@thunderid/configure-applications';

/**
 * Props for the {@link ScopeMapper} component.
 */
interface ScopeMapperProps {
  /**
   * Current scope → attributes mapping. Its keys are the scopes available to the application.
   */
  scopeClaims: ScopeClaims;
  /**
   * All available user attributes derived from user types.
   */
  userAttributes: string[];
  /**
   * Whether user attributes are still loading.
   */
  isLoadingUserAttributes: boolean;
  /**
   * Callback fired when the scope → attributes mapping changes.
   */
  onScopeClaimsChange: (scopeClaims: ScopeClaims) => void;
  /**
   * Whether inputs should be disabled (e.g. read-only resource).
   */
  disabled?: boolean;
}

/**
 * Two-panel UI for mapping user attributes to OAuth2 scopes.
 *
 * Left panel lists the mapped scopes and lets scopes be added or removed; selecting one reveals its
 * attribute mapping on the right. Mapped attributes appear as deletable chips, and available
 * attributes appear as outlined chips that can be clicked to add. Removing a scope removes it from
 * the mapping, which is what makes it unavailable to the application.
 *
 * @param props - Component props
 * @returns Scope attribute mapper UI
 */
export default function ScopeMapper({
  scopeClaims,
  userAttributes,
  isLoadingUserAttributes,
  onScopeClaimsChange,
  disabled = false,
}: ScopeMapperProps) {
  const {t} = useTranslation();
  const [selectedScope, setSelectedScope] = useState<string | null>(null);
  const [customScopeInput, setCustomScopeInput] = useState('');
  const [customScopeError, setCustomScopeError] = useState<string | null>(null);

  const scopes = Object.keys(scopeClaims).sort();
  const effectiveScope = selectedScope && scopes.includes(selectedScope) ? selectedScope : (scopes[0] ?? null);

  const availableAttributes = Array.from(new Set([...userAttributes, ...TokenConstants.ADDITIONAL_USER_ATTRIBUTES]))
    .filter((attr) => !(TokenConstants.DEFAULT_TOKEN_ATTRIBUTES as readonly string[]).includes(attr))
    .sort();

  const handleAddScope = (scope: string) => {
    if (scope in scopeClaims) return;
    onScopeClaimsChange({...scopeClaims, [scope]: []});
    setSelectedScope(scope);
  };

  const handleAddCustomScope = () => {
    const trimmed = customScopeInput.trim();
    if (!trimmed) {
      setCustomScopeError(t('applications:edit.token.scopes.add_custom.error.empty', 'Scope name cannot be empty'));
      return;
    }
    if (/\s/.test(trimmed)) {
      setCustomScopeError(
        t('applications:edit.token.scopes.add_custom.error.invalid', 'Scope name must not contain spaces'),
      );
      return;
    }
    if (trimmed in scopeClaims) {
      setCustomScopeError(
        t('applications:edit.token.scopes.add_custom.error.duplicate', 'This scope is already added'),
      );
      return;
    }

    handleAddScope(trimmed);
    setCustomScopeInput('');
    setCustomScopeError(null);
  };

  const handleRemoveScope = (scope: string) => {
    const updated = {...scopeClaims};
    delete updated[scope];
    onScopeClaimsChange(updated);
    if (selectedScope === scope) setSelectedScope(null);
  };

  const handleAdd = (attr: string) => {
    if (!effectiveScope) return;
    const current = scopeClaims[effectiveScope] ?? [];
    if (!current.includes(attr)) {
      onScopeClaimsChange({...scopeClaims, [effectiveScope]: [...current, attr]});
    }
  };

  const handleRemove = (attr: string) => {
    if (!effectiveScope) return;
    const current = scopeClaims[effectiveScope] ?? [];
    onScopeClaimsChange({...scopeClaims, [effectiveScope]: current.filter((a) => a !== attr)});
  };

  const mappedAttributes = effectiveScope ? (scopeClaims[effectiveScope] ?? []) : [];
  const unmappedAttributes = availableAttributes.filter((attr) => !mappedAttributes.includes(attr));

  return (
    <Stack spacing={2}>
      <Box
        sx={{
          display: 'flex',
          border: 1,
          borderColor: 'divider',
          borderRadius: 1,
          overflow: 'hidden',
          minHeight: 220,
        }}
      >
        {/* ── Left: Scope list ───────────────────────────────────────── */}
        <Box
          sx={{
            width: 200,
            flexShrink: 0,
            borderRight: 1,
            borderColor: 'divider',
            bgcolor: 'background.default',
            overflowY: 'auto',
          }}
        >
          {scopes.length === 0 && (
            <Typography variant="body2" color="text.disabled" sx={{p: 1.5, fontStyle: 'italic'}}>
              {t('applications:edit.token.scope_mapper.no_scopes', 'Add a scope to start mapping attributes.')}
            </Typography>
          )}

          {scopes.map((scope) => {
            const count = (scopeClaims[scope] ?? []).length;
            const isSelected = effectiveScope === scope;

            return (
              <Box
                key={scope}
                role="button"
                tabIndex={0}
                onClick={() => setSelectedScope(scope)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') setSelectedScope(scope);
                }}
                sx={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  px: 1.5,
                  py: 1.25,
                  cursor: 'pointer',
                  borderBottom: 1,
                  borderColor: 'divider',
                  bgcolor: isSelected ? 'action.selected' : 'transparent',
                  '&:hover': {
                    bgcolor: isSelected ? 'action.selected' : 'action.hover',
                  },
                  '&:hover .scope-delete-btn': {opacity: 1},
                  transition: 'background-color 0.15s ease',
                  '&:last-child': {borderBottom: 0},
                }}
              >
                <Typography variant="body2" fontWeight={isSelected ? 600 : 400} noWrap sx={{flex: 1, mr: 1}}>
                  {scope}
                </Typography>
                <Chip
                  label={count}
                  size="small"
                  color={count > 0 ? 'primary' : 'default'}
                  variant={count > 0 ? 'filled' : 'outlined'}
                  sx={{
                    minWidth: 26,
                    height: 20,
                    fontSize: 11,
                    fontWeight: 600,
                    pointerEvents: 'none',
                    '& .MuiChip-label': {px: 0.75},
                  }}
                />
                {!disabled && (
                  <Tooltip title={t('applications:edit.token.scope_mapper.remove_scope', 'Remove scope')}>
                    <IconButton
                      className="scope-delete-btn"
                      size="small"
                      onClick={(e): void => {
                        e.stopPropagation();
                        handleRemoveScope(scope);
                      }}
                      sx={{
                        ml: 0.5,
                        opacity: 0,
                        transition: 'opacity 0.15s ease',
                        // The row's hover rule is the only other thing that reveals this, so
                        // without a focus rule a keyboard user tabs onto an invisible button.
                        '&:focus-visible': {opacity: 1},
                      }}
                    >
                      <Trash2 size={14} />
                    </IconButton>
                  </Tooltip>
                )}
              </Box>
            );
          })}
        </Box>

        {/* ── Right: Attribute mapping panel ─────────────────────────── */}
        <Box sx={{flex: 1, p: 2, overflow: 'auto', bgcolor: 'background.paper'}}>
          {effectiveScope ? (
            <Stack spacing={2}>
              {/* Mapped attributes */}
              <Box>
                <Typography
                  variant="caption"
                  color="text.secondary"
                  sx={{
                    display: 'block',
                    mb: 1,
                    fontWeight: 600,
                    textTransform: 'uppercase',
                    letterSpacing: 0.5,
                  }}
                >
                  {t('applications:edit.token.scope_mapper.mapped_label', 'Mapped Attributes')}
                </Typography>

                {mappedAttributes.length === 0 ? (
                  <Typography variant="body2" color="text.disabled" sx={{fontStyle: 'italic'}}>
                    {t(
                      'applications:edit.token.scope_mapper.no_mapped',
                      'No attributes mapped yet, click an attribute below to add it',
                    )}
                  </Typography>
                ) : (
                  <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                    {mappedAttributes.map((attr) => (
                      <Chip
                        key={attr}
                        label={attr}
                        size="small"
                        color="primary"
                        onDelete={disabled ? undefined : () => handleRemove(attr)}
                      />
                    ))}
                  </Stack>
                )}
              </Box>

              <Divider />

              {/* Available attributes */}
              <Box>
                <Typography
                  variant="caption"
                  color="text.secondary"
                  sx={{
                    display: 'block',
                    mb: 1,
                    fontWeight: 600,
                    textTransform: 'uppercase',
                    letterSpacing: 0.5,
                  }}
                >
                  {t('applications:edit.token.scope_mapper.available_label', 'Available Attributes')}
                </Typography>

                {isLoadingUserAttributes && (
                  <Typography variant="body2" color="text.secondary">
                    {t('applications:edit.token.scope_mapper.loading', 'Loading available attributes...')}
                  </Typography>
                )}

                {!isLoadingUserAttributes && unmappedAttributes.length === 0 && (
                  <Typography variant="body2" color="text.disabled" sx={{fontStyle: 'italic'}}>
                    {t(
                      'applications:edit.token.scope_mapper.all_mapped',
                      'All available attributes are already mapped to this scope',
                    )}
                  </Typography>
                )}

                {!isLoadingUserAttributes && unmappedAttributes.length > 0 && (
                  <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                    {unmappedAttributes.map((attr) => (
                      <Chip
                        key={attr}
                        label={attr}
                        size="small"
                        variant="outlined"
                        onClick={disabled ? undefined : () => handleAdd(attr)}
                        sx={{
                          cursor: disabled ? 'default' : 'pointer',
                          borderStyle: 'dashed',
                          '&:hover': {
                            borderStyle: 'solid',
                            bgcolor: disabled ? 'transparent' : 'action.hover',
                          },
                          transition: 'all 0.15s ease',
                        }}
                      />
                    ))}
                  </Stack>
                )}
              </Box>
            </Stack>
          ) : null}
        </Box>
      </Box>

      {/* ── Add a scope ────────────────────────────────────────────── */}
      <Box>
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{display: 'block', mb: 0.75, fontWeight: 600, textTransform: 'uppercase', letterSpacing: 0.5}}
        >
          {t('applications:edit.token.scopes.add_label', 'Add Scope')}
        </Typography>
        <Stack direction="row" spacing={1} alignItems="flex-start">
          <TextField
            size="small"
            placeholder={t('applications:edit.token.scopes.add_custom.placeholder', 'e.g. custom:read')}
            value={customScopeInput}
            onChange={(e) => {
              setCustomScopeInput(e.target.value);
              if (customScopeError) setCustomScopeError(null);
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                handleAddCustomScope();
              }
            }}
            error={!!customScopeError}
            helperText={customScopeError ?? ''}
            sx={{width: 240}}
            disabled={disabled}
          />
          <Button variant="outlined" size="small" onClick={handleAddCustomScope} sx={{mt: '1px'}} disabled={disabled}>
            {t('applications:edit.token.scopes.add_custom.button', 'Add')}
          </Button>
        </Stack>
      </Box>
    </Stack>
  );
}
