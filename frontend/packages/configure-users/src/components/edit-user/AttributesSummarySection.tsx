// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {useResolveDisplayName} from '@thunderid/hooks';
import type {User} from '@thunderid/types';
import {Box, Chip, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetUserType from '../../api/useGetUserType';
import useGetUserTypes from '../../api/useGetUserTypes';

interface AttributesSummarySectionProps {
  user: User;
}

const formatValue = (value: unknown): string => {
  if (value === null || value === undefined) return '-';
  if (Array.isArray(value)) return value.join(', ');
  if (typeof value === 'object') return JSON.stringify(value);
  if (typeof value === 'string' || typeof value === 'number') return String(value);
  return '-';
};

/**
 * Read-only preview of this user's attribute values, with no edit affordance — used on the
 * General tab. The Attributes tab is where values are actually edited.
 */
export default function AttributesSummarySection({user}: AttributesSummarySectionProps): JSX.Element {
  const {t} = useTranslation();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});

  const {data: userTypeList} = useGetUserTypes();
  const matchedSchema = userTypeList?.types?.find((s) => s.handle === user.type);
  const {data: userTypeDetails, isLoading} = useGetUserType(matchedSchema?.id);

  const attributes = user.attributes ?? {};

  const labelFor = (key: string): string => {
    const fieldDef = userTypeDetails?.schema?.[key];
    if (fieldDef?.displayName) {
      return resolveDisplayName(fieldDef.displayName) || key;
    }
    return key;
  };

  return (
    <SettingsCard
      title={t('users:manageUser.sections.attributes.title', 'User Attributes')}
      description={t(
        'users:manageUser.sections.attributes.summaryDescription',
        "A preview of this user's attribute values. Manage them from the Attributes tab.",
      )}
    >
      {isLoading ? (
        <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
          <CircularProgress size={32} />
        </Box>
      ) : Object.keys(attributes).length > 0 ? (
        <Stack spacing={2}>
          {Object.entries(attributes).map(([key, value]) => (
            <Box key={key}>
              <Typography variant="caption" color="text.secondary">
                {labelFor(key)}
              </Typography>
              {typeof value === 'boolean' ? (
                <Box>
                  <Chip
                    label={value ? t('common:actions.yes', 'Yes') : t('common:actions.no', 'No')}
                    size="small"
                    color={value ? 'success' : 'default'}
                    variant="outlined"
                  />
                </Box>
              ) : (
                <Typography variant="body1">{formatValue(value)}</Typography>
              )}
            </Box>
          ))}
        </Stack>
      ) : (
        <Typography variant="body2" color="text.secondary">
          {t('users:manageUser.sections.attributes.empty', 'No attributes available')}
        </Typography>
      )}
    </SettingsCard>
  );
}
