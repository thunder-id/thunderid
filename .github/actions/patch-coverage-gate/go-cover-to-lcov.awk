# Converts a Go cover profile to LCOV so diff-cover can consume backend and
# frontend coverage in a single invocation (it cannot mix LCOV and XML).
# Maps Go import paths to repo-relative paths while emitting. Blocks may
# overlap on a line; a line counts as covered if any block covering it has a
# hit count > 0, so keep the maximum hit count seen per line.
/^mode:/ { next }
{
  # Only convert lines matching the full Go cover profile block shape
  # "<file>:<sl>.<sc>,<el>.<ec> <stmts> <hits>"; skip anything else so a
  # blank or malformed line cannot become a bogus LCOV record.
  colon = match($0, /:[0-9]+\.[0-9]+,[0-9]+\.[0-9]+ [0-9]+ [0-9]+$/)
  if (colon <= 1) next
  file = substr($0, 1, colon - 1)
  sub(/^github\.com\/thunder-id\/thunderid\//, "", file)
  if (file !~ /^backend\//) file = "backend/" file
  rest = substr($0, colon + 1)
  split(rest, parts, " ")
  split(parts[1], range, ",")
  split(range[1], s, ".")
  split(range[2], e, ".")
  hitcount = parts[3]
  for (l = s[1] + 0; l <= e[1] + 0; l++) {
    key = file SUBSEP l
    # Record each file's line numbers on first sight so END can emit them
    # in linear time instead of scanning the whole hits map per file.
    if (!(key in hits)) lines[file] = lines[file] " " l
    if (!(key in hits) || hitcount + 0 > hits[key]) hits[key] = hitcount + 0
    seen[file] = 1
  }
}
END {
  for (file in seen) {
    print "SF:" file
    n = split(lines[file], nums, " ")
    for (i = 1; i <= n; i++) {
      if (nums[i] != "") print "DA:" nums[i] "," hits[file SUBSEP nums[i]]
    }
    print "end_of_record"
  }
}
