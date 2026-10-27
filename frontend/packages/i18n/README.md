# @thunderid/i18n

Internationalization (i18n) package for ThunderID applications using react-i18next with full TypeScript support.

## Features

- **Type-safe translations** - Full TypeScript support with autocomplete
- **React hooks** - Easy-to-use hooks for translations
- **Namespace organization** - Organized by app and feature for better maintainability
- **Multiple apps support** - Shared translations for console, gate, and other apps

## Installation

Since this is a workspace package, install dependencies from the root:

```bash
pnpm install
```

## Quick Start

### 1. Add Package Dependency

Add the package to your app's `package.json`:

```json
{
  "dependencies": {
    "@thunderid/i18n": "workspace:^"
  }
}
```

### 2. Initialize i18n in Your App

In your app's entry point (e.g., `main.tsx`):

```tsx
import i18n from 'i18next';
import {initReactI18next} from 'react-i18next';
import enUS from '@thunderid/i18n/locales/en-US';

// Initialize i18n before rendering your app
await i18n.use(initReactI18next).init({
  resources: {
    'en-US': enUS,
  },
  lng: 'en-US',
  fallbackLng: 'en-US',
  defaultNS: 'common',
  interpolation: {
    escapeValue: false, // React already escapes by default
  },
  debug: import.meta.env.DEV,
});

// Then render your app
root.render(<App />);
```

### 3. Use Translations in Components

```tsx
import {useTranslation} from 'react-i18next';

function MyComponent() {
  const {t} = useTranslation();

  return (
    <div>
      <h1>{t('navigation.home')}</h1>
      <p>{t('common.messages.welcomeMessage')}</p>
      <button>{t('common.actions.save')}</button>
    </div>
  );
}
```

### 4. Using Translation Namespaces

The translations are organized by namespace for better modularity:

```tsx
// Common translations (shared across all apps)
const {t} = useTranslation('common');
t('actions.save');
t('status.loading');
t('form.email');

// Navigation translations
const {t} = useTranslation('navigation');
t('home');
t('users');
t('applications');

// Users translations
const {t} = useTranslation('users');
t('title');
t('addUser');
t('given_name');

// Or use the default namespace and specify the full path
const {t} = useTranslation();
t('common.actions.save');
t('navigation.home');
t('users.title');
```

## Translation Structure

All translations are organized into a single file per locale with **namespaces** for better modularity:

```
src/locales/
└── en-US.ts                    # English (US) translations with all namespaces
```

### Current Locale Structure

The `en-US.ts` file contains all namespaces organized by feature:

```typescript
const translations = {
  common: {
    /* Common shared translations */
  },
  navigation: {
    /* Navigation items */
  },
  users: {
    /* User management */
  },
  userTypes: {
    /* User type management */
  },
  integrations: {
    /* Integration management */
  },
  applications: {
    /* Application management */
  },
  dashboard: {
    /* Dashboard content */
  },
  auth: {
    /* Authentication flows */
  },
  mfa: {
    /* Multi-factor authentication */
  },
  social: {
    /* Social login */
  },
  consent: {
    /* Consent management */
  },
  errors: {
    /* Error messages */
  },
};
```

### Namespace Details

**`common`** - Shared translations across all applications:

- `actions` - Action buttons (save, cancel, delete, etc.)
- `status` - Status messages (loading, success, error, etc.)
- `form` - Form labels and placeholders
- `messages` - Common messages
- `validation` - Form validation messages
- `time` - Time-related labels

**`navigation`** - Navigation menu items:

- Dashboard, Users, Applications, Integrations, etc.

**`users`** - User management features:

- User listing, creation, editing
- User attributes and properties

**`userTypes`** - User type management features

**`integrations`** - Integration management features

**`applications`** - Application management features

**`dashboard`** - Dashboard-specific content

**`auth`** - Authentication flows (for Gate):

- Sign in, sign up, password flows

**`mfa`** - Multi-factor authentication (for Gate)

**`social`** - Social login providers (for Gate)

**`consent`** - Consent management (for Gate)

**`errors`** - Error messages and error states

## Supported Languages

- **English (en-US)** - Default language

> **Note**: Additional language support (such as Sinhala, Spanish, etc.) can be added in the future by creating new
> locale files following the same namespace structure.

## Usage Examples

### Example: Console - Users Page

