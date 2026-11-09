// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Card,
  FormControl,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  Switch,
  Typography,
} from '@wso2/oxygen-ui';
import type {ChangeEvent, JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {subjectClaimOptions, type ClaimRow} from '../models/presentation-claims';

export interface PresentationSignInCardProps {
  claims: ClaimRow[];
  allowSignIn: boolean;
  subjectClaim: string;
  /** Turning sign-in off clears the subject claim. */
  onAllowSignInChange: (allowSignIn: boolean) => void;
  onSubjectClaimChange: (subjectClaim: string) => void;
  /** The stored subject claim, in the edit form. */
  storedSubjectClaim?: string;
  /** Whether the definition enforces trusted issuers. Undefined in the create wizard, which cannot yet. */
  enforceTrustedIssuer?: boolean;
}

/**
 * Lets a presentation definition sign users in by naming the mandatory claim that identifies the holder.
 */
export default function PresentationSignInCard({
  claims,
  allowSignIn,
  subjectClaim,
  onAllowSignInChange,
  onSubjectClaimChange,
  storedSubjectClaim = '',
  enforceTrustedIssuer = undefined,
}: PresentationSignInCardProps): JSX.Element {
  const {t} = useTranslation('verifiable-presentations');
  const options = subjectClaimOptions(claims);
  const stale = subjectClaim !== '' && !options.includes(subjectClaim);

  let error = '';
  if (subjectClaim === '') {
    error = t('form.signIn.subjectClaim.required', 'Pick a subject claim.');
  } else if (stale) {
    error = t('form.signIn.subjectClaim.notMandatory', 'Must be a mandatory claim.');
  }

  return (
    <Card variant="outlined" sx={{p: 3}}>
      <Stack spacing={2}>
        <FormControlLabel
          control={
            <Switch
              checked={allowSignIn}
              onChange={(e: ChangeEvent<HTMLInputElement>): void => onAllowSignInChange(e.target.checked)}
            />
          }
          label={t('form.signIn.allow', 'Allow sign-in')}
        />
        {allowSignIn && (
          <FormControl fullWidth required error={error !== ''}>
            <FormLabel id="vp-subject-claim-label">{t('form.signIn.subjectClaim.label', 'Subject claim')}</FormLabel>
            <Select
              labelId="vp-subject-claim-label"
              id="vp-subject-claim"
              value={subjectClaim}
              displayEmpty
              renderValue={(value: string) =>
                value || (
                  <Typography component="span" color="text.secondary">
                    {t('form.signIn.subjectClaim.placeholder', 'Select a claim')}
                  </Typography>
                )
              }
              onChange={(e): void => onSubjectClaimChange(e.target.value)}
            >
              {stale && (
                <MenuItem value={subjectClaim} disabled>
                  {subjectClaim}
                </MenuItem>
              )}
              {options.map((claimName: string) => (
                <MenuItem key={claimName} value={claimName}>
                  {claimName}
                </MenuItem>
              ))}
            </Select>
            <FormHelperText>
              {error || t('form.signIn.subjectClaim.hint', 'Identifies the holder. Mandatory claims only.')}
            </FormHelperText>
          </FormControl>
        )}
        {allowSignIn && storedSubjectClaim !== '' && subjectClaim !== '' && subjectClaim !== storedSubjectClaim && (
          <Alert severity="warning">
            {t('form.signIn.claimChanged', 'Returning holders will sign in as new users.')}
          </Alert>
        )}
        {!allowSignIn && storedSubjectClaim !== '' && (
          <Alert severity="warning">
            {t('form.signIn.turnedOff', 'Sign-in flows using this definition will stop working.')}
          </Alert>
        )}
        {allowSignIn && !enforceTrustedIssuer && (
          <Alert severity="info">
            {enforceTrustedIssuer === undefined
              ? t('form.signIn.trustedIssuer.create', 'Add a trusted issuer after creating.')
              : t('form.signIn.trustedIssuer.edit', 'Also needs Enforce trusted issuer.')}
          </Alert>
        )}
      </Stack>
    </Card>
  );
}
