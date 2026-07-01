// Spectral custom function (tested against @stoplight/spectral 6.x).
//
// `given` MUST target an operation: $.paths[*][get,post,put,patch,delete].
//
// Options:
//   methods : string[] - methods that must carry the header (default ['post'])
//   header  : string   - header name (default 'Idempotency-Key')
//
// Note: this inspects operation-level header parameters. If you declare the
// idempotency header at path-item level, change `given` to $.paths[*] and merge
// pathItem.parameters as in requiredCollectionQueryParams.js.

export default (op, opts = {}, context) => {
  if (!op || typeof op !== 'object') return;

  const url = String(context.path[1]);
  const method = String(context.path[2]).toLowerCase();
  const methods = opts.methods || ['post'];
  if (!methods.includes(method)) return;

  const header = opts.header || 'Idempotency-Key';
  const want = header.toLowerCase();

  const headerNames = (Array.isArray(op.parameters) ? op.parameters : [])
    .filter((p) => p && p.in === 'header' && typeof p.name === 'string')
    .map((p) => p.name.toLowerCase());

  if (headerNames.includes(want)) return;

  return [
    {
      message: `${method.toUpperCase()} ${url} must accept the ${header} request header so clients can retry safely without duplicating writes.`,
      path: ['paths', url, method],
    },
  ];
};
