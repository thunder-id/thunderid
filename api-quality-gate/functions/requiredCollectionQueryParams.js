// Spectral custom function (tested against @stoplight/spectral 6.x).
//
// `given` MUST target a path-item object, i.e. $.paths[*], so that we can read
// BOTH path-item-level and operation-level parameters (OpenAPI does not merge
// these automatically; the effective parameter set is the union of the two).
//
// Options:
//   requireAll    : string[]  - every name must be present as a query param
//   requireOneOf  : string[]  - at least one of these must be present
//   collectionOnly: boolean   - default true; skip "item" paths ending in {id}
//
// A collection endpoint is heuristically any path whose final segment is NOT a
// path template variable. Item endpoints (.../{id}) are skipped by default.

const ITEM_SUFFIX = /\}\/?$/;

export default (pathItem, opts = {}, context) => {
  if (!pathItem || typeof pathItem !== 'object') return;

  const url = String(context.path[context.path.length - 1]);
  const get = pathItem.get;
  if (!get) return; // only list/GET collections are subject to this rule

  const collectionOnly = opts.collectionOnly !== false;
  if (collectionOnly && ITEM_SUFFIX.test(url)) return;

  const queryNames = [
    ...(Array.isArray(pathItem.parameters) ? pathItem.parameters : []),
    ...(Array.isArray(get.parameters) ? get.parameters : []),
  ]
    .filter((p) => p && p.in === 'query' && typeof p.name === 'string')
    .map((p) => p.name);

  const missingAll = (opts.requireAll || []).filter((n) => !queryNames.includes(n));

  let oneOfMissing = false;
  if (Array.isArray(opts.requireOneOf) && opts.requireOneOf.length > 0) {
    oneOfMissing = !opts.requireOneOf.some((n) => queryNames.includes(n));
  }

  if (missingAll.length === 0 && !oneOfMissing) return;

  const parts = [];
  if (missingAll.length) parts.push(`required: ${missingAll.join(', ')}`);
  if (oneOfMissing) parts.push(`one of: ${opts.requireOneOf.join(' | ')}`);

  return [
    {
      message: `Collection GET ${url} is missing query params (${parts.join('; ')}).`,
      path: ['paths', url, 'get'],
    },
  ];
};
