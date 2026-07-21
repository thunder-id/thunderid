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

import {Box, Divider, Paper, Stack, Switch, Typography} from '@wso2/oxygen-ui';
import type {BoxProps, PaperProps, TypographyProps} from '@wso2/oxygen-ui';
import type {ReactNode} from 'react';

interface SettingsCardSlotProps {
  /**
   * Props for the root Paper element
   */
  root?: PaperProps;
  /**
   * Props for the header Box element
   */
  header?: BoxProps;
  /**
   * Props for the title Typography element
   */
  title?: TypographyProps;
  /**
   * Props for the description Typography element
   */
  description?: TypographyProps;
  /**
   * Props for the content Paper element
   */
  content?: PaperProps;
}

interface SettingsCardProps {
  /**
   * Card title
   */
  title: string;
  /**
   * Optional description shown below the title. Accepts a plain string or, for descriptions
   * that need an inline link, a ReactNode.
   */
  description?: ReactNode;
  /**
   * Content of the card
   */
  children: ReactNode;
  /**
   * Optional toggle switch state
   */
  enabled?: boolean;
  /**
   * Optional toggle change handler
   */
  onToggle?: (enabled: boolean) => void;
  /**
   * Optional icon element to render to the left of the title
   */
  titleIcon?: ReactNode;
  /**
   * Optional custom action element to render in the header
   */
  headerAction?: ReactNode;
  /**
   * Optional props to pass to child elements
   */
  slotProps?: SettingsCardSlotProps;
}

/**
 * Reusable settings card component for application edit pages.
 * Provides consistent styling with optional enable/disable toggle.
 *
 * @example
 * ```tsx
 * <SettingsCard
 *   title="Quick Copy"
 *   description="Copy application credentials"
 * >
 *   <TextField label="Application ID" />
 * </SettingsCard>
 * ```
 *
 * @example With toggle
 * ```tsx
 * <SettingsCard
 *   title="Registration Flow"
 *   description="Allow users to register"
 *   enabled={isEnabled}
 *   onToggle={(enabled) => handleToggle(enabled)}
 * >
 *   <TextField label="Flow ID" />
 * </SettingsCard>
 * ```
 */
export default function SettingsCard({
  title,
  description = undefined,
  children,
  enabled = undefined,
  onToggle = undefined,
  titleIcon = undefined,
  headerAction = undefined,
  slotProps = undefined,
}: SettingsCardProps) {
  const hasToggle = enabled !== undefined && onToggle !== undefined;

  const {sx: rootSx, ...rootProps} = slotProps?.root ?? {};
  const {sx: headerSx, ...headerProps} = slotProps?.header ?? {};
  const {sx: titleSx, ...titleProps} = slotProps?.title ?? {};
  const {sx: descriptionSx, ...descriptionProps} = slotProps?.description ?? {};
  const {sx: contentSx, ...contentProps} = slotProps?.content ?? {};

  return (
    <Paper {...rootProps} sx={rootSx}>
      <Box {...headerProps} sx={{p: 3, ...(headerSx as object)}}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" spacing={2}>
          <Stack direction="row" alignItems="center" spacing={1.5}>
            {titleIcon}
            <Typography variant="h5" {...titleProps} sx={titleSx}>
              {title}
            </Typography>
          </Stack>
          <Stack direction="row" alignItems="center" spacing={2}>
            {headerAction}
            {hasToggle && (
              <Switch
                checked={enabled}
                onChange={(e) => onToggle(e.target.checked)}
                inputProps={{'aria-label': `Toggle ${title}`}}
              />
            )}
          </Stack>
        </Stack>
        {description && (
          <Typography
            variant="body2"
            {...descriptionProps}
            sx={{mt: 0.5, color: 'text.secondary', ...(descriptionSx as object)}}
          >
            {description}
          </Typography>
        )}
      </Box>
      <Divider />
      {(!hasToggle || enabled) && children && (
        <Paper {...contentProps} sx={{p: 3, ...(contentSx as object)}}>
          {children}
        </Paper>
      )}
    </Paper>
  );
}
