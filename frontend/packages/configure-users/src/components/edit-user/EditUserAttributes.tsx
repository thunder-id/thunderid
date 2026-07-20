/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {SettingsCard} from '@thunderid/components';
import {useResolveDisplayName} from '@thunderid/hooks';
import type {User} from '@thunderid/types';
import {Box, CircularProgress, Typography} from '@wso2/oxygen-ui';
import {useEffect, type JSX} from 'react';
import {useForm, useWatch} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import AttributesSummarySection from './AttributesSummarySection';
import useGetUserType from '../../api/useGetUserType';
import useGetUserTypes from '../../api/useGetUserTypes';
import renderSchemaField from '../../utils/renderSchemaField';

interface EditUserAttributesProps {
  user: User;
  editedUser: Partial<User>;
  onFieldChange: (field: keyof User, value: unknown) => void;
}

type AttributeFormData = Record<string, unknown>;

const filterAttributes = (data: AttributeFormData): AttributeFormData =>
  Object.fromEntries(Object.entries(data).filter(([, v]) => v !== '' && v !== undefined && v !== null));

/**
 * Every field edit stages directly into the page's shared editedUser state via onFieldChange —
 * the page-level Save/Reset bar is the only thing that ever persists it, same as every other
 * section. The parent remounts this component (via a `key` bumped on Save/Reset) so its local
 * react-hook-form state always starts fresh from the current attributes.
 */
export default function EditUserAttributes({user, editedUser, onFieldChange}: EditUserAttributesProps): JSX.Element {
  const {t} = useTranslation();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});

  const {data: userTypeList} = useGetUserTypes();
  const matchedSchema = userTypeList?.types?.find((s) => s.name === user.type);
  const {data: userTypeDetails, isLoading} = useGetUserType(matchedSchema?.id);

  const attributes = (editedUser.attributes ?? user.attributes ?? {}) as AttributeFormData;

  const {
    control,
    formState: {errors},
  } = useForm<AttributeFormData>({
    defaultValues: attributes,
    mode: 'onChange',
  });

  const watchedValues = useWatch({control});

  useEffect(() => {
    onFieldChange('attributes', filterAttributes(watchedValues));
  }, [watchedValues, onFieldChange]);

  if (isLoading) {
    return (
      <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
        <CircularProgress size={32} />
      </Box>
    );
  }

  // A read-only user can't be edited at all, so there's nothing for a form to do here — fall
  // back to the same summary shown on the General tab.
  if (user.isReadOnly) {
    return <AttributesSummarySection user={user} />;
  }

  const schemaFields = userTypeDetails?.schema
    ? Object.entries(userTypeDetails.schema).filter(
        ([, fieldDef]) => !((fieldDef.type === 'string' || fieldDef.type === 'number') && fieldDef.credential),
      )
    : [];

  return (
    <SettingsCard
      title={t('users:manageUser.sections.attributes.title', 'Attributes')}
      description={t('users:manageUser.sections.attributes.description', 'manage user attribute values.')}
    >
      <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
        {schemaFields.length > 0 ? (
          schemaFields.map(([fieldName, fieldDef]) =>
            renderSchemaField(fieldName, fieldDef, control, errors, resolveDisplayName),
          )
        ) : (
          <Typography variant="body2" color="text.secondary">
            {t('users:manageUser.sections.attributes.noSchema', 'No schema available for editing')}
          </Typography>
        )}
      </Box>
    </SettingsCard>
  );
}
