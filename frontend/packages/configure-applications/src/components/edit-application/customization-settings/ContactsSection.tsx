// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Kbd, SettingsCard} from '@thunderid/components';
import {Autocomplete, Chip, TextField} from '@wso2/oxygen-ui';
import {useState} from 'react';
import {useTranslation, Trans} from 'react-i18next';
import {z} from 'zod';
import type {Application} from '@thunderid/configure-applications';

/**
 * Props for the {@link ContactsSection} component.
 */
interface ContactsSectionProps {
  /**
   * The application being edited
   */
  application: Application;
  /**
   * Partial application object containing edited fields
   */
  editedApp: Partial<Application>;
  /**
   * Callback function to handle field value changes
   * @param field - The application field being updated
   * @param value - The new value for the field
   */
  onFieldChange: (field: keyof Application, value: unknown) => void;
  /**
   * Singular noun used to refer to the entity in user-visible copy (default: 'application').
   */
  entityLabel?: string;
}

/**
 * Section component for configuring application contact information.
 *
 * Provides a creatable Autocomplete field where contacts are added as chips.
 * Type an email address and press Enter to add it. Invalid emails are rejected
 * with an inline error message.
 * Changes are sent to the parent as a string array.
 *
 * @param props - Component props
 * @returns Contact information input UI within a SettingsCard
 */
export default function ContactsSection({
  application,
  editedApp,
  onFieldChange,
  entityLabel = 'application',
}: ContactsSectionProps) {
  const {t} = useTranslation();

  const [inputError, setInputError] = useState('');

  const emailSchema = z.string().email();

  const contacts = editedApp.contacts ?? application.contacts ?? [];

  const handleChange = (_event: unknown, newValue: string[], reason: string) => {
    if (reason === 'createOption') {
      const candidate = newValue[newValue.length - 1];
      const result = emailSchema.safeParse(candidate);
      if (!result.success) {
        setInputError(t('applications:edit.general.contacts.error.invalid'));
        return;
      }
    }
    setInputError('');
    onFieldChange('contacts', newValue);
  };

  return (
    <SettingsCard
      title={t('applications:edit.general.sections.contacts')}
      description={t(
        'applications:edit.general.sections.contacts.description',
        'Contact email addresses for {{entity}} administrators.',
        {entity: entityLabel},
      )}
    >
      <Autocomplete
        multiple
        freeSolo
        fullWidth
        options={[]}
        value={contacts}
        onChange={handleChange}
        onInputChange={() => {
          if (inputError) setInputError('');
        }}
        disabled={application.isReadOnly}
        renderTags={(value, getTagProps) =>
          value.map((option, index) => <Chip label={option} {...getTagProps({index})} key={option} />)
        }
        renderInput={(params) => (
          <TextField
            {...params}
            placeholder={t('applications:edit.general.contacts.placeholder')}
            error={!!inputError}
            helperText={
              inputError || <Trans i18nKey="applications:edit.general.contacts.hint" components={[<Kbd key="kbd" />]} />
            }
          />
        )}
      />
    </SettingsCard>
  );
}
