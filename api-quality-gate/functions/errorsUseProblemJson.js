// Spectral custom function (tested against @stoplight/spectral 6.x).
//
// `given` MUST target an operation: $.paths[*][get,post,put,patch,delete].
// Every 4xx / 5xx / default response MUST expose application/problem+json
// (RFC 9457 - Problem Details for HTTP APIs).

const isErrorCode = (code) => code === 'default' || /^[45]/.test(code);

export default (op, _opts, context) => {
  if (!op || typeof op !== 'object') return;

  const url = String(context.path[1]);
  const method = String(context.path[2]).toLowerCase();
  const responses = op.responses || {};

  const problems = [];
  for (const [code, resp] of Object.entries(responses)) {
    if (!isErrorCode(code)) continue;
    const content = (resp && resp.content) || {};
    if (!content['application/problem+json']) {
      problems.push({
        message: `${method.toUpperCase()} ${url} response ${code} must use application/problem+json (RFC 9457).`,
        path: ['paths', url, method, 'responses', code],
      });
    }
  }
  return problems;
};
