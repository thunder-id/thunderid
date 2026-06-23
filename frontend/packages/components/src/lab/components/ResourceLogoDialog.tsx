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

import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  TextField,
  Box,
  Typography,
  Stack,
  IconButton,
  FormHelperText,
  Divider,
} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import EmojiPicker from './EmojiPicker/EmojiPicker';
import isValidLogoUri from '../utils/isValidLogoUri';

const EMOJI_SCHEME = 'emoji:';

function isUrlValue(value: string): boolean {
  return !value.startsWith(EMOJI_SCHEME) && isValidLogoUri(value);
}

/**
 * Props for the {@link ResourceLogoDialog} component.
 *
 * @public
 */
export interface ResourceLogoDialogProps {
  /** Whether the dialog is open. */
  open: boolean;

  /** Callback to close the dialog without selecting. */
  onClose: () => void;

  /**
   * The currently committed value — an `emoji:`-prefixed string or an image URL.
   * Used to pre-populate the dialog when it opens.
   */
  value?: string;

  /**
   * Fired when the user confirms their selection.
   *
   * @param value - `emoji:<char>` or a raw image URL.
   */
  onSelect: (value: string) => void;
}

/**
 * A dialog that lets the user choose a resource logo — either by picking an
 * emoji from the {@link EmojiPicker} grid or by entering a custom image URL.
 *
 * @public
 */
export default function ResourceLogoDialog({
  open,
  onClose,
  value = '',
  onSelect,
}: ResourceLogoDialogProps): JSX.Element {
  const {t} = useTranslation('elements');
  const [pendingEmoji, setPendingEmoji] = useState<string>(() => {
    if (!open || isUrlValue(value)) return '';
    return value.startsWith(EMOJI_SCHEME) ? value.slice(EMOJI_SCHEME.length) : value;
  });
  const [pendingUrl, setPendingUrl] = useState<string>(() => (open && isUrlValue(value) ? value : ''));
  const [prevOpen, setPrevOpen] = useState<boolean>(open);
  const [prevValue, setPrevValue] = useState<string>(value);

  if (prevOpen !== open || (open && prevValue !== value)) {
    setPrevOpen(open);
    setPrevValue(value);
    if (open) {
      if (isUrlValue(value)) {
        setPendingUrl(value);
        setPendingEmoji('');
      } else {
        const raw: string = value.startsWith(EMOJI_SCHEME) ? value.slice(EMOJI_SCHEME.length) : value;
        setPendingEmoji(raw);
        setPendingUrl('');
      }
    }
  }

  const handleEmojiChange = useCallback((char: string): void => {
    setPendingEmoji(char);
    setPendingUrl('');
  }, []);

  const handleUrlChange = useCallback((url: string): void => {
    setPendingUrl(url);
    if (url) setPendingEmoji('');
  }, []);

  const isUrlValid: boolean = isValidLogoUri(pendingUrl);
  const urlHasError: boolean = Boolean(pendingUrl) && !isUrlValid;

  const handleSelect = useCallback((): void => {
    if (pendingUrl) {
      if (!isValidLogoUri(pendingUrl)) return;
      onSelect(pendingUrl);
    } else if (pendingEmoji) {
      onSelect(EMOJI_SCHEME + pendingEmoji);
    }
    onClose();
  }, [pendingUrl, pendingEmoji, onSelect, onClose]);

  const canSelect = Boolean((pendingUrl && isUrlValid) || pendingEmoji);

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        <Stack direction="row" alignItems="center" justifyContent="space-between">
          <Typography variant="h5">{t('resource_logo_dialog.title', 'Choose a Logo')}</Typography>
          <IconButton aria-label={t('resource_logo_dialog.actions.close', 'Close')} onClick={onClose} size="small">
            <X size={20} />
          </IconButton>
        </Stack>
      </DialogTitle>

      <DialogContent dividers sx={{p: 0}}>
        <Stack>
          {/* Emoji picker panel */}
          <EmojiPicker value={pendingEmoji} onChange={handleEmojiChange} />

          <Stack spacing={2} sx={{px: 2, pb: 2}}>
            <Divider>{t('resource_logo_dialog.divider.or', 'Or')}</Divider>

            {/* Custom URL */}
            <Box>
              <Typography variant="subtitle2" gutterBottom>
                {t('resource_logo_dialog.url_section.label', 'Use a custom image URL')}
              </Typography>
              <TextField
                fullWidth
                size="small"
                error={urlHasError}
                placeholder={t('resource_logo_dialog.url_section.placeholder', 'https://example.com/logo.png')}
                value={pendingUrl}
                onChange={(e) => handleUrlChange(e.target.value)}
              />
              <FormHelperText error={urlHasError}>
                {urlHasError
                  ? t(
                      'resource_logo_dialog.url_section.error_text',
                      'Enter a valid image URL (e.g. https://example.com/logo.png)',
                    )
                  : t('resource_logo_dialog.url_section.helper_text', 'Enter a direct URL to a custom logo image')}
              </FormHelperText>
            </Box>
          </Stack>
        </Stack>
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose} variant="outlined">
          {t('resource_logo_dialog.actions.cancel', 'Cancel')}
        </Button>
        <Button onClick={handleSelect} variant="contained" disabled={!canSelect}>
          {t('resource_logo_dialog.actions.select', 'Select')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
