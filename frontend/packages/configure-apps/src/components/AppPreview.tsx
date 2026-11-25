// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Attribution,
  BaseAgentList,
  BaseChangeCredential,
  BaseGroupList,
  BaseRoleList,
  BaseUserList,
  BaseUserProfile,
  ThemeContext,
  useTranslation as useSdkTranslation,
  type BaseUserProfileProps,
  type BasicAgent,
  type Group,
  type ManagedUser,
  type RoleSummary,
} from '@thunderid/react';
import {
  APP_PAGES,
  createAppSdkThemes,
  resolveAppImage,
  resolveAppTheme,
  type AppConfig,
  type AppPageConfig,
  type AppPageKey,
  type AppTheme,
  type AppThemeColors,
  type AppPageDefinition,
  type AppColorScheme,
  type ResolvedAppTheme,
} from '@thunderid/system-apps';
import {Box, Button, Stack, Typography} from '@wso2/oxygen-ui';
import {Building2, ChevronDown, Globe, Monitor, Plus} from '@wso2/oxygen-ui-icons-react';
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

/** The height of the logo in the top bar, as the app draws it. */
const LOGO_HEIGHT = 26;

/** The page frame's side gutter, as the app pads it. */
const GUTTER_SX = {px: 4};

const SAMPLE_OU = 'ou-sample';

