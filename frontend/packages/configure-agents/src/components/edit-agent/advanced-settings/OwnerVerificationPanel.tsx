// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueryClient} from '@tanstack/react-query';
import {useCapabilitiesContext, useToast} from '@thunderid/contexts';
import {AdministrationFlowData, getErrorMessage} from '@thunderid/utils';
import {Alert, Box, Button, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {RefreshCw, Send} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useSendOwnerVerification from '../../../api/useSendOwnerVerification';
import {OWNER_VERIFICATION_POLL_LIMIT_MS, OWNER_VERIFICATION_POLL_MS} from '../../../constants/agent-constants';
import AgentQueryKeys from '../../../constants/agent-query-keys';
import type {Agent} from '../../../models/agent';
import OwnerVerificationStatus from '../../OwnerVerificationStatus';

interface OwnerVerificationPanelProps {
  agent: Agent;
  /** The owner was edited and not saved, so a link would reach the saved owner instead. */
  ownerUnsaved: boolean;
  /** Handle of the flow that sends the link, from the server's `adminAgentOwnerVerificationFlow` setting. */
  handle: string;
}

/** A link sent from this page that the owner has not confirmed yet. */
interface SentLink {
  destination?: string;
  sentAt: number;
  /** The owner's verification when the link was sent, so a new one is told apart from an earlier one. */
  verifiedAtBefore?: string;
}

/**
 * Sends an agent's external owner a verification link and follows it through. While the link is sent it shows a
 * spinner; once sent, where it went and that the page is waiting for the owner, reading the agent again until the
 * owner confirms; then the owner as verified. A failed send is shown here with what to do about it.
 */
export default function OwnerVerificationPanel({
  agent,
  ownerUnsaved,
  handle,
}: OwnerVerificationPanelProps): JSX.Element {
  // The agents namespace, where a flow error's code finds the agents' copy of its message.
  const {t} = useTranslation('agents');
  const {can} = useCapabilitiesContext();
  const {showToast} = useToast();
  const queryClient = useQueryClient();
  const sendVerification = useSendOwnerVerification();
  const [sent, setSent] = useState<SentLink | null>(null);
  const [polling, setPolling] = useState(false);

  const owner = agent.owner;
  const verifiedAt = owner?.verifiedAt;
  const isMobile = owner?.idType === 'mobile';

  const refreshAgent = (): Promise<void> => queryClient.invalidateQueries({queryKey: [AgentQueryKeys.AGENT, agent.id]});

  useEffect(() => {
    if (!polling || !sent) return undefined;
    const interval = setInterval(() => {
      if (Date.now() - sent.sentAt > OWNER_VERIFICATION_POLL_LIMIT_MS) {
        setPolling(false);
        return;
      }
      refreshAgent().catch(() => undefined);
    }, OWNER_VERIFICATION_POLL_MS);
    return () => clearInterval(interval);
    // refreshAgent only closes over the agent ID, which the sent link was made for.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [polling, sent]);

  useEffect(() => {
    if (sent && verifiedAt && verifiedAt !== sent.verifiedAtBefore) {
      setSent(null);
      setPolling(false);
      showToast(
        t('agents:owner.verification.confirmedToast', 'The owner confirmed ownership of this agent'),
        'success',
      );
    }
  }, [sent, verifiedAt, showToast, t]);

  // A send that failed for the saved owner says nothing about an owner being edited.
  useEffect(() => {
    if (ownerUnsaved) sendVerification.reset();
    // reset is stable; only a newly edited owner clears the failure.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ownerUnsaved]);

  // The flow refuses a suspended agent and one whose lifespan has ended, so no link is offered for them.
  let heldHint = '';
  if (agent.status?.value === 'SUSPENDED') {
    heldHint = t(
      'agents:owner.verification.sendSuspended',
      'Unsuspend the agent to send its owner a verification link.',
    );
  } else if (agent.status?.value === 'EXPIRED') {
    heldHint = t(
      'agents:owner.verification.sendExpired',
      "This agent's lifespan has ended, so its owner cannot be sent a verification link.",
    );
  }

  const send = (): void => {
    sendVerification.mutate(
      {agentId: agent.id, handle},
      {
        onSuccess: (data) => {
          setSent({
            destination: data.data?.additionalData?.[AdministrationFlowData.MASKED_DESTINATION],
            sentAt: Date.now(),
            verifiedAtBefore: verifiedAt,
          });
          setPolling(true);
        },
      },
    );
  };

  let description: string;
  if (heldHint) {
    description = heldHint;
  } else if (ownerUnsaved) {
    description = t('agents:owner.verification.saveFirst', 'Save the new owner before you send a verification link.');
  } else if (verifiedAt) {
    description = t(
      'agents:owner.verification.verifiedDescription',
      '{{owner}} confirmed that they are accountable for this agent. Changing the owner removes the verification.',
      {owner: owner?.id},
    );
  } else if (isMobile) {
    description = t(
      'agents:owner.verification.descriptionMobile',
      'Ask {{owner}} to confirm that they are accountable for this agent. They get a link by text message and are verified once they confirm. They do not need an account.',
      {owner: owner?.id},
    );
  } else {
    description = t(
      'agents:owner.verification.descriptionEmail',
      'Ask {{owner}} to confirm that they are accountable for this agent. They get a link by email and are verified once they confirm. They do not need an account.',
      {owner: owner?.id},
    );
  }

  let sendLabel = t('agents:owner.verification.send', 'Send verification link');
  if (sendVerification.isPending) {
    sendLabel = t('common:status.sending', 'Sending...');
  } else if (sent) {
    sendLabel = t('agents:owner.verification.resendLink', 'Resend link');
  } else if (verifiedAt) {
    sendLabel = t('agents:owner.verification.resend', 'Send a new verification link');
  }

  return (
    <Box data-testid="agent-owner-verification" sx={{mt: 2, p: 2, border: 1, borderColor: 'divider', borderRadius: 1}}>
      <Stack direction="row" spacing={2} alignItems="flex-start" justifyContent="space-between">
        <Box>
          <Typography variant="subtitle2">{t('agents:owner.verification.title', 'Ownership verification')}</Typography>
          <Typography variant="body2" color="text.secondary" sx={{mt: 0.5}}>
            {description}
          </Typography>
        </Box>
        {!ownerUnsaved && <OwnerVerificationStatus owner={owner} />}
      </Stack>

      {sendVerification.isError && (
        <Alert severity="error" sx={{mt: 2}} onClose={() => sendVerification.reset()}>
          {getErrorMessage(
            sendVerification.error,
            t,
            'agents:owner.verification.sendFailed',
            'The verification link could not be sent. Please try again.',
          )}
        </Alert>
      )}

      {sent && (
        <Alert
          severity="info"
          sx={{mt: 2}}
          data-testid="agent-owner-verification-pending"
          action={
            <Button
              color="inherit"
              size="small"
              startIcon={<RefreshCw size={14} aria-hidden />}
              onClick={() => {
                refreshAgent().catch(() => undefined);
              }}
            >
              {t('agents:owner.verification.checkNow', 'Check now')}
            </Button>
          }
        >
          {sent.destination
            ? t('agents:owner.verification.pendingTo', 'Link sent to {{destination}}.', {destination: sent.destination})
            : t('agents:owner.verification.pending', 'Link sent to the owner.')}{' '}
          {polling
            ? t(
                'agents:owner.verification.pendingWaiting',
                'Waiting for the owner to open it and confirm. This page updates when they do.',
              )
            : t(
                'agents:owner.verification.pendingStopped',
                'The owner has not confirmed yet. Check again later, or resend the link.',
              )}
        </Alert>
      )}

      <Stack direction="row" spacing={1} sx={{mt: 2}}>
        <Button
          variant={verifiedAt && !sent ? 'outlined' : 'contained'}
          size="small"
          data-testid="agent-owner-send-verification"
          startIcon={
            sendVerification.isPending ? (
              <CircularProgress
                size={14}
                color="inherit"
                aria-label={t('agents:owner.verification.sending', 'Sending the verification link')}
              />
            ) : (
              <Send size={14} aria-hidden />
            )
          }
          // An agent a control plane deployed is read only here, yet its owner can still be verified.
          disabled={Boolean(heldHint) || ownerUnsaved || !can('agent', 'update') || sendVerification.isPending}
          onClick={send}
        >
          {sendLabel}
        </Button>
      </Stack>
    </Box>
  );
}
