/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package integrate provides step definitions for the in-REPL integration guides.
package integrate

// Step is one pane of the integration guide.
// Code lines may contain {{.KEY}} placeholders that are substituted with
// collected values at render time (e.g. {{.ClientID}}).
type Step struct {
	Title        string
	Body         []string // description lines shown above the code block
	CodeFile     string   // display label: "src/main.jsx", "terminal", etc.
	CodeLang     string   // "bash" or "jsx" — shown as a hint in the border
	Code         []string // code lines; {{.KEY}} is replaced at render time
	CollectKey   string   // if non-empty, pause to collect this value before showing Code
	CollectLabel string   // prompt header shown while collecting
	CollectHint  string   // textinput placeholder
	CollectURL   string   // optional URL shown in the collect prompt for the user to open
}

// VueSteps returns the ordered integration steps for adding @thunderid/vue
// to an existing Vue 3 app.
func VueSteps(baseURL string) []Step {
	return []Step{
		{
			Title:    "Install @thunderid/vue",
			Body:     []string{"Install the ThunderID Vue SDK in your project:"},
			CodeFile: "terminal",
			CodeLang: "bash",
			Code:     []string{"npm install @thunderid/vue"},
		},
		{
			Title:    "Register the Plugin",
			Body:     []string{"Register ThunderIDPlugin in src/main.js:"},
			CodeFile: "src/main.js",
			CodeLang: "js",
			Code: []string{
				`import { createApp } from 'vue'`,
				`import { ThunderIDPlugin } from '@thunderid/vue'`,
				`import App from './App.vue'`,
				`import './style.css'`,
				``,
				`const app = createApp(App)`,
				`app.use(ThunderIDPlugin)`,
				`app.mount('#app')`,
			},
		},
		{
			Title:        "Add ThunderIDProvider",
			Body:         []string{"Wrap your app in src/App.vue with ThunderIDProvider:"},
			CollectKey:   "ClientID",
			CollectLabel: "Your Client ID",
			CollectHint:  "Console → Applications → your app → Client ID",
			CodeFile:     "src/App.vue",
			CodeLang:     "vue",
			Code: []string{
				`<script setup>`,
				`import {`,
				`  SignedIn, SignedOut,`,
				`  SignInButton, SignOutButton,`,
				`} from '@thunderid/vue'`,
				`</script>`,
				``,
				`<template>`,
				`  <ThunderIDProvider`,
				`    client-id="{{.ClientID}}"`,
				`    base-url="` + baseURL + `"`,
				`  >`,
				`    <SignedIn>`,
				`      <SignOutButton>Sign Out</SignOutButton>`,
				`    </SignedIn>`,
				`    <SignedOut>`,
				`      <SignInButton>Sign In</SignInButton>`,
				`    </SignedOut>`,
				`  </ThunderIDProvider>`,
				`</template>`,
			},
		},
		{
			Title:    "Start Your App",
			Body:     []string{"Start the development server:"},
			CodeFile: "terminal",
			CodeLang: "bash",
			Code:     []string{"npm run dev"},
		},
	}
}

