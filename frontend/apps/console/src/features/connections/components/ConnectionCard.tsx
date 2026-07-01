/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {Box, Card, CardActionArea, CardContent, Chip, Stack, Typography} from '@wso2/oxygen-ui';
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
    <CardContent sx={{flex: 1, display: 'flex', flexDirection: 'column', gap: 1.5, height: '100%'}}>
      <Stack direction="row" spacing={1.5} alignItems="flex-start" justifyContent="space-between">
        <Box
          sx={{
            width: 44,
            height: 44,
            borderRadius: 2,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            bgcolor: 'action.hover',
            flexShrink: 0,
          }}
        >
          {card.logo}
        </Box>
        {card.comingSoon && <Chip size="small" label={t('card.comingSoon')} />}
      </Stack>

      <Box sx={{minWidth: 0}}>
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
            {isConfigured ? t('card.configured') : t('card.notConfigured')}
          </Typography>
        </Stack>
      </Box>

      <Typography variant="body2" color="text.secondary">
        {t(card.descriptionKey)}
      </Typography>

      <Stack direction="row" spacing={0.75} flexWrap="wrap" useFlexGap sx={{mt: 'auto'}}>
        {card.categories.map((category) => (
          <Typography key={category} variant="caption" color="text.disabled" sx={{fontWeight: 500, letterSpacing: 0.2}}>
            {`#${t(`categories.${category}`).toLocaleLowerCase()}`}
          </Typography>
        ))}
      </Stack>
    </CardContent>
  );

  return (
    <Card
      variant="outlined"
      sx={{height: '100%', display: 'flex', flexDirection: 'column'}}
      data-testid={`connection-card-${card.id}`}
    >
      {card.comingSoon ? (
        <Box sx={{flex: 1, display: 'flex', flexDirection: 'column', opacity: 0.6}}>{body}</Box>
      ) : (
        <CardActionArea
          onClick={() => onAction(card)}
          data-testid={`connection-card-action-${card.id}`}
          sx={{flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'stretch'}}
        >
          {body}
        </CardActionArea>
      )}
    </Card>
  );
}
