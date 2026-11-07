// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueryClient} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {Alert, Button} from '@wso2/oxygen-ui';
import {useEffect, useState, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';

interface FailedCall {
  response?: {status?: number};
  config?: {url?: string};
}

/** Whether a failed call was refused for want of a valid session, and the URL it called, when known. */
function unauthorized(error: unknown): {url?: string} | undefined {
  const failed = error as FailedCall | null | undefined;
  return failed?.response?.status === 401 ? {url: failed.config?.url} : undefined;
}

/**
 * Says the session has ended when a call is refused for want of one, such as an expired session or a
 * sign-in service that cannot be reached, so the pages are not taken to be empty. A refusal from the
 * control plane itself sends the console to sign in again.
 */
export default function SessionExpiredBanner(): ReactNode {
  const {t} = useTranslation();
  const queryClient = useQueryClient();
  const {signIn} = useThunderID();
  const {getServerUrl} = useConfig();
  const [expired, setExpired] = useState(false);

  useEffect(() => {
    const serverUrl = getServerUrl();
    const seen = (error: unknown): void => {
      const refused = unauthorized(error);
      if (!refused) return;
      setExpired(true);
      if (refused.url && serverUrl && refused.url.startsWith(serverUrl)) void signIn();
    };
    const queries = queryClient.getQueryCache().subscribe((event) => {
      if (event.type === 'updated' && event.action.type === 'error') seen(event.action.error);
    });
    const mutations = queryClient.getMutationCache().subscribe((event) => {
      if (event.type === 'updated' && event.action.type === 'error') seen(event.action.error);
    });
    return () => {
      queries();
      mutations();
    };
  }, [queryClient, signIn, getServerUrl]);

  if (!expired) return null;
  return (
    <Alert
      severity="error"
      data-testid="session-expired"
      sx={{mx: 3, mt: 2}}
      action={
        <Button color="inherit" size="small" onClick={() => void signIn()}>
          {t('common:session.signInAgain', 'Sign in again')}
        </Button>
      }
    >
      {t(
        'common:session.expired',
        'Your session has ended or the sign-in service could not be reached, so what is shown may be incomplete. Sign in again to continue.',
      )}
    </Alert>
  );
}
