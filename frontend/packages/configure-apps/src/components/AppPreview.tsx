// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  BaseAgentList,
  BaseChangeCredential,
  BaseGroupList,
  BaseRoleList,
  BaseUserList,
  BaseUserProfile,
  ThemeContext,
  type BaseUserProfileProps,
  type BasicAgent,
  type Group,
  type ManagedUser,
  type RoleSummary,
} from '@thunderid/react';
import {
  APP_PAGES,
  createAppSdkThemes,
  resolveAppTheme,
  type AppConfig,
  type AppPageConfig,
  type AppPageKey,
  type AppThemeColors,
  type AppPageDefinition,
  type AppColorScheme,
  type ResolvedAppTheme,
} from '@thunderid/system-apps';
import {Box, Button, Stack, Typography} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {
  useContext,
  useMemo,
  type ContextType,
  type CSSProperties,
  type JSX,
  type MouseEvent,
  type ReactNode,
} from 'react';
import {useTranslation} from 'react-i18next';
import type {AppBuilderSelection} from './AppOutline';
import {PREVIEW_COMPONENT_ATTRIBUTE, PREVIEW_PAGE_ATTRIBUTE} from '../constants/app-preview';

/** The width the apps center their content in when it is contained. */
const CONTAINED_WIDTH = 1240;

const SAMPLE_OU = 'ou-sample';

const SAMPLE_USERS: ManagedUser[] = [
  {display: 'Aiko Tanaka', id: 'u-1', ouHandle: 'engineering', ouId: SAMPLE_OU, type: 'person'},
  {display: 'Sam Rivera', id: 'u-2', ouHandle: 'engineering', ouId: SAMPLE_OU, type: 'person'},
  {display: 'Priya Sen', id: 'u-3', ouHandle: 'engineering', ouId: SAMPLE_OU, type: 'person'},
];

const SAMPLE_GROUPS: Group[] = [
  {description: 'Builds the platform', id: 'g-1', name: 'Platform team', ouHandle: 'engineering', ouId: SAMPLE_OU},
  {description: 'Answers incidents', id: 'g-2', name: 'On-call rotation', ouHandle: 'engineering', ouId: SAMPLE_OU},
];

const SAMPLE_ROLES: RoleSummary[] = [
  {
    description: 'Manages this organization unit',
    id: 'r-1',
    name: 'OU admin',
    ouHandle: 'engineering',
    ouId: SAMPLE_OU,
  },
  {description: 'Ships code', id: 'r-2', name: 'Developer', ouHandle: 'engineering', ouId: SAMPLE_OU},
];

const SAMPLE_AGENTS: BasicAgent[] = [
  {clientId: 'agt_7f3k9q2m', id: 'a-1', name: 'Deploy bot', ouHandle: 'engineering', ouId: SAMPLE_OU, type: 'workload'},
  {
    clientId: 'agt_2p8x4n1c',
    id: 'a-2',
    name: 'Support concierge',
    ouHandle: 'engineering',
    ouId: SAMPLE_OU,
    type: 'assistant',
  },
];

const SAMPLE_PROFILE: NonNullable<BaseUserProfileProps['profile']> = {
  email: 'aiko@example.com',
  family_name: 'Tanaka',
  given_name: 'Aiko',
  username: 'aiko',
};

export interface AppPreviewProps {
  config: AppConfig;
  /** The color scheme to show the app in. */
  scheme?: AppColorScheme;
  /** Called with the page a navigation entry of the preview opens. */
  onSelect?: (page: AppPageKey) => void;
  selection: AppBuilderSelection;
}

/** Marks its child as built from the named SDK component, for the builder's inspector. */
function Component({children, name}: {children: ReactNode; name: string}): JSX.Element {
  return <Box {...{[PREVIEW_COMPONENT_ATTRIBUTE]: `<${name} />`}}>{children}</Box>;
}

type ThemeVars = NonNullable<ContextType<typeof ThemeContext>>['theme']['vars'];

/** The name of the CSS custom property a `var(--name)` reference reads. */
const variableName = (reference: string | undefined): string | undefined =>
  /^var\((--[^,)]+)/.exec(reference ?? '')?.[1];

/**
 * The SDK components read their colors, corners and font from CSS custom properties on the page.
 * Setting the same properties on the preview restyles the components inside it, and only those.
 * The property names come from the SDK theme in context, so they follow its vendor prefix.
 */