// NextJSSteps returns the ordered integration steps for adding @thunderid/nextjs
// to an existing Next.js app.
func NextJSSteps(baseURL string) []Step {
	return []Step{
		{
			Title:    "Install @thunderid/nextjs",
			Body:     []string{"Install the ThunderID Next.js SDK in your project:"},
			CodeFile: "terminal",
			CodeLang: "bash",
			Code:     []string{"npm install @thunderid/nextjs"},
		},
		{
			Title:        "Set Environment Variables",
			Body:         []string{"Create .env.local with your credentials:"},
			CollectKey:   "ClientID",
			CollectLabel: "Your Client ID",
			CollectHint:  "Console → Applications → your app → Client ID",
			CodeFile:     ".env.local",
			CodeLang:     "dotenv",
			Code: []string{
				`NEXT_PUBLIC_THUNDERID_BASE_URL=` + baseURL,
				`NEXT_PUBLIC_THUNDERID_CLIENT_ID={{.ClientID}}`,
				`THUNDERID_CLIENT_SECRET=<your-client-secret>`,
				`THUNDERID_SECRET=<run: openssl rand -base64 32>`,
				`# Remove in production:`,
				`NODE_TLS_REJECT_UNAUTHORIZED=0`,
			},
		},
		{
			Title:    "Add ThunderIDProvider to Layout",
			Body:     []string{"Wrap your root layout in app/layout.tsx:"},
			CodeFile: "app/layout.tsx",
			CodeLang: "tsx",
			Code: []string{
				`import { ThunderIDProvider }`,
				`  from '@thunderid/nextjs/server'`,
				``,
				`export default function RootLayout({ children }) {`,
				`  return (`,
				`    <html lang="en">`,
				`      <body>`,
				`        <ThunderIDProvider>`,
				`          {children}`,
				`        </ThunderIDProvider>`,
				`      </body>`,
				`    </html>`,
				`  )`,
				`}`,
			},
		},
		{
			Title:    "Add the ThunderID Proxy",
			Body:     []string{"Create proxy.ts to handle auth routing:"},
			CodeFile: "proxy.ts",
			CodeLang: "ts",
			Code: []string{
				`import {`,
				`  thunderIDProxy,`,
				`  createRouteMatcher,`,
				`} from '@thunderid/nextjs/server'`,
				``,
				`const isProtected = createRouteMatcher([])`,
				``,
				`export default thunderIDProxy(`,
				`  async (thunderid, request) => {`,
				`    if (isProtected(request))`,
				`      await thunderid.protectRoute()`,
				`  }`,
				`)`,
				``,
				`export const config = {`,
				`  matcher: [`,
				`    '/((?!_next/static|_next/image|favicon.ico).*)',`,
				`  ],`,
				`}`,
			},
		},
		{
			Title:    "Build with ThunderID Components",
			Body:     []string{"Update app/page.tsx with auth components:"},
			CodeFile: "app/page.tsx",
			CodeLang: "tsx",
			Code: []string{
				`import {`,
				`  SignedIn, SignedOut,`,
				`  SignInButton, UserDropdown,`,
				`} from "@thunderid/nextjs"`,
				``,
				`export default function Home() {`,
				`  return (`,
				`    <section>`,
				`      <SignedIn>`,
				`        <UserDropdown />`,
				`      </SignedIn>`,
				`      <SignedOut>`,
				`        <SignInButton>Sign In</SignInButton>`,
				`      </SignedOut>`,
				`    </section>`,
				`  )`,
				`}`,
			},
		},
		{
			Title:    "Start Your App",
			Body:     []string{"Start the development server:"},
			CodeFile: "terminal",
			CodeLang: "bash",
			Code:     []string{"npm run dev"},
		},
	}
}

// NuxtSteps returns the ordered integration steps for adding @thunderid/nuxt
// to an existing Nuxt 3 app.
func NuxtSteps(baseURL string) []Step {
	return []Step{
		{
			Title:    "Install @thunderid/nuxt",
			Body:     []string{"Install the ThunderID Nuxt module:"},
			CodeFile: "terminal",
			CodeLang: "bash",
			Code:     []string{"npm install @thunderid/nuxt"},
		},
		{
			Title:    "Register the Module",
			Body:     []string{"Add @thunderid/nuxt to nuxt.config.ts:"},
			CodeFile: "nuxt.config.ts",
			CodeLang: "ts",
			Code: []string{
				`export default defineNuxtConfig({`,
				`  modules: ['@thunderid/nuxt'],`,
				`})`,
			},
		},
		{
			Title:        "Set Up Environment Variables",
			Body:         []string{"Create .env with your credentials:"},
			CollectKey:   "ClientID",
			CollectLabel: "Your Client ID",
			CollectHint:  "Console → Applications → your app → Client ID",
			CodeFile:     ".env",
			CodeLang:     "dotenv",
			Code: []string{
				`NUXT_PUBLIC_THUNDERID_BASE_URL=` + baseURL,
				`NUXT_PUBLIC_THUNDERID_CLIENT_ID={{.ClientID}}`,
				`THUNDERID_CLIENT_SECRET=<your-client-secret>`,
				`THUNDERID_SESSION_SECRET=<run: openssl rand -base64 32>`,
			},
		},
		{
			Title:    "Wrap App with ThunderIDRoot",
			Body:     []string{"Update app.vue to use ThunderIDRoot:"},
			CodeFile: "app.vue",
			CodeLang: "vue",
			Code: []string{
				`<template>`,
				`  <ThunderIDRoot>`,
				`    <NuxtPage />`,
				`  </ThunderIDRoot>`,
				`</template>`,
			},
		},
		{
			Title:    "Add Sign-In and Sign-Out",
			Body:     []string{"Create pages/index.vue with auth components:"},
			CodeFile: "pages/index.vue",
			CodeLang: "vue",
			Code: []string{
				`<template>`,
				`  <main>`,
				`    <SignedIn>`,
				`      <SignOutButton>Sign Out</SignOutButton>`,
				`    </SignedIn>`,
				`    <SignedOut>`,
				`      <SignInButton>Sign In</SignInButton>`,
				`    </SignedOut>`,
				`  </main>`,
				`</template>`,
			},
		},
		{
			Title:    "Start Your App",
			Body:     []string{"Start the development server:"},
			CodeFile: "terminal",
			CodeLang: "bash",
			Code:     []string{"npm run dev"},
		},
	}
}


