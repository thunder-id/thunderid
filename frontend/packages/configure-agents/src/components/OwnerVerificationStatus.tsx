// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Chip} from '@wso2/oxygen-ui';
import {CircleCheck, Clock} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {AgentOwner} from '../models/agent';
import {isVerifiableOwner} from '../utils/externalOwner';

interface OwnerVerificationStatusProps {
  owner?: AgentOwner;
}

/**
 * Shows whether an agent's external owner has proved control of its email address or mobile number, with the
 * date and time it did. Nothing is shown for an owner that cannot be verified, such as a local owner.
 */
export default function OwnerVerificationStatus({owner = undefined}: OwnerVerificationStatusProps): JSX.Element | null {
  const {t, i18n} = useTranslation();

  if (!owner || !isVerifiableOwner(owner)) return null;

  if (owner.verifiedAt) {
    return (
      <Chip
        size="small"
        color="success"
        variant="outlined"
        icon={<CircleCheck size={14} aria-hidden />}
        data-testid="agent-owner-verified"
        label={t('agents:owner.verification.verified', 'Verified on {{date}}', {
          date: new Date(owner.verifiedAt).toLocaleString(i18n?.language, {dateStyle: 'medium', timeStyle: 'short'}),
        })}
      />
    );
  }

  return (
    <Chip
      size="small"
      color="warning"
      variant="outlined"
      icon={<Clock size={14} aria-hidden />}
      data-testid="agent-owner-not-verified"
      label={t('agents:owner.verification.notVerified', 'Not verified')}
    />
  );
}
