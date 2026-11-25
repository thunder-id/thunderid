// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Stack, TextField} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AskEveryTimeSetting from './AskEveryTimeSetting';
import type {AuthorizationDetailTypeForm} from '../../hooks/useAuthorizationDetailTypeForm';

export interface GeneralSectionProps {
  form: AuthorizationDetailTypeForm;
}

/**
 * Collects, for a new authorization details type, how clients refer to it, how it is named in the
 * Console, and whether the user approves each request.
 */
export default function GeneralSection({form}: GeneralSectionProps): JSX.Element {
  const {t} = useTranslation();
  const nameServerError = form.saveErrorTarget === 'name' ? form.saveError : null;
  const typeServerError = form.saveErrorTarget === 'type' ? form.saveError : null;

  let nameHelper: string | undefined;
  if (nameServerError) nameHelper = nameServerError;
  else if (form.nameError)
    nameHelper = t('authorizationDetailTypes:form.nameError', 'Enter a name of up to 100 characters.');
  else if (form.nameMissing) nameHelper = t('authorizationDetailTypes:form.nameRequired', 'Enter a name.');

  let typeHelper: string;
  if (typeServerError) typeHelper = typeServerError;
  else if (form.typeError)
    typeHelper = t(
      'authorizationDetailTypes:form.typeError',
      'Use up to 512 ASCII letters, digits or symbols, without spaces.',
    );
  else if (form.typeMissing)
    typeHelper = t('authorizationDetailTypes:form.typeRequired', 'Enter the type identifier clients send.');
  else
    typeHelper = t(
      'authorizationDetailTypes:form.typeHint',
      'Clients send this as the type. It cannot be changed later.',
    );

  return (
    <Stack spacing={2}>
      <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', md: '1fr 1fr'}, gap: 2}}>
        <TextField
          labelPlacement="top"
          label={t('authorizationDetailTypes:form.name', 'Name')}
          required
          fullWidth
          data-save-error-target="name"
          id="authz-type-name"
          size="small"
          value={form.name}
          // A new type starts from its name.
          // eslint-disable-next-line jsx-a11y/no-autofocus
          autoFocus
          error={form.nameError || form.nameMissing || Boolean(nameServerError)}
          placeholder={t('authorizationDetailTypes:form.namePlaceholder', 'e.g., Payment')}
          onChange={(e) => form.setName(e.target.value)}
          helperText={nameHelper}
        />
        <TextField
          labelPlacement="top"
          label={t('authorizationDetailTypes:form.type', 'Type identifier')}
          required
          fullWidth
          data-save-error-target="type"
          id="authz-type-value"
          size="small"
          value={form.typeValue}
          error={form.typeError || form.typeMissing || Boolean(typeServerError)}
          placeholder={t('authorizationDetailTypes:form.typePlaceholder', 'e.g., payment_initiation')}
          onChange={(e) => form.setTypeValue(e.target.value)}
          helperText={typeHelper}
        />
      </Box>
      <TextField
        labelPlacement="top"
        label={t('authorizationDetailTypes:form.description', 'Description')}
        fullWidth
        id="authz-type-description"
        size="small"
        multiline
        minRows={2}
        value={form.description}
        placeholder={t(
          'authorizationDetailTypes:form.descriptionPlaceholder',
          'e.g., A single payment from the user’s account',
        )}
        onChange={(e) => form.setDescription(e.target.value)}
      />
      <AskEveryTimeSetting form={form} />
    </Stack>
  );
}
