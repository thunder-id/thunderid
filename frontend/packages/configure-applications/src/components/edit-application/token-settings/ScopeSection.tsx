// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box, Typography} from '@wso2/oxygen-ui';
import {useTranslation} from 'react-i18next';
import ScopeMapper from './ScopeMapper';
import type {ScopeClaims} from '../../../models/oauth';

/**
 * Props for the {@link ScopeSection} component.
 */
interface ScopeSectionProps {
  /**
   * Current scope → attributes mapping from the top-level scope_claims field.
   */
  scopeClaims: ScopeClaims;
  /**
   * All available user attributes derived from user types.
   */
  userAttributes: string[];
  /**
   * Loading state for user attributes fetch.
   */
  isLoadingUserAttributes: boolean;
  /**
   * Whether the entity has any allowed user types. Their schemas are where selectable attributes
   * come from, so with none configured there is nothing to map.
   */
  hasAllowedUserTypes: boolean;
  /**
   * Callback fired when the scope → attributes mapping changes.
   */
  onScopeClaimsChange: (scopeClaims: ScopeClaims) => void;
  /**
   * Singular noun used to refer to the entity in user-visible copy (default: 'application').
   */
  entityLabel?: string;
  /**
   * Whether inputs should be disabled (e.g. read-only resource).
   */
  disabled?: boolean;
  /**
   * Whether only the scope names take effect, for an entity whose grants issue an access token
   * alone (token exchange, JWT bearer). Those grants keep a listed custom scope as an OpenID
   * Connect scope (a standard one is kept either way) but never add its mapped attributes, so the
   * description says so and the hint about adding allowed user types is dropped. Defaults to false.
   */
  scopesOnly?: boolean;
}

/**
 * Scope → user attribute mapping editor. It renders on the combined ID Token and User Info tab,
 * above the panels it feeds, or, for an entity issued only an access token on behalf of a user,
 * below that access token.
 *
 * The mapping is the single source of truth: a scope is available to the {@link entityLabel} only
 * while it has an entry here, and the attributes mapped to it are the ones exposed when it is
 * requested. One instance serves both tabs, so the heading says so explicitly rather than leaving
 * the reader to infer it from the placement.
 *
 * @param props - Component props
 * @returns Scope attribute mapping configuration
 */
export default function ScopeSection({
  scopeClaims,
  userAttributes,
  isLoadingUserAttributes,
  hasAllowedUserTypes,
  onScopeClaimsChange,
  entityLabel = 'application',
  disabled = false,
  scopesOnly = false,
}: ScopeSectionProps) {
  const {t} = useTranslation();

  return (
    <Box>
      <Typography variant="subtitle2" sx={{mb: 1}}>
        {t('applications:edit.token.scopes_card.title', 'Scopes & user attribute mappings')}
      </Typography>
      <Typography variant="body2" color="text.disabled" sx={{mb: 2}}>
        {scopesOnly
          ? t(
              'applications:edit.token.scopes_card.description.scopesOnly',
              'A custom scope listed here is kept as an OpenID Connect scope instead of being treated as a resource server permission. Its mapped attributes are not added to the access token.',
            )
          : t('applications:edit.token.scopes_card.description', {
              defaultValue:
                'Configure the OAuth2 scopes available to this {{entity}} and the user attributes each exposes. This mapping is shared by the ID Token and the User Info endpoint.',
              entity: entityLabel,
            })}
      </Typography>

      {!scopesOnly && !hasAllowedUserTypes && !isLoadingUserAttributes && (
        <Alert severity="info" sx={{mb: 2}}>
          {t('applications:edit.token.scopes_card.no_user_types', {
            defaultValue:
              'Selectable attributes come from the schemas of this {{entity}}’s allowed user types. Add one to map user attributes to scopes.',
            entity: entityLabel,
          })}
        </Alert>
      )}

      <ScopeMapper
        scopeClaims={scopeClaims}
        userAttributes={userAttributes}
        isLoadingUserAttributes={isLoadingUserAttributes}
        onScopeClaimsChange={onScopeClaimsChange}
        disabled={disabled}
      />
    </Box>
  );
}
