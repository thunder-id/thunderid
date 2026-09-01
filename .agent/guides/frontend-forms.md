---
paths:
  - "frontend/**/components/**/*Section.tsx"
  - "frontend/**/pages/*CreatePage.tsx"
  - "frontend/**/pages/*AddPage.tsx"
---

# Frontend Forms

`react-hook-form` is used here for **validation only**, not as the value store. The page owns the values and the save
button; the section owns validity. Copying a conventional RHF tutorial into this codebase produces a form that does not
participate in the Edit page's Save/Reset contract.

Canonical example:
`frontend/apps/console/src/features/applications/components/edit-application/access/AccessSection.tsx`.

## The shape

```tsx
const accessSchema = z.object({
  url: z.string().url('Please enter a valid URL').or(z.literal('')).optional(),
});
type AccessFormData = z.infer<typeof accessSchema>;

const {
  control,
  trigger,
  formState: {errors},
} = useForm<AccessFormData>({
  resolver: zodResolver(accessSchema),
  mode: 'onChange',
  defaultValues: {url: editedApp.url ?? application.url ?? ''},
});

// Validate default values on mount so stale validation state doesn't survive a remount.
useEffect(() => {
  void trigger();
}, [trigger]);

// Report validity upward; the parent page owns the save button.
useEffect(() => {
  onValidationChange?.(!!errors.url);
}, [errors.url, onValidationChange]);
```

Points that differ from ordinary RHF usage, each load-bearing:

- The zod schema is declared **inside the component body**, because it often closes over fetched data (option lists,
  translations).
- `mode: 'onChange'`, so validity is live rather than on submit.
- Destructure only `control`, `trigger`, and `formState.errors`. There is **no `handleSubmit` and no `register`** —
  submission belongs to the page.
- `defaultValues` seed from the overlay first, then the server value: `editedX.field ?? serverValue ?? ''`.
- The mount-time `trigger()` matters because these sections get remounted by the reset-key mechanism. Without it, a
  remounted section starts with empty validation state and the page's save button can be wrongly enabled. See
  `.agent/guides/frontend-edit-pages.md`.
- Every field is a `<Controller>` whose `onChange` fires **twice**: `field.onChange(e)` to drive validation, and
  `onFieldChange('url', e.target.value)` to lift the value to the page. Omitting the second means the user's edit never
  reaches the payload; omitting the first means validation never runs.

## Markup shape

```tsx
<FormControl fullWidth>
  <FormLabel htmlFor={id}>{t('...label', 'Label')}</FormLabel>
  <TextField
    id={id}
    error={!!errors.url}
    helperText={errors.url?.message ?? t('...hint', 'Hint text')}
  />
</FormControl>
```

The `FormLabel htmlFor` / `TextField id` pairing is the accessibility contract and nothing enforces it, so check it.

## The `??` trap

`editedX.field ?? original.field` cannot express "the user cleared this field", because a deliberate empty value falls
through to the server value. Where clearing is meaningful, test for presence instead:

```tsx
// 'description' in editedGroup means the user has touched the field; otherwise fall back to the server value.
const description = 'description' in editedGroup ? editedGroup.description : group.description;
```

See `frontend/packages/configure-groups/src/pages/GroupEditPage.tsx`. The `??` form is much more common in the codebase,
so it is the easy thing to copy and the wrong thing for any clearable field. Note that `isEqualIgnoringEmpty` from
`@thunderid/utils` fixes the *dirty-check* side of this, not the *read* side.

## Known wart

Existing zod messages are hardcoded English (`z.string().url('Please enter a valid URL')`) and bypass i18n. Do not
propagate that in new code: resolve the message through `t()` with a fallback default, per the i18n rule in
`frontend/AGENTS.md`.

## Flagging this in review

Flag: a `<Controller>` whose `onChange` does not both validate and lift the value; a section using `handleSubmit`; a
missing mount-time `trigger()` in a section that can be remounted; a missing `onValidationChange` where the parent owns
the save button; a `FormLabel` with no matching `htmlFor`/`id`; a `??` fallback on a field the user can legitimately
clear; and a new hardcoded English validation message.