const SAMPLE_USERS: ManagedUser[] = [
  {
    attributes: {email: 'aiko@example.com'},
    display: 'Aiko Tanaka',
    id: 'u-1',
    ouHandle: 'engineering',
    ouId: SAMPLE_OU,
    type: 'person',
  },
  {
    attributes: {email: 'sam@example.com'},
    display: 'Sam Rivera',
    id: 'u-2',
    ouHandle: 'engineering',
    ouId: SAMPLE_OU,
    type: 'person',
  },
  {
    attributes: {email: 'priya@example.com'},
    display: 'Priya Sen',
    id: 'u-3',
    ouHandle: 'engineering',
    ouId: SAMPLE_OU,
    type: 'person',
  },
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
  {
    attributes: {name: 'Deploy bot'},
    clientId: 'agt_7f3k9q2m',
    id: 'a-1',
    ouHandle: 'engineering',
    ouId: SAMPLE_OU,
    type: 'workload',
  },
  {
    attributes: {name: 'Support concierge'},
    clientId: 'agt_2p8x4n1c',
    id: 'a-2',
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
  /** The logo of the application the app signs in with, which the app shows without its own. */
  applicationLogoUrl?: string;
  /** The look of the app, read from the Design theme it takes. Without one, the app's own look. */
  appTheme?: AppTheme;
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

/** The list's search does not run in the preview; it is there to show the search the page offers. */
const ignoreSearch = (): void => undefined;

function PageContent({
  page,
  theme,
}: {
  page: AppPageConfig;
  theme: ResolvedAppTheme & {scheme: AppColorScheme};
}): JSX.Element {
  const {t} = useTranslation('apps');
  const {t: sdkT} = useSdkTranslation();
  const definition: AppPageDefinition | undefined = [...APP_PAGES.ADMIN, ...APP_PAGES.ACCOUNT].find(
    (candidate: AppPageDefinition) => candidate.key === page.key,
  );
  // The first component lists the page's items; the second, when there is one, creates them.
  const [listComponent = '', createComponent] = definition?.components ?? [];
  const canCreate: boolean = page.actions['create'] === true;
  const canAdd: boolean = canCreate || page.actions['invite'] === true;
  // People are added directly when admins may create them, else invited, as the app labels it.
  const addLabelKey: string =
    page.key === 'users' && !canCreate ? 'manage:actions.invite.users' : `manage:actions.add.${page.key}`;
  const actions: ReactNode = canAdd ? (
    <Box component="span" {...{[PREVIEW_COMPONENT_ATTRIBUTE]: `<${createComponent} />`}}>
      <Button
        variant="contained"
        startIcon={<Plus size={16} />}
        tabIndex={-1}
        sx={{
          // The Console theme paints its buttons with an !important gradient, which would cover the
          // app's color; a more specific !important rule outranks it.
          '&&&:not(:disabled)': {background: `${theme.colorSchemes[theme.scheme].primary} !important`},
          borderRadius: `${theme.borderRadius * 2.5}px`,
          boxShadow: 'none',
          textTransform: 'none',
        }}
      >
        {t(addLabelKey, t('manage:actions.add', 'Add'))}
      </Button>
    </Box>
  ) : undefined;
  const listProps = {
    actions,
    description: t(`manage:${page.key}.description`, ''),
    title: t(`manage:nav.${page.key}`, page.key),
  };

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
          <BaseGroupList groups={SAMPLE_GROUPS} {...listProps} />
        </Component>
      );
    case 'roles':
      return (
        <Component name={listComponent}>
          <BaseRoleList roles={SAMPLE_ROLES} {...listProps} />
        </Component>
      );
    case 'agents':
      return (
        <Component name={listComponent}>
          <BaseAgentList
            agents={SAMPLE_AGENTS}
            onSearch={ignoreSearch}
            searchAttributes={[{label: sdkT('admin.agents.list.search.name'), value: 'name'}]}
            {...listProps}
          />
        </Component>
      );
    default:
      return (
        <Component name={listComponent}>
          <BaseUserList
            users={SAMPLE_USERS}
            onSearch={ignoreSearch}
            searchAttributes={[
              {label: sdkT('admin.users.list.search.username'), value: 'username'},
              {label: sdkT('admin.users.list.search.email'), value: 'email'},
            ]}
            {...listProps}
          />
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
  applicationLogoUrl = undefined,
  appTheme = undefined,
  config,
  onSelect = undefined,
  scheme = 'light',
  selection,
}: AppPreviewProps): JSX.Element {
  const {t} = useTranslation('apps');
  const theme: ResolvedAppTheme = useMemo(() => resolveAppTheme(appTheme), [appTheme]);
  // The app's own logo for the color scheme shown, else the application's, as the app picks it.
  const logoUrl: string | undefined = resolveAppImage(config.branding.logo, scheme) ?? applicationLogoUrl;
  const sdkThemes = useMemo(() => createAppSdkThemes(appTheme), [appTheme]);
  const sdkVariables: CSSProperties = useSdkThemeVariables(sdkThemes[scheme]);
  const colors: Required<AppThemeColors> = theme.colorSchemes[scheme];
  const visiblePages: AppPageConfig[] = config.pages.filter((page: AppPageConfig) => page.enabled);
  const current: AppPageConfig | undefined =
    visiblePages.find((page: AppPageConfig) => page.key === selection) ?? visiblePages[0];
  const isSideBar: boolean = config.navigation.layout === 'SIDE_BAR';
  // A contained app centers its pages in a column, as the app does; its bars span the window.
  const columnSx =
    config.content.width === 'CONTAINED' ? {maxWidth: CONTAINED_WIDTH, mx: 'auto', width: '100%'} : {width: '100%'};

  const isAccount: boolean = config.type === 'ACCOUNT';
  // The Account App always opens on a home page, ahead of the pages the builder turns on.
  const navEntries: {active: boolean; key: string; label: string; page?: AppPageKey}[] = [
    ...(isAccount ? [{active: false, key: 'home', label: t('builder.pages.home', 'Home')}] : []),
    ...visiblePages.map((page: AppPageConfig) => ({
      active: page.key === current?.key,
      key: page.key,
      label: t(`builder.pages.${page.key}`, page.key),
      page: page.key,
    })),
  ];

  const sideBarItemSx = (active: boolean) => ({
    px: 1,
    py: 0.75,
    borderRadius: `${theme.borderRadius}px`,
    bgcolor: active ? `color-mix(in srgb, ${colors.primary} 10%, transparent)` : 'transparent',
  });

  // A tab of the top bar's second row, underlined in the primary color when current.
  const tabSx = (active: boolean) => ({
    px: 0.5,
    mx: 1.5,
    pt: 1,
    pb: 1.25,
    mb: '-1px',
    fontSize: 15,
    whiteSpace: 'nowrap',
    borderBottom: 2,
    borderColor: active ? colors.primary : 'transparent',
  });

  const navItems: JSX.Element[] = navEntries.map(({active, key, label, page}) => (
    <Typography
      key={key}
      variant="body2"
      fontWeight={active ? 600 : 400}
      {...(page ? {[PREVIEW_PAGE_ATTRIBUTE]: page} : {})}
      sx={{
        color: active ? colors.textPrimary : colors.textSecondary,
        ...(isSideBar ? sideBarItemSx(active) : tabSx(active)),
        ...(page && onSelect ? {cursor: 'pointer', '&:hover': {color: colors.textPrimary}} : {}),
      }}
    >
      {label}
    </Typography>
  ));

  // The app's brand: its logo, its name, and the organization unit the admin manages.
  const brand: JSX.Element = (
    <Stack direction="row" alignItems="center" spacing={1.5} sx={{minWidth: 0}}>
      {logoUrl && <Box component="img" src={logoUrl} alt="" sx={{height: LOGO_HEIGHT, width: 'auto'}} />}
      <Typography noWrap sx={{fontSize: 16, fontWeight: 600}}>
        {config.branding.title ?? (isAccount ? t('account:title', 'MyAccount') : t('manage:title', 'Admin'))}
      </Typography>
      {config.type === 'ADMIN' && (
        <Stack direction="row" alignItems="center" spacing={1.5} sx={{minWidth: 0}}>
          <Typography sx={{color: colors.textSecondary}} aria-hidden>
            /
          </Typography>
          <Stack direction="row" alignItems="center" spacing={1} sx={{minWidth: 0}}>
            <Building2 size={16} strokeWidth={1.75} aria-hidden />
            <Typography noWrap sx={{fontSize: 15, fontWeight: 500}}>
              {t('builder.preview.organization', 'Engineering')}
            </Typography>
            <Box
              component="span"
              sx={{
                px: 0.75,
                py: 0.25,
                borderRadius: 1,
                bgcolor: `color-mix(in srgb, ${colors.textPrimary} 6%, transparent)`,
                color: colors.textSecondary,
                fontSize: 11,
                fontWeight: 600,
                letterSpacing: '0.04em',
              }}
            >
              {t('manage:organizationUnit.badge', 'OU')}
            </Box>
          </Stack>
        </Stack>
      )}
    </Stack>
  );

  // The color scheme switch and the signed-in admin's menu, drawn but not working.
  // The language the app opens in, as its short code, such as EN.
  const languageCode: string | undefined = config.languages?.menu
    ? (config.languages.default ?? config.languages.offered?.[0] ?? 'en').split('-')[0].toUpperCase()
    : undefined;
  const userControls: JSX.Element = (
    <Stack direction="row" alignItems="center" spacing={1.5} sx={{flexShrink: 0}} aria-hidden>
      {languageCode && (
        <Stack
          direction="row"
          alignItems="center"
          spacing={0.5}
          sx={{
            px: 1,
            height: 32,
            borderRadius: 16,
            border: 1,
            borderColor: colors.border,
            bgcolor: colors.surface,
            fontSize: 12,
            fontWeight: 600,
          }}
        >
          <Globe size={14} />
          <span>{languageCode}</span>
          <ChevronDown size={12} />
        </Stack>
      )}
      <Box
        sx={{
          display: 'grid',
          placeItems: 'center',
          width: 44,
          height: 44,
          borderRadius: '50%',
          border: 1,
          borderColor: colors.border,
          bgcolor: colors.surface,
        }}
      >
        <Monitor size={20} />
      </Box>
      <Box
        sx={{
          display: 'grid',
          placeItems: 'center',
          width: 32,
          height: 32,
          borderRadius: '50%',
          bgcolor: colors.primary,
          color: sdkThemes[scheme].colors?.primary?.contrastText ?? '#fff',
          fontSize: 14,
          fontWeight: 600,
        }}
      >
        A
      </Box>
    </Stack>
  );

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
        isAccount
          ? t('builder.preview.accountLabel', 'MyAccount preview')
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
      <Box sx={{borderBottom: 1, borderColor: colors.border, bgcolor: colors.surface, flexShrink: 0}}>
        <Box sx={GUTTER_SX}>
          <Stack direction="row" alignItems="center" spacing={3} sx={{py: 1.75}}>
            {isSideBar ? <Box /> : brand}
            <Box sx={{flex: 1}} />
            {userControls}
          </Stack>
          {!isSideBar && <Box sx={{display: 'flex', ml: -1.5}}>{navItems}</Box>}
        </Box>
      </Box>
      <Stack direction="row" sx={{flex: 1, minHeight: 0}} data-testid="app-preview-body">
        {isSideBar && (
          <Stack
            spacing={0.5}
            sx={{width: 240, flexShrink: 0, p: 2, borderRight: 1, borderColor: colors.border, bgcolor: colors.surface}}
          >
            <Box sx={{px: 1, pb: 1.5}}>{brand}</Box>
            {navItems}
          </Stack>
        )}
        <Box sx={{flex: 1, minWidth: 0, overflow: 'auto', display: 'flex', flexDirection: 'column'}}>
          <Box sx={{...GUTTER_SX, ...columnSx, flex: 1, pt: 5, pb: 4}}>
            {current ? (
              <PageContent page={current} theme={{...theme, scheme}} />
            ) : (
              <Typography sx={{color: colors.textSecondary}}>
                {t('builder.preview.empty', 'Turn on a page to see it here.')}
              </Typography>
            )}
          </Box>
          {(config.branding.showSecuredBy || Boolean(config.branding.links?.length)) && (
            <Stack
              direction="row"
              alignItems="center"
              justifyContent="center"
              flexWrap="wrap"
              spacing={2}
              // The Account App's footer is a dark band, as the app draws it.
              sx={{...GUTTER_SX, py: 3, ...(isAccount ? {bgcolor: 'grey.900', color: 'grey.300'} : {})}}
            >
              {config.branding.links?.map((link) => (
                <Typography
                  key={`${link.label}-${link.url}`}
                  variant="caption"
                  sx={{color: isAccount ? 'common.white' : colors.textSecondary, textDecoration: 'underline'}}
                >
                  {link.label}
                </Typography>
              ))}
              {config.branding.showSecuredBy && <Attribution />}
            </Stack>
          )}
        </Box>
      </Stack>
    </Box>
  );
}
