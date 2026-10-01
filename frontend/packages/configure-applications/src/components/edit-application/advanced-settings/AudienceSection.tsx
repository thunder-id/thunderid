// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {FormControl, FormLabel, TextField} from '@wso2/oxygen-ui';
import {useTranslation} from 'react-i18next';

/**
 * Props for the {@link AudienceSection} component.
 */
interface AudienceSectionProps {
  /**
   * Current default audience value (aud claim) for tokens not bound to a resource server.
   */
  audience: string;
  /**
   * Callback fired whenever the audience value changes.
   */
  onAudienceChange: (audience: string) => void;
  /**
   * Singular noun used to refer to the entity in user-visible copy (default: 'application').
   */
  entityLabel?: string;
  /**
   * Whether the input should be disabled (e.g. read-only resource).
   */
  disabled?: boolean;
}

/**
 * Settings card for configuring the default audience of access tokens that are not bound to a
 * resource server (OIDC-only or scopeless requests). When left empty, such a token's `aud` claim
 * falls back to the client_id. The value is free-text so it can be any audience identifier,
 * including external ones not registered in ThunderID.
 *
 * @param props - Component props
 * @returns Default audience configuration within a SettingsCard
 */
export default function AudienceSection({
  audience,
  onAudienceChange,
  entityLabel = 'application',
  disabled = false,
}: AudienceSectionProps) {
  const {t} = useTranslation();

  return (
    <SettingsCard
      title={t('applications:edit.advanced.audience.title', 'Default Audience')}
      description={t(
        'applications:edit.advanced.audience.description',
        "The default aud for access tokens that don't target a resource server (OIDC only or scopeless).",
      )}
    >
      <FormControl fullWidth size="small">
        <FormLabel htmlFor="access_token_default_audience">
          {t('applications:edit.advanced.audience.label', 'Default audience (aud)')}
        </FormLabel>
        <TextField
          id="access_token_default_audience"
          size="small"
          fullWidth
          value={audience}
          disabled={disabled}
          onChange={(e) => onAudienceChange(e.target.value.trim())}
          inputProps={{maxLength: 2048}}
          placeholder={t('applications:edit.advanced.audience.placeholder', 'e.g. https://api.example.com')}
          helperText={t('applications:edit.advanced.audience.hint', 'Leave empty to use the {{entity}} client ID.', {
            entity: entityLabel,
          })}
        />
      </FormControl>
    </SettingsCard>
  );
}
