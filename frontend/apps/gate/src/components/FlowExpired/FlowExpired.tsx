// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {FlowExpiryRedirect} from '@thunderid/react';
import {Box, LinearProgress, Link, Typography} from '@wso2/oxygen-ui';
import {ArrowLeft, Timer} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useEffect, useRef} from 'react';
import {Trans, useTranslation} from 'react-i18next';

interface FlowExpiredProps {
  /** The name of the application the flow was started for, from the flow metadata. */
  applicationName?: string;
  /** The SDK's countdown back to the application, or null when there is no application URL. */
  redirect: FlowExpiryRedirect | null;
}

/**
 * Shown in place of a flow whose execution expired. The SDK runs the countdown and the redirect back to the
 * application; this renders it in Gate's own design.
 */
export default function FlowExpired({applicationName = undefined, redirect}: FlowExpiredProps): JSX.Element {
  const {t} = useTranslation();
  const headingRef = useRef<HTMLHeadingElement>(null);
  // An empty name is as good as none.
  const application: string =
    applicationName !== undefined && applicationName !== ''
      ? applicationName
      : t('auth:flowExpired.theApplication', 'the application');
  // The countdown shows only while it runs, not while it waits for the user to come back.
  const isCountdownShown: boolean = redirect?.isCountingDown === true;

  // The expired state replaces the flow in place, so move focus to it for a screen reader to read it.
  useEffect(() => {
    headingRef.current?.focus();
  }, []);

  // Truncates a long application name, so the link stays on one line.
  const nameInLink: JSX.Element = (
    <Box component="span" sx={{minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'}} />
  );

  return (
    <Box
      data-testid="flow-expired"
      sx={{display: 'flex', flexDirection: 'column', alignItems: 'center', textAlign: 'center', gap: 1.5, py: 1}}
    >
      {isCountdownShown && redirect && (
        // Spans the card and takes its rounded corners, so the countdown line runs along the card's top edge.
        <Box
          aria-hidden
          sx={{position: 'absolute', inset: 0, borderRadius: 'inherit', overflow: 'hidden', pointerEvents: 'none'}}
        >
          <LinearProgress
            data-testid="flow-expired-progress"
            variant="determinate"
            color="warning"
            value={(redirect.secondsLeft / redirect.countdownSeconds) * 100}
            sx={{
              height: 3,
              bgcolor: 'transparent',
              // Moves on steadily between the one-second ticks.
              '& .MuiLinearProgress-bar': {transition: 'transform 1s linear'},
            }}
          />
        </Box>
      )}
      <Box sx={{color: 'warning.main', display: 'flex'}} aria-hidden>
        <Timer size={44} strokeWidth={1.6} />
      </Box>
      <Typography ref={headingRef} tabIndex={-1} variant="h5" component="h1" sx={{outline: 'none'}}>
        {t('auth:flowExpired.title', 'Session expired')}
      </Typography>
      <Typography color="text.secondary">
        {isCountdownShown && redirect
          ? t('auth:flowExpired.redirecting', 'Taking you back to {{application}} in {{seconds}}s.', {
              application,
              seconds: redirect.secondsLeft,
            })
          : t('auth:flowExpired.description', 'Please go back to {{application}} and try again.', {application})}
      </Typography>
      {redirect && (
        <Link
          href={redirect.url}
          underline="none"
          variant="subtitle1"
          sx={{
            mt: 1,
            display: 'inline-flex',
            alignItems: 'center',
            gap: 0.75,
            maxWidth: '100%',
            minWidth: 0,
            // The theme underlines every link, whatever its underline prop.
            textDecoration: 'none',
          }}
        >
          <Box component="span" sx={{display: 'flex', flex: 'none'}} aria-hidden>
            <ArrowLeft size={16} strokeWidth={2.4} />
          </Box>
          {/* Keeps the text on one line, with its own spaces, so only the name shortens. */}
          <Box component="span" sx={{display: 'flex', minWidth: 0, whiteSpace: 'pre'}}>
            {isCountdownShown ? (
              <Trans
                i18nKey="auth:flowExpired.backToApplication"
                defaults="Back to <name>{{application}}</name> now"
                values={{application}}
                components={{name: nameInLink}}
              />
            ) : (
              <Trans
                i18nKey="auth:flowExpired.returnToApplication"
                defaults="Back to <name>{{application}}</name>"
                values={{application}}
                components={{name: nameInLink}}
              />
            )}
          </Box>
        </Link>
      )}
    </Box>
  );
}
