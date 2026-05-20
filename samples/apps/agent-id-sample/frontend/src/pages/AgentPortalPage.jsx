import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useThunderID } from "@thunderid/react";
import { Bot, Copy, Plus, RefreshCw, Trash2, X } from "lucide-react";
import {
  addAgentRoleAssignment,
  createAgent,
  deleteAgent,
  extractAgentOAuthCreds,
  getDefaultOrganizationUnit,
  listAgents,
  listRoles
} from "../api/thunder";

const DEFAULT_OU_HANDLE =
  import.meta.env.VITE_THUNDER_DEFAULT_OU_HANDLE || "default";

export function AgentPortalPage() {
  const { isSignedIn, isLoading: authLoading, signIn, getAccessToken } = useThunderID();

  if (authLoading) {
    return (
      <main className="bookings-page">
        <p className="empty-state management-message">Loading…</p>
      </main>
    );
  }

  if (!isSignedIn) {
    return (
      <main className="bookings-page">
        <section className="management-empty">
          <div>
            <p className="eyebrow">Agent Portal</p>
            <h1>Sign in to manage agents.</h1>
            <p>Admins can create agents and assign Thunder roles to them.</p>
          </div>
          <button
            className="dashboard-action dashboard-action--secondary"
            type="button"
            onClick={() => signIn({ acr_values: "urn:thunder:auth:user" })}
          >
            Sign in
          </button>
        </section>
      </main>
    );
  }

  return <AgentPortal getAccessToken={getAccessToken} />;
}

