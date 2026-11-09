// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  FormControl,
  FormControlLabel,
  FormLabel,
  MenuItem,
  Radio,
  RadioGroup,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {AgentOwner, AgentOwnerIdType} from '../models/agent';
import {getExternalOwnerIdError} from '../utils/externalOwner';

type OwnerKind = 'internal' | 'external';

interface AgentOwnerFieldProps {
  /** Prefix for the field's element IDs. The label's ID is `<id>-label`, for the user picker to point at. */
  id: string;
  /** The owner the field opens with. An external one opens on External with its type and ID. */
  initialOwner?: AgentOwner;
  /** The picker of this deployment's users, shown while Internal is chosen. */
  internalPicker: ReactNode;
  /** Called with the external owner as it is entered, and with undefined when Internal is chosen. */
  onExternalChange: (owner: AgentOwner | undefined) => void;
  disabled?: boolean;
  required?: boolean;
  size?: 'small' | 'medium';
}

const ID_TYPES: AgentOwnerIdType[] = ['email', 'mobile', 'uuid'];

/**
 * Chooses an agent's owner: a user of this deployment, through the picker the caller supplies, or
 * someone managed in another identity system, by the type and value of their ID. An external owner
 * is stored as entered and is never looked up here.
 */
export default function AgentOwnerField({
  id,
  initialOwner = undefined,
  internalPicker,
  onExternalChange,
  disabled = false,
  required = false,
  size = 'medium',
}: AgentOwnerFieldProps): JSX.Element {
  const {t} = useTranslation();
  const startsExternal = initialOwner?.isLocal === false;
  const [kind, setKind] = useState<OwnerKind>(startsExternal ? 'external' : 'internal');
  const [idType, setIdType] = useState<AgentOwnerIdType>(startsExternal ? (initialOwner?.idType ?? 'uuid') : 'email');
  const [externalId, setExternalId] = useState<string>(startsExternal ? (initialOwner?.id ?? '') : '');
  const [touched, setTouched] = useState<boolean>(false);

  const emitExternal = (nextType: AgentOwnerIdType, nextId: string): void => {
    onExternalChange({id: nextId.trim(), idType: nextType, isLocal: false});
  };

  const handleKindChange = (next: OwnerKind): void => {
    setKind(next);
    if (next === 'external') {
      emitExternal(idType, externalId);
    } else {
      onExternalChange(undefined);
    }
  };

  const idTypeLabels: Record<AgentOwnerIdType, string> = {
    email: t('agents:owner.idType.email', 'Email'),
    mobile: t('agents:owner.idType.mobile', 'Mobile number'),
    uuid: t('agents:owner.idType.uuid', 'User ID'),
  };
  const placeholders: Record<AgentOwnerIdType, string> = {
    email: t('agents:owner.placeholder.email', 'jane@example.com'),
    mobile: t('agents:owner.placeholder.mobile', '+94771234567'),
    uuid: t('agents:owner.placeholder.uuid', "The user's ID in their identity system"),
  };

  const idError = getExternalOwnerIdError(externalId, idType);
  const showError = idError !== undefined && (touched || externalId.trim() !== '');
  let idHelperText = t('agents:owner.externalHint', 'Stored as entered. It is not looked up in this deployment.');
  if (showError && idError === 'empty') {
    idHelperText = t('agents:owner.errors.empty', "Enter the owner's ID.");
  } else if (showError && idError === 'tooLong') {
    idHelperText = t('agents:owner.errors.tooLong', 'Use at most 256 characters.');
  } else if (showError && idType === 'email') {
    idHelperText = t('agents:owner.errors.email', 'Enter an email address, such as jane@example.com.');
  } else if (showError && idType === 'mobile') {
    idHelperText = t(
      'agents:owner.errors.mobile',
      'Enter the number in international format: a + followed by the country code and number.',
    );
  }

  return (
    <FormControl fullWidth size={size} required={required} disabled={disabled}>
      <FormLabel id={`${id}-label`}>{t('agents:owner.label', 'Owner')}</FormLabel>
      <RadioGroup
        row
        aria-labelledby={`${id}-label`}
        value={kind}
        onChange={(event) => handleKindChange(event.target.value as OwnerKind)}
      >
        <FormControlLabel
          value="internal"
          control={<Radio size={size} />}
          label={t('agents:owner.kind.internal', 'Internal')}
          disabled={disabled}
        />
        <FormControlLabel
          value="external"
          control={<Radio size={size} />}
          label={t('agents:owner.kind.external', 'External')}
          disabled={disabled}
        />
      </RadioGroup>
      <Typography variant="caption" color="text.secondary" sx={{mb: 1}}>
        {kind === 'internal'
          ? t('agents:owner.kind.internalHint', 'A user in this deployment.')
          : t('agents:owner.kind.externalHint', 'Someone managed in another identity system.')}
      </Typography>

      {kind === 'internal' ? (
        internalPicker
      ) : (
        <Stack direction={{xs: 'column', sm: 'row'}} spacing={2} alignItems="flex-start">
          <FormControl size={size} sx={{minWidth: 180}} disabled={disabled}>
            <FormLabel id={`${id}-id-type-label`}>{t('agents:owner.idTypeLabel', 'ID type')}</FormLabel>
            <Select
              id={`${id}-id-type`}
              labelId={`${id}-id-type-label`}
              value={idType}
              onChange={(event) => {
                const next = event.target.value as AgentOwnerIdType;
                setIdType(next);
                emitExternal(next, externalId);
              }}
            >
              {ID_TYPES.map((type) => (
                <MenuItem key={type} value={type}>
                  {idTypeLabels[type]}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <FormControl size={size} fullWidth disabled={disabled}>
            <FormLabel htmlFor={`${id}-external-id`}>{idTypeLabels[idType]}</FormLabel>
            <TextField
              id={`${id}-external-id`}
              size={size}
              value={externalId}
              placeholder={placeholders[idType]}
              disabled={disabled}
              error={showError}
              helperText={idHelperText}
              onBlur={() => setTouched(true)}
              onChange={(event) => {
                setExternalId(event.target.value);
                emitExternal(idType, event.target.value);
              }}
            />
          </FormControl>
        </Stack>
      )}
    </FormControl>
  );
}
