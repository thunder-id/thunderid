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

import {Button, Grid, InputAdornment, Paper, Skeleton, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {Search, SearchX, X} from '@wso2/oxygen-ui-icons-react';
import {type JSX, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AddCustomConnectionCard from './AddCustomConnectionCard';
import ConnectionCard from './ConnectionCard';
import ConnectionCategoryFilters, {type CategoryFilterValue} from './ConnectionCategoryFilters';
import useConnections from '../api/useConnections';
import {CONNECTION_VENDOR_META} from '../config/connectionVendorMeta';
import type {ConnectionCardModel} from '../models/connection';
import buildConnectionCards from '../utils/buildConnectionCards';

const SKELETON_COUNT = 6;

export default function ConnectionsList(): JSX.Element {
  const {t} = useTranslation('connections');
  const navigate = useNavigate();

  const [search, setSearch] = useState('');
  const [category, setCategory] = useState<CategoryFilterValue>('all');

  const connectionsQuery = useConnections();

  const cards: ConnectionCardModel[] = useMemo(
    () => buildConnectionCards(connectionsQuery.data?.connections ?? [], CONNECTION_VENDOR_META),
    [connectionsQuery.data?.connections],
  );

  const filteredCards: ConnectionCardModel[] = useMemo(() => {
    const term: string = search.trim().toLowerCase();
    return cards.filter((card) => {
      const matchesCategory: boolean = category === 'all' || card.categories.includes(category);
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
  }, [cards, category, search, t]);

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
  const hasFilters: boolean = search.trim() !== '' || category !== 'all';

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
        <ConnectionCategoryFilters selected={category} onSelect={setCategory} />
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
            <Grid key={index} size={{xs: 12, sm: 6, md: 4}}>
              <Skeleton variant="rounded" height={220} />
            </Grid>
          ))}
        </Grid>
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
            <Grid key={card.id} size={{xs: 12, sm: 6, md: 4}}>
              <ConnectionCard card={card} onAction={handleAction} />
            </Grid>
          ))}
          {!hasFilters && (
            <Grid size={{xs: 12, sm: 6, md: 4}}>
              <AddCustomConnectionCard onClick={() => void navigate('/connections/create')} />
            </Grid>
          )}
        </Grid>
      )}
    </Stack>
  );
}