```tsx
import {useTranslation} from 'react-i18next';
import {Button, Typography} from '@wso2/oxygen-ui';

export function UsersPage() {
  const {t} = useTranslation('users');

  return (
    <div>
      <Typography variant="h4">{t('title')}</Typography>
      <Button variant="contained">{t('addUser')}</Button>

      {/* Table with translated headers */}
      <table>
        <thead>
          <tr>
            <th>{t('given_name')}</th>
            <th>{t('family_name')}</th>
            <th>{t('email')}</th>
            <th>{t('role')}</th>
            <th>{t('status')}</th>
            <th>{t('actions')}</th>
          </tr>
        </thead>
      </table>
    </div>
  );
}
```

### Example: Gate - Sign In Page

```tsx
import {useTranslation} from 'react-i18next';
import {Button, TextField, Typography} from '@wso2/oxygen-ui';

export function SignInPage() {
  const {t} = useTranslation('auth');

  return (
    <div>
      <Typography variant="h4">{t('welcomeBack')}</Typography>
      <Typography variant="h5">{t('signIn')}</Typography>

      <TextField label={t('common:form.email')} placeholder={t('enterEmail')} required />
      <TextField type="password" label={t('common:form.password')} placeholder={t('enterPassword')} required />

      <Button variant="contained">{t('signIn')}</Button>
      <Button variant="text">{t('forgotPassword')}</Button>
    </div>
  );
}
```

### Example: Form Validation

```tsx
import {useTranslation} from 'react-i18next';
import {useForm} from 'react-hook-form';

export function UserForm() {
  const {t} = useTranslation('common');
  const {
    register,
    handleSubmit,
    formState: {errors},
  } = useForm();

  return (
    <form>
      <input
        {...register('email', {
          required: t('validation.required'),
          pattern: {
            value: /^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$/i,
            message: t('validation.invalidEmail'),
          },
        })}
      />
      {errors.email && <span>{errors.email.message}</span>}

      <button type="submit">{t('actions.submit')}</button>
    </form>
  );
}
```

## TypeScript Support

This package is fully typed. You'll get autocomplete for all translation keys:

```tsx
// ✅ TypeScript will autocomplete these
t('common.actions.save');
t('users.title');
t('auth.signIn');

// ❌ TypeScript will error on invalid keys
t('common.invalid.key'); // Error: Key does not exist
```

## Adding New Translations

To add new translations:

### 1. Edit the locale file

Edit `src/locales/en-US.ts` and add your new translations to the appropriate namespace:

```tsx
const translations = {
  common: {
    // ... existing translations
    newFeature: {
      title: 'New Feature',
      description: 'This is a new feature',
    },
  },
  // ... other namespaces
} as const;
```

### 2. Rebuild the package

```bash
cd packages/i18n
pnpm build
```

TypeScript will automatically pick up the new keys and provide autocomplete!

### 3. Use the new translation

```tsx
const {t} = useTranslation('common');
<h1>{t('newFeature.title')}</h1>;
```

## Adding New Languages (Future)

To add support for additional languages:

### 1. Create a New Locale File

Create a new file in `src/locales/` (e.g., `si-LK.ts` for Sinhala):

```tsx
const translations = {
  common: {
    actions: {
      add: 'එකතු කරන්න',
      edit: 'සංස්කරණය කරන්න',
      delete: 'මකන්න',
      // ... translate all keys
    },
  },
  // ... translate all namespaces
} as const;

export default translations;
```

### 2. Update package.json exports

Add the new locale to `package.json`:

```json
{
  "exports": {
    "./locales/si-LK": {
      "types": "./dist/locales/si-LK.d.ts",
      "import": "./dist/locales/si-LK.js",
      "require": "./dist/locales/si-LK.cjs"
    }
  }
}
```

### 3. Use in your app

```tsx
import enUS from '@thunderid/i18n/locales/en-US';
import siLK from '@thunderid/i18n/locales/si-LK';

await i18n.use(initReactI18next).init({
  resources: {
    'en-US': enUS,
    'si-LK': siLK,
  },
  lng: 'en-US',
  // ... rest of config
});
```

## Development

```bash
# Build the package
pnpm build

# Watch mode for development
pnpm dev

# Type checking
pnpm typecheck

# Linting
pnpm lint
```

## Integration Guide

### Console App

Complete integration example for `apps/admin-console/src/main.tsx`:

```tsx
import * as ReactDOM from 'react-dom/client';
import {StrictMode} from 'react';
import {ConfigProvider} from '@thunderid/contexts';
import i18n from 'i18next';
import {initReactI18next} from 'react-i18next';
import enUS from '@thunderid/i18n/locales/en-US';
import AppWithConfig from './AppWithConfig';

// Initialize i18n before rendering the app
await i18n.use(initReactI18next).init({
  resources: {
    'en-US': enUS,
  },
  lng: 'en-US',
  fallbackLng: 'en-US',
  defaultNS: 'common',
  interpolation: {
    escapeValue: false, // React already escapes by default
  },
  debug: import.meta.env.DEV,
});

ReactDOM.createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ConfigProvider>
      <AppWithConfig />
    </ConfigProvider>
  </StrictMode>,
);
```

