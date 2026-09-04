// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AllowedOriginTypes, createRow, rowKey, type AllowedOriginDraftRow} from '@thunderid/configure-settings';
import {
  Alert,
  Box,
  Button,
  Checkbox,
  FormControl,
  FormLabel,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Plus, Trash} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import CorsOriginsEditor from './CorsOriginsEditor';
import DevServerLogo from './DevServerLogo';
import useApplicationCreate from '../../contexts/ApplicationCreate/useApplicationCreate';
import {isValidRedirectUriTransport} from '../../utils/isValidRedirectUriFormat';

/** Pure URI-format check, permissive of path/host wildcards (the backend enforces wildcard rules). */
function isValidUriFormat(uri: string): boolean {
  try {
    const schemeEnd = uri.indexOf('://');
    let uriForValidation = uri;
    if (schemeEnd !== -1) {
      const pathStart = uri.indexOf('/', schemeEnd + 3);
      const hostPart = pathStart !== -1 ? uri.slice(schemeEnd + 3, pathStart) : uri.slice(schemeEnd + 3);
      if (hostPart.includes('*')) {
        const sanitizedHost = hostPart.replace(/\*/g, 'wildcard-placeholder');
        uriForValidation = uri.slice(0, schemeEnd + 3) + sanitizedHost + (pathStart !== -1 ? uri.slice(pathStart) : '');
      }
    }
    // eslint-disable-next-line no-new
    new URL(uriForValidation);
    return true;
  } catch {
    return false;
  }
}

interface UriListEditorProps {
  /** Already-translated field label. */
  title: string;
  /** Already-translated description. */
  description: string;
  /** Already-translated input placeholder. */
  placeholder: string;
  /** Already-translated "add" button label. */
  addLabel: string;
  uris: string[];
  onUrisChange: (uris: string[]) => void;
  /** Whether an empty row is an error (redirect URIs are required, post-logout ones are optional). */
  required: boolean;
}

function UriListEditor({
  title,
  description,
  placeholder,
  addLabel,
  uris,
  onUrisChange,
  required,
}: UriListEditorProps): JSX.Element {
  const {t} = useTranslation();
  const [errors, setErrors] = useState<Record<number, string>>({});

  const validate = (uri: string, index: number): boolean => {
    if (!uri || uri.trim() === '') {
      if (required) {
        setErrors((prev) => ({
          ...prev,
          [index]: t('applications:edit.general.redirectUris.error.empty', 'Invalid Redirect: URI must not be empty.'),
        }));
        return false;
      }
      setErrors((prev) => {
        const next = {...prev};
        delete next[index];
        return next;
      });
      return false;
    }
    if (!isValidUriFormat(uri) || (required && !isValidRedirectUriTransport(uri))) {
      setErrors((prev) => ({
        ...prev,
        [index]: t(
          'applications:edit.general.redirectUris.error.invalid',
          'Invalid Redirect: Please enter a valid URL. HTTP requires localhost, 127.0.0.1, or [::1].',
        ),
      }));
      return false;
    }
    setErrors((prev) => {
      const next = {...prev};
      delete next[index];
      return next;
    });
    return true;
  };

  const reindexErrors = (removedIndex: number): void => {
    setErrors((prev) => {
      const next = {...prev};
      delete next[removedIndex];
      const reindexed: Record<number, string> = {};
      Object.entries(next).forEach(([key, value]) => {
        const oldIndex = parseInt(key, 10);
        reindexed[oldIndex > removedIndex ? oldIndex - 1 : oldIndex] = value;
      });
      return reindexed;
    });
  };

  const handleRemove = (index: number): void => {
    reindexErrors(index);
    onUrisChange(uris.filter((_, i) => i !== index));
  };

  const handleChange = (index: number, value: string): void => {
    const next = [...uris];
    next[index] = value;
    onUrisChange(next);
  };

  const handleBlur = (index: number): void => {
    validate(uris[index], index);
  };

  // An empty list renders as a bare "Add URI" button with no field to type into, which reads as
  // broken rather than "nothing added yet". Show one empty row by default instead; typing into it
  // (or removing it) operates on the real (currently empty) `uris` array via the index above.
  const displayUris = uris.length > 0 ? uris : [''];

  // Appends relative to what's on screen (displayUris), not the possibly-empty backing `uris`
  // array — otherwise the first click while the list is empty turns [] into [''], which renders
  // identically to the placeholder row already shown and looks like the click did nothing.
  const handleAdd = (): void => onUrisChange([...displayUris, '']);

  return (
    <FormControl fullWidth>
      <FormLabel>{title}</FormLabel>
      <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 2}}>
        {description}
      </Typography>

      <Stack spacing={2}>
        {displayUris.map((uri, index) => (
          // IMPORTANT: Do not remove the suppression since it affects functionality.
          // eslint-disable-next-line react/no-array-index-key
          <Stack key={index} direction="row" spacing={1} alignItems="flex-start">
            <FormControl fullWidth required={required} sx={{flex: 1}}>
              <TextField
                fullWidth
                value={uri}
                onChange={(e) => handleChange(index, e.target.value)}
                onBlur={() => handleBlur(index)}
                error={!!errors[index]}
                helperText={errors[index]}
                placeholder={placeholder}
              />
            </FormControl>
            <Tooltip title={t('common:actions.delete', 'Delete')}>
              <IconButton onClick={() => handleRemove(index)} color="error" sx={{mt: 1}}>
                <Trash size={20} />
              </IconButton>
            </Tooltip>
          </Stack>
        ))}

        <Box>
          <Button variant="text" color="primary" startIcon={<Plus />} onClick={handleAdd} size="small">
            {addLabel}
          </Button>
        </Box>
      </Stack>
    </FormControl>
  );
}

