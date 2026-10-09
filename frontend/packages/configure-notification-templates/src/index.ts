// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// API Hooks
export {default as useGetNotificationTemplates} from './api/useGetNotificationTemplates';
export type {UseGetNotificationTemplatesParams} from './api/useGetNotificationTemplates';
export {default as useDeleteNotificationTemplate} from './api/useDeleteNotificationTemplate';
export type {DeleteNotificationTemplateVariables} from './api/useDeleteNotificationTemplate';

// Models & Types
export type {
  EmailTemplate,
  EmailTemplateContent,
  NotificationChannel,
  NotificationTemplate,
  SmsTemplate,
  SmsTemplateContent,
  TemplateColorScheme,
  TemplateDesign,
  TemplateSummary,
} from './models/notification-template';
export type {TemplateLink, TemplateListResponse} from './models/responses';

// Constants
export {default as NotificationTemplateQueryKeys} from './constants/notification-template-query-keys';

// Components
export {default as NotificationTemplatesList} from './components/NotificationTemplatesList';
export type {NotificationTemplatesListProps} from './components/NotificationTemplatesList';
export {default as NotificationTemplateDeleteDialog} from './components/NotificationTemplateDeleteDialog';
export type {NotificationTemplateDeleteDialogProps} from './components/NotificationTemplateDeleteDialog';

// Pages
export {default as NotificationTemplatesListPage} from './pages/NotificationTemplatesListPage';
export {default as NotificationTemplateEditPage} from './pages/NotificationTemplateEditPage';
export {default as NotificationTemplateCreatePage} from './pages/NotificationTemplateCreatePage';

// Routes
export type {NotificationTemplateRoutePaths} from './hooks/useNotificationTemplateRoutes';
export {
  defaultNotificationTemplateRoutePaths,
  default as useNotificationTemplateRoutes,
} from './hooks/useNotificationTemplateRoutes';
