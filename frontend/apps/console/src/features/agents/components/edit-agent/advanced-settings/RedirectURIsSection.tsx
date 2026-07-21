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

import {
  Alert,
  Box,
  Button,
  FormControl,
  FormLabel,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Plus, Trash} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {OAuth2Config} from '../../../../applications/models/oauth';
import type {OAuthAgentConfig} from '../../../models/agent';

const REDIRECT_USING_GRANTS = ['authorization_code'];

interface RedirectURIsSectionProps {
  oauth2Config?: OAuthAgentConfig;
  onOAuth2ConfigChange?: (updates: Partial<OAuth2Config>) => void;
  /**
   * Whether inputs should be disabled (e.g. read-only resource).
   */
  disabled?: boolean;
}

const isValidURL = (value: string): boolean => {
  try {
    return Boolean(new URL(value));
  } catch {
    return false;
  }
};

export default function RedirectURIsSection({
  oauth2Config = undefined,
  onOAuth2ConfigChange = undefined,
  disabled = false,
}: RedirectURIsSectionProps): JSX.Element | null {
  const {t} = useTranslation();
  const [errors, setErrors] = useState<Record<number, string>>({});

  const grantTypes = oauth2Config?.grantTypes ?? [];
  const usesRedirect = grantTypes.some((g) => REDIRECT_USING_GRANTS.includes(g));
  const uris = oauth2Config?.redirectUris ?? [];

  // The authorization-code grant cannot complete without at least one valid redirect URI. The
  // page-level Save guard computes this same check independently from state (see AgentEditPage),
  // since this section unmounts when its tab isn't active.
  const hasValidUri = uris.some((u) => {
    if (!u.trim()) return false;
    try {
      return Boolean(new URL(u));
    } catch {
      return false;
    }
  });
  const isMissingRequiredUri = usesRedirect && !hasValidUri;

  // Hide entirely when no redirect-using grant is selected — redirect URIs are meaningless then.
  if (!oauth2Config || !usesRedirect) return null;

  const isEditable = Boolean(onOAuth2ConfigChange) && !disabled;

  const commit = (next: string[]): void => {
    if (!isEditable) return;
    onOAuth2ConfigChange?.({redirectUris: next});
  };

  const handleChange = (index: number, value: string): void => {
    const next = [...uris];
    next[index] = value;
    if (value.trim()) {
      setErrors((prev) => {
        const copy = {...prev};
        delete copy[index];
        return copy;
      });
    }
    commit(next);
  };

  const handleBlur = (index: number): void => {
    const value = uris[index] ?? '';
    if (!value.trim()) {
      setErrors((prev) => ({
        ...prev,
        [index]: t('agents:edit.advanced.redirectUris.error.empty', 'URI cannot be empty'),
      }));
      return;
    }
    if (!isValidURL(value)) {
      setErrors((prev) => ({
        ...prev,
        [index]: t('agents:edit.advanced.redirectUris.error.invalid', 'Enter a valid URL'),
      }));
    }
  };

  const handleAdd = (): void => {
    commit([...uris, '']);
  };

  const handleRemove = (index: number): void => {
    const next = uris.filter((_, i) => i !== index);
    setErrors((prev) => {
      const copy: Record<number, string> = {};
      Object.entries(prev).forEach(([key, value]) => {
        const i = parseInt(key, 10);
        if (i < index) copy[i] = value;
        else if (i > index) copy[i - 1] = value;
      });
      return copy;
    });
    commit(next);
  };

  return (
    <Box>
      <FormLabel>{t('agents:edit.advanced.redirectUris.title', 'Authorized redirect URIs')}</FormLabel>
      <Typography variant="caption" color="text.secondary" sx={{display: 'block', mt: 0.5, mb: 1.5}}>
        {t('agents:edit.advanced.redirectUris.description', 'For use with requests from a web server')}
      </Typography>
      <FormControl fullWidth>
        <Stack spacing={2}>
          {isMissingRequiredUri && (
            <Alert severity="error" data-testid="agent-redirect-uris-required">
              {t(
                'agents:edit.advanced.redirectUris.required',
                'The Authorization Code grant requires at least one valid redirect URI.',
              )}
            </Alert>
          )}
          {uris.map((uri, index) => (
            // eslint-disable-next-line react/no-array-index-key
            <Stack key={index} direction="row" spacing={1} alignItems="flex-start">
              <FormControl fullWidth required sx={{flex: 1}}>
                <TextField
                  fullWidth
                  id={`agent-redirect-uri-${index}`}
                  value={uri}
                  onChange={(e) => handleChange(index, e.target.value)}
                  onBlur={() => handleBlur(index)}
                  error={!!errors[index]}
                  helperText={errors[index]}
                  placeholder="https://example.com/callback"
                  disabled={!isEditable}
                />
              </FormControl>
              {isEditable && (
                <Tooltip title={t('common:actions.delete', 'Delete')}>
                  <IconButton onClick={() => handleRemove(index)} color="error" sx={{mt: 1}}>
                    <Trash size={20} />
                  </IconButton>
                </Tooltip>
              )}
            </Stack>
          ))}
          {isEditable && (
            <Box>
              <Button variant="outlined" startIcon={<Plus />} onClick={handleAdd} size="small">
                {t('agents:edit.advanced.redirectUris.addUri', 'Add URI')}
              </Button>
            </Box>
          )}
        </Stack>
      </FormControl>
    </Box>
  );
}
