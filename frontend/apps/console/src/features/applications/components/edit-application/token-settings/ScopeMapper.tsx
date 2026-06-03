/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {Box, Stack, Typography, Chip, Alert, Divider} from '@wso2/oxygen-ui';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';
import TokenConstants from '../../../constants/token-constants';
import type {ScopeClaims} from '../../../models/oauth';

/**
 * Props for the {@link ScopeMapper} component.
 */
interface ScopeMapperProps {
  /**
   * Active OAuth2 scopes to map attributes to.
   */
  scopes: string[];
  /**
   * Current scope → attributes mapping.
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
 * Left panel shows the list of active scopes; selecting one reveals its
 * attribute mapping on the right. Mapped attributes appear as deletable chips,
 * and available attributes appear as outlined chips that can be clicked to add.
 *
 * @param props - Component props
 * @returns Scope attribute mapper UI
 */
export default function ScopeMapper({
  scopes,
  scopeClaims,
  userAttributes,
  isLoadingUserAttributes,
  onScopeClaimsChange,
  disabled = false,
}: ScopeMapperProps) {
  const {t} = useTranslation();
  const [selectedScope, setSelectedScope] = useState<string | null>(null);
  const effectiveScope = selectedScope && scopes.includes(selectedScope) ? selectedScope : (scopes[0] ?? null);

  const availableAttributes = Array.from(new Set([...userAttributes, ...TokenConstants.ADDITIONAL_USER_ATTRIBUTES]))
    .filter((attr) => !(TokenConstants.DEFAULT_TOKEN_ATTRIBUTES as readonly string[]).includes(attr))
    .sort();

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

  if (scopes.length === 0) {
    return (
      <Alert severity="info">
        {t(
          'applications:edit.token.scope_mapper.no_scopes',
          'Add at least one scope above to start mapping attributes.',
        )}
      </Alert>
    );
  }

  const mappedAttributes = effectiveScope ? (scopeClaims[effectiveScope] ?? []) : [];
  const unmappedAttributes = availableAttributes.filter((attr) => !mappedAttributes.includes(attr));

  return (
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
          width: 176,
          flexShrink: 0,
          borderRight: 1,
          borderColor: 'divider',
          bgcolor: 'background.default',
          overflowY: 'auto',
        }}
      >
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
                    'No attributes mapped yet — click an attribute below to add it',
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
  );
}
