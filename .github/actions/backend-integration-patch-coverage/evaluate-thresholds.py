#!/usr/bin/env python3
"""Applies per-package coverage thresholds to diff-cover's per-file results.

diff-cover enforces one global --fail-under, which cannot express "oauth2/dpop is
held to 90% but a formatting helper is not". It does report per-file covered and
uncovered line numbers in its JSON output though, so the per-package verdict is
derived here instead: group the changed lines by Go package, resolve that
package's threshold, and compare.

Resolution, per package. Applicable tags are the config's `tags` entries whose
name is a label on the pull request.

  If any tag applies, the tag layer decides on its own:
    1. Tags naming the package win, highest value across them. Tag defaults are
       not consulted while any tag names it.
    2. Otherwise the highest `default` among the applicable tags applies.

  The base layer is used only when no tag applies, or when the applicable tags
  neither name the package nor set a default:
    3. The base `packages` entry, longest matching prefix.
    4. Otherwise the global `default`.

Longest matching prefix wins in every package map, so a parent path's threshold is
inherited by its subpackages.

Usage:
  evaluate-thresholds.py <diff-cover.json> <config.yml> <labels-json> <summary-out>

Exits 0 when every non-exempt package meets its threshold, 1 otherwise.
"""

import json
import math
import os
import sys

import yaml


def load_labels(raw):
    """Accepts a JSON array, as toJSON() emits, or a comma-separated list."""
    raw = (raw or "").strip()
    if not raw:
        return []
    if raw.startswith("["):
        return [str(x) for x in json.loads(raw)]
    return [part.strip() for part in raw.split(",") if part.strip()]


def package_of(path):
    """A Go package is its directory."""
    return os.path.dirname(path)


def longest_prefix(entries, package):
    """Returns (key, value) for the longest matching package prefix, or None."""
    match = None
    for key in entries or {}:
        trimmed = key.rstrip("/")
        if package == trimmed or package.startswith(trimmed + "/"):
            if match is None or len(trimmed) > len(match):
                match = trimmed
    if match is None:
        return None
    # Return the original key so provenance quotes what the file actually says.
    for key in entries:
        if key.rstrip("/") == match:
            return key, entries[key]
    return None


def inherited_suffix(key, package):
    return "" if key.rstrip("/") == package else f", inherited from `{key}`"


def resolve(package, config, labels):
    """Returns (threshold, provenance) for one package."""
    tags = config.get("tags") or {}
    applicable = [t for t in labels if t in tags]

    # Tier 1: applicable tags that name the package. Highest wins, and tag
    # defaults are deliberately not consulted while any tag names it.
    named = []
    for tag in applicable:
        hit = longest_prefix((tags[tag] or {}).get("packages"), package)
        if hit is not None:
            key, value = hit
            named.append((value, f"tag `{tag}`{inherited_suffix(key, package)}"))
    if named:
        return max(named, key=lambda pair: pair[0])

    # Tier 2: the highest default among the applicable tags.
    defaults = [
        ((tags[tag] or {}).get("default"), f"tag `{tag}` default")
        for tag in applicable
        if (tags[tag] or {}).get("default") is not None
    ]
    if defaults:
        return max(defaults, key=lambda pair: pair[0])

    # Tier 3: the base package map, which applies regardless of labels but only
    # when the applicable tags said nothing at all about this package.
    hit = longest_prefix(config.get("packages"), package)
    if hit is not None:
        key, value = hit
        return value, f"base entry `{key}`" if key.rstrip("/") == package else \
            f"base entry, inherited from `{key}`"

    # Tier 4.
    return config.get("default", 70), "global default"


def _is_percentage(value):
    """A finite number between 0 and 100. bool is excluded, being an int subclass."""
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        return False
    return math.isfinite(value) and 0 <= value <= 100


def _check_package_map(entries, where, errors):
    if not isinstance(entries, dict):
        errors.append(f"`{where}` must be a mapping of package path to threshold.")
        return
    for key, value in entries.items():
        if not _is_percentage(value):
            errors.append(f"`{where}.{key}` must be a number between 0 and 100, got {value!r}.")


def validate_config(config):
    """Returns a list of problems. Values are checked before they are compared to a
    percentage: an unvalidated string threshold raises a TypeError mid-run and the
    job then reports a traceback with no summary at all, and a negative threshold
    silently lets any coverage pass."""
    if not isinstance(config, dict):
        return ["The config root must be a mapping."]

    errors = []

    if "default" in config and not _is_percentage(config["default"]):
        errors.append(f"`default` must be a number between 0 and 100, got {config['default']!r}.")

    if "min_changed_lines" in config:
        value = config["min_changed_lines"]
        if isinstance(value, bool) or not isinstance(value, int) or value < 0:
            errors.append(
                f"`min_changed_lines` must be a non-negative integer, got {value!r}.")

    if config.get("packages") is not None:
        _check_package_map(config["packages"], "packages", errors)

    tags = config.get("tags")
    if tags is not None and not isinstance(tags, dict):
        errors.append("`tags` must be a mapping of label name to threshold group.")
    elif isinstance(tags, dict):
        for name, body in tags.items():
            if body is None:
                continue
            if not isinstance(body, dict):
                errors.append(f"`tags.{name}` must be a mapping with `default` and/or `packages`.")
                continue
            # Catches the flat form, where package paths sit directly under the tag
            # rather than under `packages`. That shape parses cleanly and would
            # otherwise resolve to the global default while looking correct.
            unknown = sorted(set(body) - {"default", "packages"})
            if unknown:
                errors.append(
                    f"`tags.{name}` has unexpected key(s) {', '.join(unknown)}. "
                    f"Package thresholds belong under `tags.{name}.packages`.")
            if "default" in body and not _is_percentage(body["default"]):
                errors.append(
                    f"`tags.{name}.default` must be a number between 0 and 100, "
                    f"got {body['default']!r}.")
            if body.get("packages") is not None:
                _check_package_map(body["packages"], f"tags.{name}.packages", errors)

    return errors