interface DevServerBannerProps {
  devServer: {id: string; label: string; url: string};
  showCors: boolean;
  redirectUris: string[];
  onRedirectUrisChange: (uris: string[]) => void;
  corsOrigins: AllowedOriginDraftRow[];
  onCorsOriginsChange: (origins: AllowedOriginDraftRow[]) => void;
}

/**
 * Quick-add banner offering to prefill the template's conventional local dev server URL into the
 * redirect URIs (and, for CORS-applicable templates, the CORS Allowed Origins) list, so admins
 * developing locally don't have to type it in by hand.
 */
function DevServerBanner({
  devServer,
  showCors,
  redirectUris,
  onRedirectUrisChange,
  corsOrigins,
  onCorsOriginsChange,
}: DevServerBannerProps): JSX.Element {
  const {t} = useTranslation();

  const handleQuickAdd = (): void => {
    if (!redirectUris.includes(devServer.url)) {
      onRedirectUrisChange([...redirectUris, devServer.url]);
    }
    // The dev server URL is always an exact origin, never a pattern.
    const devServerRow = createRow(AllowedOriginTypes.ORIGIN, devServer.url);
    if (showCors && !corsOrigins.some((row) => rowKey(row) === rowKey(devServerRow))) {
      onCorsOriginsChange([...corsOrigins, devServerRow]);
    }
  };

  return (
    <Alert severity="info" icon={<DevServerLogo id={devServer.id} />} data-testid="application-dev-server-banner">
      {t('applications:onboarding.configure.details.devServer.banner', 'Using {{label}}? Its dev server runs on', {
        label: devServer.label,
      })}{' '}
      <code>{devServer.url}</code> {t('applications:onboarding.configure.details.devServer.byDefault', 'by default.')}{' '}
      <Box
        component="span"
        onClick={handleQuickAdd}
        sx={{color: 'primary.main', fontWeight: 600, cursor: 'pointer', textDecoration: 'underline'}}
      >
        {showCors
          ? t(
              'applications:onboarding.configure.details.devServer.addToRedirectAndCors',
              'Add it to redirect URIs & CORS origins',
            )
          : t('applications:onboarding.configure.details.devServer.addToRedirect', 'Add it to redirect URIs')}
      </Box>
    </Alert>
  );
}

