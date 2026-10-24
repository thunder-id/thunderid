// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useHistory, useLocation} from '@docusaurus/router';
import useIsBrowser from '@docusaurus/useIsBrowser';
import {Box} from '@wso2/oxygen-ui';
import {JSX, useMemo} from 'react';
import EcosystemCTA from './EcosystemCTA';
import EcosystemFilters from './EcosystemFilters';
import EcosystemGrid from './EcosystemGrid';
import EcosystemHero from './EcosystemHero';
import EcosystemToolbar from './EcosystemToolbar';
import {
  activeChips,
  EcosystemFacetGroup,
  EcosystemFilters as Filters,
  EMPTY_FILTERS,
  filtersFromSearch,
  filtersToSearch,
  selectEntries,
  toggleFacet,
} from './presentation';
import {useEcosystem} from '@site/src/hooks/useEcosystem';

/**
 * The SDKs & tools marketplace.
 *
 * A faceted browse: the panel on the left narrows by category, maintainer, and
 * availability, while search and the result count sit directly above the grids.
 * Filtering lives here so the panel's counts and the results always derive from
 * one piece of state.
 *
 * That state is the query string, so a filtered listing can be shared as a
 * link. The static HTML is rendered unfiltered, so the URL is only read once
 * the browser takes over, which keeps a shared link from causing a hydration
 * mismatch. Changes use history.replace, so each keystroke or toggle does not
 * add a history entry.
 */
export default function EcosystemPage(): JSX.Element {
  const entries = useEcosystem();
  const location = useLocation();
  const history = useHistory();
  const isBrowser = useIsBrowser();

  const filters = useMemo(
    () => (isBrowser ? filtersFromSearch(location.search, entries) : EMPTY_FILTERS),
    [isBrowser, location.search, entries],
  );

  // Read from history.location rather than the render-time location, so back-to-back
  // updates in one handler each build on the latest query string.
  const setFilters = (update: (current: Filters) => Filters): void => {
    const current = history.location;
    history.replace({
      pathname: current.pathname,
      search: filtersToSearch(update(filtersFromSearch(current.search, entries)), current.search),
      hash: current.hash,
    });
  };

  const results = useMemo(() => selectEntries(entries, filters), [entries, filters]);
  const hasActive = activeChips(filters).length > 0;

  const handleToggle = (group: EcosystemFacetGroup, key: string): void => {
    setFilters((current) => toggleFacet(current, group, key));
  };

  return (
    <Box>
      <EcosystemHero />
      <Box
        sx={{
          maxWidth: 1200,
          mx: 'auto',
          px: {xs: 2, sm: 4},
          pb: {xs: 5, md: 7},
          display: 'grid',
          gridTemplateColumns: {xs: '1fr', md: '230px minmax(0, 1fr)'},
          gap: {xs: 4, md: 6},
          alignItems: 'start',
        }}
      >
        <EcosystemFilters
          entries={entries}
          filters={filters}
          onToggle={handleToggle}
          onClearAll={() => setFilters((current) => ({...EMPTY_FILTERS, query: current.query}))}
          hasActive={hasActive}
        />
        <Box sx={{minWidth: 0}}>
          <EcosystemToolbar
            filters={filters}
            onQueryChange={(query) => setFilters((current) => ({...current, query}))}
            onToggle={handleToggle}
            resultCount={results.length}
            totalCount={entries.length}
          />
          <EcosystemGrid query={filters.query} items={results} />
        </Box>
      </Box>
      <EcosystemCTA />
    </Box>
  );
}
