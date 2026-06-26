/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {Box, Card, IconButton, Tooltip, Typography} from '@wso2/oxygen-ui';
import {ArrowUpRight, Eye, Trash2} from '@wso2/oxygen-ui-icons-react';
import {type JSX, type MouseEvent, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';

export interface ItemCardProps {
  thumbnail: ReactNode;
  name: string;
  onClick: () => void;
  isReadOnly?: boolean;
  onDelete?: () => void;
}

export default function ItemCard({
  thumbnail,
  name,
  onClick,
  isReadOnly = false,
  onDelete = undefined,
}: ItemCardProps): JSX.Element {
  const {t} = useTranslation('design');

  const handleDelete = (e: MouseEvent<HTMLButtonElement>): void => {
    e.stopPropagation();
    onDelete?.();
  };

  const showDelete = !isReadOnly && Boolean(onDelete);

  return (
    <Card
      onClick={isReadOnly ? undefined : onClick}
      sx={{
        cursor: isReadOnly ? 'default' : 'pointer',
        ...(!isReadOnly && {
          '&:hover': {
            borderColor: 'primary.main',
            boxShadow: '0 4px 20px rgba(0,0,0,0.1)',
            transform: 'translateY(-2px)',
            '& .card-overlay': {opacity: 1},
          },
        }),
      }}
    >
      <Box sx={{aspectRatio: '4/3', overflow: 'hidden', position: 'relative'}}>
        {thumbnail}

        {isReadOnly ? (
          <Tooltip title={t('common:status.readOnly', 'Read Only')}>
            <Box
              sx={{
                position: 'absolute',
                top: 8,
                right: 8,
                bgcolor: 'background.paper',
                borderRadius: '50%',
                width: 28,
                height: 28,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                boxShadow: 1,
              }}
            >
              <Eye size={14} />
            </Box>
          </Tooltip>
        ) : (
          <Box
            className="card-overlay"
            sx={{
              position: 'absolute',
              inset: 0,
              bgcolor: 'rgba(0,0,0,0.35)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              opacity: 0,
              transition: 'opacity 0.18s ease',
              backdropFilter: 'blur(2px)',
            }}
          >
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 0.5,
                bgcolor: 'common.background',
                borderRadius: 2,
                px: 1.5,
                py: 0.75,
              }}
            >
              <ArrowUpRight size={13} />
              <Typography variant="caption" sx={{fontWeight: 600, fontSize: '0.75rem'}}>
                {t('common.item_card.actions.open_in_builder.label', 'Open in builder')}
              </Typography>
            </Box>
          </Box>
        )}
      </Box>

      <Box
        sx={{
          px: 1.5,
          py: 1,
          borderTop: '1px solid',
          borderColor: 'divider',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 0.5,
        }}
      >
        <Typography
          variant="body2"
          sx={{
            fontWeight: 500,
            fontSize: '0.8125rem',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            flex: 1,
          }}
        >
          {name}
        </Typography>

        {showDelete && (
          <Tooltip title={t('common:actions.delete', 'Delete')}>
            <IconButton
              size="small"
              aria-label={t('common:actions.delete', 'Delete')}
              onClick={handleDelete}
              sx={{
                color: 'error.main',
                width: 24,
                height: 24,
                flexShrink: 0,
                '&:hover': {bgcolor: 'error.50'},
              }}
            >
              <Trash2 size={13} />
            </IconButton>
          </Tooltip>
        )}
      </Box>
    </Card>
  );
}
