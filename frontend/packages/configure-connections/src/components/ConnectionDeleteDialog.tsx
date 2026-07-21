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

import {
  Alert,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  List,
  ListItem,
  ListItemText,
  Typography,
} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetConnectionUsages from '../api/useGetConnectionUsages';
import type {ConnectionType} from '../models/connection';

const MAX_VISIBLE_USAGES = 5;

interface ConnectionDeleteDialogProps {
  open: boolean;
  connectionType: ConnectionType;
  connectionId: string;
  connectionName: string;
  isPending: boolean;
  onConfirm: () => void;
  onClose: () => void;
}

export default function ConnectionDeleteDialog({
  open,
  connectionType,
  connectionId,
  connectionName,
  isPending,
  onConfirm,
  onClose,
}: ConnectionDeleteDialogProps): JSX.Element {
  const {t} = useTranslation('connections');

  const {data: usagesData, isLoading: isLoadingUsages} = useGetConnectionUsages(
    connectionType,
    connectionId || undefined,
    open,
  );

  const usagesKnown = usagesData !== undefined && usagesData.totalResults !== null;
  const blockingUsages = usagesData?.usages.filter((usage) => usage.behaviorOnDelete === 'restrict') ?? [];
  const hasBlockingUsages = usagesKnown && blockingUsages.length > 0;
  const visibleBlocking = blockingUsages.slice(0, MAX_VISIBLE_USAGES);
  const hiddenBlockingCount = blockingUsages.length - visibleBlocking.length;

  return (
    <Dialog open={open} onClose={isPending ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t('delete.title')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>{t('delete.message', {name: connectionName})}</DialogContentText>

        {isLoadingUsages ? (
          <Alert severity="info" icon={<CircularProgress size={16} />}>
            {t('delete.usages.loading')}
          </Alert>
        ) : hasBlockingUsages ? (
          <Alert severity="error">
            <Typography variant="body2" sx={{mb: 1}}>
              {t('delete.blocking.title')}
            </Typography>
            <List dense disablePadding>
              {visibleBlocking.map((usage) => (
                <ListItem key={usage.id} disableGutters sx={{py: 0}}>
                  <ListItemText primary={<Typography variant="body2">{usage.displayName}</Typography>} />
                </ListItem>
              ))}
              {hiddenBlockingCount > 0 && (
                <ListItem disableGutters sx={{py: 0}}>
                  <ListItemText
                    primary={
                      <Typography variant="body2" color="text.secondary">
                        {t('delete.usages.more', {count: hiddenBlockingCount})}
                      </Typography>
                    }
                  />
                </ListItem>
              )}
            </List>
          </Alert>
        ) : null}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button
          onClick={onConfirm}
          color="error"
          variant="contained"
          disabled={isPending || isLoadingUsages || hasBlockingUsages}
          data-testid="connection-delete-confirm"
        >
          {t('common:actions.delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
