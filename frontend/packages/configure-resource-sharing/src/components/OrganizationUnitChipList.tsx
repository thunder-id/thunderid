// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationUnit} from '@thunderid/configure-organization-units';
import {Chip, Stack, Typography} from '@wso2/oxygen-ui';
import {Plus, X} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface OrganizationUnitChipListProps {
  ouIds: string[];
  /**
   * Loaded units by id. A unit not loaded yet is shown by its id.
   */
  unitsById: Map<string, OrganizationUnit>;
  /**
   * Removes one unit. Omit for a read-only list.
   */
  onRemove?: (ouId: string) => void;
  /**
   * Opens a picker to add units. Omit for a read-only list.
   */
  onAdd?: () => void;
  addLabel?: string;
  emptyLabel: string;
}

/**
 * A wrapping row of organization units as chips, used for exceptions.
 */
export default function OrganizationUnitChipList({
  ouIds,
  unitsById,
  onRemove = undefined,
  onAdd = undefined,
  addLabel = undefined,
  emptyLabel,
}: OrganizationUnitChipListProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Stack direction="row" flexWrap="wrap" gap={1} alignItems="center">
      {ouIds.length === 0 && !onAdd && (
        <Typography variant="body2" color="text.secondary">
          {emptyLabel}
        </Typography>
      )}
      {ouIds.map((ouId) => {
        const name = unitsById.get(ouId)?.name ?? ouId;

        return (
          <Chip
            key={ouId}
            label={name}
            variant="outlined"
            onDelete={onRemove ? () => onRemove(ouId) : undefined}
            deleteIcon={
              <X
                size={14}
                aria-label={t('resourceSharing:exceptions.remove', 'Remove {{name}} from exceptions', {name})}
              />
            }
          />
        );
      })}
      {onAdd && (
        <Chip
          icon={<Plus size={14} />}
          label={addLabel}
          variant="outlined"
          onClick={onAdd}
          sx={{borderStyle: 'dashed'}}
        />
      )}
    </Stack>
  );
}
