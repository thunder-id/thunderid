// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, QueryErrorNotice} from '@thunderid/components';
import {Box, CircularProgress, PageContent} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Navigate, useParams} from 'react-router';
import useGetNotificationTemplate from '../api/useGetNotificationTemplate';
import TemplateEditor from '../components/edit-template/TemplateEditor';
import useNotificationTemplateRoutes from '../hooks/useNotificationTemplateRoutes';
import {isNotificationChannel, type NotificationChannel} from '../models/notification-template';

/**
 * Edit page for a single notification template. Reads the channel and id from the route,
 * loads the template, and renders the editor once it is available.
 *
 * @public
 */
export default function NotificationTemplateEditPage(): JSX.Element {
  const {t} = useTranslation('notificationTemplates');
  const routes = useNotificationTemplateRoutes();
  const {channel: channelParam, id} = useParams<{channel: string; id: string}>();
  const channel: NotificationChannel = channelParam === 'sms' ? 'sms' : 'email';

  const {data: template, isLoading, error, refetch} = useGetNotificationTemplate(channel, id ?? '');

  // An unknown channel segment is not a real page; send it to the canonical list.
  if (!isNotificationChannel(channelParam)) {
    return <Navigate to={routes.notificationTemplates.list()} replace />;
  }

  const title =
    template?.displayName ??
    (channel === 'email'
      ? t('navigation:pages.emailTemplates', 'Email templates')
      : t('navigation:pages.smsTemplates', 'SMS templates'));

  return (
    <PageContent>
      <DocumentTitle title={title} />
      {error ? (
        <QueryErrorNotice
          error={error}
          t={t}
          title={t('editor.loadError', 'Failed to load template')}
          onRetry={() => void refetch()}
        />
      ) : !id ? (
        <QueryErrorNotice
          error={new Error('missing id')}
          t={t}
          title={t('editor.loadError', 'Failed to load template')}
        />
      ) : isLoading || !template ? (
        <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
          <CircularProgress />
        </Box>
      ) : (
        <TemplateEditor key={template.id} channel={channel} template={template} />
      )}
    </PageContent>
  );
}
