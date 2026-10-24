// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, FormControlLabel, Radio, RadioGroup, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';

export interface AccessChoice<T extends string> {
  value: T;
  title: string;
  description: string;
  /**
   * Shows the choice but keeps it from being picked, for choices the console can keep but not create.
   */
  disabled?: boolean;
}

export interface AccessChoiceCardsProps<T extends string> {
  /**
   * Accessible name of the radio group.
   */
  label: string;
  value: T;
  choices: AccessChoice<T>[];
  /**
   * Called with the choice the admin picked. The value only changes when the parent applies it,
   * which lets the parent confirm a switch first.
   */
  onChange: (value: T) => void;
  disabled?: boolean;
}

/**
 * A group of mutually exclusive choices drawn as selectable cards, each with a title and a one-line
 * description.
 */
export default function AccessChoiceCards<T extends string>({
  label,
  value,
  choices,
  onChange,
  disabled = false,
}: AccessChoiceCardsProps<T>): JSX.Element {
  return (
    <RadioGroup
      aria-label={label}
      value={value}
      onChange={(_event, next) => onChange(next as T)}
      sx={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 1.5}}
    >
      {choices.map((choice) => {
        const selected = choice.value === value;
        const choiceDisabled = disabled || choice.disabled === true;

        return (
          <FormControlLabel
            key={choice.value}
            value={choice.value}
            disabled={choiceDisabled}
            control={
              <Radio
                size="small"
                // Read-only keeps the chosen option marked, so the saved choice still reads at a glance.
                sx={{mt: -0.5, '&.Mui-disabled.Mui-checked': {color: 'primary.main'}}}
              />
            }
            label={
              <Box>
                <Typography variant="body1" fontWeight={600}>
                  {choice.title}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {choice.description}
                </Typography>
              </Box>
            }
            sx={{
              m: 0,
              p: 2,
              alignItems: 'flex-start',
              gap: 0.5,
              border: '1px solid',
              borderColor: selected ? 'primary.main' : 'divider',
              borderRadius: '10px',
              bgcolor: selected ? 'action.selected' : 'transparent',
              '& .MuiFormControlLabel-label.Mui-disabled': {color: 'text.primary'},
              // An option that can't be picked is dimmed, unless it is the chosen one, so a read-only
              // group no longer looks pickable and still shows what was chosen.
              ...(choiceDisabled && !selected ? {opacity: 0.5} : {}),
              cursor: choiceDisabled ? 'default' : 'pointer',
            }}
          />
        );
      })}
    </RadioGroup>
  );
}
