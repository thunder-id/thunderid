// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CredentialResetSection, type CredentialResetField} from '@thunderid/components';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useUpdateUserCredentials from '../../api/useUpdateUserCredentials';
import getUserErrorMessage from '../../utils/getUserErrorMessage';

interface CredentialsTabPanelProps {
  userId: string;
  credentialFields: CredentialResetField[];
}

export default function CredentialsTabPanel({userId, credentialFields}: CredentialsTabPanelProps): JSX.Element | null {
  const {t} = useTranslation('users');
  const updateCredentials = useUpdateUserCredentials();

  return (
    <CredentialResetSection
      fields={credentialFields}
      onReset={(field, newValue) =>
        updateCredentials.mutateAsync({userId, data: {credentials: {[field.fieldName]: newValue}}})
      }
      getErrorMessage={(error) =>
        getUserErrorMessage(error, t, 'updateCredentials.error', 'Failed to update credentials. Please try again.')
      }
    />
  );
}