function AgentPortal({ getAccessToken }) {
  const getAccessTokenRef = useRef(getAccessToken);
  useEffect(() => {
    getAccessTokenRef.current = getAccessToken;
  }, [getAccessToken]);

  const [agents, setAgents] = useState([]);
  const [roles, setRoles] = useState([]);
  const [ou, setOu] = useState(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [createdCreds, setCreatedCreds] = useState(null);

  const loadAll = useCallback(async () => {
    setIsLoading(true);
    setError("");
    try {
      const accessToken = await getAccessTokenRef.current();
      const [agentsResp, rolesResp, ouResp] = await Promise.all([
        listAgents(accessToken),
        listRoles(accessToken),
        getDefaultOrganizationUnit(accessToken, DEFAULT_OU_HANDLE)
      ]);
      setAgents(agentsResp?.agents || []);
      setRoles(rolesResp?.roles || []);
      setOu(ouResp || null);
    } catch (err) {
      setError(formatError(err));
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  async function handleCreate({ name, description, selectedRoleIds }) {
    setError("");
    if (!ou?.id) {
      setError("Default organization unit not loaded yet.");
      return null;
    }
    const accessToken = await getAccessTokenRef.current();
    const created = await createAgent(accessToken, {
      name,
      description,
      ouId: ou.id
    });

    for (const roleId of selectedRoleIds) {
      try {
        await addAgentRoleAssignment(accessToken, roleId, created.id);
      } catch (assignErr) {
        setError(`Agent created but role assignment failed: ${formatError(assignErr)}`);
      }
    }

    const creds = extractAgentOAuthCreds(created);
    setCreatedCreds({
      agentName: created.name,
      clientId: creds.clientId,
      clientSecret: creds.clientSecret
    });
    setShowCreate(false);
    await loadAll();
    return created;
  }

  async function handleDelete(agentId) {
    if (!window.confirm("Delete this agent? Its OAuth client will stop working immediately.")) {
      return;
    }
    setError("");
    try {
      const accessToken = await getAccessTokenRef.current();
      await deleteAgent(accessToken, agentId);
      await loadAll();
    } catch (err) {
      setError(formatError(err));
    }
  }

  return (
    <main className="bookings-page">
      <section className="management-header">
        <div>
          <p className="eyebrow">Agent Portal</p>
          <h1>Agents</h1>
          <p style={{ marginTop: 4, color: "#666" }}>
            Provision agent identities with roles to perform actions on Wayfinder.
            The agent can then sign in via the &ldquo;Log in as Agent&rdquo; path
            using the credentials shown here.
          </p>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <button
            className="dashboard-action dashboard-action--secondary"
            type="button"
            onClick={() => loadAll()}
            disabled={isLoading}
          >
            <RefreshCw size={16} /> Refresh
          </button>
          <button
            className="dashboard-action"
            type="button"
            onClick={() => setShowCreate(true)}
            disabled={!ou?.id}
          >
            <Plus size={16} /> Create agent
          </button>
        </div>
      </section>

      {error && (
        <div className="api-status api-status--error" role="status">
          {error}
        </div>
      )}

      {createdCreds && (
        <CredentialsCallout creds={createdCreds} onDismiss={() => setCreatedCreds(null)} />
      )}

      <section
        className="management-panel"
        aria-label="Agents"
        style={{ minHeight: 0 }}
      >
        {isLoading && <p className="empty-state management-message">Loading agents…</p>}
        {!isLoading && agents.length === 0 && (
          <div className="management-empty-state">
            <h2>No agents yet</h2>
            <p>Create your first agent to mint credentials for it.</p>
          </div>
        )}
        {!isLoading && agents.length > 0 && (
          <AgentTable agents={agents} onDelete={handleDelete} />
        )}
      </section>

      {showCreate && (
        <CreateAgentModal
          roles={roles}
          ou={ou}
          onCancel={() => setShowCreate(false)}
          onCreate={handleCreate}
        />
      )}
    </main>
  );
}

function AgentTable({ agents, onDelete }) {
  return (
    <>
      <div className="booking-table-heading" aria-hidden="true">
        <span>Name</span>
        <span>Agent ID</span>
        <span>Description</span>
        <span></span>
      </div>
      {agents.map((agent) => (
        <div className="booking-row" key={agent.id} role="row">
          <div className="booking-route">
            <Bot size={16} style={{ marginBottom: 4 }} />
            <strong>{agent.name}</strong>
            <small>{agent.type || "default"}</small>
          </div>
          <div className="booking-cell">
            <strong style={{ wordBreak: "break-all" }}>{agent.clientId || "—"}</strong>
            <span>Sign-in identifier</span>
          </div>
          <div className="booking-cell">
            <span>{agent.description || "—"}</span>
          </div>
          <div className="booking-price" style={{ alignItems: "flex-end" }}>
            <button
              className="dashboard-action dashboard-action--secondary"
              type="button"
              onClick={(event) => {
                event.preventDefault();
                onDelete(agent.id);
              }}
              title="Delete agent"
            >
              <Trash2 size={16} />
            </button>
          </div>
        </div>
      ))}
    </>
  );
}

const HIDDEN_ROLE_NAMES = new Set(["administrator"]);

function CreateAgentModal({ roles, ou, onCancel, onCreate }) {
  const assignableRoles = useMemo(
    () => roles.filter((role) => !HIDDEN_ROLE_NAMES.has((role.name || "").trim().toLowerCase())),
    [roles]
  );
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [selectedRoles, setSelectedRoles] = useState(() => new Set());
  const [submitting, setSubmitting] = useState(false);
  const [localError, setLocalError] = useState("");

  function toggleRole(roleId) {
    setSelectedRoles((prev) => {
      const next = new Set(prev);
      if (next.has(roleId)) {
        next.delete(roleId);
      } else {
        next.add(roleId);
      }
      return next;
    });
  }

  async function handleSubmit(event) {
    event.preventDefault();
    setLocalError("");
    if (!name.trim()) {
      setLocalError("Name is required.");
      return;
    }
    setSubmitting(true);
    try {
      await onCreate({
        name,
        description,
        selectedRoleIds: Array.from(selectedRoles)
      });
    } catch (err) {
      setLocalError(formatError(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div style={modalBackdrop} onClick={onCancel}>
      <div style={modalCard} onClick={(e) => e.stopPropagation()}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <h2 style={{ margin: 0 }}>Create agent</h2>
          <button
            type="button"
            onClick={onCancel}
            aria-label="Close"
            style={iconButton}
          >
            <X size={18} />
          </button>
        </div>
        <p style={{ color: "#666", marginTop: 4 }}>
          Agent will be created in OU{" "}
          <code>{ou?.handle || "default"}</code>.
        </p>
        <form onSubmit={handleSubmit} style={{ display: "grid", gap: 12, marginTop: 8 }}>
          <label style={fieldLabel}>
            Name
            <input
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. trip-research-bot"
              style={fieldInput}
            />
          </label>
          <label style={fieldLabel}>
            Description
            <input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What is this agent for?"
              style={fieldInput}
            />
          </label>
          <fieldset style={{ border: "1px solid #e2e2e2", borderRadius: 8, padding: 12 }}>
            <legend style={{ fontSize: 13, color: "#444", padding: "0 6px" }}>
              Roles ({assignableRoles.length} available)
            </legend>
            {assignableRoles.length === 0 && (
              <p style={{ margin: 0, color: "#888" }}>No roles available.</p>
            )}
            <div style={{ maxHeight: 200, overflowY: "auto", display: "grid", gap: 4 }}>
              {assignableRoles.map((role) => (
                <label
                  key={role.id}
                  style={{
                    display: "flex",
                    gap: 8,
                    alignItems: "flex-start",
                    padding: "4px 0"
                  }}
                >
                  <input
                    type="checkbox"
                    checked={selectedRoles.has(role.id)}
                    onChange={() => toggleRole(role.id)}
                  />
                  <span>
                    <strong>{role.name}</strong>
                    {role.description && (
                      <span style={{ color: "#666", display: "block", fontSize: 12 }}>
                        {role.description}
                      </span>
                    )}
                  </span>
                </label>
              ))}
            </div>
          </fieldset>
          {localError && (
            <div className="api-status api-status--error" role="status">
              {localError}
            </div>
          )}
          <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
            <button
              type="button"
              className="dashboard-action dashboard-action--secondary"
              onClick={onCancel}
              disabled={submitting}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="dashboard-action"
              disabled={submitting}
            >
              {submitting ? "Creating…" : "Create agent"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

function CredentialsCallout({ creds, onDismiss }) {
  return (
    <section
      style={{
        background: "#0f172a",
        color: "white",
        padding: 16,
        borderRadius: 12,
        marginBottom: 16
      }}
    >
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h2 style={{ margin: 0, fontSize: 18 }}>
          Credentials for {creds.agentName}
        </h2>
        <button
          type="button"
          onClick={onDismiss}
          aria-label="Dismiss credentials"
          style={{ ...iconButton, color: "white" }}
        >
          <X size={18} />
        </button>
      </div>
      <p style={{ margin: "4px 0 12px", color: "#cbd5e1" }}>
        Copy these now — the agent secret will not be shown again.
      </p>
      <CredField label="Agent ID" value={creds.clientId} />
      <CredField label="Agent Secret" value={creds.clientSecret} mono secret />
    </section>
  );
}

function CredField({ label, value, mono, secret }) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    try {
      await navigator.clipboard.writeText(value || "");
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // ignore
    }
  }
  return (
    <div style={{ display: "grid", gap: 4, marginBottom: 8 }}>
      <span style={{ fontSize: 12, color: "#94a3b8" }}>{label}</span>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          background: "#1e293b",
          padding: "8px 10px",
          borderRadius: 6
        }}
      >
        <code
          style={{
            flex: 1,
            wordBreak: "break-all",
            fontFamily: mono ? "monospace" : undefined,
            fontSize: 13,
            color: secret ? "#fde68a" : "white"
          }}
        >
          {value || "—"}
        </code>
        <button
          type="button"
          onClick={copy}
          aria-label={`Copy ${label}`}
          style={{
            background: "transparent",
            color: copied ? "#86efac" : "white",
            border: "1px solid #334155",
            borderRadius: 6,
            padding: "4px 8px",
            cursor: "pointer",
            display: "inline-flex",
            alignItems: "center",
            gap: 4
          }}
        >
          <Copy size={14} />
          {copied ? "Copied" : "Copy"}
        </button>
      </div>
    </div>
  );
}

function formatError(err) {
  if (!err) return "Unknown error";
  if (err.status === 401 || err.status === 403) {
    return `${err.message} — make sure you are signed in as an admin user with the 'system' scope.`;
  }
  return err.message || String(err);
}

const modalBackdrop = {
  position: "fixed",
  inset: 0,
  background: "rgba(15, 23, 42, 0.55)",
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  zIndex: 50
};

const modalCard = {
  background: "white",
  borderRadius: 12,
  padding: 20,
  width: "min(540px, 92vw)",
  maxHeight: "85vh",
  overflowY: "auto",
  boxShadow: "0 24px 60px rgba(0,0,0,0.25)"
};

const fieldLabel = {
  display: "grid",
  gap: 4,
  fontSize: 13,
  color: "#374151"
};

const fieldInput = {
  padding: "8px 10px",
  borderRadius: 6,
  border: "1px solid #d1d5db",
  fontSize: 14
};

const iconButton = {
  background: "transparent",
  border: "none",
  cursor: "pointer",
  padding: 4,
  color: "inherit"
};
