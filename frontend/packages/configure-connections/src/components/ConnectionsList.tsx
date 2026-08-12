// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {Button, Grid, InputAdornment, Paper, Skeleton, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {Search, SearchX, X} from '@wso2/oxygen-ui-icons-react';
import {type JSX, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AddCustomConnectionCard from './AddCustomConnectionCard';
import ConnectionCard from './ConnectionCard';
import ConnectionCategoryFilters, {type CategoryFilterValue} from './ConnectionCategoryFilters';
import useConnections from '../api/useConnections';
import {CONNECTION_VENDOR_META, getAvailableConnectionCategories} from '../config/connectionVendorMeta';
import useConnectionRoutes from '../hooks/useConnectionRoutes';
import type {ConnectionCardModel, ConnectionCategory} from '../models/connection';
import buildConnectionCards from '../utils/buildConnectionCards';

const SKELETON_COUNT = 6;

export default function ConnectionsList(): JSX.Element {
  const {t} = useTranslation('connections');
  const navigate = useNavigate();
  const routes = useConnectionRoutes();

  const [search, setSearch] = useState('');
  const [category, setCategory] = useState<CategoryFilterValue>('all');

  const connectionsQuery = useConnections();

  const cards: ConnectionCardModel[] = useMemo(
    () => buildConnectionCards(connectionsQuery.data?.connections ?? [], CONNECTION_VENDOR_META, routes),
    [connectionsQuery.data?.connections, routes],
  );

  const availableCategories: ConnectionCategory[] = useMemo(() => getAvailableConnectionCategories(cards), [cards]);

  // Reset a selection whose last card disappeared.
  if (category !== 'all' && !availableCategories.includes(category)) {
    setCategory('all');
  }

  // Avoid a stale filtered render before React applies the reset.
  const activeCategory: CategoryFilterValue =
    category === 'all' || availableCategories.includes(category) ? category : 'all';

  const filteredCards: ConnectionCardModel[] = useMemo(() => {
    const term: string = search.trim().toLowerCase();
    return cards.filter((card) => {
      const matchesCategory: boolean = activeCategory === 'all' || card.categories.includes(activeCategory);
      if (!matchesCategory) {
        return false;
      }
      if (!term) {
        return true;
      }
      const haystack: string = [card.displayName, card.vendorKey, ...card.categories.map((c) => t(`categories.${c}`))]
        .join(' ')
        .toLowerCase();
      return haystack.includes(term);
    });
  }, [activeCategory, cards, search, t]);

  const handleAction = (card: ConnectionCardModel): void => {
    if (!card.navTarget) {
      return;
    }
    void navigate(card.navTarget);
  };

  const clearFilters = (): void => {
    setSearch('');
    setCategory('all');
  };

  const isLoading: boolean = connectionsQuery.isLoading;
  const hasFilters: boolean = search.trim() !== '' || activeCategory !== 'all';

  return (
    <Stack direction="column" spacing={3} data-testid="connections-list">
      <Stack direction="column" spacing={2}>
        <TextField
          fullWidth
          size="small"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t('listing.search.placeholder')}
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <Search size={16} />
                </InputAdornment>
              ),
            },
          }}
        />
        <ConnectionCategoryFilters categories={availableCategories} selected={activeCategory} onSelect={setCategory} />
        <Stack direction="row" alignItems="center" justifyContent="space-between">
          <Typography variant="body2" color="text.secondary">
            {isLoading ? t('listing.loading') : t('listing.showingCount', {count: filteredCards.length})}
          </Typography>
          {hasFilters && (
            <Button size="small" variant="text" startIcon={<X size={16} />} onClick={clearFilters}>
              {t('listing.clearFilters')}
            </Button>
          )}
        </Stack>
      </Stack>

      {isLoading ? (
        <Grid container spacing={2}>
          {Array.from({length: SKELETON_COUNT}).map((_, index) => (
            // eslint-disable-next-line react/no-array-index-key
            <Grid key={index} size={{xs: 12, sm: 6, md: 4, xl: 3}}>
              <Skeleton variant="rounded" height={220} />
            </Grid>
          ))}
        </Grid>
      ) : connectionsQuery.error ? (
        <QueryErrorNotice
          error={connectionsQuery.error}
          t={t}
          variant="block"
          title={t('listing.loadError', 'Failed to load connections')}
          onRetry={() => void connectionsQuery.refetch()}
        />
      ) : filteredCards.length === 0 ? (
        <Paper variant="outlined" sx={{p: 8, textAlign: 'center'}}>
          <Stack direction="column" spacing={2} alignItems="center">
            <SearchX size={40} />
            <Typography variant="h6">{t('listing.empty.title')}</Typography>
            <Typography variant="body2" color="text.secondary" sx={{maxWidth: 420}}>
              {t('listing.empty.description')}
            </Typography>
            {hasFilters && (
              <Button variant="contained" startIcon={<X size={16} />} onClick={clearFilters}>
                {t('listing.clearFilters')}
              </Button>
            )}
          </Stack>
        </Paper>
      ) : (
        <Grid container spacing={2}>
          {filteredCards.map((card) => (
            <Grid key={card.id} size={{xs: 12, sm: 6, md: 4, xl: 3}}>
              <ConnectionCard card={card} onAction={handleAction} />
            </Grid>
          ))}
          {!hasFilters && (
            <Grid size={{xs: 12, sm: 6, md: 4, xl: 3}}>
              <AddCustomConnectionCard onClick={() => void navigate(routes.connections.create())} />
            </Grid>
          )}
        </Grid>
      )}
    </Stack>
  );
}
