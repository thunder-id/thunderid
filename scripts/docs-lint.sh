#!/usr/bin/env bash
# docs-lint.sh — validate ThunderID documentation files
#
# Usage:
#   ./scripts/docs-lint.sh [file...]
#   With no args: checks all docs/content/**/*.mdx
#   With args: checks only the listed files
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
    # Skip agent skill definitions and repo meta-docs (AGENTS.md, README.md) even
    # if passed explicitly — they're not docs/content pages and don't follow its
    # conventions (frontmatter, <ProductName />, etc.).
    case "$f" in
      "$REPO_ROOT"/.agent/skills/*|"$REPO_ROOT"/.claude/skills/*) continue ;;
      */AGENTS.md|*/README.md) continue ;;
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

# ─── 1. Vale (prose) ──────────────────────────────────────────────────────────

echo "┌─ Vale"
if command -v vale &>/dev/null; then
  # Show all alerts but fail only on error-level
  vale --config "$REPO_ROOT/.vale.ini" "${FILES[@]}" || true
  if ! vale --config "$REPO_ROOT/.vale.ini" --minAlertLevel=error --output=line "${FILES[@]}" &>/dev/null; then
    FAILED=1
    echo "│  ❌ Vale found error-level issues (see above)"
  else
    echo "│  ✅ No Vale errors"
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

  # Verify frontmatter exists
  FIRST_LINE=$(head -1 "$FILE")
  if [[ "$FIRST_LINE" != "---" ]]; then
    echo "│  ❌ $REL — missing frontmatter (file must start with ---)"
    STRUCT_FAILED=1
    continue
  fi

  # Extract frontmatter block (lines between first and second ---)
  FRONTMATTER=$(awk '/^---$/{delim++; if(delim==2) exit; next} delim==1' "$FILE")

  # 2a. title required
  if ! echo "$FRONTMATTER" | grep -qP '^title:'; then
    echo "│  ❌ $REL — frontmatter missing 'title'"
    STRUCT_FAILED=1
  fi

  # 2b. description required, no bad prefix; length is a soft floor/ceiling, not a hard range
  DESC_LINE=$(echo "$FRONTMATTER" | grep -P '^description:' | head -1)
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
    if echo "$DESC" | grep -qiP '^(This page|This guide|This document|This section)'; then
      echo "│  ❌ $REL — description must not start with 'This page/guide/document/section'"
      STRUCT_FAILED=1
    fi
  fi

  # 2c. toc_progress: quickstart ↔ <Stepper> consistency
  HAS_STEPPER=$(grep -cP '<Stepper\b' "$FILE" 2>/dev/null || echo 0)
  HAS_TOC=$(echo "$FRONTMATTER" | grep -cP '^toc_progress:\s*quickstart' 2>/dev/null || echo 0)
  if [[ "$HAS_STEPPER" -gt 0 && "$HAS_TOC" -eq 0 ]]; then
    echo "│  ❌ $REL — has <Stepper> but missing 'toc_progress: quickstart' in frontmatter"
    STRUCT_FAILED=1
  elif [[ "$HAS_STEPPER" -eq 0 && "$HAS_TOC" -gt 0 ]]; then
    echo "│  ❌ $REL — has 'toc_progress: quickstart' but no <Stepper> found"
    STRUCT_FAILED=1
  fi

  # 2d. Unlabeled code blocks (warning only)
  while IFS= read -r match; do
    LINENO=$(echo "$match" | cut -d: -f1)
    echo "│  ⚠ $REL:$LINENO — unlabeled code block (add a language tag)"
  done < <(grep -nP '^```\s*$' "$FILE" 2>/dev/null || true)

  # 2e. Absolute internal links
  while IFS= read -r match; do
    LINENO=$(echo "$match" | cut -d: -f1)
    echo "│  ❌ $REL:$LINENO — absolute internal link (use a relative path instead of /docs/...)"
    STRUCT_FAILED=1
  done < <(grep -nP '\[.*?\]\(/docs/' "$FILE" 2>/dev/null || true)

  # 2f. Stepper stepNode/as attribute mismatch
  STEPPER_LINE=$(grep -nP '<Stepper\b' "$FILE" 2>/dev/null | head -1 || true)
  if [[ -n "$STEPPER_LINE" ]]; then
    STEP_NODE=$(echo "$STEPPER_LINE" | grep -oP 'stepNode="\K[^"]+' || true)
    AS_VAL=$(echo "$STEPPER_LINE" | grep -oP '\bas="\K[^"]+' || true)
    if [[ -n "$STEP_NODE" && -n "$AS_VAL" && "$STEP_NODE" != "$AS_VAL" ]]; then
      LINENO=$(echo "$STEPPER_LINE" | cut -d: -f1)
      echo "│  ❌ $REL:$LINENO — Stepper stepNode=\"$STEP_NODE\" does not match as=\"$AS_VAL\""
      STRUCT_FAILED=1
    fi
  fi

  # 2g. Empty image alt text (warning only)
  while IFS= read -r match; do
    LINENO=$(echo "$match" | cut -d: -f1)
    echo "│  ⚠ $REL:$LINENO — image with empty alt text"
  done < <(grep -nP '!\[\]\(' "$FILE" 2>/dev/null || true)

  # 2h. Line dividers (--- in body, outside frontmatter and code blocks)
  while IFS= read -r lineno; do
    echo "│  ❌ $REL:$lineno — line divider (---) in body; use a section heading instead"
    STRUCT_FAILED=1
  done < <(awk 'BEGIN{d=0;c=0} /^```/{c=!c;next} c{next} /^---$/{d++;if(d>2)print NR;next}' "$FILE" 2>/dev/null || true)

  # 2i. Heading hierarchy (no downward skips, e.g. H1 -> H3), outside frontmatter and code blocks
  while IFS= read -r match; do
    LINENO="${match%%:*}"
    SKIP="${match#*:}"
    echo "│  ❌ $REL:$LINENO — heading hierarchy skip ($SKIP)"
    STRUCT_FAILED=1
  done < <(awk '
    BEGIN{c=0;d=0;prev=0}
    /^---$/{d++;next}
    d<2{next}
    /^```/{c=!c;next}
    c{next}
    /^#{1,6}[ \t]/{
      match($0, /^#+/)
      level=RLENGTH
      if (prev>0 && level>prev+1) printf "%d:H%d -> H%d\n", NR, prev, level
      prev=level
    }
  ' "$FILE" 2>/dev/null || true)

  # 2j. Hardcoded "ThunderID" in prose (must use <ProductName /> instead), outside frontmatter and code blocks
  while IFS= read -r lineno; do
    echo "│  ❌ $REL:$lineno — hardcoded 'ThunderID' in prose; use <ProductName /> instead"
    STRUCT_FAILED=1
  done < <(awk '
    BEGIN{c=0;d=0}
    /^---$/{d++;next}
    d<2{next}
    /^```/{c=!c;next}
    c{next}
    /ThunderID/{print NR}
  ' "$FILE" 2>/dev/null || true)

done

if [[ $STRUCT_FAILED -eq 0 ]]; then
  echo "│  ✅ All structural checks passed"
fi
[[ $STRUCT_FAILED -eq 1 ]] && FAILED=1
echo ""

# ─── 3. Sidebar orphan check ──────────────────────────────────────────────────
# Runs on the full content tree regardless of which files were passed —
# orphan detection requires the complete picture.

echo "┌─ Sidebar orphans"

ALLOWLIST="$REPO_ROOT/.orphan-allowlist"

ALL_CONTENT_IDS=$(find "$REPO_ROOT/docs/content" -name '*.mdx' -print0 2>/dev/null \
  | xargs -0 -I{} bash -c 'f="{}"; f="${f#'"$REPO_ROOT"'/docs/content/}"; echo "${f%.mdx}"' \
  | sort)

SIDEBAR_FILES=("$REPO_ROOT/docs/sidebars.ts")
while IFS= read -r -d '' f; do SIDEBAR_FILES+=("$f"); done \
  < <(find "$REPO_ROOT/docs/content/sdks" -name 'sidebar.ts' -print0 2>/dev/null)

SIDEBAR_IDS=$(grep -horP "(?<=\bid: ')[^']+" "${SIDEBAR_FILES[@]}" 2>/dev/null | sort -u)

ALL_ORPHANS=$(comm -23 \
  <(echo "$ALL_CONTENT_IDS") \
  <(echo "$SIDEBAR_IDS") 2>/dev/null || true)

if [[ -z "$ALL_ORPHANS" ]]; then
  echo "│  ✅ All pages registered in a sidebar"
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
