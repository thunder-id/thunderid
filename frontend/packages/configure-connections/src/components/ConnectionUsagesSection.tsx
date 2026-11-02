// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard} from '@thunderid/components';
import {List, ListItem, ListItemText, Skeleton, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetConnectionUsages from '../api/useGetConnectionUsages';
import {ConnectionUsageResourceTypes, type ConnectionType} from '../models/connection';

interface ConnectionUsagesSectionProps {
  connectionType: ConnectionType;
  connectionId: string;
}

/**
 * Lists the resources that reference a connection.
 */
export default function ConnectionUsagesSection({
  connectionType,
  connectionId,
}: ConnectionUsagesSectionProps): JSX.Element {
  const {t} = useTranslation('connections');
  const usagesQuery = useGetConnectionUsages(connectionType, connectionId);
  const usages = usagesQuery.data?.usages ?? [];
  const usagesKnown = usagesQuery.data !== undefined && usagesQuery.data.totalResults !== null;

  const resourceTypeLabel = (resourceType: string): string =>
    resourceType === ConnectionUsageResourceTypes.FLOW
      ? t('detail.usages.resourceType.flow', 'Flow')
      : t('detail.usages.resourceType.other', 'Resource');

  let content: JSX.Element;
  if (usagesQuery.isLoading) {
    content = <Skeleton variant="rounded" height={40} />;
  } else if (usagesQuery.error) {
    content = (
      <QueryErrorNotice
        error={usagesQuery.error}
        t={t}
        variant="inline"
        fallbackKey="detail.usages.loadError"
        fallbackDefaultValue="Failed to load the resources that use this connection."
        onRetry={() => void usagesQuery.refetch()}
      />
    );
  } else if (!usagesKnown) {
    content = (
      <Typography variant="body2" color="text.secondary">
        {t('detail.usages.unavailable', 'The resources that use this connection could not be determined.')}
      </Typography>
    );
  } else if (usages.length === 0) {
    content = (
      <Typography variant="body2" color="text.secondary">
        {t('detail.usages.empty', 'No resources use this connection.')}
      </Typography>
    );
  } else {
    content = (
      <List dense disablePadding data-testid="connection-usages-list">
        {usages.map((usage) => (
          <ListItem key={`${usage.resourceType}-${usage.id}`} disableGutters>
            <ListItemText primary={usage.displayName || usage.id} secondary={resourceTypeLabel(usage.resourceType)} />
          </ListItem>
        ))}
      </List>
    );
  }

  return (
    <SettingsCard
      title={t('detail.usages.title', 'Usages')}
      description={t(
        'detail.usages.description',
        'Resources that use this connection. A connection cannot be deleted while a resource uses it.',
      )}
    >
      {content}
    </SettingsCard>
  );
}