/**
 * Allowed Redirect URLs, Post-Logout Redirect URIs, and (for CORS-applicable templates) CORS
 * Allowed Origins editors for the Configuration step, shown only for OAuth2 redirect-capable
 * templates (authorization_code grant). Mirrors the editing pattern used for the same fields on
 * an already-created application's Access settings.
 */
export default function ConfigureRedirectUris(): JSX.Element {
  const {t} = useTranslation();
  const {
    redirectUris,
    setRedirectUris,
    postLogoutRedirectUris,
    setPostLogoutRedirectUris,
    corsOrigins,
    setCorsOrigins,
    selectedTemplateConfig,
  } = useApplicationCreate();

  const {devServer, capabilities} = selectedTemplateConfig ?? {};
  const showCors = Boolean(capabilities?.cors);

  // Post-logout redirect URIs default to mirroring the authorized redirect URIs, so admins don't
  // have to maintain two identical lists. Unchecking reveals a separate editor; re-checking
  // discards those custom entries and goes back to mirroring the redirect URIs above.
  const [useSameAsRedirect, setUseSameAsRedirect] = useState(true);

  useEffect(() => {
    if (useSameAsRedirect) {
      setPostLogoutRedirectUris(redirectUris);
    }
  }, [useSameAsRedirect, redirectUris, setPostLogoutRedirectUris]);

  return (
    <Stack direction="column" spacing={4} data-testid="application-configure-redirect-uris">
      {devServer && (
        <DevServerBanner
          devServer={devServer}
          showCors={showCors}
          redirectUris={redirectUris}
          onRedirectUrisChange={setRedirectUris}
          corsOrigins={corsOrigins}
          onCorsOriginsChange={setCorsOrigins}
        />
      )}

      <UriListEditor
        title={t('applications:edit.general.redirectUris.title', 'Authorized redirect URIs')}
        description={t('applications:edit.general.redirectUris.description', 'For use with requests from a web server')}
        placeholder={t(
          'applications:onboarding.configure.details.redirectUris.placeholder',
          'https://example.com/callback',
        )}
        addLabel={t('applications:edit.general.redirectUris.addUri', 'Add URI')}
        uris={redirectUris}
        onUrisChange={setRedirectUris}
        required
      />

      <Box sx={{display: 'flex', alignItems: 'flex-start', gap: 1}}>
        <Checkbox
          checked={useSameAsRedirect}
          onChange={(_event, checked) => setUseSameAsRedirect(checked)}
          sx={{mt: -0.5}}
          data-testid="application-post-logout-same-as-redirect-checkbox"
        />
        <Box>
          <Typography variant="body2" fontWeight={600}>
            {t(
              'applications:edit.general.postLogoutRedirectUris.sameAsRedirect.title',
              'Use the same URLs for post-logout redirect',
            )}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t(
              'applications:edit.general.postLogoutRedirectUris.sameAsRedirect.description',
              'Reuse the redirect URIs above instead of maintaining a separate list',
            )}
          </Typography>
        </Box>
      </Box>

      {!useSameAsRedirect && (
        <UriListEditor
          title={t('applications:edit.general.postLogoutRedirectUris.title', 'Post-Logout Redirect URIs')}
          description={t(
            'applications:edit.general.postLogoutRedirectUris.description',
            'Allowed URIs to redirect to after logout. A post_logout_redirect_uri passed to the logout endpoint must match one of these.',
          )}
          placeholder={t(
            'applications:onboarding.configure.details.postLogoutRedirectUris.placeholder',
            'https://example.com/logged-out',
          )}
          addLabel={t('applications:edit.general.postLogoutRedirectUris.addUri', 'Add URI')}
          uris={postLogoutRedirectUris}
          onUrisChange={setPostLogoutRedirectUris}
          required={false}
        />
      )}

      {showCors && <CorsOriginsEditor rows={corsOrigins} onRowsChange={setCorsOrigins} />}
    </Stack>
  );
}
