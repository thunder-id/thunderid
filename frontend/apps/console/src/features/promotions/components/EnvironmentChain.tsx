// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box, Button, Card, Chip, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {ArrowRight} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import DataPlaneStatusChip from './DataPlaneStatusChip';
import PromoteDialog from './PromoteDialog';
import useEnvManagerUrl from '../api/useEnvManagerUrl';
import useGetEnvironments from '../api/useGetEnvironments';
import useSetManagedEnvironment from '../api/useSetManagedEnvironment';
import type {Environment} from '../models/promotion';

/**
 * Shows the promotion chain: one card per environment, in rank order, each with its version state
 * and a promote action into the next environment.
 */
export default function EnvironmentChain(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const baseUrl: string | undefined = useEnvManagerUrl();
  const {data, isLoading, error} = useGetEnvironments();
  const [promotion, setPromotion] = useState<{from: Environment; to: Environment} | undefined>(undefined);
  // Promotion is gated on a scope; the rest of the environment actions are not. A server that does
  // not report the flag is one that does not gate, so the action stays available.
  const canPromote: boolean = data?.canPromote ?? true;
  const setManaged = useSetManagedEnvironment();

  if (!baseUrl) {
    return (
      <Alert severity="info">
        {t('promotions:notConfigured', 'Promotions are available on the Control Plane console only.')}
      </Alert>
    );
  }

  if (isLoading) {
    return (
      <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return (
      <Box sx={{py: 8, textAlign: 'center'}}>
        <Typography variant="h6" color="error" gutterBottom>
          {t('promotions:listing.error', 'Failed to load environments')}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {error.message || t('common:messages.somethingWentWrong', 'Something went wrong')}
        </Typography>
      </Box>
    );
  }

  const environments: Environment[] = data?.environments ?? [];

  if (environments.length === 0) {
    return (
      <Alert severity="info">
        {t('promotions:listing.empty', 'No environments are registered in the environment manager yet.')}
      </Alert>
    );
  }

  const byId = new Map<string, Environment>(environments.map((env: Environment) => [env.id, env]));

  return (
    <>
      <Stack spacing={2}>
        {environments.map((env: Environment) => {
          // An environment can promote into several others, so render one action per outgoing edge.
          const successors: Environment[] = (env.promotesToResolved ?? [])
            .map((id: string) => byId.get(id))
            .filter((candidate): candidate is Environment => Boolean(candidate));
          const predecessors: string[] = (env.promotedFrom ?? [])
            .map((id: string) => byId.get(id)?.name ?? id)
            .filter(Boolean);

          return (
            <Card key={env.id} sx={{p: 2}}>
              <Stack direction="row" spacing={2} alignItems="center" flexWrap="wrap" useFlexGap>
                <Box sx={{flexGrow: 1, minWidth: 200}}>
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Typography variant="subtitle1" sx={{fontWeight: 600}}>
                      {env.name}
                    </Typography>
                    {env.managedByControlPlane && (
                      <Chip
                        size="small"
                        color="primary"
                        variant="outlined"
                        label={t('promotions:listing.managed', 'Managed here')}
                        title={t(
                          'promotions:listing.managedHint',
                          'Editing configuration here edits this environment, and a credential created here is issued against it.',
                        )}
                      />
                    )}
                    {env.hasPendingChanges && (
                      <Chip size="small" color="warning" label={t('promotions:listing.pending', 'Pending changes')} />
                    )}
                    <DataPlaneStatusChip status={env.dataPlane} />
                  </Stack>
                  <Typography variant="caption" color="text.secondary" display="block">
                    {t('promotions:listing.versionState', 'Latest v{{latest}} · Applied v{{applied}}', {
                      applied: env.appliedSeq || '-',
                      latest: env.latestSeq || '-',
                    })}
                  </Typography>
                  {predecessors.length > 0 && (
                    <Typography variant="caption" color="text.secondary" display="block">
                      {t('promotions:listing.promotedFrom', 'Promoted from: {{sources}}', {
                        sources: predecessors.join(', '),
                      })}
                    </Typography>
                  )}
                </Box>

                <Button
                  onClick={() => {
                    void navigate(`/promotions/${env.id}`);
                  }}
                >
                  {t('promotions:listing.viewHistory', 'History')}
                </Button>

                {!env.managedByControlPlane && (
                  <Button
                    disabled={setManaged.isPending}
                    onClick={() => {
                      setManaged.mutate(env.id);
                    }}
                  >
                    {t('promotions:listing.manageHere', 'Manage here')}
                  </Button>
                )}

                {canPromote &&
                  successors.map((next: Environment) => (
                    <Button
                      key={next.id}
                      variant="contained"
                      endIcon={<ArrowRight size={16} />}
                      disabled={env.latestSeq === 0}
                      onClick={() => {
                        setPromotion({from: env, to: next});
                      }}
                    >
                      {t('promotions:listing.promoteTo', 'Promote to {{target}}', {target: next.name})}
                    </Button>
                  ))}
              </Stack>
            </Card>
          );
        })}
      </Stack>

      {promotion && (
        <PromoteDialog
          open
          fromEnvId={promotion.from.id}
          fromEnvName={promotion.from.name}
          toEnvId={promotion.to.id}
          toEnvName={promotion.to.name}
          toDataPlaneConnected={promotion.to.dataPlane?.connected ?? false}
          onClose={() => {
            setPromotion(undefined);
          }}
        />
      )}
    </>
  );
}
