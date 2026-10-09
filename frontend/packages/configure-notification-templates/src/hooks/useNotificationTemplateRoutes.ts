// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';
import {useMemo} from 'react';
import type {NotificationChannel} from '../models/notification-template';

/**
 * Route paths this package needs from the host application.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * package rendered standalone in a unit test), `useNotificationTemplateRoutes` falls back to
 * `defaultNotificationTemplateRoutePaths` below.
 *
 * @public
 */
export interface NotificationTemplateRoutePaths {
  notificationTemplates: {
    list: () => string;
    channel: (channel: NotificationChannel) => string;
    create: (channel: NotificationChannel) => string;
    detail: (channel: NotificationChannel, id: string) => string;
  };
}

/**
 * Default notification-template paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultNotificationTemplateRoutePaths: NotificationTemplateRoutePaths = {
  notificationTemplates: {
    list: () => '/notification-templates',
    channel: (channel) => `/notification-templates/${channel}`,
    create: (channel) => `/notification-templates/${channel}/create`,
    detail: (channel, id) => `/notification-templates/${channel}/${id}`,
  },
};

/**
 * Resolves the notification-template route paths, preferring the host application's configuration
 * (supplied via `RoutesProvider`) and falling back to this package's own defaults.
 *
 * Components should never hardcode these destination paths; they should call this hook and build
 * the destination from the returned functions instead.
 *
 * @public
 */
export default function useNotificationTemplateRoutes(): NotificationTemplateRoutePaths {
  const {notificationTemplates} = useRoutes<Partial<NotificationTemplateRoutePaths>>();
  return useMemo(
    () => ({
      notificationTemplates: notificationTemplates ?? defaultNotificationTemplateRoutePaths.notificationTemplates,
    }),
    [notificationTemplates],
  );
}
