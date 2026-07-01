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

import {Box, Card, CardActionArea, Stack, Typography} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface AddCustomConnectionCardProps {
  onClick: () => void;
}

/**
 * Dashed "ghost" card appended to the end of the connections grid — starts the add-custom
 * connection wizard for vendors not in the catalog.
 */
export default function AddCustomConnectionCard({onClick}: AddCustomConnectionCardProps): JSX.Element {
  const {t} = useTranslation('connections');

  return (
    <Card
      variant="outlined"
      sx={{height: '100%', borderStyle: 'dashed', bgcolor: 'transparent'}}
      data-testid="connection-add-custom-card"
    >
      <CardActionArea
        onClick={onClick}
        sx={{height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center', p: 3}}
      >
        <Stack direction="column" spacing={1.5} alignItems="center" textAlign="center">
          <Box
            sx={{
              width: 44,
              height: 44,
              borderRadius: 2,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              bgcolor: 'action.hover',
            }}
          >
            <Plus size={20} />
          </Box>
          <Typography variant="subtitle1" fontWeight={600}>
            {t('card.addCustom.title')}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{maxWidth: 260}}>
            {t('card.addCustom.description')}
          </Typography>
        </Stack>
      </CardActionArea>
    </Card>
  );
}
