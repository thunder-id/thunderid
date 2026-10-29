// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, PageLoader} from '@thunderid/components';
import {OrganizationUnitPickerScreen} from '@thunderid/configure-organization-units';
import {Home} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface FlowOrganizationUnitStepProps {
  /** Whether the organization units are still loading, which shows a spinner instead of the picker. */
  isLoading: boolean;
  /** The selected organization unit id. */
  value: string;
  /** Call when the user picks a different organization unit. */
  onChange: (ouId: string) => void;
  /** Call when the back link is clicked. */
  onBack: () => void;
  /** Call when Continue is clicked. */
  onContinue: () => void;
}

/**
 * The full-screen step that asks which organization unit a new flow belongs to. It is shown only when the
 * deployment has more than one organization unit; a single-unit deployment uses that unit without asking.
 */
export default function FlowOrganizationUnitStep({
  isLoading,
  value,
  onChange,
  onBack,
  onContinue,
}: FlowOrganizationUnitStepProps): JSX.Element {
  const {t} = useTranslation();
  const pageTitle: string = t('flows:listing.addFlow', 'Create new flow');

  if (isLoading) {
    return (
      <>
        <DocumentTitle title={pageTitle} />
        <PageLoader />
      </>
    );
  }

  return (
    <>
      <DocumentTitle title={pageTitle} />
      <OrganizationUnitPickerScreen
        icon={<Home size={26} />}
        title={t('flows:create.organizationUnit.title', 'Where should this flow belong?')}
        subtitle={t(
          'flows:create.organizationUnit.subtitle',
          "Choose the organization unit that will own this flow. You can't change this once created.",
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
