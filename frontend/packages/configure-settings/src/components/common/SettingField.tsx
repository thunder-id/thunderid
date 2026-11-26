// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import {RotateCcw} from '@wso2/oxygen-ui-icons-react';
import {useId, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props for {@link SettingField}.
 *
 * @public
 */
export interface SettingFieldProps {
  /** The setting's name. */
  label: string;
  /** What the setting does, shown below the control. */
  hint?: string;
  /** Whether the section sets this value (Custom) rather than leaving it to the default. */
  custom: boolean;
  /**
   * Removes the value from the section, so the default applies again. Offered only for a custom
   * value; leave it out where the value can't be changed here, such as in a locked section.
   */
  onReset?: () => void;
  /** Whether the setting can't be changed right now, as while a save is in flight. Disables the reset too. */
  disabled?: boolean;
  /**
   * Renders the control that edits the value. It receives the id of the hint, for the control's
   * `aria-describedby`, or `undefined` when there is no hint.
   */
  children: (hintId: string | undefined) => ReactNode;
}

/**
 * One setting of a server-config section: its name, whether it holds the default or a value set
 * here, a way back to the default, the control that edits it, and what it does.
 *
 * The state is spelled out in a chip rather than shown by color.
 *
 * @param props - The setting's name, state and control
 * @returns The rendered row
 *
 * @public
 */
export default function SettingField({
  label,
  hint = undefined,
  custom,
  onReset = undefined,
  disabled = false,
  children,
}: SettingFieldProps): JSX.Element {
  const {t} = useTranslation();
  const labelId: string = useId();
  const hintId: string = useId();

  return (
    <Box>
      <Stack direction="row" spacing={1} useFlexGap alignItems="center" sx={{flexWrap: 'wrap', mb: 1}}>
        <Typography id={labelId} variant="subtitle2">
          {label}
        </Typography>
        <Chip
          size="small"
          variant="outlined"
          color={custom ? 'primary' : 'default'}
          label={custom ? t('settings:field.custom', 'Custom') : t('settings:field.default', 'Default')}
        />
        {custom && onReset && (
          <Button
            variant="text"
            size="small"
            startIcon={<RotateCcw size={14} />}
            disabled={disabled}
            // The field the reset belongs to is read from the label.
            aria-describedby={labelId}
            onClick={onReset}
          >
            {t('settings:field.resetToDefault', 'Reset to default')}
          </Button>
        )}
      </Stack>
      {children(hint ? hintId : undefined)}
      {hint && (
        <Typography id={hintId} variant="body2" color="text.secondary" sx={{mt: 1}}>
          {hint}
        </Typography>
      )}
    </Box>
  );
}
