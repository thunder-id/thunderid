// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Card, CardContent, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {ConnectionCardModel} from '../models/connection';

interface ConnectionCardProps {
  card: ConnectionCardModel;
  onAction: (card: ConnectionCardModel) => void;
}

export default function ConnectionCard({card, onAction}: ConnectionCardProps): JSX.Element {
  const {t} = useTranslation('connections');
  const isConfigured: boolean = card.status === 'configured';

  const body: JSX.Element = (
    <CardContent sx={{flex: 1, p: 2.5, '&:last-child': {pb: 2.5}}}>
      <Stack direction="column" spacing={2}>
        <Stack direction="row" alignItems="flex-start" justifyContent="space-between">
          {card.logo}
          {card.comingSoon && <Chip size="small" label={t('card.comingSoon', 'Coming soon')} />}
          {card.isDefault && (
            <Chip
              size="small"
              color="primary"
              variant="outlined"
              label={t('card.default', 'Default')}
              data-testid="connection-card-default-chip"
            />
          )}
        </Stack>

        <Stack direction="column" spacing={0.75}>
          <Typography variant="subtitle1" fontWeight={600} noWrap>
            {card.displayName}
          </Typography>
          <Stack direction="row" spacing={0.75} alignItems="center">
            <Box
              sx={{
                width: 8,
                height: 8,
                borderRadius: '50%',
                bgcolor: isConfigured ? 'success.main' : 'text.disabled',
              }}
            />
            <Typography variant="body2" color="text.secondary">
              {isConfigured ? t('card.configured', 'Configured') : t('card.notConfigured', 'Not configured')}
            </Typography>
          </Stack>
          <Typography variant="body2" color="text.secondary" sx={{lineHeight: 1.5}}>
            {t(card.descriptionKey, 'Connection description')}
          </Typography>
        </Stack>

        <Stack direction="row" spacing={0.75} flexWrap="wrap">
          {card.categories.map((category) => (
            <Typography
              key={category}
              variant="caption"
              color="text.disabled"
              sx={{fontWeight: 500, letterSpacing: 0.2}}
            >
              {`#${t(`categories.${category}`, 'Other').toLocaleLowerCase()}`}
            </Typography>
          ))}
        </Stack>
      </Stack>
    </CardContent>
  );

  return (
    <Card
      variant="outlined"
      sx={{
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        transition: 'border-color 0.15s',
        ...(card.comingSoon ? {} : {'&:hover': {borderColor: 'primary.main'}}),
      }}
      data-testid={`connection-card-${card.id}`}
    >
      {card.comingSoon ? (
        <Box sx={{flex: 1, display: 'flex', flexDirection: 'column', opacity: 0.6}}>{body}</Box>
      ) : (
        <Box
          role="button"
          tabIndex={0}
          onClick={() => onAction(card)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              onAction(card);
            }
          }}
          data-testid={`connection-card-action-${card.id}`}
          sx={{flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'stretch', cursor: 'pointer'}}
        >
          {body}
        </Box>
      )}
    </Card>
  );
}
