// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {OrganizationUnitPickerScreen} from '@thunderid/configure-organization-units';
import {Home} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface ConnectionOuPickerStepProps {
  title: string;
  value: string;
  onChange: (ouId: string) => void;
  onBack: () => void;
  onContinue: () => void;
}

/**
 * Full-screen step shown before a connection wizard, for choosing the organization unit the connection belongs to.
 */
export default function ConnectionOuPickerStep({
  title,
  value,
  onChange,
  onBack,
  onContinue,
}: ConnectionOuPickerStepProps): JSX.Element {
  const {t} = useTranslation('connections');

  return (
    <>
      <DocumentTitle title={title} />
      <OrganizationUnitPickerScreen
        icon={<Home size={26} />}
        title={t('wizard.organizationUnit.title', 'Where should this connection belong?')}
        subtitle={t(
          'wizard.organizationUnit.subtitle',
          "Choose the organization unit that will own this connection. You can't change this once created.",
        )}
        value={value}
        onChange={onChange}
        onBack={onBack}
        onContinue={onContinue}
        backLabel={t('common:actions.back', 'Back')}
        continueLabel={t('common:actions.continue', 'Continue')}
      />
    </>
  );
}
