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
  addComponent,
  addImports,
  addPlugin,
  addRouteMiddleware,
  addServerHandler,
  addServerPlugin,
  createResolver,
  defineNuxtModule,
  extendViteConfig,
} from '@nuxt/kit';
import type {Nuxt} from '@nuxt/schema';
import {defu} from 'defu';
import type {ThunderIDNuxtConfig, ThunderIDSessionPayload, ThunderIDSSRData} from './runtime/types';

type ViteUserConfig = Parameters<Parameters<typeof extendViteConfig>[0]>[0];

const PACKAGE_NAME: string = '@thunderid/nuxt';

type ServerRoute = {
  handler: string;
  method?: 'patch' | 'post';
  route: string;
};

export default defineNuxtModule<ThunderIDNuxtConfig>({
  defaults: {},
  meta: {
    configKey: 'thunderid',
    name: PACKAGE_NAME,
  },
  setup(userOptions: ThunderIDNuxtConfig, nuxt: Nuxt) {
    const {resolve} = createResolver(import.meta.url);

    // Merge config: env vars (highest) -> nuxt.config.ts userOptions -> hard defaults (lowest)
    const publicConfig: ThunderIDNuxtConfig = defu(
      // Layer 1: environment variables — only win when actually set
      {
        afterSignInUrl: process.env['NUXT_PUBLIC_THUNDERID_AFTER_SIGN_IN_URL'],
        afterSignOutUrl: process.env['NUXT_PUBLIC_THUNDERID_AFTER_SIGN_OUT_URL'],
        applicationId: process.env['NUXT_PUBLIC_THUNDERID_APPLICATION_ID'],
        baseUrl: process.env['NUXT_PUBLIC_THUNDERID_BASE_URL'],
        clientId: process.env['NUXT_PUBLIC_THUNDERID_CLIENT_ID'],
        signInUrl: process.env['NUXT_PUBLIC_THUNDERID_SIGN_IN_URL'],
        signUpUrl: process.env['NUXT_PUBLIC_THUNDERID_SIGN_UP_URL'],
      },
      // Layer 2: nuxt.config.ts options
      userOptions,
      // Layer 3: hard defaults
      {
        afterSignInUrl: '/',
        afterSignOutUrl: '/',
        scopes: ['openid', 'profile'],
      },
    );

    const privateConfig: {clientSecret: string; sessionSecret: string} = {
      clientSecret: process.env['THUNDERID_CLIENT_SECRET'] || userOptions.clientSecret || '',
      sessionSecret: process.env['THUNDERID_SESSION_SECRET'] || userOptions.sessionSecret || '',
    };

    // Config validation deliberately does not happen here. `setup()` runs during
    // `nuxt module-build prepare` (SDK build) and during the consumer's build —
    // neither is the right moment to complain about unset runtime env vars.
    // Missing/invalid config is reported where it matters:
    //  - Server: runtime/server/plugins/thunderid-ssr.ts (refuses to initialize)
    //  - Client: runtime/plugins/thunderid.ts (dev-time browser console warning)

    const {options} = nuxt;

    // Security: ensure secrets are never in public runtime config
    options.runtimeConfig.thunderid = defu(
      (options.runtimeConfig.thunderid as Record<string, unknown>) || {},
      privateConfig,
    ) as {clientSecret: string; sessionSecret: string};

    options.runtimeConfig.public.thunderid = defu(
      (options.runtimeConfig.public.thunderid as Record<string, unknown>) || {},
      {
        afterSignInUrl: publicConfig.afterSignInUrl,
        afterSignOutUrl: publicConfig.afterSignOutUrl,
        applicationId: publicConfig.applicationId,
        baseUrl: publicConfig.baseUrl,
        clientId: publicConfig.clientId,
        platform: publicConfig.platform,
        preferences: publicConfig.preferences,
        scopes: publicConfig.scopes,
        signInUrl: publicConfig.signInUrl,
        signUpUrl: publicConfig.signUpUrl,
        tokenRequest: publicConfig.tokenRequest,
      },
    ) as {
      afterSignInUrl: string;
      afterSignOutUrl: string;
      applicationId?: string;
      baseUrl: string;
      clientId: string;
      platform?: ThunderIDNuxtConfig['platform'];
      preferences: ThunderIDNuxtConfig['preferences'];
      scopes: string[];
      signInUrl?: string;
      signUpUrl?: string;
      tokenRequest?: ThunderIDNuxtConfig['tokenRequest'];
    };

    // Ensure clientSecret never leaks to public config
    const publicThunderID: Record<string, unknown> = options.runtimeConfig.public.thunderid as Record<string, unknown>;
    if (publicThunderID?.['clientSecret']) {
      delete publicThunderID['clientSecret'];
      // eslint-disable-next-line no-console
      console.error(
        `[${PACKAGE_NAME}] SECURITY: clientSecret found in public config. Removed. Use THUNDERID_CLIENT_SECRET env var.`,
      );
    }
    if (publicThunderID?.['sessionSecret']) {
      delete publicThunderID['sessionSecret'];
      // eslint-disable-next-line no-console
      console.error(
        `[${PACKAGE_NAME}] SECURITY: sessionSecret found in public config. Removed. Use THUNDERID_SESSION_SECRET env var.`,
      );
    }

    // Register server API routes
    const serverRoutes: ServerRoute[] = [
      // ── Auth flow ──────────────────────────────────────────────────────
      {handler: resolve('./runtime/server/routes/auth/session/signin.get'), route: '/api/auth/signin'},
      {
        handler: resolve('./runtime/server/routes/auth/session/signin.post'),
        method: 'post' as const,
        route: '/api/auth/signin',
      },
      {
        handler: resolve('./runtime/server/routes/auth/session/signup.post'),
        method: 'post' as const,
        route: '/api/auth/signup',
      },
      {handler: resolve('./runtime/server/routes/auth/session/callback.get'), route: '/api/auth/callback'},
      {
        handler: resolve('./runtime/server/routes/auth/session/callback.post'),
        method: 'post' as const,
        route: '/api/auth/callback',
      },
      {
        handler: resolve('./runtime/server/routes/auth/session/signout.post'),
        method: 'post' as const,
        route: '/api/auth/signout',
      },
      // ── Session / token ───────────────────────────────────────────────
      {handler: resolve('./runtime/server/routes/auth/session/session.get'), route: '/api/auth/session'},
      {handler: resolve('./runtime/server/routes/auth/session/token.get'), route: '/api/auth/token'},
      // ── User ──────────────────────────────────────────────────────────
      {handler: resolve('./runtime/server/routes/auth/user/user.get'), route: '/api/auth/user'},
      {handler: resolve('./runtime/server/routes/auth/user/profile.get'), route: '/api/auth/user/profile'},
      {
        handler: resolve('./runtime/server/routes/auth/user/profile.patch'),
        method: 'patch' as const,
        route: '/api/auth/user/profile',
      },
      // ── Organisations ─────────────────────────────────────────────────
      {
        handler: resolve('./runtime/server/routes/auth/organizations/index.get'),
        route: '/api/auth/organizations',
      },
      {
        handler: resolve('./runtime/server/routes/auth/organizations/index.post'),
        method: 'post' as const,
        route: '/api/auth/organizations',
      },
      {
        handler: resolve('./runtime/server/routes/auth/organizations/me.get'),
        route: '/api/auth/organizations/me',
      },
      {
        handler: resolve('./runtime/server/routes/auth/organizations/current.get'),
        route: '/api/auth/organizations/current',
      },
      {
        handler: resolve('./runtime/server/routes/auth/organizations/id.get'),
        route: '/api/auth/organizations/:id',
      },
      {
        handler: resolve('./runtime/server/routes/auth/organizations/switch.post'),
        method: 'post' as const,
        route: '/api/auth/organizations/switch',
      },
      // ── Branding ──────────────────────────────────────────────────────
      {handler: resolve('./runtime/server/routes/auth/branding/branding.get'), route: '/api/auth/branding'},
    ];

    serverRoutes.forEach((sr: ServerRoute): void => {
      addServerHandler({handler: sr.handler, method: 'method' in sr ? sr.method : undefined, route: sr.route});
    });

    // Register server plugin for SSR auth state + rich SSR data fetching
    addServerPlugin(resolve('./runtime/server/plugins/thunderid-ssr'));

    // Register client plugin
    addPlugin(resolve('./runtime/plugins/thunderid'));

    // Register named route middleware for page protection
    addRouteMiddleware({
      name: 'auth',
      path: resolve('./runtime/middleware/auth'),
    });

    // Auto-import composables and utilities
    addImports([
      // Core auth composable (Nuxt-specific wrapper around @thunderid/vue)
      {from: resolve('./runtime/composables/useThunderID'), name: 'useThunderID'},
      // Composables from @thunderid/vue — auto-imported directly, no local wrappers
      {from: '@thunderid/vue', name: 'useUser'},
      {from: '@thunderid/vue', name: 'useOrganization'},
      {from: '@thunderid/vue', name: 'useFlow'},
      {from: '@thunderid/vue', name: 'useFlowMeta'},
      {from: '@thunderid/vue', name: 'useTheme'},
      {from: '@thunderid/vue', name: 'useBranding'},
      // useI18n aliased to `useThunderIDI18n` to avoid collision with @nuxtjs/i18n
      {as: 'useThunderIDI18n', from: '@thunderid/vue', name: 'useI18n'},
      // Middleware factory
      {from: resolve('./runtime/middleware/defineThunderIDMiddleware'), name: 'defineThunderIDMiddleware'},
    ]);

    // Register the Nuxt-specific root component that mounts the full Vue
    // provider tree (I18nProvider, BrandingProvider, ThemeProvider, etc.).
    // Users wrap their `app.vue` with `<ThunderIDRoot>` — matching the way
    // Next.js users wrap their app with `<ThunderIDServerProvider>`.
    addComponent({
      filePath: resolve('./runtime/components/ThunderIDRoot'),
      name: 'ThunderIDRoot',
    });

    // Register Nuxt-specific component containers with the `ThunderID` prefix.
    //
    // Each container lives at `./runtime/components/<Name>.ts` and:
    //   1. Imports the corresponding BaseXxx from @thunderid/vue (not the Vue container).
    //   2. Wires composables through `#imports` (Nuxt auto-import layer).
    //   3. Uses `navigateTo` from `#app` for all navigation — SSR-safe, no window.location.
    //
    // This mirrors the Next.js SDK pattern where Base components come from
    // @thunderid/react and host-specific containers live in the Next.js package.
    //
    // NOTE: Composables (useUser, useOrganization, useTheme, useBranding,
    // useFlow, useI18n) remain direct re-exports from @thunderid/vue via
    // addImports above — only the components need Nuxt wrappers.

    // ── Control flow ────────────────────────────────────────────────────────
    addComponent({filePath: resolve('./runtime/components/control/SignedIn'), name: 'ThunderIDSignedIn'});
    addComponent({filePath: resolve('./runtime/components/control/SignedOut'), name: 'ThunderIDSignedOut'});
    addComponent({filePath: resolve('./runtime/components/control/Loading'), name: 'ThunderIDLoading'});

    // ── Action buttons ───────────────────────────────────────────────────────
    addComponent({filePath: resolve('./runtime/components/actions/SignInButton'), name: 'ThunderIDSignInButton'});
    addComponent({filePath: resolve('./runtime/components/actions/SignOutButton'), name: 'ThunderIDSignOutButton'});
    addComponent({filePath: resolve('./runtime/components/actions/SignUpButton'), name: 'ThunderIDSignUpButton'});

    // ── Embedded auth flows ──────────────────────────────────────────────────
    addComponent({filePath: resolve('./runtime/components/auth/SignIn'), name: 'ThunderIDSignIn'});
    addComponent({filePath: resolve('./runtime/components/auth/SignUp'), name: 'ThunderIDSignUp'});

    // ── User ─────────────────────────────────────────────────────────────────
    addComponent({filePath: resolve('./runtime/components/user/User'), name: 'ThunderIDUser'});
    addComponent({filePath: resolve('./runtime/components/user/UserProfile'), name: 'ThunderIDUserProfile'});
    addComponent({filePath: resolve('./runtime/components/user/UserDropdown'), name: 'ThunderIDUserDropdown'});

    // ── Organization ─────────────────────────────────────────────────────────
    addComponent({filePath: resolve('./runtime/components/organization/Organization'), name: 'ThunderIDOrganization'});
    addComponent({
      filePath: resolve('./runtime/components/organization/OrganizationProfile'),
      name: 'ThunderIDOrganizationProfile',
    });
    addComponent({
      filePath: resolve('./runtime/components/organization/OrganizationSwitcher'),
      name: 'ThunderIDOrganizationSwitcher',
    });
    addComponent({
      filePath: resolve('./runtime/components/organization/OrganizationList'),
      name: 'ThunderIDOrganizationList',
    });
    addComponent({
      filePath: resolve('./runtime/components/organization/CreateOrganization'),
      name: 'ThunderIDCreateOrganization',
    });

    // ── Auth callback ────────────────────────────────────────────────────────
    addComponent({filePath: resolve('./runtime/components/auth/Callback'), name: 'ThunderIDCallback'});

    // Tell Vite to pre-bundle the CJS-only packages that @thunderid/browser,
    // @thunderid/javascript, and @thunderid/vue carry as external dependencies.
    // Without this, Vite serves them raw from disk via @fs URLs and fails with
    // "Export 'X' is not defined in module" errors when installed from npm.
    // This is only needed for the client Vite build; Nitro handles the server.
    extendViteConfig(
      (viteConfig: ViteUserConfig) => {
        const deps: string[] = [
          '@thunderid/browser',
          '@thunderid/javascript',
          '@thunderid/vue',
          'base64url',
          'fast-sha256',
        ];

        const existingInclude: string[] = (viteConfig.optimizeDeps?.include as string[]) ?? [];
        const newDeps: string[] = deps.filter((dep: string) => !existingInclude.includes(dep));

        Object.assign(viteConfig, {
          optimizeDeps: {
            ...viteConfig.optimizeDeps,
            include: [...existingInclude, ...newDeps],
          },
        });
      },
      {client: true},
    );
  },
});

declare module '@nuxt/schema' {
  interface NuxtConfig {
    thunderid?: ThunderIDNuxtConfig;
  }
  interface NuxtOptions {
    thunderid?: ThunderIDNuxtConfig;
  }
  interface PublicRuntimeConfig {
    thunderid: {
      afterSignInUrl: string;
      afterSignOutUrl: string;
      applicationId?: string;
      baseUrl: string;
      clientId: string;
      platform?: ThunderIDNuxtConfig['platform'];
      preferences?: ThunderIDNuxtConfig['preferences'];
      scopes: string[];
      signInUrl?: string;
      signUpUrl?: string;
    };
  }

  interface RuntimeConfig {
    thunderid: {
      clientSecret: string;
      sessionSecret: string;
    };
  }
}

declare module 'h3' {
  interface H3EventContext {
    thunderid?: {
      isSignedIn?: boolean;
      session?: ThunderIDSessionPayload | {sub?: string} | null;
      ssr?: ThunderIDSSRData;
    };
  }
}
