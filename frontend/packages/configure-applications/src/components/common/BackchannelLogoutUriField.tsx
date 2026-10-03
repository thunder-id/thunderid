// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useThunderID} from '@thunderid/react';
import {FormControl, FormLabel, TextField, Typography} from '@wso2/oxygen-ui';
import {useEffect, useState, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import validateBackchannelLogoutUri from '../../utils/validateBackchannelLogoutUri';

interface BackchannelLogoutDiscovery {
  backchannel_logout_supported?: boolean;
}

/**
 * Props for the {@link BackchannelLogoutUriField} component.
 */
interface BackchannelLogoutUriFieldProps {
  /** The registered back-channel logout URI. */
  value?: string;
  /** Whether the client is public, which requires an https URI. */
  publicClient?: boolean;
  /** Called on blur with the trimmed URI once it is valid. When omitted the field is read-only. */
  onChange?: (uri: string) => void;
  /** Called whenever the field's validity changes. */
  onValidationChange?: (hasError: boolean) => void;
  /** Whether the input should be disabled (e.g. read-only resource). */
  disabled?: boolean;
}

/**
 * Text field for the OIDC Back-Channel Logout URI of an application or agent. It renders only when
 * the server's discovery document advertises `backchannel_logout_supported`, validates with the
 * rules the server applies on save, and commits on blur like the redirect URI fields.
 *
 * @param props - Component props
 * @returns The field, or null when the server does not support back-channel logout
 */
export default function BackchannelLogoutUriField({
  value = '',
  publicClient = false,
  onChange = undefined,
  onValidationChange = undefined,
  disabled = false,
}: BackchannelLogoutUriFieldProps): ReactNode {
  const {t} = useTranslation();
  const {discovery} = useThunderID();
  const [draft, setDraft] = useState(value);
  const [touched, setTouched] = useState(false);

  const supported =
    (discovery as {wellKnown?: BackchannelLogoutDiscovery} | undefined)?.wellKnown?.backchannel_logout_supported ===
    true;
  const result = validateBackchannelLogoutUri(draft, publicClient);
  // A hidden field must not block saving with an error the user cannot see.
  const hasError = supported && !result.valid;

  useEffect(() => {
    onValidationChange?.(hasError);
  }, [hasError, onValidationChange]);

  if (!supported) return null;

  const isEditable = Boolean(onChange) && !disabled;
  // A saved value that turns invalid, such as http once the client becomes public, shows at once;
  // a value being typed shows its error only after blur.
  const showError = !result.valid && (touched || draft === value);

  const handleBlur = (): void => {
    setTouched(true);
    if (result.valid) {
      onChange?.(draft.trim());
    }
  };

  return (
    <FormControl fullWidth>
      <FormLabel htmlFor="backchannel-logout-uri-input">
        {t('applications:edit.general.backchannelLogoutUri.title', 'Back-Channel Logout URI')}
      </FormLabel>
      <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 2}}>
        {t(
          'applications:edit.general.backchannelLogoutUri.description',
          'Endpoint that receives a logout token when a session this client took part in ends. Leave empty to turn off notifications.',
        )}
      </Typography>
      <TextField
        fullWidth
        id="backchannel-logout-uri-input"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={handleBlur}
        error={showError}
        helperText={showError ? t(result.errorKey ?? '', result.errorDefault ?? '') : undefined}
        placeholder="https://example.com/backchannel-logout"
        disabled={!isEditable}
      />
    </FormControl>
  );
}
