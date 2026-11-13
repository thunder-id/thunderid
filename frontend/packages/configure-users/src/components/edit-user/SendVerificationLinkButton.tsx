// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes, useAdministrationOperation, useCapabilitiesContext, useToast} from '@thunderid/contexts';
import type {AccessStatus} from '@thunderid/types';
import {Button, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Send} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetAttributeVerificationFlow from '../../api/useGetAttributeVerificationFlow';
import useSendUserAttributeVerification from '../../api/useSendUserAttributeVerification';
import getUserErrorMessage from '../../utils/getUserErrorMessage';

interface SendVerificationLinkButtonProps {
  userId: string;
  attribute: string;
  /**
   * The attribute's display name, which names the button for assistive technology.
   */
  label: string;
  /**
   * The user's effective access status. A suspended or expired user cannot be sent a link.
   */
  accessStatus?: AccessStatus;
  /**
   * The field holds an unsaved value, which can be sent a link only once saved.
   */
  edited?: boolean;
}

/**
 * Sends a user a link to verify an email. The caller decides which fields show it. Shown only where
 * administration runs through flows, to operators who can update users, and when the server configures
 * the flow. Disabled for a suspended or expired user, whom the flow refuses. An unsaved value shows a
 * save hint instead.
 */
export default function SendVerificationLinkButton({
  userId,
  attribute,
  label,
  accessStatus = undefined,
  edited = false,
}: SendVerificationLinkButtonProps): JSX.Element | null {
  const {t} = useTranslation('users');
  const {can} = useCapabilitiesContext();
  const mode = useAdministrationOperation('users', 'verifyAttribute');
  const {showToast} = useToast();
  const sendVerification = useSendUserAttributeVerification();
  const offered = mode === AdministrationModes.FLOW && can('user', 'update');
  const {data: handle} = useGetAttributeVerificationFlow(offered);

  if (!offered || !handle) {
    return null;
  }

  let heldHint = '';
  if (accessStatus === 'SUSPENDED') {
    heldHint = t('users:verification.sendSuspended', 'Unsuspend the user to send a verification link.');
  } else if (accessStatus === 'EXPIRED') {
    heldHint = t(
      'users:verification.sendExpired',
      "This user's account has expired, so a verification link cannot be sent.",
    );
  }

  if (edited) {
    return heldHint ? null : (
      <Typography variant="caption" color="text.secondary" sx={{whiteSpace: 'nowrap'}}>
        {t('users:verification.saveToVerify', 'Save to verify')}
      </Typography>
    );
  }

  return (
    <Tooltip title={heldHint || t('users:verification.send', 'Send verification link')}>
      <span>
        <Button
          size="small"
          startIcon={<Send size={14} />}
          variant="outlined"
          disabled={Boolean(heldHint) || sendVerification.isPending}
          aria-label={
            sendVerification.isPending
              ? undefined
              : t('users:verification.sendFor', 'Send verification link for {{attribute}}', {attribute: label})
          }
          onClick={() =>
            sendVerification.mutate(
              {userId, attribute, handle},
              {
                onError: (error: Error) =>
                  showToast(
                    getUserErrorMessage(
                      error,
                      t,
                      'users:verification.sendError',
                      'Could not send the verification link. Please try again.',
                    ),
                    'error',
                  ),
              },
            )
          }
          data-testid={`send-verification-${attribute}`}
          sx={{whiteSpace: 'nowrap'}}
        >
          {sendVerification.isPending
            ? t('common:status.sending', 'Sending...')
            : t('users:verification.verify', 'Verify')}
        </Button>
      </span>
    </Tooltip>
  );
}
