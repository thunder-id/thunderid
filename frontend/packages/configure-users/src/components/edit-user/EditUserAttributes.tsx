// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {useResolveDisplayName} from '@thunderid/hooks';
import type {User} from '@thunderid/types';
import {orderAttributes} from '@thunderid/utils';
import {Box, CircularProgress, Typography} from '@wso2/oxygen-ui';
import {useEffect, useRef, type JSX} from 'react';
import {useForm, useWatch} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import AttributesSummarySection from './AttributesSummarySection';
import SendVerificationLinkButton from './SendVerificationLinkButton';
import VerificationStatus from './VerificationStatus';
import useGetUserType from '../../api/useGetUserType';
import useGetUserTypes from '../../api/useGetUserTypes';
import UserConstants from '../../constants/user-constants';
import getVerifiableAttributes from '../../utils/getVerifiableAttributes';
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
  const matchedSchema = userTypeList?.types?.find((s) => s.handle === user.type);
  const {data: userTypeDetails, isLoading} = useGetUserType(matchedSchema?.id);

  const attributes = (editedUser.attributes ?? user.attributes ?? {}) as AttributeFormData;
  const verifiable = getVerifiableAttributes(user, userTypeDetails?.schema);

  const {
    control,
    formState: {errors},
  } = useForm<AttributeFormData>({
    defaultValues: attributes,
    mode: 'onChange',
  });

  const watchedValues = useWatch({control});

  // Staging re-renders the page, which can recreate onFieldChange. Keying the effect on the
  // callback would restage and loop, so keep it keyed on the watched values only.
  const onFieldChangeRef = useRef(onFieldChange);
  useEffect(() => {
    onFieldChangeRef.current = onFieldChange;
  }, [onFieldChange]);

  useEffect(() => {
    onFieldChangeRef.current('attributes', filterAttributes(watchedValues));
  }, [watchedValues]);

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
    ? orderAttributes(userTypeDetails.schema).filter(
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
          schemaFields.map(([fieldName, fieldDef]) => {
            // The status is for the stored value, so it hides while the field holds an edited one.
            const value = watchedValues[fieldName];
            const edited = value !== user.attributes?.[fieldName];
            const showStatus = verifiable.has(fieldName) && !edited;
            // An unverified value can be sent a link, and a non-blank edited one once saved.
            const sendable = edited ? typeof value === 'string' && value.trim() !== '' : !verifiable.get(fieldName);
            const resolvedLabel = fieldDef.displayName ? resolveDisplayName(fieldDef.displayName) : '';
            const showAction =
              verifiable.has(fieldName) && UserConstants.ADMIN_VERIFIABLE_TYPES.includes(fieldDef.type) && sendable;

            return renderSchemaField(
              fieldName,
              fieldDef,
              control,
              errors,
              resolveDisplayName,
              showStatus ? <VerificationStatus verifiedAt={verifiable.get(fieldName)} /> : undefined,
              showAction ? (
                <SendVerificationLinkButton
                  userId={user.id}
                  attribute={fieldName}
                  label={resolvedLabel !== '' ? resolvedLabel : fieldName}
                  accessStatus={user.status?.value}
                  edited={edited}
                />
              ) : undefined,
            );
          })
        ) : (
          <Typography variant="body2" color="text.secondary">
            {t('users:manageUser.sections.attributes.noSchema', 'No schema available for editing')}
          </Typography>
        )}
      </Box>
    </SettingsCard>
  );
}
