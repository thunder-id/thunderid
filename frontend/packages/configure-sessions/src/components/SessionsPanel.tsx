// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard} from '@thunderid/components';
import {Box, Button, CircularProgress, Stack, TablePagination, Typography} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import RevokeSessionsDialog, {type RevokeTarget} from './RevokeSessionsDialog';
import SessionItem from './SessionItem';
import useGetSessions from '../api/useGetSessions';
import type {SessionFilter} from '../models/session';

const PAGE_SIZES = [10, 25, 50];

export interface SessionsPanelProps {
  /** The user or application whose sessions are shown. */
  filter: SessionFilter;
}

/**
 * The live SSO sessions of a user or of an application, each with the action to revoke it, and for a
 * user the action to revoke them all. On an application's panel each session also names its user.
 */
export default function SessionsPanel({filter}: SessionsPanelProps): JSX.Element {
  const {t} = useTranslation('sessions');
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(PAGE_SIZES[0]);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<RevokeTarget | null>(null);
  const userId = 'userId' in filter ? filter.userId : undefined;

  const {data, isLoading, error, refetch} = useGetSessions(filter, pageSize, page * pageSize);
  const sessions = data?.sessions ?? [];
  const total = data?.totalResults ?? 0;

  // Revoking the last session on a later page leaves that page empty, so step back to one that has sessions.
  if (page > 0 && sessions.length === 0 && data) {
    setPage(page - 1);
  }

  let content: JSX.Element;
  if (error) {
    content = (
      <QueryErrorNotice
        error={error}
        t={t}
        variant="inline"
        fallbackKey="sessions:panel.loadError"
        fallbackDefaultValue="Failed to load sessions"
        onRetry={() => void refetch()}
      />
    );
  } else if (isLoading) {
    content = (
      <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
        <CircularProgress size={32} />
      </Box>
    );
  } else if (sessions.length === 0) {
    content = (
      <Typography variant="body2" color="text.secondary" sx={{py: 2}}>
        {t('panel.empty', 'No active sessions')}
      </Typography>
    );
  } else {
    content = (
      <Stack spacing={1.5}>
        {sessions.map((session) => (
          <SessionItem
            key={session.id}
            session={session}
            expanded={expandedId === session.id}
            onToggle={() => setExpandedId(expandedId === session.id ? null : session.id)}
            showUser={!userId}
            onRevoke={() => setRevokeTarget({sessionId: session.id})}
          />
        ))}
        {total > PAGE_SIZES[0] && (
          <TablePagination
            component="div"
            count={total}
            page={page}
            onPageChange={(_event, value) => setPage(value)}
            rowsPerPage={pageSize}
            rowsPerPageOptions={PAGE_SIZES}
            onRowsPerPageChange={(event) => {
              setPageSize(Number(event.target.value));
              setPage(0);
            }}
          />
        )}
      </Stack>
    );
  }

  return (
    <SettingsCard
      title={t('panel.title', 'Active sessions')}
      description={
        userId
          ? t('panel.userDescription', 'Where this user is signed in. Revoking a session signs it out on that device.')
          : t(
              'panel.appDescription',
              'Sessions that signed in to this application. Revoking a session signs the user out of every application in it.',
            )
      }
      headerAction={
        userId && sessions.length > 0 ? (
          <Button variant="contained" color="error" onClick={() => setRevokeTarget({userId})}>
            {t('revokeAll.action', 'Revoke All')}
          </Button>
        ) : undefined
      }
    >
      {content}
      <RevokeSessionsDialog target={revokeTarget} onClose={() => setRevokeTarget(null)} />
    </SettingsCard>
  );
}
