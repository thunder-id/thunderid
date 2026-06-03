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

import {Box, Stack, Typography, Chip, Tooltip, TextField, Button} from '@wso2/oxygen-ui';
import {useState, useEffect, useRef} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Well-known OIDC scopes offered as quick-add chips when not yet active.
 */
const KNOWN_SCOPES = ['openid', 'profile', 'email', 'phone', 'address', 'groups', 'roles'] as const;

/**
 * Props for the {@link ScopeSelector} component.
 */
interface ScopeSelectorProps {
  /**
   * Current list of active OAuth2 scopes.
   */
  scopes: string[];
  /**
   * Callback fired whenever the scope list changes.
   */
  onScopesChange: (scopes: string[]) => void;
  /**
   * Singular noun used to refer to the entity in user-visible copy (default: 'application').
   */
  entityLabel?: string;
  /**
   * Whether inputs should be disabled (e.g. read-only resource).
   */
  disabled?: boolean;
}

/**
 * Self-contained scope management widget.
 *
 * - Active scopes are shown as deletable chips (default colour, × on all).
 * - Inactive well-known scopes appear as outlined chips that can be clicked to add.
 * - An input + Add button lets users add any custom scope name.
 * All changes are reflected immediately via `onScopesChange`.
 */
export default function ScopeSelector({
  scopes,
  onScopesChange,
  entityLabel = 'application',
  disabled = false,
}: ScopeSelectorProps) {
  const {t} = useTranslation();
  const [customScopeInput, setCustomScopeInput] = useState('');
  const [customScopeError, setCustomScopeError] = useState<string | null>(null);
  const [pendingRemovals, setPendingRemovals] = useState<Set<string>>(new Set());
  const [pendingAdditions, setPendingAdditions] = useState<Set<string>>(new Set());
  const removalTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());
  const additionTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());
  const scopesRef = useRef(scopes);

  useEffect(() => {
    scopesRef.current = scopes;
  }, [scopes]);

  // Derive filtered pending sets during render to avoid stale entries without needing effects
  const activePendingRemovals = new Set([...pendingRemovals].filter((s) => scopes.includes(s)));
  const activePendingAdditions = new Set([...pendingAdditions].filter((s) => !scopes.includes(s)));

  // Known scopes not yet active — shown as quick-add chips (exclude pending additions too)
  const inactiveKnownScopes = (KNOWN_SCOPES as readonly string[]).filter(
    (s) => !scopes.includes(s) && !activePendingAdditions.has(s),
  );

  // Cancel all timers on unmount
  useEffect(() => {
    const removals = removalTimers.current;
    const additions = additionTimers.current;
    return () => {
      removals.forEach((timer) => clearTimeout(timer));
      additions.forEach((timer) => clearTimeout(timer));
    };
  }, []);

  const handleRemove = (scope: string) => {
    // Stage the deletion visually (chip goes outlined)
    setPendingRemovals((prev) => new Set([...prev, scope]));

    // Cancel any existing timer for this scope
    const existing = removalTimers.current.get(scope);
    if (existing) clearTimeout(existing);

    // Commit the removal after a short delay (use ref to avoid stale closure)
    const timer = setTimeout(() => {
      onScopesChange(scopesRef.current.filter((s) => s !== scope));
      removalTimers.current.delete(scope);
    }, 600);
    removalTimers.current.set(scope, timer);
  };

  const handleAdd = (scope: string) => {
    if (!scopes.includes(scope)) {
      onScopesChange([...scopes, scope]);
    }
  };

  const handleAddCustom = () => {
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
    if (scopes.includes(trimmed) || pendingAdditions.has(trimmed)) {
      setCustomScopeError(
        t('applications:edit.token.scopes.add_custom.error.duplicate', 'This scope is already added'),
      );
      return;
    }

    // Show immediately as pending, then commit to parent after a short delay
    setPendingAdditions((prev) => new Set([...prev, trimmed]));

    const existing = additionTimers.current.get(trimmed);
    if (existing) clearTimeout(existing);

    const timer = setTimeout(() => {
      onScopesChange([...scopesRef.current, trimmed]);
      additionTimers.current.delete(trimmed);
    }, 600);
    additionTimers.current.set(trimmed, timer);

    setCustomScopeInput('');
    setCustomScopeError(null);
  };

  const hasSuggestions = inactiveKnownScopes.length > 0;

  return (
    <Box>
      <Typography variant="subtitle2" gutterBottom>
        {t('applications:edit.token.scopes.title', 'Scopes')}
      </Typography>
      <Typography variant="body2" color="text.disabled" sx={{mb: 2}}>
        {t('applications:edit.token.scopes.hint', 'Toggle the OAuth2 scopes available to this {{entity}}.', {
          entity: entityLabel,
        })}
      </Typography>

      <Stack spacing={2}>
        {/* Active scopes + pending custom additions */}
        {(scopes.length > 0 || activePendingAdditions.size > 0) && (
          <Box>
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{display: 'block', mb: 0.75, fontWeight: 600, textTransform: 'uppercase', letterSpacing: 0.5}}
            >
              {t('applications:edit.token.scopes.active_label', 'Active')}
            </Typography>
            <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
              {scopes.map((scope) => {
                const isPendingRemoval = activePendingRemovals.has(scope);
                return (
                  <Chip
                    key={scope}
                    label={scope}
                    size="small"
                    variant={isPendingRemoval ? 'outlined' : 'filled'}
                    color="default"
                    onDelete={disabled ? undefined : () => handleRemove(scope)}
                    sx={{transition: 'all 0.2s ease'}}
                  />
                );
              })}
              {[...activePendingAdditions].map((scope) => (
                <Chip
                  key={scope}
                  label={scope}
                  size="small"
                  variant="outlined"
                  color="primary"
                  onDelete={
                    disabled
                      ? undefined
                      : () => {
                          const timer = additionTimers.current.get(scope);
                          if (timer) clearTimeout(timer);
                          additionTimers.current.delete(scope);
                          setPendingAdditions((prev) => {
                            const next = new Set(prev);
                            next.delete(scope);
                            return next;
                          });
                        }
                  }
                  sx={{transition: 'all 0.2s ease'}}
                />
              ))}
            </Stack>
          </Box>
        )}

        {/* Suggested (inactive known) scopes */}
        {hasSuggestions && (
          <Box>
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{display: 'block', mb: 0.75, fontWeight: 600, textTransform: 'uppercase', letterSpacing: 0.5}}
            >
              {t('applications:edit.token.scopes.suggested_label', 'Suggested')}
            </Typography>
            <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
              {inactiveKnownScopes.map((scope) => (
                <Tooltip key={scope} title={t('applications:edit.token.click_to_add', 'Click to add')}>
                  <Chip
                    label={scope}
                    size="small"
                    variant="outlined"
                    onClick={disabled ? undefined : () => handleAdd(scope)}
                    sx={{cursor: disabled ? 'default' : 'pointer', borderStyle: 'dashed'}}
                  />
                </Tooltip>
              ))}
            </Stack>
          </Box>
        )}

        {/* Custom scope input */}
        <Box>
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{display: 'block', mb: 0.75, fontWeight: 600, textTransform: 'uppercase', letterSpacing: 0.5}}
          >
            {t('applications:edit.token.scopes.custom_label', 'Custom')}
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
                  handleAddCustom();
                }
              }}
              error={!!customScopeError}
              helperText={customScopeError ?? ''}
              sx={{width: 240}}
              disabled={disabled}
            />
            <Button variant="outlined" size="small" onClick={handleAddCustom} sx={{mt: '1px'}} disabled={disabled}>
              {t('applications:edit.token.scopes.add_custom.button', 'Add')}
            </Button>
          </Stack>
        </Box>
      </Stack>
    </Box>
  );
}
