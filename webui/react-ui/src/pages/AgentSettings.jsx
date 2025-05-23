import { useState, useEffect } from "react";
import { useParams, useOutletContext, useNavigate } from "react-router-dom";
import { useAgent } from "../hooks/useAgent";
import { agentApi } from "../utils/api";
import AgentForm from "../components/AgentForm";
import Header from "../components/Header";
import { AgentStatus, AgentActionButtons } from "../components/AgentComponents";

function AgentSettings() {
  const { name } = useParams();
  const { showToast } = useOutletContext();
  const navigate = useNavigate();
  const [metadata, setMetadata] = useState(null);
  const [formData, setFormData] = useState({});

  // Update document title
  useEffect(() => {
    if (name) {
      document.title = `Agent Settings: ${name} - LocalAGI`;
    }
    return () => {
      document.title = "LocalAGI";
    };
  }, [name]);

  // Use our custom agent hook
  const { agent, loading, updateAgent, toggleAgentStatus, deleteAgent } =
    useAgent(name);

  // Fetch metadata on component mount
  useEffect(() => {
    const fetchMetadata = async () => {
      try {
        const response = await agentApi.getAgentConfigMetadata();
        setMetadata(response);
      } catch (err) {
        console.error("Error fetching agent metadata:", err);
        showToast("Failed to load agent metadata", "error");
      }
    };
    fetchMetadata();
  }, [showToast]);

  useEffect(() => {
    if (agent) {
      setFormData(agent);
    }
  }, [agent]);

  // Header action handlers
  const handlePauseResume = async () => {
    try {
      await toggleAgentStatus();
      showToast(agent?.active ? "Agent paused" : "Agent resumed", "success");
    } catch (err) {
      console.error("Error toggling agent status:", err);
      showToast("Failed to update agent status", "error");
    }
  };

  const handleDelete = async () => {
    if (!window.confirm("Are you sure you want to delete this agent?")) return;
    try {
      await deleteAgent();
      showToast("Agent deleted", "success");
      navigate("/agents");
    } catch (err) {
      console.error("Error deleting agent:", err);
      showToast("Failed to delete agent", "error");
    }
  };

  return (
    <div className="dashboard-container">
      <div className="main-content-area">
        <div className="header-container">
          <Header
            title="Agent Settings"
            description="Configure and manage the agent's settings, connectors, and capabilities."
            name={name}
          />

          <div className="header-right">
            <AgentActionButtons
              agent={agent}
              loading={loading}
              onPauseResume={handlePauseResume}
              onDelete={handleDelete}
            />
          </div>
        </div>

        {/* Agent Form */}
        <div className="section-box">
          {metadata && formData ? (
            <AgentForm
              isEdit
              formData={formData}
              setFormData={setFormData}
              onSubmit={updateAgent}
              loading={loading}
              submitButtonText="Save Changes"
              metadata={metadata}
            />
          ) : (
            <div style={{ color: "var(--text-light)", padding: 24 }}>
              Loading agent configuration...
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default AgentSettings;
