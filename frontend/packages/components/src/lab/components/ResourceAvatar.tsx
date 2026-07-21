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

import {Avatar, Box, IconButton, useTheme} from '@wso2/oxygen-ui';
import type {AvatarProps} from '@wso2/oxygen-ui';
import {Edit} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback} from 'react';
import type {ReactNode, JSX, KeyboardEvent} from 'react';
import ResourceLogoDialog from './ResourceLogoDialog';

const EMOJI_SCHEME = 'emoji:';

function isUrl(value: string): boolean {
  return value.startsWith('http://') || value.startsWith('https://');
}

function resolveDisplayValue(value: string): string {
  return value.startsWith(EMOJI_SCHEME) ? value.slice(EMOJI_SCHEME.length) : value;
}

/**
 * Props for the {@link ResourceAvatar} component.
 *
 * @public
 */
export interface ResourceAvatarProps extends Omit<AvatarProps, 'onSelect'> {
  /**
   * Whether the avatar is editable. When `true`, the edit button is shown and the dialog can be opened.
   */
  editable?: boolean;

  /**
   * The icon value — an `emoji:`-prefixed string, a raw emoji character, or an image URL.
   * When empty or undefined, renders the fallback.
   */
  value?: string;

  /**
   * Size in pixels for both width and height. Defaults to 40.
   */
  size?: number;

  /**
   * Fallback icon rendered when value is empty or when a URL fails to load.
   */
  fallback?: ReactNode;

  /**
   * When provided, the avatar becomes editable: an overlay pencil button is
   * shown and clicking either the avatar or the button opens
   * {@link ResourceLogoDialog}. The callback receives the confirmed value
   * (`emoji:<char>` or a raw URL).
   */
  onSelect?: (value: string) => void;

  /**
   * Accessible label for the edit button (only relevant when `onSelect` is set).
   * Defaults to `"Change logo"`.
   */
  editAriaLabel?: string;

  /**
   * Optional click handler (used in read-only contexts, e.g. selecting from a
   * suggestion list). If `onSelect` is also provided, `onSelect` takes
   * precedence for opening the dialog.
   */
  onClick?: () => void;
}

/**
 * A smart avatar that renders a resource icon from an emoji or image URL.
 *
 * **Read-only mode** (no `onSelect`): renders just the Avatar.
 *
 * **Edit mode** (`onSelect` provided): wraps the Avatar in a relative container,
 * shows an overlaid pencil button, and manages a {@link ResourceLogoDialog}
 * internally. No external state or dialog wiring needed by the caller.
 *
 * @example
 * ```tsx
 * // Read-only
 * <ResourceAvatar value="emoji:🐼" size={40} fallback={<AppWindow />} />
 *
 * // Editable
 * <ResourceAvatar
 *   editable
 *   value={app.logoUrl}
 *   size={40}
 *   fallback="emoji:🖥️"
 *   onSelect={(val) => setApp({...app, logoUrl: val})}
 * />
 * ```
 *
 * @public
 */
export default function ResourceAvatar({
  editable = false,
  value = undefined,
  size = 40,
  fallback = null,
  sx,
  onSelect = undefined,
  editAriaLabel = 'Change logo',
  onClick = undefined,
  ...rest
}: ResourceAvatarProps): JSX.Element {
  const theme = useTheme();

  const [isDialogOpen, setIsDialogOpen] = useState<boolean>(false);
  const [imgErrorUrl, setImgErrorUrl] = useState<string | null>(null);

  const hasValue = Boolean(value);
  const displayValue: string = hasValue ? resolveDisplayValue(value!) : '';
  const isUrlValue: boolean = Boolean(displayValue) && isUrl(displayValue);
  const imgError: boolean = imgErrorUrl === displayValue && Boolean(displayValue);

  const resolvedFallbackIcon: ReactNode =
    typeof fallback === 'string' && fallback.startsWith(EMOJI_SCHEME) ? fallback.slice(EMOJI_SCHEME.length) : fallback;

  const handleOpenDialog = useCallback((): void => {
    setIsDialogOpen(true);
  }, []);

  const handleCloseDialog = useCallback((): void => {
    setIsDialogOpen(false);
  }, []);

  const handleImgError = useCallback((): void => {
    setImgErrorUrl(displayValue);
  }, [displayValue]);

  const handleSelect = useCallback(
    (val: string): void => {
      onSelect?.(val);
      setIsDialogOpen(false);
    },
    [onSelect],
  );

  let avatarContent: ReactNode;
  if (isUrlValue) {
    avatarContent = (
      <>
        <img
          src={displayValue}
          alt="logo"
          onError={handleImgError}
          style={
            imgError ? {display: 'none'} : {width: '100%', height: '100%', objectFit: 'cover', textAlign: 'center'}
          }
        />
        {imgError && resolvedFallbackIcon}
      </>
    );
  } else {
    avatarContent = displayValue || resolvedFallbackIcon;
  }

  const isInteractive = Boolean(onSelect ?? onClick);
  const handleKeyDown = isInteractive
    ? (e: KeyboardEvent): void => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          if (onSelect) {
            handleOpenDialog();
          } else {
            onClick?.();
          }
        }
      }
    : undefined;

  const avatar = (
    <Avatar
      src={undefined}
      role={isInteractive ? 'button' : undefined}
      tabIndex={isInteractive ? 0 : undefined}
      onClick={onSelect ? handleOpenDialog : onClick}
      onKeyDown={handleKeyDown}
      sx={{
        width: size,
        height: size,
        color: 'text.primary',
        backgroundColor: theme.vars?.palette.grey[800],
        fontSize: `${Math.round(size * 0.4)}px`,
        cursor: isInteractive ? 'pointer' : undefined,
        ...(onSelect ? {'&:hover': {opacity: 0.8}} : {}),
        '&:focus-visible': isInteractive ? {outline: '2px solid', outlineOffset: '2px'} : undefined,
        ...theme.applyStyles('light', {
          backgroundColor: theme.palette.grey[700],
          color: theme.palette.primary.contrastText,
        }),
        ...sx,
      }}
      {...rest}
    >
      {avatarContent}
    </Avatar>
  );

  if (!onSelect) return avatar;

  return (
    <Box sx={{position: 'relative', display: 'inline-flex'}}>
      {avatar}
      {editable && (
        <IconButton
          size="small"
          aria-label={editAriaLabel}
          onClick={handleOpenDialog}
          sx={{
            position: 'absolute',
            bottom: -4,
            right: -4,
            bgcolor: 'background.paper',
            boxShadow: 1,
            '&:hover': {bgcolor: 'action.hover'},
          }}
        >
          <Edit size={14} />
        </IconButton>
      )}
      <ResourceLogoDialog open={isDialogOpen} onClose={handleCloseDialog} value={value} onSelect={handleSelect} />
    </Box>
  );
}
