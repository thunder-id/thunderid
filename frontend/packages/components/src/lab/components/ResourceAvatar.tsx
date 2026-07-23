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

import type {AvatarShape} from '@thunderid/react';
import {
  Avatar,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Stack,
  Typography,
  useTheme,
} from '@wso2/oxygen-ui';
import type {AvatarProps} from '@wso2/oxygen-ui';
import {Edit, X} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useRef} from 'react';
import type {ReactNode, JSX, KeyboardEvent} from 'react';
import {useTranslation} from 'react-i18next';
import LogoPicker from './LogoPicker/LogoPicker';
import {resolveResourceIcon} from './resourceIconSchemes';

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
   * shown and clicking either the avatar or the button opens a dialog with
   * {@link LogoPicker}. The callback fires with each picked value
   * (`emoji:<char>`, `avatar:...`, or a raw URL) as the user browses the
   * picker, so the avatar updates live behind the dialog. Clicking "Cancel"
   * reverts to the value the dialog opened with; clicking "Save" keeps the
   * latest pick and, if {@link onSave} is provided, persists it.
   */
  onSelect?: (value: string) => void;

  /**
   * Called when the user clicks "Save" in the logo picker dialog, to persist
   * the value already reported via {@link onSelect}. When omitted, "Save"
   * simply closes the dialog and keeps the locally-applied value.
   */
  onSave?: () => void | Promise<void>;

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

  /**
   * Seed text used to default the picker dialog's avatar tab (e.g. the resource's name).
   */
  seedText?: string;

  /**
   * Avatar shapes offered in the edit dialog's {@link LogoPicker}. Forwarded as-is.
   *
   * @defaultValue ['rounded']
   */
  supportedShapes?: AvatarShape[];
}

/**
 * A smart avatar that renders a resource icon from an emoji or image URL.
 *
 * **Read-only mode** (no `onSelect`): renders just the Avatar.
 *
 * **Edit mode** (`onSelect` provided): wraps the Avatar in a relative container,
 * shows an overlaid pencil button, and manages a dialog wrapping {@link LogoPicker}
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
  onSave = undefined,
  editAriaLabel = 'Change logo',
  onClick = undefined,
  seedText = '',
  supportedShapes = undefined,
  variant = 'circular',
  ...rest
}: ResourceAvatarProps): JSX.Element {
  const theme = useTheme();
  const {t} = useTranslation('elements');

  const [isDialogOpen, setIsDialogOpen] = useState<boolean>(false);
  const [isSaving, setIsSaving] = useState<boolean>(false);
  const [imgErrorUrl, setImgErrorUrl] = useState<string | null>(null);
  const originalValueRef = useRef<string | undefined>(value);

  const hasValue = Boolean(value);
  const resolvedIcon = hasValue ? resolveResourceIcon(value!) : null;
  const imgSrc: string | undefined = resolvedIcon?.type === 'image' ? resolvedIcon.src : undefined;
  const imgError: boolean = Boolean(imgSrc) && imgErrorUrl === imgSrc;

  const resolvedFallback = typeof fallback === 'string' ? resolveResourceIcon(fallback, seedText) : null;

  // When no value is set, the avatar renders `fallback` (if it's a resource-icon spec).
  // Pre-select that same spec in the picker dialog instead of opening on nothing.
  const pickerValue: string = hasValue ? value! : typeof fallback === 'string' ? fallback : '';
  const resolvedFallbackIcon: ReactNode =
    resolvedFallback?.type === 'emoji' ? (
      resolvedFallback.char
    ) : resolvedFallback?.type === 'image' ? (
      <img
        src={resolvedFallback.src}
        alt="logo"
        style={{width: '100%', height: '100%', objectFit: 'cover', textAlign: 'center'}}
      />
    ) : (
      fallback
    );

  const handleOpenDialog = useCallback((): void => {
    originalValueRef.current = value;
    setIsDialogOpen(true);
  }, [value]);

  const handleCancel = useCallback((): void => {
    if (isSaving) return;
    if (originalValueRef.current !== value) {
      onSelect?.(originalValueRef.current ?? '');
    }
    setIsDialogOpen(false);
  }, [isSaving, onSelect, value]);

  const handleImgError = useCallback((): void => {
    setImgErrorUrl(imgSrc ?? null);
  }, [imgSrc]);

  const handleLogoChange = useCallback(
    (val: string): void => {
      onSelect?.(val);
    },
    [onSelect],
  );

  const handleSaveClick = useCallback(async (): Promise<void> => {
    if (onSave) {
      setIsSaving(true);
      try {
        await onSave();
      } catch {
        setIsSaving(false);
        return;
      }
      setIsSaving(false);
    }
    setIsDialogOpen(false);
  }, [onSave]);

  let avatarContent: ReactNode;
  if (imgSrc) {
    avatarContent = (
      <>
        <img
          src={imgSrc}
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
    avatarContent = (resolvedIcon?.type === 'emoji' && resolvedIcon.char) || resolvedFallbackIcon;
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
      variant={variant}
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
            borderRadius: variant === 'circular' ? '50%' : 1,
            '&:hover': {bgcolor: 'action.hover'},
          }}
        >
          <Edit size={14} />
        </IconButton>
      )}
      <Dialog open={isDialogOpen} onClose={handleCancel} maxWidth="sm" fullWidth>
        <DialogTitle>
          <Stack direction="row" alignItems="center" justifyContent="space-between">
            <Typography variant="h5">{t('resource_logo_dialog.title', 'Choose a Logo')}</Typography>
            <IconButton
              aria-label={t('resource_logo_dialog.actions.close', 'Close')}
              onClick={handleCancel}
              size="small"
              disabled={isSaving}
            >
              <X size={20} />
            </IconButton>
          </Stack>
        </DialogTitle>
        <DialogContent dividers>
          <LogoPicker
            value={pickerValue}
            onChange={handleLogoChange}
            seedText={seedText}
            supportedShapes={supportedShapes}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={handleCancel} variant="outlined" disabled={isSaving}>
            {t('resource_logo_dialog.actions.cancel', 'Cancel')}
          </Button>
          <Button onClick={() => void handleSaveClick()} variant="contained" disabled={isSaving}>
            {t('resource_logo_dialog.actions.save', 'Save')}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
