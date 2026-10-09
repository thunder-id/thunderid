// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import NotificationTemplatesList from '../components/NotificationTemplatesList';
import useNotificationTemplateRoutes from '../hooks/useNotificationTemplateRoutes';
import type {NotificationChannel} from '../models/notification-template';

/**
 * Landing page for a single notification channel's templates. The channel is read from the
 * `:channel` route parameter; the Email and SMS variants are reached from the sidebar.
 *
 * @public
 */
export default function NotificationTemplatesListPage(): JSX.Element {
  const {t} = useTranslation('notificationTemplates');
  const navigate = useNavigate();
  const logger = useLogger('NotificationTemplatesListPage');
  const routes = useNotificationTemplateRoutes();
  const {channel: channelParam} = useParams<{channel: string}>();
  const channel: NotificationChannel = channelParam === 'sms' ? 'sms' : 'email';

  const handleCreate = (): void => {
    (async (): Promise<void> => {
      await navigate(routes.notificationTemplates.create(channel));
    })().catch((error: unknown) => {
      logger.error('Failed to navigate to create notification template page', {error, channel});
    });
  };

  const title =
    channel === 'email' ? t('listing.title.email', 'Email Templates') : t('listing.title.sms', 'SMS Templates');
  const subtitle =
    channel === 'email'
      ? t('listing.subtitle.email', 'Create, customize, and localize the email notifications your applications send.')
      : t('listing.subtitle.sms', 'Create, customize, and localize the SMS notifications your applications send.');

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{title}</PageTitle.Header>
        <PageTitle.SubHeader>{subtitle}</PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            data-testid="notification-template-add-button"
            variant="contained"
            startIcon={<Plus size={18} />}
            onClick={handleCreate}
          >
            {t('listing.addTemplate', 'New template')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      <NotificationTemplatesList channel={channel} />
    </PageContent>
  );
}
