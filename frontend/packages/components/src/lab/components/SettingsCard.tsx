// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
                slotProps={{input: {role: 'switch', 'aria-label': `Toggle ${title}`}}}
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
      {(!hasToggle || enabled) && children && (
        <>
          <Divider />
          <Paper {...contentProps} sx={{p: 3, ...(contentSx as object)}}>
            {children}
          </Paper>
        </>
      )}
    </Paper>
  );
}
