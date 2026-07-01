// Spectral custom function (tested against @stoplight/spectral 6.x).
//
// `given` MUST target an operation, i.e. $.paths[*][get,post,put,patch,delete],
// so context.path === ['paths', url, method].
//
// Options:
//   base  : string[] - required on every operation (default 400,401,403,500)
//   write : string[] - additionally required on POST/PUT/PATCH/DELETE (e.g. 409)
//
// Item endpoints (.../{id}) additionally require 404.

const DEFAULT_BASE = ['400', '401', '403', '500'];
const WRITE_METHODS = ['post', 'put', 'patch', 'delete'];
const ITEM_SUFFIX = /\}\/?$/;

export default (op, opts = {}, context) => {
  if (!op || typeof op !== 'object') return;

  const url = String(context.path[1]);
  const method = String(context.path[2]).toLowerCase();
  const responses = op.responses || {};

  const required = new Set(opts.base || DEFAULT_BASE);
  if (ITEM_SUFFIX.test(url)) required.add('404');
  if (WRITE_METHODS.includes(method)) {
    (opts.write || []).forEach((c) => required.add(String(c)));
  }

  const missing = [...required].filter((code) => !(code in responses));
  if (missing.length === 0) return;

  return [
    {
      message: `${method.toUpperCase()} ${url} is missing standard error responses: ${missing.join(', ')}.`,
      path: ['paths', url, method, 'responses'],
    },
  ];
};
