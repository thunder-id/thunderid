// Client for Thunder admin REST APIs (agents, roles, organization units).
// Calls go directly from the browser to the Thunder server using the access
// token issued at sign-in. The token must carry the appropriate `system:*`
// scopes — see SCOPES in src/main.jsx.

const THUNDER_BASE_URL =
  import.meta.env.VITE_THUNDER_BASE_URL ||
  import.meta.env.VITE_THUNDER_BASE_URL ||
  "https://localhost:8090";

async function thunderRequest(path, accessToken, options = {}) {
  const headers = {
    Accept: "application/json",
    ...(options.body ? { "Content-Type": "application/json" } : {}),
    ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
    ...options.headers
  };
  const response = await fetch(`${THUNDER_BASE_URL}${path}`, { ...options, headers });
  const text = await response.text();
  const body = text ? safeJson(text) : null;
  if (!response.ok) {
    const message =
      (body && (body.description || body.message || body.error)) ||
      `Thunder request failed (${response.status})`;
    const error = new Error(message);
    error.status = response.status;
    error.body = body;
    throw error;
  }
  return body;
}

function safeJson(text) {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

export async function getDefaultOrganizationUnit(accessToken, handle = "default") {
  const data = await thunderRequest(
    `/organization-units/tree/${encodeURIComponent(handle)}`,
    accessToken
  );
  return data;
}

export async function listAgents(accessToken, { limit = 50, offset = 0 } = {}) {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  const data = await thunderRequest(`/agents?${params.toString()}`, accessToken);
  return data;
}

export async function getAgent(accessToken, agentId) {
  return thunderRequest(`/agents/${encodeURIComponent(agentId)}`, accessToken);
}

export async function listRoles(accessToken, { limit = 100, offset = 0 } = {}) {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  const data = await thunderRequest(`/roles?${params.toString()}`, accessToken);
  return data;
}

export async function getRoleAssignments(accessToken, roleId, { type = "agent" } = {}) {
  const params = new URLSearchParams({ type, include: "display", limit: "100" });
  return thunderRequest(
    `/roles/${encodeURIComponent(roleId)}/assignments?${params.toString()}`,
    accessToken
  );
}

// Creates an agent with an OAuth2 inbound auth profile that supports the
// client_credentials grant. The returned object includes the freshly issued
// clientSecret in inboundAuthConfig[0].config.clientSecret — display it to the
// admin immediately because Thunder will not return it on subsequent reads.
export async function createAgent(accessToken, { name, description, ouId }) {
  const safeName = name.trim();
  const clientId = `WAYFINDER-AGENT-${safeName
    .toUpperCase()
    .replace(/[^A-Z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 40) || "AGENT"}-${Date.now().toString(36)}`;

  const payload = {
    name: safeName,
    description: description?.trim() || undefined,
    type: "default",
    ouId,
    inboundAuthConfig: [
      {
        type: "oauth2",
        config: {
          clientId,
          grantTypes: ["client_credentials"],
          tokenEndpointAuthMethod: "client_secret_basic",
          publicClient: false,
          pkceRequired: false,
          token: {
            accessToken: {
              validityPeriod: 3600
            }
          }
        }
      }
    ]
  };

  return thunderRequest(`/agents`, accessToken, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function deleteAgent(accessToken, agentId) {
  return thunderRequest(`/agents/${encodeURIComponent(agentId)}`, accessToken, {
    method: "DELETE"
  });
}

export async function addAgentRoleAssignment(accessToken, roleId, agentId) {
  return thunderRequest(
    `/roles/${encodeURIComponent(roleId)}/assignments/add`,
    accessToken,
    {
      method: "POST",
      body: JSON.stringify({
        assignments: [{ id: agentId, type: "agent" }]
      })
    }
  );
}

export async function removeAgentRoleAssignment(accessToken, roleId, agentId) {
  return thunderRequest(
    `/roles/${encodeURIComponent(roleId)}/assignments/remove`,
    accessToken,
    {
      method: "POST",
      body: JSON.stringify({
        assignments: [{ id: agentId, type: "agent" }]
      })
    }
  );
}

export function extractAgentOAuthCreds(agentResponse) {
  const oauth = agentResponse?.inboundAuthConfig?.find((c) => c.type === "oauth2");
  return {
    clientId: oauth?.config?.clientId || "",
    clientSecret: oauth?.config?.clientSecret || ""
  };
}
