---
paths:
  - "frontend/**/pages/*EditPage.tsx"
  - "frontend/**/components/edit-*/**/*.tsx"
---

# Frontend Edit Pages: the Reset-Key Contract

An Edit page tracks changes in a local `editedX` overlay and exposes Save/Reset through `UnsavedChangesBar`. Any child
section that keeps its **own** state derived from server data will not re-derive when the page clears that overlay:
`useState` seeded from props, `react-hook-form` `defaultValues`, a locally managed list such as redirect URIs, or an
in-place text-edit draft. Calling `setEditedX({})` alone leaves the section showing stale edits even though the
unsaved-changes bar has disappeared.

The fix is to force those children to remount by changing their React `key`.

## The parent side

Canonical implementation: `frontend/apps/console/src/features/applications/pages/ApplicationEditPage.tsx`.

1. Hold the key: `const [sectionResetKey, setSectionResetKey] = useState(0);`
2. Bump it in `onReset`, alongside clearing the overlay and every validation flag:
   ```tsx
   onReset={() => {
     updateApplication.reset();
     setEditedApp({});
     setAccessSettingsInvalid(false);   // ...and every other validation flag
     setSectionResetKey((key) => key + 1);
   }}
   ```
3. Bump it again after a successful save, but **only once `refetch()` has resolved**:
   ```tsx
   setEditedApp({});
   await refetch();
   // Bumped only after refetch resolves to prevent stale data being passed to the remounted sections.
   setSectionResetKey((key) => key + 1);
   ```
   Bumping before the refetch resolves remounts the children against stale server data. Nothing enforces this ordering
   except that comment, so preserve it.
4. Pass it to every child section that holds such state.

## The child side

The wrapper's whole job is to apply the key, as in
`frontend/apps/console/src/features/applications/components/edit-application/access/EditAccessSettings.tsx`:

```tsx
<AccessSection key={sectionResetKey} ... />
```

Alternatively use it as a `useEffect` dependency that re-syncs local state. A section may also forward the raw key to a
child of its own (see `.../edit-agent/tokens/EditTokensSettings.tsx`).

## Five ways this breaks

1. **Two names for one pattern.** `sectionResetKey` in applications and agents, `attributesResetKey` in
   `configure-users`' `UserEditPage.tsx`. A grep for one misses the other, so search for `ResetKey`.
2. **It is silently optional.** Children declare `sectionResetKey = 0` as a default, so forgetting to pass it compiles
   and renders fine. The bug only appears when a user hits Reset and sees stale values.
3. **It is order-dependent.** See the `refetch()` rule above.
4. **Two reset surfaces must stay in sync.** `onReset` and the post-save path each clear state and bump independently,
   and `onReset` must also clear every validation flag. Adding a sixth flag means touching three places.
5. **It is not universal, and nothing tells you when that changes.** `GroupEditPage.tsx` and `RoleEditPage.tsx` have no
   reset key, which is currently correct because they have no local-state sections. The moment someone adds one, the
   page needs the whole contract.

## Flagging this in review

If an Edit page renders child sections that hold server-derived local state but no reset key reaches them, that is a
defect: Save/Reset will leave stale local state behind. On the child side, a component with such state that neither
receives nor uses a reset key is the same defect. Do not flag purely presentational components, or components whose
only state is UI-only (an "is editing" toggle, dropdown open/closed) with nothing server-derived to go stale. Do not
flag pages where all editing flows through the single `editedX` object with no per-section local state.
