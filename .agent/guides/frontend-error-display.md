---
paths:
  - "frontend/**/*.tsx"
  - "frontend/**/*.ts"
---

# Frontend Error Display

Where a failure renders is a product decision, not an implementation detail, and it is the thing most often got wrong in
this codebase. The short version: **failed writes render inline, next to the form or action that failed. Failed reads
render where the missing data was going to go. Toasts are for confirmations, and for failures with nowhere on the page
to live.**

A reviewer applies the same rules, so treat each bullet below as something to flag as well as something to follow.

- A mutation failure renders inline wherever the action has a form or dialog — create wizards, edit pages, and confirm
  dialogs. Put an `<Alert severity="error">` beside the submit control, driven off the mutation's own `error` state. The
  user has to read the reason, correct an input, and retry, and a toast dismisses itself before that. Some messages are
  long and interpolated (e.g. `errors.APP-1039`) and do not fit a snackbar at all.
- Do not put the error toast in the mutation hook. A hook cannot know whether its caller has somewhere to show the
  message, and a hook-level `onError` plus a call-site one is the standard way duplicates get introduced. Hooks keep
  their `onSuccess` toast; failures are the caller's decision.
- Mutations fired straight from a row action, toggle, or menu item may toast, because there is no form to attach to and
  no dialog to keep open. Decide by asking where the user's attention already is and whether they need the text to
  persist while they fix something, not by the HTTP verb.
- Mutation successes stay toasts — `showToast(t('create.success'), 'success')` in the hook's `onSuccess`. That is
  already the pattern in every `use{Create,Update,Delete}*` hook.
- Read failures render in place of the data. If a query is a surface's primary content — a list, a grid, a detail page,
  a tab body — render an error component where that content would have gone, with a way to retry. A toast alongside it
  is fine, but never instead of it: a toast auto-dismisses and leaves the user staring at an empty page with no
  explanation. If the query is secondary and the page is still usable without it (a picker's options, an optional count,
  a background prefetch), a toast on its own is right, because there is nowhere natural to put it inline. React Query
  has no query `onError`, so a query hook cannot toast on its own — reaching for one means adding a render-phase or
  effect watcher on `isError`, which is a signal the error belongs inline.
- Render every read failure with `QueryErrorNotice` from `@thunderid/components` rather than a hand-rolled
  `ListingTable.EmptyState` or `<Alert>`. Use `variant="block"` (the default) for a list, grid, or page body, and
  `variant="inline"` for a tab section or in-card region. It takes the error and `t` and resolves the message itself —
  there is no `description` prop, so `error.message` is not representable through it. Pass `resolveErrorMessage` for a
  feature-specific resolver (e.g. `getUserErrorMessage`); it defaults to `getErrorMessage`. Pass `onRetry` for the
  default refresh button and `action` for a second action stacked below it (e.g. an edit page's "Back to X"); with no
  `onRetry`, `action` renders alone.
- Clear an inline error as soon as the user acts on it. Any change to the form invalidates the message, so a
  duplicate-name error must disappear when the name is edited rather than sitting there contradicting the field. Reset
  the mutation (`mutation.reset()`) or the local error state from whatever the form's field-change path is, and on
  cancel or reset too. A stale error next to a now-valid form is worse than no error.
- Never surface server-returned error text. No `error.message`, no `response.data.message`, and no `description`
  `defaultValue` from the error envelope. That text is unlocalized, is written for API consumers rather than end users,
  and leaks backend and HTTP wording into the product. This applies to read failures exactly as much as to writes. Note
  that `error.message ?? t('fallback')` never reaches the fallback — `Error.message` is always a string — so that idiom
  silently guarantees the raw text wins.
- Resolve every message through the error catalog: `getErrorMessage` from `@thunderid/utils`, or a feature-specific
  wrapper such as `getApplicationErrorMessage`, so a backend error code maps to `errors.<CODE>` with a generic fallback
  key. Pass the fallback's default string too, per the i18n Fallback Values rule below, so a missing key degrades to
  readable English instead of rendering `create.error` at the user. Codes shared across services (e.g. `SSE-4030` for an
  authorization denial) resolve from `common:errors.<CODE>` when the feature namespace has no entry, so they need
  mapping only once. This requires `t` to forward an explicit `ns:` prefix, so a per-call-site namespace wrapper must
  pass keys that already carry one straight through.
- Keep one surface per failure. TanStack Query fires both `useMutation({onError})` and the per-call
  `mutate(vars, {onError})`, so defining an error surface in both is how duplicates appear. Two toasts for one failure
  are worse than visibly duplicated: the second silently replaces the first, since `ToastProvider` holds only one
  message at a time, so the user may never see the specific one.
- An error from a dialog belongs in that dialog. If the action was fired from a modal, close the modal only in the
  success path and render the `<Alert>` inside it, next to the confirm button. Closing on submit, or rendering the error
  on the surface behind the modal, leaves the user looking at a re-enabled dialog with no explanation. When the parent
  component owns the mutation because it also serves a non-dialog path, pass the error into the dialog as a prop rather
  than rendering it in the parent. Never discard the user's input — a selection, a filled form — on failure; only clear
  it once the mutation succeeds, or the error costs them the whole task.

## Related

- Every message resolved here still needs an i18n fallback default. See the i18n rule in `frontend/AGENTS.md`.
