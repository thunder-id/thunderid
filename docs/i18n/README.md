# ThunderID Documentation Internationalization (i18n)

This directory contains translations for the ThunderID documentation.

## Directory Structure

```
i18n/
├── <locale>/
│   ├── docusaurus-plugin-content-docs/
│   │   └── current/
│   │       └── ... (translated documentation files)
│   ├── docusaurus-plugin-content-blog/
│   │   └── ... (translated blog posts)
│   └── docusaurus-theme-classic/
│       ├── navbar.json (navbar translations)
│       └── footer.json (footer translations)
└── README.md (this file)
```

## Supported Locales

| Locale | Language | Status |
|--------|----------|--------|
| `en-US` | English (US) | ✅ Default |

## Contributing Translations

We welcome community contributions for translations! Here's how you can help:

### 1. Adding a New Language

1. Open an issue to discuss adding support for your language
2. Fork the repository
3. Copy the `en-US` directory structure to your locale (e.g., `es-ES` for Spanish)
4. Add your locale to `docusaurus.config.ts`:

```typescript
i18n: {
  defaultLocale: 'en-US',
  locales: ['en-US', 'es-ES'], // Add your locale here
  localeConfigs: {
    'en-US': { ... },
    'es-ES': {
      label: 'Español',
      direction: 'ltr',
      htmlLang: 'es-ES',
    },
  },
},
```

5. Translate the content files
6. Submit a pull request

### 2. Translating Content

#### Documentation (MDX/Markdown files)

- Copy files from `content/` to `i18n/<locale>/docusaurus-plugin-content-docs/current/`
- Translate the content while keeping the file structure intact
- Keep code blocks, links, and images unchanged unless they need localization

#### UI Strings (JSON files)

- Translate strings in `docusaurus-theme-classic/navbar.json` and `footer.json`
- Keep the JSON keys unchanged, only translate the values

### 3. Guidelines

- Use ISO locale codes in `<LANG>-<COUNTRY>` format (e.g., `en-US`, `es-ES`, `ja-JP`)
- Maintain consistent terminology across translations
- Keep technical terms that don't have good translations
- Test your translations locally before submitting

### 4. Testing Locally

```bash
# Start docs with a specific locale
cd docs
npm run start -- --locale <your-locale>

# Build all locales
npm run build
```

## Locale Codes Reference

Common locale codes:
- `en-US` - English (United States)
- `en-GB` - English (United Kingdom)
- `es-ES` - Spanish (Spain)
- `fr-FR` - French (France)
- `de-DE` - German (Germany)
- `ja-JP` - Japanese (Japan)
- `ko-KR` - Korean (Korea)
- `zh-CN` - Chinese (Simplified, China)
- `zh-TW` - Chinese (Traditional, Taiwan)
- `pt-BR` - Portuguese (Brazil)

## Resources

- [Docusaurus i18n Guide](https://docusaurus.io/docs/i18n/introduction)
- [ISO 639-1 Language Codes](https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes)
- [ISO 3166-1 Country Codes](https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2)

## Questions?

If you have questions about translations, please open a [discussion](https://github.com/thunder-id/thunderid/discussions) or [issue](https://github.com/thunder-id/thunderid/issues).
