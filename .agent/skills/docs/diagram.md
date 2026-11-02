# Authoring Mermaid Diagrams

The ThunderID Mermaid theme is applied **globally and automatically**. Every ```mermaid block
in the docs already gets the house style, Plus Jakarta Sans, soft rounded cards, a calm float
shadow, hairline borders, pill edge-label chips, clean curves, and full light/dark theming,
with no styling from you.

So this reference is not about *how diagrams look*. It is about getting the diagram **right**:
choosing the correct type, gathering the real content from the user instead of inventing it,
and constructing it with the conventions the theme expects.

Work in four steps: **decide the type → gather the content → construct → confirm.**

---

## Step 1: Decide what the diagram is for, then pick the type

Do not default to a flowchart. Match the diagram's job to its type:

| The diagram shows… | Use |
|---|---|
| A flow between parties over time (an OAuth sign-in, an API handshake, a webhook round trip) | **sequence diagram** |
| A decision, branching, or a lifecycle with outcomes | **flowchart** |
| How components / systems fit together (an architecture) | **flowchart with subgraphs** (see the architecture note) |
| A state machine (session, token, or resource lifecycle) | **stateDiagram-v2** |
| Anything else, or you are unsure | **stop and ask the user** what they want to convey |

If the user's request doesn't make the type obvious, ask before drawing.

## Step 2: Gather the real content first, never invent it

A diagram that is pretty but wrong is worse than no diagram. **You usually do not have the real
steps or components, the user does.** Before constructing, collect the specifics, and if they are
missing from the request, **ask the user** with targeted questions. Verify factual details
(endpoints, parameters, component names) against `api/*.yaml` and the code where you can, see
`tech.md`.

What to gather, by type:

- **Flow (sequence):** the participants in the order they first act, and *every step* as
  `sender -> receiver: message`, marking which are responses and which are internal self-steps.
  If the user gives a vague ask ("show the login flow"), get the ordered, step-by-step detail
  from them or from the spec before drawing, do not guess the protocol.
- **Decision / lifecycle (flowchart):** the states or steps, and for each decision, its exact
  branches and outcomes.
- **Architecture (structure):** there is no standard shape here, so **ask the user directly**:
  what are the components, how do they group, what connects to what, and in which direction?
  Do not force an architecture into the flow format.
- **State machine:** the states and the events that transition between them.

Only proceed once the content is real and confirmed.

## Step 3: Construct

Styling is automatic. Follow the per-type conventions below.

### Sequence diagrams (flows)

Match this shape:

```
sequenceDiagram
    autonumber
    participant U as User
    participant App as App
    participant AS as ThunderID
    participant API as Your API

    U->>App: Click sign in
    App->>App: Generate the PKCE challenge
    App->>AS: Authorization request to /oauth2/authorize
    AS-->>App: Authorization code
    App->>AS: Exchange code + verifier at /oauth2/token
    AS-->>App: Access, refresh, ID tokens
    App->>API: Call the API with the access token
    API-->>App: Protected resource
```

- Order participants **left to right in the order they first act** (User, App, ThunderID, API).
- Declare short aliases (`participant AS as ThunderID`) so columns stay narrow.
- Always `autonumber`.
- `->>` for a request, `-->>` for a response, a self-message (`AS->>AS: ...`) for an internal step.
- The theme handles participants, lifelines, messages, and the number badges, no styling needed.

**Code-block notes on a step.** To attach a request or snippet to a step, use a `Note` and
start the diagram with this init line:

```
%%{init: {"sequence": {"noteFontFamily": "monospace", "noteFontSize": 12, "noteMargin": 16, "noteAlign": "left"}}}%%
```

It is required: without it Mermaid measures the note in a proportional font but renders it in
monospace, so long lines overflow the box (the global config's `sequence` settings do not reach
Mermaid here, only an inline `%%{init}` does). Then write the note with one short line per param:

```
Note right of App: POST /oauth2/token<br/>grant_type=authorization_code<br/>code=CODE<br/>code_verifier=CODE_VERIFIER
```

The note renders as a themed code panel automatically (that styling is global). Keep lines
short, put full copy-paste requests in a fenced code block near the diagram, not the note.

### Flowcharts (decisions, lifecycles, architecture)

**Readability comes first, never ship a diagram that renders tiny.** Mermaid scales the whole
diagram down uniformly to fit the page width, so the wider a diagram is, the smaller its text
becomes. Width is the enemy of legibility. Keep every node readable:

- **Default to top-down (`flowchart TD`).** It grows into vertical space instead of being
  squeezed sideways. Reserve `LR` for genuinely short flows (2-3 nodes) that fit the page width.
- **Keep it narrow.** Short chains, one-line node labels where you can (long multi-line labels
  widen *and* lengthen the diagram), and lean on the compact global `rankSpacing`. Don't bump
  `rankSpacing` for one diagram, it stretches every other one.
- **Two or more independent scenarios: use a separate diagram per scenario, not one diagram
  with several subgraphs.** Disconnected subgraphs lay out side by side, which widens the whole
  thing back into unreadable territory. Two short separate diagrams each render at full size.
- **Check the render** at normal page width, in both themes, before finishing. If the text looks
  small, the diagram is too wide, make it narrower (fewer/shorter nodes, `TD`) or split it.

**Shapes, use only these three:**

| Shape | Syntax | Use for |
|---|---|---|
| Rounded rect | `A[Label]` | steps, most nodes |
| Stadium (pill) | `A([Label])` | start, end, states |
| Diamond | `A{Label}` | decisions only |

Avoid the dated shapes: subroutine `[[ ]]`, hexagon `{{ }}`, circle `(( ))`, cylinder `[( )]`.

**Color, a few semantic roles applied with `:::`:**

Default (no class) is a neutral card, keep most nodes neutral. Reach for a role only to signal
meaning:

| Class | Use for |
|---|---|
| `:::accent` | the one primary / key node |
| `:::success` | a positive end state (cool teal) |
| `:::danger` | destructive or critical (delete, secrets) |
| `:::muted` | de-emphasized or terminal |

These four are theme-derived and re-theme automatically. To add an icon to a role node, use a
separate `class` statement.

**Edge labels:** `A -->|label| B` renders as a pill chip. Keep labels to one or two words. Do
**not** hang many labeled edges that converge on one node, Mermaid cannot space those apart and
the chips overlap.

**Icons (optional):** an opt-in Lucide set lives in `custom.css`: `ic-app`, `ic-user`,
`ic-service`, `ic-datastore`, `ic-key`, `ic-lock`, `ic-globe`, `ic-shield`, `ic-flow`,
`ic-token`, `ic-check`, `ic-alert`. Apply with `class <node> ic-key`. An icon adds width Mermaid
doesn't measure, so use it only on short-label nodes and check the render.

**Architecture specifics:** group related components in `subgraph` blocks and connect groups
with labeled edges. A rich, icon-grid marketing architecture (multiple icons per panel,
gradient hero, bespoke layout) is **not** something Mermaid can produce, that is a hand-built
SVG/React graphic. If the user wants that fidelity, say so rather than approximating it.

### Other types

For a **stateDiagram-v2** (lifecycles as states + transitions) the theme styles it too. For any
type not covered here, confirm the intent with the user and check the render, the theme may need
a small addition in `custom.css`.

## Inline styling

**Prefer the global theme and the role classes**, they keep diagrams consistent and theme-aware
for free. But **inline styling is allowed where the theme genuinely falls short**: a one-off
emphasis, a domain-specific color the roles don't cover, a bespoke diagram. When you reach for
`classDef`, `style`, `linkStyle`, or `%%{init}`:

- **Keep it theme-aware.** A hardcoded hex that reads well in one color mode often fails in the
  other. Pick a color that works on both grounds, or set it knowing it is a single-mode choice.
- **Style the exception, not the world.** Override the one node or edge that needs it, don't
  redefine the palette per diagram.

If a change should apply to *every* diagram, it belongs in the global theme, not inline:

- Visual theme: `docs/src/css/custom.css` (search for the Mermaid section)
- Layout / font / curve / spacing: the `mermaid` block in `docs/docusaurus.config.ts`

## Pitfalls (learned the hard way)

- **Never bold node labels** or set `font-weight` on them. Mermaid measures label width at
  normal weight, so anything heavier overflows and clips the last characters.
- **Theme-vary via theme-scoped blocks only.** A token that differs by mode
  (`[data-theme='light']` / `[data-theme='dark']`) must live in a scoped block, never the shared
  base, or it bleeds across modes and breaks the other one.
- **No glows or heavy effects**, the look is deliberately calm.
- **Don't converge many labeled edges on one node** (see edge labels).
- **Don't lay a long chain out left-to-right (`LR`).** Mermaid shrinks wide diagrams to fit the
  page width, making nodes and text tiny. Use `TD` and the vertical space past a few nodes.
- **Don't pack independent scenarios into one diagram with several subgraphs.** Disconnected
  subgraphs render side by side and the whole diagram scales down tiny. Use one diagram per
  scenario instead.
- **Never `!important` a font-family onto generic Mermaid `text` in CSS.** It also hits the
  element Mermaid uses to *measure* text, so text is measured in one font and rendered in
  another and overflows its box. Set diagram fonts via the Mermaid config or a `%%{init}` block.

## Before you finish

State in one line that you are following `diagram.md`, then confirm: the **type** fits the
diagram's job, the **content is real and verified with the user**, the diagram **renders at a
readable size** (not scaled down tiny, narrow it or split it if wide), flowcharts use only the
three approved shapes and keep most nodes neutral, and any inline styling is a scoped,
theme-aware exception.
