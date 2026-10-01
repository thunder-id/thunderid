// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Card, CardContent, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import {CircleCheck} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface SelectableConnectionCardProps {
  description: string;
  disabled?: boolean;
  icon: JSX.Element;
  label: string;
  selected: boolean;
  tags?: string[];
  testId: string;
  onSelect: () => void;
}

export default function SelectableConnectionCard({
  description,
  disabled = false,
  icon,
  label,
  selected,
  tags = [],
  testId,
  onSelect,
}: SelectableConnectionCardProps): JSX.Element {
  const {t} = useTranslation('connections');

  return (
    <Card
      variant="outlined"
      role="button"
      tabIndex={disabled ? -1 : 0}
      aria-pressed={selected}
      aria-disabled={disabled}
      data-testid={testId}
      onClick={disabled ? undefined : onSelect}
      onKeyDown={(event) => {
        if (!disabled && (event.key === 'Enter' || event.key === ' ')) {
          event.preventDefault();
          onSelect();
        }
      }}
      sx={{
        cursor: disabled ? 'not-allowed' : 'pointer',
        opacity: disabled ? 0.6 : 1,
        borderColor: selected ? 'primary.main' : 'divider',
        transition: 'border-color 0.15s',
        '&:hover': disabled ? {} : {borderColor: 'primary.main'},
        '&:focus-visible': disabled ? {} : {outline: 'none', borderColor: 'primary.main'},
      }}
    >
      <CardContent sx={{p: 2.5, '&:last-child': {pb: 2.5}}}>
        <Stack direction="column" spacing={2}>
          <Stack direction="row" justifyContent="space-between" alignItems="flex-start">
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                width: 48,
                height: 48,
                color: selected ? 'primary.main' : 'text.secondary',
              }}
            >
              {icon}
            </Box>
            {disabled ? (
              <Chip size="small" label={t('card.comingSoon', 'Coming soon')} />
            ) : (
              selected && <CircleCheck size={20} color="var(--mui-palette-primary-main)" />
            )}
          </Stack>
          <Stack direction="column" spacing={0.75}>
            <Typography variant="subtitle1" sx={{fontWeight: 600, lineHeight: 1.3}}>
              {label}
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{lineHeight: 1.5}}>
              {description}
            </Typography>
          </Stack>
          {tags.length > 0 && (
            <Stack direction="row" spacing={0.75} flexWrap="wrap">
              {tags.map((tag) => (
                <Typography
                  key={tag}
                  variant="caption"
                  color="text.disabled"
                  sx={{fontWeight: 500, letterSpacing: 0.2}}
                >
                  #{tag.toLocaleLowerCase()}
                </Typography>
              ))}
            </Stack>
          )}
        </Stack>
      </CardContent>
    </Card>
  );
}