function useSdkThemeVariables(sdkTheme: ReturnType<typeof createAppSdkThemes>[AppColorScheme]): CSSProperties {
  const vars: ThemeVars | undefined = useContext(ThemeContext)?.theme.vars;

  return useMemo(() => {
    if (!vars) return {};
    const colors = sdkTheme.colors ?? {};
    const pairs: [string | undefined, string | undefined][] = [
      [vars.colors.primary.main, colors.primary?.main],
      [vars.colors.primary.contrastText, colors.primary?.contrastText],
      [vars.colors.primary.dark, colors.primary?.dark],
      [vars.colors.background.surface, colors.background?.surface],
      [vars.colors.background.body.main, colors.background?.body?.main],
      [vars.colors.background.disabled, colors.background?.disabled],
      [vars.colors.text.primary, colors.text?.primary],
      [vars.colors.text.secondary, colors.text?.secondary],
      [vars.colors.border, colors.border],
      [vars.borderRadius.small, sdkTheme.borderRadius?.small],
      [vars.borderRadius.medium, sdkTheme.borderRadius?.medium],
      [vars.borderRadius.large, sdkTheme.borderRadius?.large],
      [vars.typography.fontFamily, sdkTheme.typography?.fontFamily],
    ];

    return Object.fromEntries(
      pairs
        .map(([reference, value]) => [variableName(reference), value])
        .filter((pair): pair is [string, string] => Boolean(pair[0] && pair[1])),
    ) as CSSProperties;
  }, [vars, sdkTheme]);
}

function PageContent({
  page,
  theme,
}: {
  page: AppPageConfig;
  theme: ResolvedAppTheme & {scheme: AppColorScheme};
}): JSX.Element {
  const {t} = useTranslation('apps');
  const definition: AppPageDefinition | undefined = [...APP_PAGES.ADMIN, ...APP_PAGES.ACCOUNT].find(
    (candidate: AppPageDefinition) => candidate.key === page.key,
  );
  // The first component lists the page's items; the second, when there is one, creates them.
  const [listComponent = '', createComponent] = definition?.components ?? [];
  const canAdd: boolean = page.actions['create'] === true || page.actions['invite'] === true;
  const actions: ReactNode = canAdd ? (
    <Box component="span" {...{[PREVIEW_COMPONENT_ATTRIBUTE]: `<${createComponent} />`}}>
      <Button
        size="small"
        variant="contained"
        startIcon={<Plus size={14} />}
        tabIndex={-1}
        sx={{
          bgcolor: theme.colorSchemes[theme.scheme].primary,
          borderRadius: `${theme.borderRadius * 2.5}px`,
          boxShadow: 'none',
          textTransform: 'none',
        }}
      >
        {t('builder.preview.add', 'Add')}
      </Button>
    </Box>
  ) : undefined;

  switch (page.key) {
    case 'profile':
      return (
        <Component name={listComponent}>
          <BaseUserProfile profile={SAMPLE_PROFILE} flattenedProfile={SAMPLE_PROFILE} editable={page.actions['edit']} />
        </Component>
      );
    case 'security':
      return page.actions['password'] ? (
        <Component name={listComponent}>
          <BaseChangeCredential />
        </Component>
      ) : (
        <Typography sx={{color: theme.colorSchemes[theme.scheme].textSecondary}}>
          {t('builder.preview.noActions', 'Nothing to change here yet.')}
        </Typography>
      );
    case 'groups':
      return (
        <Component name={listComponent}>
          <BaseGroupList groups={SAMPLE_GROUPS} actions={actions} />
        </Component>
      );
    case 'roles':
      return (
        <Component name={listComponent}>
          <BaseRoleList roles={SAMPLE_ROLES} actions={actions} />
        </Component>
      );
    case 'agents':
      return (
        <Component name={listComponent}>
          <BaseAgentList agents={SAMPLE_AGENTS} actions={actions} />
        </Component>
      );
    default:
      return (
        <Component name={listComponent}>
          <BaseUserList users={SAMPLE_USERS} actions={actions} />
        </Component>
      );
  }
}

/**
 * A live preview of the app as it will look with the current configuration, rendered from the
 * same SDK components the app uses, with sample data. It fills its container like the app fills
 * the browser window.
 */
