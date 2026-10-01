// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {KeyRound} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import HomeNextStepCard from './HomeNextStepCard';

export default function MFACard(): JSX.Element {
  const {t} = useTranslation('home');

  return (
    <HomeNextStepCard
      icon={<KeyRound size={24} />}
      title={t('next_steps.mfa.title', 'Multi-factor Authentication')}
      description={t(
        'next_steps.mfa.description',
        'Protect users by enabling an additional verification factor to the sign-in process.',
      )}
      primaryLabel={t('next_steps.mfa.actions.primary.label', 'Configure Flows')}
      primaryRoute="/flows"
    />
  );
}
