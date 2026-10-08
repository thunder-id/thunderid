// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormControl, FormLabel, TextField, Typography} from '@wso2/oxygen-ui';
import {useState, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import useValidationReport from '../../hooks/useValidationReport';
import validateBackchannelLogoutUri from '../../utils/validateBackchannelLogoutUri';

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
 * Text field for the OIDC Back-Channel Logout URI of an application or agent. It validates with the
 * rules the server applies on save and commits on blur like the redirect URI fields.
 *
 * @param props - Component props
 * @returns The field
 */
export default function BackchannelLogoutUriField({
  value = '',
  publicClient = false,
  onChange = undefined,
  onValidationChange = undefined,
  disabled = false,
}: BackchannelLogoutUriFieldProps): ReactNode {
  const {t} = useTranslation();
  const [draft, setDraft] = useState(value);
  const [touched, setTouched] = useState(false);

  // When the value changes from outside, such as a Reset that discards unsaved edits, show it rather
  // than the draft, so a discarded URI cannot be committed again on the next blur.
  const [syncedValue, setSyncedValue] = useState(value);
  if (value !== syncedValue) {
    setSyncedValue(value);
    setDraft(value);
    setTouched(false);
  }

  const result = validateBackchannelLogoutUri(draft, publicClient);

  const isEditable = Boolean(onChange) && !disabled;
  // A saved value that turns invalid, such as http once the client becomes public, shows at once;
  // a value being typed shows its error only after blur. The report to the page follows the same
  // rule, so the save bar never names an error the field is not yet showing.
  const showError = !result.valid && (touched || draft === value);
  useValidationReport(onValidationChange, showError);

  const handleBlur = (): void => {
    setTouched(true);
    // Committing an unchanged value would still count as an edit to the page, which clears a save error.
    if (result.valid && draft.trim() !== value) {
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
          'Endpoint that receives a logout token when a session this client took part in ends. Leave empty to turn off notifications. By default the server refuses localhost and private network addresses.',
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