### Gate App

Similar integration for `apps/gate/src/main.tsx`:

```tsx
import {StrictMode} from 'react';
import {createRoot} from 'react-dom/client';
import i18n from 'i18next';
import {initReactI18next} from 'react-i18next';
import enUS from '@thunderid/i18n/locales/en-US';
import App from './App';
import './index.css';

// Initialize i18n before rendering
await i18n.use(initReactI18next).init({
  resources: {
    'en-US': enUS,
  },
  lng: 'en-US',
  fallbackLng: 'en-US',
  defaultNS: 'common',
  interpolation: {
    escapeValue: false,
  },
  debug: import.meta.env.DEV,
});

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
```

## Advanced Features

### Using Trans Component for Complex Translations

```tsx
import {Trans} from 'react-i18next';

function Component() {
  return (
    <Trans i18nKey="common.messages.welcome">
      Welcome <strong>{{name: user.name}}</strong>!
    </Trans>
  );
}
```

### Interpolation

```tsx
// Translation
{
  "welcome": "Welcome, {{name}}!"
}

// Usage
const { t } = useTranslation('common');
<p>{t('welcome', { name: user.name })}</p>
```

### Pluralization

```tsx
// Add to translations
{
  "users": {
    "count_one": "{{count}} user",
    "count_other": "{{count}} users"
  }
}

// Use in component
const { t } = useTranslation('users');
<p>{t('count', { count: userCount })}</p>
```

### Context-Specific Translations

```tsx
// Different translations based on context
const { t } = useTranslation('users');
<button>{t('actions.add', { context: 'male' })}</button>
<button>{t('actions.add', { context: 'female' })}</button>
```

## Best Practices

### 1. Use Specific Namespaces

```tsx
// ✅ Good - Specific namespace
const {t} = useTranslation('users');
t('title');

// ❌ Avoid - Always using full paths
const {t} = useTranslation();
t('users.title');
```

### 2. Organize by Feature

Keep related translations in the same namespace for better maintainability.

### 3. Consistent Naming

Use consistent naming conventions:

- Actions: `add`, `edit`, `delete`, `save`, etc.
- Labels: descriptive names like `given_name`, `email`, etc.
- Messages: `success`, `error`, `warning`, etc.

### 4. Avoid Hardcoded Strings

```tsx
// ✅ Good
<button>{t('common.actions.save')}</button>

// ❌ Bad
<button>Save</button>
```

## Troubleshooting

### Translations Not Loading

1. Make sure i18n is initialized before rendering the app
2. Check that the language file is properly imported
3. Verify the translation keys exist in the locale file
4. Check the browser console for any i18n errors

### TypeScript Errors

1. Rebuild the i18n package: `pnpm build --filter @thunderid/i18n`
2. Restart your TypeScript server in VSCode
3. Ensure the package is listed in dependencies
4. Check that types are being exported correctly

### Missing Translation Keys

If you see translation keys instead of translated text:

1. Verify the key exists in the locale file
2. Check that you're using the correct namespace
3. Ensure the namespace is loaded in your i18n config
4. Check for typos in the translation key

## Contributing

When adding new translations:

1. Add translations to all existing locale files (currently only `en-US.ts`)
2. Use the `as const` assertion for type safety
3. Follow the existing namespace structure
4. Rebuild the package and test in the consuming app
5. Document any new namespaces in this README

## License

Apache-2.0

---

## Migration Notes

### Manual i18n Setup Migration

If migrating from a manual i18n setup:

1. Install the `@thunderid/i18n` package
2. Remove local translation files
3. Import translations from `@thunderid/i18n/locales/en-US`
4. Update all `useTranslation()` calls to use the correct namespaces
5. Update translation keys to match the new structure

### Adding Future Languages

When adding support for additional languages:

1. Create a new locale file in `src/locales/` (e.g., `si-LK.ts`)
2. Translate all namespaces
3. Update `package.json` exports
4. Update this README with the new language
5. Test in all applications

## Resources

- [react-i18next Documentation](https://react.i18next.com/)
- [i18next Documentation](https://www.i18next.com/)
- [TypeScript with i18next](https://www.i18next.com/overview/typescript)
