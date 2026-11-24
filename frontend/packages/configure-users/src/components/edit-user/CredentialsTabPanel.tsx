// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CredentialResetSection, type CredentialResetField, type CredentialResetLinkOption} from '@thunderid/components';
import type {AccessStatus} from '@thunderid/types';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useSendUserCredentialReset from '../../api/useSendUserCredentialReset';
import useUpdateUserCredentials from '../../api/useUpdateUserCredentials';
import useCredentialResetLink from '../../hooks/useCredentialResetLink';
import getUserErrorMessage from '../../utils/getUserErrorMessage';

interface CredentialsTabPanelProps {
  userId: string;
  credentialFields: CredentialResetField[];
  /**
   * Names of the credentials a reset link can reset: the text credentials of the user's type.
   */
  resetLinkFieldNames?: string[];
  /**
   * The user's effective access status. A suspended or expired user cannot be sent a reset link.
   */
  accessStatus?: AccessStatus;
}

export default function CredentialsTabPanel({
  userId,
  credentialFields,
  resetLinkFieldNames = [],
  accessStatus = undefined,
}: CredentialsTabPanelProps): JSX.Element | null {
  const {t} = useTranslation('users');
  const updateCredentials = useUpdateUserCredentials();
  const sendReset = useSendUserCredentialReset();
  const {handle, heldReason} = useCredentialResetLink(accessStatus);

  const getResetLink = (field: CredentialResetField): CredentialResetLinkOption | undefined => {
    if (!handle || !resetLinkFieldNames.includes(field.fieldName)) {
      return undefined;
    }

    return {
      onSend: () => sendReset.mutateAsync({userId, credential: field.fieldName, label: field.label, handle}),
      getErrorMessage: (error) =>
        getUserErrorMessage(error, t, 'credentialResetLink.error', 'Could not send the reset link. Please try again.'),
      heldReason,
    };
  };

  return (
    <CredentialResetSection
      fields={credentialFields}
      onReset={(field, newValue) =>
        updateCredentials.mutateAsync({userId, data: {credentials: {[field.fieldName]: newValue}}})
      }
      getErrorMessage={(error) =>
        getUserErrorMessage(error, t, 'updateCredentials.error', 'Failed to update credentials. Please try again.')
      }
      getResetLink={getResetLink}
    />
  );
}