def unmatched_config_paths(config):
    """Config paths that no longer exist. Left unchecked, thresholds rot silently."""
    missing = []
    for key in config.get("packages") or {}:
        if not os.path.isdir(key.rstrip("/")):
            missing.append(("packages", key))
    for tag, body in (config.get("tags") or {}).items():
        for key in (body or {}).get("packages") or {}:
            if not os.path.isdir(key.rstrip("/")):
                missing.append((f"tags.{tag}", key))
    return missing


def main():
    if len(sys.argv) != 5:
        print(__doc__, file=sys.stderr)
        return 2

    report_path, config_path, labels_raw, summary_path = sys.argv[1:5]

    with open(config_path) as handle:
        config = yaml.safe_load(handle) or {}
    labels = load_labels(labels_raw)
    min_lines = config.get("min_changed_lines", 0)

    with open(report_path) as handle:
        report = json.load(handle)

    lines = ["### 🛡️ Backend Integration Patch Coverage", ""]

    # Structure and value checks come first: everything below assumes thresholds
    # are numbers it can compare against a percentage.
    invalid = validate_config(config)
    if invalid:
        lines += [
            f"**Failed — `{config_path}` is not valid.**",
            "",
        ]
        lines += [f"- {problem}" for problem in invalid]
        write_summary(summary_path, lines)
        return 1

    # A stale path fails the check rather than quietly not applying.
    missing = unmatched_config_paths(config)
    if missing:
        lines += [
            "**Failed — the threshold config names packages that do not exist.**",
            "",
            "Thresholds on these paths can never apply, so they would rot unnoticed:",
            "",
        ]
        lines += [f"- `{key}` under `{where}`" for where, key in missing]
        write_summary(summary_path, lines)
        return 1

    # Group the changed lines by package.
    packages = {}
    for path, stats in (report.get("src_stats") or {}).items():
        covered = len(stats.get("covered_lines") or [])
        missed = len(stats.get("violation_lines") or [])
        entry = packages.setdefault(package_of(path), {"covered": 0, "coverable": 0, "files": 0})
        entry["covered"] += covered
        entry["coverable"] += covered + missed
        entry["files"] += 1

    if not packages:
        lines += [
            "**N/A — the diff contains no changed coverable backend lines.**",
            "",
            "Changed lines that Go does not instrument, such as comments and declarations,",
            "are excluded from the measurement.",
        ]
        write_summary(summary_path, lines)
        return 0

    rows, failures, exempt = [], [], []
    for package in sorted(packages):
        stats = packages[package]
        coverable = stats["coverable"]
        threshold, provenance = resolve(package, config, labels)

        # A package can reach here with nothing coverable, for instance a
        # comment-only change to an instrumented file. Guarded explicitly rather
        # than relying on min_changed_lines being above zero, which it need not be.
        if coverable == 0 or coverable < min_lines:
            exempt.append((package, stats["covered"], coverable, threshold, provenance))
            continue

        percent = 100.0 * stats["covered"] / coverable
        ok = percent >= threshold
        if not ok:
            failures.append(package)
        rows.append((package, stats["covered"], coverable, percent, threshold, provenance, ok))

    failed = bool(failures)
    if failed:
        lines.append(f"**Failed** — {len(failures)} package(s) below threshold.")
    elif rows:
        lines.append(f"**Passed** — {len(rows)} package(s) met their thresholds.")
    else:
        lines.append("**Passed** — no changed package had enough coverable lines to measure.")
    lines.append("")

    if labels:
        lines.append(f"- Labels considered: {', '.join(f'`{label}`' for label in labels)}")
    else:
        lines.append("- No labels on this pull request, so no tag group applies. Thresholds "
                     "come from the base package map, then the global default.")
    lines.append(f"- Minimum changed coverable lines to be measured: {min_lines}")
    lines.append("")

    if rows:
        lines += [
            "| Package | Covered | Coverable | % | Required | Threshold from |",
            "|---|---:|---:|---:|---:|---|",
        ]
        for package, covered, coverable, percent, threshold, provenance, ok in rows:
            mark = "" if ok else " ❌"
            lines.append(
                f"| `{package}`{mark} | {covered} | {coverable} | {percent:.1f}% | "
                f"{threshold}% | {provenance} |"
            )
        lines.append("")

    if exempt:
        lines.append(f"<details><summary>{len(exempt)} package(s) exempt "
                     f"(nothing coverable, or fewer than {min_lines} changed coverable "
                     f"lines)</summary>")
        lines.append("")
        # Raw counts, not a percentage. The exemption exists because the
        # denominator is too small for a ratio to mean anything, so printing one
        # would invite action on a figure the check has just called unreliable.
        # The counts still show whether a package is wholly untested, and are what
        # min_changed_lines has to be calibrated against.
        for package, covered, coverable, threshold, provenance in exempt:
            if coverable == 0:
                detail = "no coverable lines"
            else:
                detail = f"{covered} of {coverable} changed coverable lines covered"
            lines.append(f"- `{package}` — {detail}, would need {threshold}% ({provenance})")
        lines += ["", "</details>", ""]

    if failed:
        lines += ["Uncovered lines per file are listed in the diff-cover report below.", ""]

    write_summary(summary_path, lines)
    return 1 if failed else 0


def write_summary(path, lines):
    # File only. The caller cats this through tee, so echoing here would print the
    # whole verdict twice in the job log.
    with open(path, "w") as handle:
        handle.write("\n".join(lines) + "\n")


if __name__ == "__main__":
    sys.exit(main())
