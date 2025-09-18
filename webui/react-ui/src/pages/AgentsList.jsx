import { useState, useEffect } from "react";
import { Link, useOutletContext } from "react-router-dom";
import Header from "../components/Header";

function AgentsList() {
  const [agents, setAgents] = useState([]);
  const [statuses, setStatuses] = useState({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const { showToast } = useOutletContext();

  // Fetch agents data
  const fetchAgents = async () => {
    setLoading(true);
    try {
      const response = await fetch("/api/agents");
      if (!response.ok) {
        throw new Error(`Server responded with status: ${response.status}`);
      }

      const data = await response.json();
      setAgents(data.agents || []);
      setStatuses(data.statuses || {});
    } catch (err) {
      console.error("Error fetching agents:", err);
      setError("Failed to load agents");
    } finally {
      setLoading(false);
    }
  };

  // Toggle agent status (pause/start)
  const toggleAgentStatus = async (id, name, isActive) => {
    try {
      const endpoint = isActive
        ? `/api/agent/${id}/pause`
        : `/api/agent/${id}/start`;
      const response = await fetch(endpoint, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
      });

      if (response.ok) {
        // Update local state
        setStatuses((prev) => ({
          ...prev,
          [id]: !isActive,
        }));

        // Show success toast
        const action = isActive ? "paused" : "started";
        showToast(`Agent "${name}" ${action} successfully`, "success");

        // Refresh the agents list to ensure we have the latest data
        fetchAgents();
      } else {
        const errorData = await response.json().catch(() => null);
        throw new Error(
          errorData?.error || `Server responded with status: ${response.status}`
        );
      }
    } catch (err) {
      console.error(`Error toggling agent status:`, err);
      showToast(`Failed to update agent status: ${err.message}`, "error");
    }
  };

  // Delete an agent
  const deleteAgent = async (id, name) => {
    if (
      !confirm(
        `Are you sure you want to delete agent "${name}"? This action cannot be undone.`
      )
    ) {
      return;
    }

    try {
      const response = await fetch(`/api/agent/${id}`, {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
      });

      if (response.ok) {
        // Remove from local state
        setAgents((prev) => prev.filter((agent) => agent.id !== id));
        setStatuses((prev) => {
          const newStatuses = { ...prev };
          delete newStatuses[id];
          return newStatuses;
        });

        // Show success toast
        showToast(`Agent "${name}" deleted successfully`, "success");
      } else {
        const errorData = await response.json().catch(() => null);
        throw new Error(
          errorData?.error || `Server responded with status: ${response.status}`
        );
      }
    } catch (err) {
      console.error(`Error deleting agent:`, err);
      showToast(`Failed to delete agent: ${err.message}`, "error");
    }
  };

  useEffect(() => {
    document.title = "Agents - LocalAGI";
    return () => {
      document.title = "LocalAGI"; // Reset title when component unmounts
    };
  }, []);

  // Load agents on mount
  useEffect(() => {
    fetchAgents();
  }, []);

  if (loading) {
    return (
      <div className="loading-container">
        <div className="spinner"></div>
      </div>
    );
  }

  if (error) {
    return <div className="error">{error}</div>;
  }

  return (
    <div className="dashboard-container agent-list">
      <div className="main-content-area">
        <div className="header-container">
          <Header
            title="Agents"
            description="Easily manage, access, and interact with all your agents from one place."
            titleExtra={
              <div className="stat-card">
                {agents.length}
              </div>
            }
          />

          <Link to="/templates" className="action-btn create-agent-btn">
            <i className="fas fa-plus"></i> Create Agent
          </Link>
        </div>

        {agents.length > 0 ? (
          <div className="agents-grid-list agents-grid">
            {agents.map(({ id, name }) => (
              <div
                key={id}
                className="agent-card"
                data-agent={id}
                data-active={statuses[id]}
              >
                <div className="agent-content text-center">
                 <div className="agent-header">
                    <h3>
                      <i className="fas fa-robot"></i> {name}
                    </h3>
                    <div
                      className={`status-badge ${
                        statuses[id]
                          ? "status-active"
                          : "status-paused"
                      }`}
                    >
                      {statuses[id] ? "Active" : "Paused"}
                    </div>
                  </div>

                  <div className="agent-actions">
                    <Link
                      to={`/talk/${id}`}
                      className="agent-action-btn chat-btn"
                    >
                      <i className="fas fa-comment"></i> Chat
                    </Link>
                    <Link
                      to={`/status/${id}`}
                      className="agent-action-btn status-btn"
                    >
                      <i className="fas fa-chart-line"></i> Status
                    </Link>
                    <Link
                      to={`/settings/${id}`}
                      className="agent-action-btn settings-btn"
                    >
                      <i className="fas fa-cog"></i> Settings
                    </Link>
                  </div>

                  <div className="agent-actions mt-2">
                    <button
                      className="primary-btn"
                      onClick={() => toggleAgentStatus(id, name, statuses[id])}
                    >
                      {statuses[id] ? (
                        <>
                          <i className="fas fa-pause"></i> Pause
                        </>
                      ) : (
                        <>
                          <i className="fas fa-play"></i> Start
                        </>
                      )}
                    </button>

                    <button
                      className="action-btn delete-btn"
                      onClick={() => deleteAgent(id, name)}
                    >
                      <i className="fas fa-trash-alt"></i> Delete
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="no-agents">
            <h2>No Agents Found</h2>
            <p>Get started by creating your first agent</p>
            <Link to="/templates" className="action-btn">
              <i className="fas fa-plus"></i> Create Agent
            </Link>
          </div>
        )}
      </div>
    </div>
  );
}

export default AgentsList;
