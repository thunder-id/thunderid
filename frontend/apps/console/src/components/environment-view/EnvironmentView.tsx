// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import type {Environment} from '@thunderid/contexts';
import {useConfig, useEnvironment} from '@thunderid/contexts';
import {ThunderIDContext} from '@thunderid/react';
import {
  Box,
  Button,
  CircularProgress,
  createTheme,
  PageContent,
  Paper,
  ThemeProvider,
  Typography,
  useTheme,
} from '@wso2/oxygen-ui';
import {useCallback, useContext, useMemo, useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {useLocation} from 'react-router';
import EnvironmentApi, {type Request} from './environmentApi';
import EnvironmentResourceDetail from './EnvironmentResourceDetail';
import {
  EnvironmentViewContextProvider,
  type EnvironmentViewContextType,
  type IndexedResource,
} from './EnvironmentViewContext';
import {layoutOf, layoutOfType, nameOf, type ResourceLayout} from './layouts';
import type {AppliedResource, StoredValue} from './models';
import SetValueDialog from './SetValueDialog';
import {useAppliedConfiguration, useStoredValues} from './useEnvironmentData';
import type {ValueReference} from './values';
import {ROUTE_SEGMENTS} from '../../configs/RouteConfig';

/** Sections that belong to the configuration alone: an environment has nothing of its own there. */
const CONFIGURATION_ONLY: readonly string[] = [
  ROUTE_SEGMENTS.importExport,
  ROUTE_SEGMENTS.export,
  ROUTE_SEGMENTS.importConfiguration,
  ROUTE_SEGMENTS.welcome,
];

/** Steps of a path that lead to making or changing something, which an environment does not do. */
const CHANGING_STEPS: readonly string[] = ['create', 'add', 'types', 'configure'];

/**
 * What the configuration's pages offer that an environment does not: the actions in a page's title,
 * such as creating, and the actions that delete, which the pages colour as errors. Both are kept
 * from showing through the theme, which reaches every page without the page knowing.
 */
const WITHOUT_CHANGES = {
  components: {
    MuiPageTitle: {styleOverrides: {actions: {display: 'none'}}},
    MuiIconButton: {styleOverrides: {colorError: {display: 'none'}}},
  },
};

function Notice({children, action = undefined}: {children: string; action?: ReactNode}): JSX.Element {
  return (
    <PageContent>
      <Paper variant="outlined" sx={{p: 4, textAlign: 'center'}}>
        <Typography variant="body2" color="text.secondary" sx={{mb: action ? 2 : 0}}>
          {children}
        </Typography>
        {action}
      </Paper>
    </PageContent>
  );
}

/**
 * Serves the configuration's own pages, given as `children`, from what the environment runs: their
 * reads are answered from it, and what they offer that would change it is not shown. The pages keep
 * a cache of their own for each environment, so what one shows is never shown for another, nor for
 * the configuration.
 */
function EnvironmentPages({
  environment,
  api,
  children,
}: {
  environment: Environment;
  api: EnvironmentApi;
  children: ReactNode;
}): JSX.Element {
  const thunderID = useContext(ThunderIDContext);
  const outerTheme = useTheme();
  const theme = useMemo(() => createTheme(outerTheme, WITHOUT_CHANGES), [outerTheme]);
  if (!thunderID) {
    throw new Error('The environment view is shown outside the ThunderIDProvider');
  }
  const {getServerUrl} = useConfig();
  const serverUrl = getServerUrl();
  const queryClient = useMemo(() => new QueryClient({defaultOptions: {queries: {retry: false}}}), []);
  const request = useCallback(
    async (config?: Request): Promise<unknown> => {
      const answer = api.answer(config ?? {}, serverUrl);
      if (answer !== undefined) {
        return {data: answer, status: 200};
      }
      return thunderID.http.request(config as Parameters<typeof thunderID.http.request>[0]);
    },
    [api, serverUrl, thunderID],
  );
  const value = useMemo(
    () => ({...thunderID, http: {...thunderID.http, request: request as typeof thunderID.http.request}}),
    [thunderID, request],
  );
  return (
    <QueryClientProvider client={queryClient} key={environment.id}>
      <ThunderIDContext.Provider value={value}>
        <ThemeProvider theme={theme}>{children}</ThemeProvider>
      </ThunderIDContext.Provider>
    </QueryClientProvider>
  );
}

interface Editing {
  reference: ValueReference;
  list: boolean;
}

/** Where a path leads within an environment. */
type Destination =
  | {kind: 'pages'}
  | {kind: 'through'}
  | {kind: 'changing'}
  | {kind: 'resource'; layout: ResourceLayout; id: string};

function destinationOf(pathname: string): Destination {
  const [segment, ...rest] = pathname.split('/').filter(Boolean).map(decodeURIComponent);
  if (segment === undefined || segment === ROUTE_SEGMENTS.home) return {kind: 'pages'};
  if (CONFIGURATION_ONLY.includes(segment) || rest.some((step: string) => CHANGING_STEPS.includes(step))) {
    return {kind: 'changing'};
  }
  const layout = layoutOf(segment);
  if (!layout) return {kind: 'through'};
  if (segment === ROUTE_SEGMENTS.settings) return {kind: 'resource', layout, id: 'cors'};
  if (rest.length === 0) return {kind: 'pages'};
  // A connection's page is at its type, then its id; a theme's or a layout's below its kind.
  if (segment === ROUTE_SEGMENTS.connections && rest.length < 2) return {kind: 'changing'};
  return {kind: 'resource', layout, id: rest[rest.length - 1]};
}

/**
 * The console as one environment runs it. The home page and the lists are the configuration's own
 * pages, shown from the version the environment's gateway applied, with nothing that would change
 * it. A resource has a page of its own, laid out as its page in the configuration is but only for
 * reading: the one thing that can be set is a value the gateway fills the configuration with. A
 * section that is not configuration, such as gateway management, is the page it always is.
 */
export default function EnvironmentView({
  environment,
  children,
}: {
  environment: Environment;
  children: ReactNode;
}): ReactNode {
  const {t} = useTranslation();
  const {select} = useEnvironment();
  const {pathname} = useLocation();
  const destination = destinationOf(pathname);
  const configuration = useAppliedConfiguration(environment.id);
  const variables = useStoredValues(environment.id, 'variable');
  const secrets = useStoredValues(environment.id, 'secret');
  const [editing, setEditing] = useState<Editing | null>(null);
  const api = useMemo(() => new EnvironmentApi(configuration.data), [configuration.data]);

  const context: EnvironmentViewContextType = useMemo(() => {
    const resources = new Map<string, IndexedResource>();
    configuration.data?.resources.forEach((applied: AppliedResource) => {
      const shownUnder = layoutOfType(applied.resourceType);
      if (shownUnder) {
        resources.set(applied.id, {
          name: nameOf(applied.resource, applied.id, shownUnder),
          path: `/${shownUnder.segment}/${encodeURIComponent(applied.id)}`,
        });
      }
    });
    return {
      environment,
      variables: new Map(
        (variables.data ?? []).flatMap((value: StoredValue) =>
          value.value === undefined ? [] : [[value.name, value.value] as [string, string]],
        ),
      ),
      secrets: new Set((secrets.data ?? []).filter((value: StoredValue) => value.exists).map((value) => value.name)),
      resources,
      edit: (reference: ValueReference, list: boolean) => setEditing({reference, list}),
    };
  }, [environment, configuration.data, variables.data, secrets.data]);

  if (destination.kind === 'through') {
    return children;
  }
  if (destination.kind === 'changing') {
    return (
      <Notice
        action={
          <Button variant="contained" onClick={() => select(undefined)}>
            {t('common:environment.showConfiguration', 'Show the configuration')}
          </Button>
        }
      >
        {t('common:environment.configurationOnly', 'This belongs to the configuration, not to an environment.')}
      </Notice>
    );
  }
  if (configuration.isLoading) {
    return (
      <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
        <CircularProgress size={28} />
      </Box>
    );
  }
  if (configuration.error || !configuration.data) {
    return (
      <Notice>
        {t('common:environment.error', 'What {{environment}} runs could not be read.', {environment: environment.name})}
      </Notice>
    );
  }
  if (!configuration.data.version) {
    return (
      <Notice>
        {t('common:environment.nothingDeployed', 'Nothing has been deployed to {{environment}} yet.', {
          environment: environment.name,
        })}
      </Notice>
    );
  }
  if (destination.kind === 'pages') {
    return (
      <EnvironmentPages environment={environment} api={api}>
        {children}
      </EnvironmentPages>
    );
  }

  const {layout, id} = destination;
  const applied = configuration.data.resources.find(
    (resource: AppliedResource) => layout.types.includes(resource.resourceType) && resource.id === id,
  );
  const held = editing ? (editing.reference.kind === 'secret' ? secrets.data : variables.data) : undefined;
  const stored = held?.find((value: StoredValue) => value.name === editing?.reference.name);
  return (
    <EnvironmentViewContextProvider value={context}>
      <EnvironmentResourceDetail layout={layout} applied={applied} />
      {editing && (
        <SetValueDialog
          gatewayId={environment.id}
          environmentName={environment.name}
          reference={editing.reference}
          list={editing.list}
          current={stored?.value}
          description={stored?.description}
          onClose={() => setEditing(null)}
        />
      )}
    </EnvironmentViewContextProvider>
  );
}
