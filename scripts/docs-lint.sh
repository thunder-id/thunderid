#!/usr/bin/env bash
# docs-lint.sh — validate ThunderID documentation files
#
# Usage:
#   ./scripts/docs-lint.sh [file...]
#   With no args: checks all docs/content/**/*.mdx
#   With args: checks only the listed files
#
# Env:
#   DOCS_LINT_BASE_REF — if set to a git ref (branch/commit), Vale and the
#     line-numbered structural checks only fail on lines actually added or
#     modified since that ref (diffed against the merge-base with HEAD).
#     Pre-existing issues elsewhere in a touched file are left alone. New
#     files are always checked in full. Unset: whole-file checks (legacy
#     behavior).
#
# Runs:
#   1. Vale (prose rules: em/en dashes, inclusive language, sign-in/redirect-URI terminology, H1 title case)
#   2. Structural checks (frontmatter, heading hierarchy, Stepper config, code blocks,
#      ProductName usage, links, alt text, line dividers)
#   3. Sidebar orphan check
#
# Exit codes:
#   0 — all checks passed (warnings may still be printed)
#   1 — one or more errors found

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FAILED=0

# ─── Collect files ────────────────────────────────────────────────────────────

FILES=()
if [[ $# -eq 0 ]]; then
  while IFS= read -r -d '' f; do
    FILES+=("$f")
  done < <(find "$REPO_ROOT/docs/content" -name '*.mdx' -print0 2>/dev/null)
else
  for f in "$@"; do
    [[ "$f" == /* ]] || f="$PWD/$f"
    # Skip agent skill definitions and repo meta-docs (AGENTS.md, README.md,
    # ARCHITECTURE.md) even if passed explicitly — they're not docs/content pages
    # and don't follow its conventions (frontmatter, <ProductName />, etc.).
    case "$f" in
      "$REPO_ROOT"/.agent/skills/*|"$REPO_ROOT"/.claude/skills/*) continue ;;
      */AGENTS.md|*/README.md|*/ARCHITECTURE.md) continue ;;
    esac
    FILES+=("$f")
  done
fi

if [[ ${#FILES[@]} -eq 0 ]]; then
  echo "No .mdx files found."
  exit 0
fi

echo "Checking ${#FILES[@]} file(s)..."
echo ""

# ─── Changed-line scoping ─────────────────────────────────────────────────────
# When DOCS_LINT_BASE_REF is set, errors only count if their line was added or
# modified since the merge-base with that ref. New files (not present at the
# merge-base) are always checked in full.
#
# Tracked via a scratch directory (not associative arrays) so this runs on
# bash 3.2 (the macOS default) as well as the bash 5 CI runner.

LINE_SCOPED=0
MERGE_BASE=""
SCOPE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/docs-lint.XXXXXX")"
trap 'rm -rf "$SCOPE_DIR"' EXIT

if [[ -n "${DOCS_LINT_BASE_REF:-}" ]]; then
  if MERGE_BASE=$(git -C "$REPO_ROOT" merge-base "$DOCS_LINT_BASE_REF" HEAD 2>/dev/null); then
    LINE_SCOPED=1
    for FILE in "${FILES[@]}"; do
      REL="${FILE#"$REPO_ROOT"/}"
      if ! git -C "$REPO_ROOT" cat-file -e "$MERGE_BASE:$REL" 2>/dev/null; then
        mkdir -p "$SCOPE_DIR/new/$(dirname "$REL")"
        touch "$SCOPE_DIR/new/$REL"
        continue
      fi
      mkdir -p "$SCOPE_DIR/lines/$(dirname "$REL")"
      git -C "$REPO_ROOT" diff -U0 --no-color "$MERGE_BASE" -- "$FILE" 2>/dev/null | awk '
        /^@@/ {
          match($0, /\+[0-9]+(,[0-9]+)?/)
          spec = substr($0, RSTART + 1, RLENGTH - 1)
          n = split(spec, parts, ",")
          start = parts[1]
          count = (n > 1) ? parts[2] : 1
          for (i = 0; i < count; i++) print start + i
        }
      ' > "$SCOPE_DIR/lines/$REL"
    done
  else
    echo "⚠ DOCS_LINT_BASE_REF='$DOCS_LINT_BASE_REF' — could not resolve merge-base; falling back to whole-file checks" >&2
  fi
fi

# is_changed_line REL line_no — true if that line should be checked
is_changed_line() {
  [[ $LINE_SCOPED -eq 0 ]] && return 0
  [[ -e "$SCOPE_DIR/new/$1" ]] && return 0
  [[ -f "$SCOPE_DIR/lines/$1" ]] && grep -qxF "$2" "$SCOPE_DIR/lines/$1" 2>/dev/null
}

# ─── 1. Vale (prose) ──────────────────────────────────────────────────────────

echo "┌─ Vale"
if command -v vale &>/dev/null; then
  # Show all alerts but fail only on error-level (scoped to changed lines, if applicable)
  vale --config "$REPO_ROOT/.vale.ini" "${FILES[@]}" || true
  VALE_STRUCT_FAILED=0
  if [[ $LINE_SCOPED -eq 1 ]] && command -v jq &>/dev/null; then
    VALE_JSON=$(vale --config "$REPO_ROOT/.vale.ini" --output=JSON "${FILES[@]}" 2>/dev/null || echo '{}')
    while IFS=$'\t' read -r VFILE VLINE; do
      [[ -z "$VFILE" ]] && continue
      VREL="${VFILE#"$REPO_ROOT"/}"
      if is_changed_line "$VREL" "$VLINE"; then
        VALE_STRUCT_FAILED=1
      fi
    done < <(echo "$VALE_JSON" | jq -r 'to_entries[] | .key as $f | .value[] | select(.Severity=="error") | "\($f)\t\(.Line)"' 2>/dev/null)
  else
    if [[ $LINE_SCOPED -eq 1 ]]; then
      echo "│  ⚠ jq not found — cannot scope Vale to changed lines; falling back to whole-file Vale check"
    fi
    if ! vale --config "$REPO_ROOT/.vale.ini" --minAlertLevel=error --output=line "${FILES[@]}" &>/dev/null; then
      VALE_STRUCT_FAILED=1
    fi
  fi
  if [[ $VALE_STRUCT_FAILED -eq 1 ]]; then
    FAILED=1
    if [[ $LINE_SCOPED -eq 1 ]]; then
      echo "│  ❌ Vale found error-level issues on changed lines (see above)"
    else
      echo "│  ❌ Vale found error-level issues (see above)"
    fi
  else
    if [[ $LINE_SCOPED -eq 1 ]]; then
      echo "│  ✅ No Vale errors on changed lines"
    else
      echo "│  ✅ No Vale errors"
    fi
  fi
else
  echo "│  ⚠ vale not found — install with: brew install vale && vale sync"
fi
echo ""

# ─── 2. Structural checks ─────────────────────────────────────────────────────

echo "┌─ Structural"
STRUCT_FAILED=0

for FILE in "${FILES[@]}"; do
  REL="${FILE#"$REPO_ROOT"/}"

  # Does the diff touch the frontmatter block? Gates the file-level checks
  # (2a-2c) below when line-scoped — an untouched frontmatter isn't this PR's
  # concern.
  FRONTMATTER_TOUCHED=1
  if [[ $LINE_SCOPED -eq 1 && ! -e "$SCOPE_DIR/new/$REL" ]]; then
    FRONTMATTER_END=$(awk '/^---$/{c++; if(c==2){print NR; exit}}' "$FILE")
    [[ -z "$FRONTMATTER_END" ]] && FRONTMATTER_END=1
    FRONTMATTER_TOUCHED=0
    for ((I = 1; I <= FRONTMATTER_END; I++)); do
      if is_changed_line "$REL" "$I"; then
        FRONTMATTER_TOUCHED=1
        break
      fi
    done
  fi

  # Verify frontmatter exists
  FIRST_LINE=$(head -1 "$FILE")
  if [[ "$FIRST_LINE" != "---" ]]; then
    if [[ $FRONTMATTER_TOUCHED -eq 1 ]]; then
      echo "│  ❌ $REL — missing frontmatter (file must start with ---)"
      STRUCT_FAILED=1
    fi
    continue
  fi

  # Extract frontmatter block (lines between first and second ---)
  FRONTMATTER=$(awk '/^---$/{delim++; if(delim==2) exit; next} delim==1' "$FILE")

  if [[ $FRONTMATTER_TOUCHED -eq 1 ]]; then
    # 2a. title required
    if ! echo "$FRONTMATTER" | grep -qE '^title:'; then
      echo "│  ❌ $REL — frontmatter missing 'title'"
      STRUCT_FAILED=1
    fi

    # 2b. description required, no bad prefix; length is a soft floor/ceiling, not a hard range
    DESC_LINE=$(echo "$FRONTMATTER" | grep -E '^description:' | head -1)
    if [[ -z "$DESC_LINE" ]]; then
      echo "│  ❌ $REL — frontmatter missing 'description'"
      STRUCT_FAILED=1
    else
      DESC=$(echo "$DESC_LINE" | sed 's/^description:[[:space:]]*//' | sed "s/^[\"']//;s/[\"']$//")
      DESC_LEN=${#DESC}
      if [[ $DESC_LEN -lt 70 ]]; then
        echo "│  ⚠ $REL — description is $DESC_LEN chars (likely too thin; Google may ignore it and pick its own snippet)"
      elif [[ $DESC_LEN -gt 200 ]]; then
        echo "│  ⚠ $REL — description is $DESC_LEN chars (likely to get truncated in search results)"
      fi
      if echo "$DESC" | grep -qiE '^(This page|This guide|This document|This section)'; then
        echo "│  ❌ $REL — description must not start with 'This page/guide/document/section'"
        STRUCT_FAILED=1
      fi
    fi

    # 2c. docType required, must be one of the recognized values
    DOCTYPE=$(echo "$FRONTMATTER" | grep -E '^docType:' | head -1 | sed 's/^docType:[[:space:]]*//' | sed "s/^[\"']//;s/[\"']$//;s/[[:space:]]*$//")
    if [[ -z "$DOCTYPE" ]]; then
      echo "│  ❌ $REL — frontmatter missing 'docType'"
      STRUCT_FAILED=1
    elif [[ ! "$DOCTYPE" =~ ^(quickstart|guide|concept|reference|use-case|community)$ ]]; then
      echo "│  ❌ $REL — docType '$DOCTYPE' is not one of: quickstart, guide, concept, reference, use-case, community"
      STRUCT_FAILED=1
    fi

  fi

  # 2d. Unlabeled code blocks (warning only)
  while IFS= read -r line_no; do
    is_changed_line "$REL" "$line_no" && echo "│  ⚠ $REL:$line_no — unlabeled code block (add a language tag)"
  done < <(
    awk '
      BEGIN { in_fence=0 }
      /^[ \t]*```[^`]*[ \t]*$/ {
        if (in_fence) {
          in_fence=0
          next
        }

        if ($0 ~ /^[ \t]*```[ \t]*$/) {
          print NR
        }

        in_fence=1
        next
      }
    ' "$FILE" 2>/dev/null || true
  )

  # 2d-2. Mislabeled closing code fences (error)
  while IFS= read -r line_no; do
    if is_changed_line "$REL" "$line_no"; then
      echo "│  ❌ $REL:$line_no — labeled closing code fence (use plain triple backticks to close fenced blocks)"
      STRUCT_FAILED=1
    fi
  done < <(
    awk '
      function opens_fence(line,    s, i, c, run) {
        s=line
        sub(/^[ ]{0,3}/, "", s)
        c=substr(s,1,1)
        if (c!="`" && c!="~") return 0
        i=1
        while (substr(s,i,1)==c) i++
        run=i-1
        if (run<3) return 0
        open_ch=c
        open_count=run
        return 1
      }

      function closes_fence(line,    s, i, run, rest) {
        s=line
        sub(/^[ ]{0,3}/, "", s)
        if (substr(s,1,1)!=open_ch) return 0
        i=1
        while (substr(s,i,1)==open_ch) i++
        run=i-1
        if (run<open_count) return 0
        rest=substr(s, i)
        return (rest ~ /^[ \t]*$/)
      }

      function is_bad_closer(line,    s) {
        if (open_ch!="`" || open_count!=3) return 0
        s=line
        sub(/^[ ]{0,3}/, "", s)
        return (s ~ /^```[A-Za-z0-9_-]+[ \t]*$/)
      }

      BEGIN { in_fence=0; open_ch=""; open_count=0 }
      {
        if (!in_fence) {
          if (opens_fence($0)) in_fence=1
          next
        }

        if (closes_fence($0)) {
          in_fence=0
          open_ch=""
          open_count=0
          next
        }

        if (is_bad_closer($0)) {
          print NR
          in_fence=0
          open_ch=""
          open_count=0
        }
      }
    ' "$FILE" 2>/dev/null || true
  )

  # 2e. Absolute internal links (Markdown [text](/docs/...) and JSX/HTML href="/docs/...").
  # Excludes /docs/next/releases — that one has no docs/content page behind it; it mirrors
  # docs/docusaurus.product.config.ts's releasesUrl, a site route outside the content tree.
  while IFS= read -r match; do
    line_no=$(echo "$match" | cut -d: -f1)
    if is_changed_line "$REL" "$line_no"; then
      echo "│  ❌ $REL:$line_no — absolute internal link (use a relative path instead of /docs/...)"
      STRUCT_FAILED=1
    fi
  done < <(grep -nE '\[[^]]*\]\(/docs/|href=["'"'"']/docs/' "$FILE" 2>/dev/null | grep -vE '/docs/next/releases' || true)

  # 2g. Empty image alt text (warning only)
  while IFS= read -r match; do
    line_no=$(echo "$match" | cut -d: -f1)
    is_changed_line "$REL" "$line_no" && echo "│  ⚠ $REL:$line_no — image with empty alt text"
  done < <(grep -nP '!\[\]\(' "$FILE" 2>/dev/null || true)

  # 2h. Line dividers (--- in body, outside frontmatter and code blocks)
  while IFS= read -r lineno; do
    if is_changed_line "$REL" "$lineno"; then
      echo "│  ❌ $REL:$lineno — line divider (---) in body; use a section heading instead"
      STRUCT_FAILED=1
    fi
  done < <(awk 'BEGIN{d=0;fence=0;cb=0} /^[ \t]*```/{fence=!fence;next} fence{next} /<CodeBlock[ \t>]/{cb=1;next} /<\/CodeBlock>/{cb=0;next} cb{next} /^---$/{d++;if(d>2)print NR;next}' "$FILE" 2>/dev/null || true)

  # 2i. Heading hierarchy (no downward skips, e.g. H1 -> H3), outside frontmatter and code blocks
  while IFS= read -r match; do
    line_no="${match%%:*}"
    SKIP="${match#*:}"
    if is_changed_line "$REL" "$line_no"; then
      echo "│  ❌ $REL:$line_no — heading hierarchy skip ($SKIP)"
      STRUCT_FAILED=1
    fi
  done < <(awk '
    BEGIN{fence=0;cb=0;d=0;prev=0}
    /^---$/{d++;next}
    d<2{next}
    /^[ \t]*```/{fence=!fence;next}
    fence{next}
    /<CodeBlock[ \t>]/{cb=1;next}
    /<\/CodeBlock>/{cb=0;next}
    cb{next}
    /^#{1,6}[ \t]/{
      match($0, /^#+/)
      level=RLENGTH
      if (prev>0 && level>prev+1) printf "%d:H%d -> H%d\n", NR, prev, level
      prev=level
    }
  ' "$FILE" 2>/dev/null || true)

  # 2j. Hardcoded "ThunderID" in prose (must use <ProductName /> instead), outside frontmatter,
  # code blocks, and inline code spans (code identifiers like `ThunderIDClient` are allowed —
  # see AGENTS.md's Product Name Rules exception for code identifiers)
  while IFS= read -r lineno; do
    if is_changed_line "$REL" "$lineno"; then
      echo "│  ❌ $REL:$lineno — hardcoded 'ThunderID' in prose; use <ProductName /> instead"
      STRUCT_FAILED=1
    fi
  done < <(awk '
    BEGIN{fence=0;cb=0;d=0}
    /^---$/{d++;next}
    d<2{next}
    /^[ \t]*```/{fence=!fence;next}
    fence{next}
    /<CodeBlock[ \t>]/{cb=1;next}
    /<\/CodeBlock>/{cb=0;next}
    cb{next}
    {stripped=$0; gsub(/`[^`]*`/, "", stripped); gsub(/<[A-Z][A-Za-z0-9]*[^>]*\/?>/, "", stripped); gsub(/<\/[A-Z][A-Za-z0-9]*>/, "", stripped)}
    stripped ~ /ThunderID/{print NR}
  ' "$FILE" 2>/dev/null || true)

done

if [[ $STRUCT_FAILED -eq 0 ]]; then
  echo "│  ✅ All structural checks passed"
fi
[[ $STRUCT_FAILED -eq 1 ]] && FAILED=1
echo ""

# ─── 3. Sidebar orphan check ──────────────────────────────────────────────────
# Orphan detection itself runs on the full content tree regardless of which
# files were passed — it requires the complete picture of what's registered.
# But when line-scoped, only pages newly added by this PR are held to it;
# pre-existing orphans elsewhere in the tree aren't this PR's concern.

echo "┌─ Sidebar orphans"

ALLOWLIST="$REPO_ROOT/.orphan-allowlist"

ALL_CONTENT_IDS=$(find "$REPO_ROOT/docs/content" -name '*.mdx' -print0 2>/dev/null \
  | xargs -0 -I{} bash -c 'f="{}"; f="${f#'"$REPO_ROOT"'/docs/content/}"; echo "${f%.mdx}"' \
  | sort)

SIDEBAR_FILES=("$REPO_ROOT/docs/sidebars.ts")
while IFS= read -r -d '' f; do SIDEBAR_FILES+=("$f"); done \
  < <(find "$REPO_ROOT/docs/content/sdks" -name 'sidebar.ts' -print0 2>/dev/null)

SIDEBAR_IDS=$(grep -horE "id: '[^']+'" "${SIDEBAR_FILES[@]}" 2>/dev/null | sed -E "s/.*id: '([^']+)'.*/\1/" | sort -u)

ALL_ORPHANS=$(comm -23 \
  <(echo "$ALL_CONTENT_IDS") \
  <(echo "$SIDEBAR_IDS") 2>/dev/null || true)

if [[ $LINE_SCOPED -eq 1 ]]; then
  NEW_ORPHANS=()
  while IFS= read -r id; do
    [[ -z "$id" ]] && continue
    [[ -e "$SCOPE_DIR/new/docs/content/$id.mdx" ]] && NEW_ORPHANS+=("$id")
  done <<< "$ALL_ORPHANS"
  ALL_ORPHANS=""
  [[ ${#NEW_ORPHANS[@]} -gt 0 ]] && ALL_ORPHANS=$(printf '%s\n' "${NEW_ORPHANS[@]}")
fi

if [[ -z "$ALL_ORPHANS" ]]; then
  if [[ $LINE_SCOPED -eq 1 ]]; then
    echo "│  ✅ No new pages orphaned from a sidebar"
  else
    echo "│  ✅ All pages registered in a sidebar"
  fi
else
  ALLOWED_IDS=""
  if [[ -f "$ALLOWLIST" ]]; then
    # Strip comments and inline comments, then sort
    ALLOWED_IDS=$(grep -v '^\s*#' "$ALLOWLIST" | grep -v '^\s*$' \
      | sed 's/#.*//' | awk '{$1=$1; print}' | sort)
  fi

  ORPHAN_FAILED=0
  while IFS= read -r id; do
    [[ -z "$id" ]] && continue
    if echo "$ALLOWED_IDS" | grep -qxF "$id"; then
      echo "│  ⚠ $id — not in any sidebar (known; add to sidebars.ts to clear)"
    else
      echo "│  ❌ $id — not registered in docs/sidebars.ts; add it before merging"
      ORPHAN_FAILED=1
    fi
  done <<< "$ALL_ORPHANS"

  [[ $ORPHAN_FAILED -eq 1 ]] && FAILED=1
fi
echo ""

# ─── Summary ──────────────────────────────────────────────────────────────────

if [[ $FAILED -eq 0 ]]; then
  echo "✅  docs-lint passed"
else
  echo "❌  docs-lint failed — fix errors above before merging"
  exit 1
fi
