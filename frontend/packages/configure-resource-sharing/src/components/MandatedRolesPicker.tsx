// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Autocomplete, Chip, CircularProgress, FormControl, FormLabel, TextField} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetMandatableRoles from '../api/useGetMandatableRoles';
import type {MandatableRole} from '../models/mandatable-role';

export interface MandatedRolesPickerProps {
  /**
   * The child unit the roles are mandated in.
   */
  ouId: string;
  /**
   * The unit issuing the share, whose roles are the candidates.
   */
  issuerOuId: string;
  /**
   * The unit's name, for the field's hint.
   */
  unitName: string;
  value: string[];
  readOnly?: boolean;
  onChange: (roleIds: string[]) => void;
}

/**
 * The roles a user share mandates in one named child unit. The unit cannot remove them.
 */
export default function MandatedRolesPicker({
  ouId,
  issuerOuId,
  unitName,
  value,
  readOnly = false,
  onChange,
}: MandatedRolesPickerProps): JSX.Element {
  const {t} = useTranslation();
  const {data, isLoading, isError} = useGetMandatableRoles(ouId, issuerOuId);
  const candidates: MandatableRole[] = data?.candidates ?? [];
  const fieldId = `mandated-roles-${ouId}`;

  // A mandated role the candidates no longer list still shows, by the name the issuer sees it under or
  // else by its ID, so it can be removed.
  const selected: MandatableRole[] = value.map(
    (id) => candidates.find((role) => role.id === id) ?? {id, name: data?.names[id] ?? id, ouId: issuerOuId},
  );

  return (
    <FormControl fullWidth sx={{maxWidth: 480}}>
      <FormLabel htmlFor={fieldId}>{t('resourceSharing:mandatedRoles.label', 'Mandated roles')}</FormLabel>
      <Autocomplete
        multiple
        fullWidth
        size="small"
        id={fieldId}
        readOnly={readOnly}
        disableClearable={readOnly}
        loading={isLoading}
        options={candidates}
        value={selected}
        getOptionLabel={(role: MandatableRole) => role.name}
        isOptionEqualToValue={(option: MandatableRole, current: MandatableRole) => option.id === current.id}
        noOptionsText={t(
          'resourceSharing:mandatedRoles.noOptions',
          'No roles of yours are shared with this organization unit.',
        )}
        onChange={(_event, roles: MandatableRole[]) => onChange(roles.map((role) => role.id))}
        renderInput={(params) => (
          <TextField
            {...params}
            placeholder={
              value.length === 0 ? t('resourceSharing:mandatedRoles.placeholder', 'Select roles') : undefined
            }
            error={isError}
            helperText={
              isError
                ? t('resourceSharing:mandatedRoles.loadError', 'Failed to load roles.')
                : t(
                    'resourceSharing:mandatedRoles.hint',
                    '{{name}} holds these roles for the user and cannot remove them.',
                    {
                      name: unitName,
                    },
                  )
            }
            InputProps={{
              ...params.InputProps,
              endAdornment: (
                <>
                  {isLoading ? <CircularProgress color="inherit" size={20} /> : null}
                  {params.InputProps.endAdornment}
                </>
              ),
            }}
          />
        )}
        renderTags={(roles: MandatableRole[], getTagProps) =>
          roles.map((role, index) => <Chip label={role.name} {...getTagProps({index})} key={role.id} />)
        }
      />
    </FormControl>
  );
}