// ReactSteps returns the ordered integration steps for adding @thunderid/react
// to an existing React app. baseURL is the running ThunderID instance URL and
// is embedded directly into the ThunderIDProvider code snippet.
func ReactSteps(baseURL string) []Step {
	return []Step{
		{
			Title:        "Get your Client ID",
			CollectKey:   "ClientID",
			CollectLabel: "Your Client ID",
			CollectHint:  "Console → Applications → your app → Client ID",
			CollectURL:   baseURL + "/console",
		},
		{
			Title:    "Install @thunderid/react",
			Body:     []string{"Install the ThunderID React SDK in your project:"},
			CodeFile: "terminal",
			CodeLang: "bash",
			Code:     []string{"npm install @thunderid/react"},
		},
		{
			Title:    "Add ThunderIDProvider",
			Body:     []string{"Wrap your root component with ThunderIDProvider in src/main.jsx:"},
			CodeFile: "src/main.jsx",
			CodeLang:     "jsx",
			Code: []string{
				`import { StrictMode } from 'react'`,
				`import { createRoot } from 'react-dom/client'`,
				`import { ThunderIDProvider } from '@thunderid/react'`,
				`import App from './App.jsx'`,
				`import './index.css'`,
				``,
				`createRoot(document.getElementById('root')).render(`,
				`  <StrictMode>`,
				`    <ThunderIDProvider`,
				`      clientId="{{.ClientID}}"`,
				`      baseUrl="` + baseURL + `"`,
				`    >`,
				`      <App />`,
				`    </ThunderIDProvider>`,
				`  </StrictMode>`,
				`)`,
			},
		},
		{
			Title:    "Add Sign-In and Sign-Out",
			Body:     []string{"Update src/App.jsx to add auth components:"},
			CodeFile: "src/App.jsx",
			CodeLang: "jsx",
			Code: []string{
				`import {`,
				`  SignedIn, SignedOut,`,
				`  SignInButton, SignOutButton, Loading`,
				`} from '@thunderid/react'`,
				``,
				`function App() {`,
				`  return (`,
				`    <>`,
				`      <Loading>`,
				`        <div>Loading authentication...</div>`,
				`      </Loading>`,
				`      <SignedOut>`,
				`        <SignInButton>Sign In</SignInButton>`,
				`      </SignedOut>`,
				`      <SignedIn>`,
				`        <SignOutButton>Sign Out</SignOutButton>`,
				`      </SignedIn>`,
				`    </>`,
				`  )`,
				`}`,
			},
		},
		{
			Title:    "Display User Profile",
			Body:     []string{"Use the User component to show profile info in src/App.jsx:"},
			CodeFile: "src/App.jsx",
			CodeLang: "jsx",
			Code: []string{
				`import {`,
				`  SignedIn, SignedOut,`,
				`  SignInButton, SignOutButton, Loading, User`,
				`} from '@thunderid/react'`,
				``,
				`function App() {`,
				`  return (`,
				`    <>`,
				`      <Loading><div>Loading...</div></Loading>`,
				`      <SignedOut><SignInButton>Sign In</SignInButton></SignedOut>`,
				`      <SignedIn>`,
				`        <SignOutButton>Sign Out</SignOutButton>`,
				`        <User>`,
				`          {(user) => user && (`,
				`            <div>`,
				`              <h2>Welcome, {user.name}!</h2>`,
				`              <p>{user.email}</p>`,
				`            </div>`,
				`          )}`,
				`        </User>`,
				`      </SignedIn>`,
				`    </>`,
				`  )`,
				`}`,
			},
		},
		{
			Title: "Start Your App",
			Body: []string{
				"Start the development server:",
			},
			CodeFile: "terminal",
			CodeLang: "bash",
			Code:     []string{"npm run dev"},
		},
	}
}
