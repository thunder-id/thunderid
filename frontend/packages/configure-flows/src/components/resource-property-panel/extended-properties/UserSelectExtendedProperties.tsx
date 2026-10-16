// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {Hash, SquareFunction} from '@wso2/oxygen-ui-icons-react';
import type {ReactElement, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Hint} from '../../resources/elements/hint';
import PropertySection from '../PropertySection';

/**
 * The callback a USER_SELECT loads its users with, as it is written in code, so it is not translated.
 */
const LOADER_CALLBACK = 'fetchUsers(...)';

/**
 * Props interface of {@link ReadOnlyRow}
 */
interface ReadOnlyRowPropsInterface {
  children: ReactNode;
  icon: ReactNode;
  monospace?: boolean;
  testId: string;
}

/**
 * A boxed, non-editable value with an icon in front of it.
 *
 * @param props - Props injected to the component.
 * @returns The ReadOnlyRow component.
 */
function ReadOnlyRow({children, icon, monospace = false, testId}: ReadOnlyRowPropsInterface): ReactElement {
  return (
    <Box
      data-testid={testId}
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1,
        px: 1.5,
        py: 1,
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1,
        bgcolor: 'action.hover',
      }}
    >
      {icon}
      <Typography variant="body2" sx={monospace ? {fontFamily: 'monospace', fontSize: '0.8125rem'} : undefined}>
        {children}
      </Typography>
    </Box>
  );
}

/**
 * Read-only description of what a USER_SELECT reads and what it sends. The loader belongs to the app that renders the
 * flow and the sent value is fixed, so neither is stored in the flow and there is nothing here to edit.
 *
 * @returns The UserSelectExtendedProperties component.
 */
function UserSelectExtendedProperties(): ReactElement {
  const {t} = useTranslation();

  return (
    <>
      <PropertySection title={t('flows:core.elements.userSelect.dataSource.title', 'Data source')}>
        <ReadOnlyRow testId="user-select-data-source" icon={<SquareFunction size={16} />} monospace>
          {LOADER_CALLBACK}
        </ReadOnlyRow>
        <Hint hint={t('flows:core.elements.userSelect.dataSource.hint')} />
      </PropertySection>
      <PropertySection title={t('flows:core.elements.userSelect.submitValue.title', 'Submit value')}>
        <ReadOnlyRow testId="user-select-submit-value" icon={<Hash size={16} />}>
          {t('flows:core.elements.userSelect.submitValue.value')}
        </ReadOnlyRow>
        <Hint hint={t('flows:core.elements.userSelect.submitValue.hint')} />
      </PropertySection>
    </>
  );
}

export default UserSelectExtendedProperties;
