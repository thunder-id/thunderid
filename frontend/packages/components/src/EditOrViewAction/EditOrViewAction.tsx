// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {IconButton, Tooltip} from '@wso2/oxygen-ui';
import {Eye, Pencil} from '@wso2/oxygen-ui-icons-react';
import type {JSX, MouseEvent} from 'react';
import {useTranslation} from 'react-i18next';

export interface EditOrViewActionProps {
  /** Whether the admin may update the row. Otherwise the action reads View and opens the page read-only. */
  canEdit: boolean;
  /** Opens the row. The click does not reach the row itself. */
  onClick: (event: MouseEvent<HTMLButtonElement>) => void;
  /** Label of the edit action. Defaults to Edit. */
  editLabel?: string;
}

/**
 * The row action that opens a resource: Edit for an admin who may update it, View for one who may only view it.
 */
export default function EditOrViewAction({
  canEdit,
  onClick,
  editLabel = undefined,
}: EditOrViewActionProps): JSX.Element {
  const {t} = useTranslation();
  const label: string = canEdit ? (editLabel ?? t('common:actions.edit', 'Edit')) : t('common:actions.view', 'View');

  return (
    <Tooltip title={label}>
      <IconButton
        aria-label={label}
        size="small"
        onClick={(e: MouseEvent<HTMLButtonElement>) => {
          e.stopPropagation();
          onClick(e);
        }}
      >
        {canEdit ? <Pencil size={16} /> : <Eye size={16} />}
      </IconButton>
    </Tooltip>
  );
}
