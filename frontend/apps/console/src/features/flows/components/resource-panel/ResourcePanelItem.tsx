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

import {Box, Card, CardContent, IconButton, Stack, Typography} from '@wso2/oxygen-ui';
import {PlusIcon} from '@wso2/oxygen-ui-icons-react';
import React, {type HTMLAttributes, type ReactElement} from 'react';
import type {Resource} from '../../models/resources';
import ResourceDisplayImage from '../ResourceDisplayImage';

/**
 * Props interface of {@link ResourcePanelItem}
 */
export interface ResourcePanelItemProps extends Omit<HTMLAttributes<HTMLDivElement>, 'resource'> {
  /**
   * The resource item.
   */
  resource: Resource;
  /**
   * The type of the resource item.
   */
  type?: 'draggable' | 'static';
  /**
   * Callback to be triggered when a resource add button is clicked.
   * @param resource - Added resource.
   */
  onAdd?: (resource: Resource) => void;
  /**
   * Flag to disable the panel item.
   */
  disabled?: boolean;
}

/**
 * Resource panel item component.
 *
 * @param props - Props injected to the component.
 * @returns The ResourcePanelItem component.
 */
function ResourcePanelItem({
  children,
  resource,
  type = 'static',
  onAdd = undefined,
  disabled = false,
}: ResourcePanelItemProps): ReactElement | React.ReactNode {
  return (
    children ?? (
      <Card
        elevation={0}
        sx={{
          transition: 'background-color 0.2s ease-in-out',
          cursor: type === 'draggable' ? 'grab' : 'default',
          '&:hover': {
            backgroundColor: 'action.hover',
          },
          '&:active': {
            cursor: type === 'draggable' ? 'grabbing' : 'default',
          },
        }}
      >
        <CardContent
          sx={{
            p: 1.5,
            '&:last-child': {
              pb: 1.5,
            },
          }}
        >
          <Box display="flex" justifyContent="space-between" alignItems="center" gap={1}>
            <Stack direction="row" spacing={1.5} alignItems="center" flex={1}>
              <ResourceDisplayImage
                image={resource?.display?.image}
                label={resource?.display?.label}
                preserveColor={resource?.display?.preserveImageColor}
              />
              <Stack direction="column" spacing={0.25} flex={1}>
                <Typography variant="body2" fontWeight={500} color="text.primary">
                  {resource?.display?.label}
                </Typography>
                {resource?.display?.description && (
                  <Typography variant="caption" color="text.secondary" sx={{lineHeight: 1.3}}>
                    {resource?.display?.description}
                  </Typography>
                )}
              </Stack>
            </Stack>
            {onAdd && (
              <IconButton
                onClick={() => onAdd(resource)}
                disabled={disabled}
                size="small"
                sx={{
                  height: 28,
                  width: 28,
                  border: '1px solid',
                  borderColor: 'divider',
                  borderRadius: 1,
                  backgroundColor: 'action.selected',
                  '&:hover': {
                    backgroundColor: 'primary.main',
                    borderColor: 'primary.main',
                    color: 'primary.contrastText',
                  },
                  '&.Mui-disabled': {
                    backgroundColor: 'action.disabledBackground',
                    borderColor: 'divider',
                  },
                }}
              >
                <PlusIcon size={14} />
              </IconButton>
            )}
          </Box>
        </CardContent>
      </Card>
    )
  );
}

export default ResourcePanelItem;