export default function AppPreview({
  config,
  onSelect = undefined,
  scheme = 'light',
  selection,
}: AppPreviewProps): JSX.Element {
  const {t} = useTranslation('apps');
  const theme: ResolvedAppTheme = useMemo(() => resolveAppTheme(config.appTheme), [config.appTheme]);
  const sdkThemes = useMemo(() => createAppSdkThemes(config.appTheme), [config.appTheme]);
  const sdkVariables: CSSProperties = useSdkThemeVariables(sdkThemes[scheme]);
  const colors: Required<AppThemeColors> = theme.colorSchemes[scheme];
  const visiblePages: AppPageConfig[] = config.pages.filter((page: AppPageConfig) => page.enabled);
  const current: AppPageConfig | undefined =
    visiblePages.find((page: AppPageConfig) => page.key === selection) ?? visiblePages[0];
  const isSideBar: boolean = config.navigation.layout === 'SIDE_BAR';
  // A contained app centers its navigation and pages in a column, as the app does.
  const columnSx =
    config.content.width === 'CONTAINED' ? {maxWidth: CONTAINED_WIDTH, mx: 'auto', width: '100%'} : {width: '100%'};

  const isAccount: boolean = config.type === 'ACCOUNT';
  // The Account App always opens on a home page, and its side bar items are pills pinned to the edge.
  const navEntries: {active: boolean; key: string; label: string; page?: AppPageKey}[] = [
    ...(isAccount ? [{active: false, key: 'home', label: t('builder.pages.home', 'Home')}] : []),
    ...visiblePages.map((page: AppPageConfig) => ({
      active: page.key === current?.key,
      key: page.key,
      label: t(`builder.pages.${page.key}`, page.key),
      page: page.key,
    })),
  ];

  const sideBarItemSx = (active: boolean) =>
    isAccount
      ? {
          ml: -1.5,
          pl: 2.5,
          py: 0.75,
          borderRadius: '0 16px 16px 0',
          bgcolor: active ? `color-mix(in srgb, ${colors.primary} 16%, transparent)` : 'transparent',
        }
      : {
          px: 1,
          py: 0.75,
          borderRadius: `${theme.borderRadius}px`,
          bgcolor: active ? `color-mix(in srgb, ${colors.primary} 10%, transparent)` : 'transparent',
        };

  const navItems: JSX.Element[] = navEntries.map(({active, key, label, page}) => (
    <Typography
      key={key}
      variant="body2"
      fontWeight={active ? 600 : 400}
      {...(page ? {[PREVIEW_PAGE_ATTRIBUTE]: page} : {})}
      sx={{
        color: active ? colors.textPrimary : colors.textSecondary,
        ...(isSideBar
          ? sideBarItemSx(active)
          : {py: 1.25, borderBottom: 2, borderColor: active ? colors.primary : 'transparent'}),
        ...(page && onSelect ? {cursor: 'pointer', '&:hover': {color: colors.textPrimary}} : {}),
      }}
    >
      {label}
    </Typography>
  ));

  // The preview is a picture of the app, not the app: clicks only switch the page shown.
  const handleClickCapture = (event: MouseEvent<HTMLElement>): void => {
    event.preventDefault();
    event.stopPropagation();
    const page = (event.target as HTMLElement)
      .closest(`[${PREVIEW_PAGE_ATTRIBUTE}]`)
      ?.getAttribute(PREVIEW_PAGE_ATTRIBUTE);
    if (page) {
      onSelect?.(page as AppPageKey);
    }
  };

  return (
    <Box
      role="region"
      aria-label={
        config.type === 'ACCOUNT'
          ? t('builder.preview.accountLabel', 'Account App preview')
          : t('builder.preview.label', 'Admin App preview')
      }
      onClickCapture={handleClickCapture}
      style={sdkVariables}
      sx={{
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        bgcolor: colors.background,
        color: colors.textPrimary,
        fontFamily: theme.fontFamily,
        colorScheme: scheme,
        '& .MuiTypography-root': {fontFamily: 'inherit'},
      }}
    >
      {!isSideBar && (
        <Box sx={{borderBottom: 1, borderColor: colors.border, bgcolor: colors.surface, flexShrink: 0}}>
          <Box sx={{px: 3, pt: 2, ...columnSx}}>
            <Stack direction="row" spacing={1} alignItems="center">
              <Typography variant="subtitle1" fontWeight={600}>
                ThunderID
              </Typography>
              {config.type === 'ADMIN' && (
                <>
                  <Typography sx={{color: colors.textSecondary}}>/</Typography>
                  <Typography variant="body2">{t('builder.preview.organization', 'Engineering')}</Typography>
                </>
              )}
            </Stack>
            <Stack direction="row" spacing={3} sx={{mt: 1}}>
              {navItems}
            </Stack>
          </Box>
        </Box>
      )}
      <Stack direction="row" sx={{flex: 1, minHeight: 0, ...columnSx}} data-testid="app-preview-body">
        {isSideBar && (
          <Stack
            spacing={0.5}
            sx={{width: 240, flexShrink: 0, p: 2, borderRight: 1, borderColor: colors.border, bgcolor: colors.surface}}
          >
            <Typography variant="subtitle1" fontWeight={600} sx={{px: 1, pb: 1.5}}>
              ThunderID
            </Typography>
            {navItems}
          </Stack>
        )}
        <Box sx={{flex: 1, minWidth: 0, overflow: 'auto', px: 4, py: 3}}>
          {current ? (
            <PageContent page={current} theme={{...theme, scheme}} />
          ) : (
            <Typography sx={{color: colors.textSecondary}}>
              {t('builder.preview.empty', 'Turn on a page to see it here.')}
            </Typography>
          )}
        </Box>
      </Stack>
      {config.branding.showSecuredBy && (
        <Box sx={{borderTop: 1, borderColor: colors.border, flexShrink: 0}}>
          <Typography
            variant="caption"
            color={colors.textSecondary}
            component="p"
            sx={{m: 0, px: 3, py: 1.25, textAlign: 'end', ...columnSx}}
          >
            {t('builder.preview.securedBy', 'Secured by ThunderID')}
          </Typography>
        </Box>
      )}
    </Box>
  );
}
