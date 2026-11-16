// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery} from '@tanstack/react-query';
import {CopyableField, QueryErrorNotice} from '@thunderid/components';
import type {EnvironmentValue, RuntimeEnvironment, RuntimeEnvironmentsContextType} from '@thunderid/contexts';
import {Box, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import ApplicationQueryKeys from '../../../constants/application-query-keys';

/** The height of a `CopyableField` value row, which a row standing in for a value matches. */
const ROW_HEIGHT = 41;

/**
 * Props for the {@link EnvironmentClientIdField} component.
 */
interface EnvironmentClientIdFieldProps {
  environment: RuntimeEnvironment;
  /** The resource's type, as a deploy names it. */
  resourceType?: string;
  resourceId: string;
  /** The field's label. Defaults to the application's client ID label. */
  label?: string;
  /** Reads what a field holds in an environment, as the console's environments provide it. */
  getValue: RuntimeEnvironmentsContextType['getValue'];
}

/**
 * An application's client ID in one environment. Each environment holds its own, so it is read from the
 * environment rather than from the application, and an environment that holds no client ID for it says so
 * in its place.
 */
export default function EnvironmentClientIdField({
  environment,
  resourceType = 'application',
  resourceId,
  label: labelOverride = undefined,
  getValue,
}: EnvironmentClientIdFieldProps): JSX.Element {
  const {t} = useTranslation();
  const label = labelOverride ?? t('applications:edit.general.labels.clientId', 'Client ID');
  const {data, error, refetch} = useQuery<EnvironmentValue>({
    queryKey: [ApplicationQueryKeys.ENVIRONMENT_VALUE, environment.id, resourceId, 'clientId'],
    queryFn: () => getValue(environment.id, {resourceType, resourceId, field: 'clientId'}),
    retry: false,
  });

  // A failed read shows in place of the value, also where an earlier read of it succeeded.
  if (data?.status === 'set' && !error) {
    return <CopyableField label={label} value={data.value} />;
  }

  let content: ReactNode;
  if (error) {
    content = <QueryErrorNotice error={error} t={t} variant="inline" onRetry={() => void refetch()} />;
  } else if (!data) {
    content = (
      <CircularProgress
        size={16}
        aria-label={t(
          'applications:edit.overview.environments.clientIdLoading',
          'Loading the client ID in {{environment}}',
          {
            environment: environment.name,
          },
        )}
      />
    );
  } else {
    // Laid out as a value's row is, and as tall, so the fields below do not move between environments.
    content = (
      <Stack
        direction="row"
        alignItems="center"
        sx={{
          border: 1,
          borderColor: 'divider',
          borderStyle: 'dashed',
          borderRadius: 1,
          px: 1.25,
          minHeight: ROW_HEIGHT,
        }}
      >
        <Typography variant="caption" color="text.secondary">
          {t('applications:edit.overview.environments.clientIdUnset', 'No client ID is set in {{environment}} yet.', {
            environment: environment.name,
          })}
        </Typography>
      </Stack>
    );
  }

  return (
    <Box sx={{mb: 1.5, '&:last-child': {mb: 0}}}>
      <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 0.5}}>
        {label}
      </Typography>
      {content}
    </Box>
  );
}
