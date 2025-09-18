import { useState, useEffect } from "react";
import { useParams, useOutletContext, useNavigate } from "react-router-dom";
import { useAgent } from "../hooks/useAgent";
import { agentApi } from "../utils/api";
import AgentForm from "../components/AgentForm";
import Header from "../components/Header";
import { AgentStatus, AgentActionButtons } from "../components/AgentComponents";
import useIsMobile from "../hooks/useMobileDetect";

function AgentSettings() {
  const { id } = useParams();
  const { showToast } = useOutletContext();
  const navigate = useNavigate();
  const [metadata, setMetadata] = useState(null);
  const [formData, setFormData] = useState({});
  const [loading, setLoading] = useState(false);
  const [activeSection, setActiveSection] = useState(null);
  const isMobile = useIsMobile();

  // Use our custom agent hook
  const { agent, deleteAgent, setAgent, fetchAgent } = useAgent(id);

  // Update document title
  useEffect(() => {
    if (agent) {
      document.title = `Agent Settings: ${agent.name} - LocalAGI`;
    }
    return () => {
      document.title = "LocalAGI";
    };
  }, [agent]);

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
  }, []);

  useEffect(() => {
    if (agent) {
      setFormData(agent);
    }
  }, [agent]);

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
        setAgent((prevAgent) => ({
          ...prevAgent,
          active: !isActive,
        }));

        // Show success toast
        const action = isActive ? "paused" : "started";

        showToast(`Agent "${name}" ${action} successfully`, "success");
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

  // Header action handlers
  const handlePauseResume = async (isActive) => {
    try {
      const success = await toggleAgentStatus(id, agent.name, isActive);
      if (success) {
        showToast(
          `Agent "${agent.name}" ${isActive ? "resumed" : "paused"}`,
          "success"
        );
      }
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

  const updateAgent = async (config) => {
    setLoading(true);

    try {
      await agentApi.updateAgentConfig(id, config);
      showToast && showToast("Agent updated successfully!", "success");
      // Refresh agent data after update
      await fetchAgent();
    } catch (err) {
      if (err?.message) {
        showToast &&
          showToast(
            err.message.charAt(0).toUpperCase() + err.message.slice(1),
            "error"
          );
      } else {
        showToast && showToast("Failed to create agent", "error");
      }
      if(error.section) {
        setActiveSection(error.section);
      }
      console.error("Error updating agent:", err);
    } finally {
      setLoading(false);
    }
  };

  if (!agent) {
    return <div></div>;
  }

  return (
    <div className="dashboard-container">
      <div className="main-content-area">
        <div className="header-container">
          <Header
            title="Agent Settings"
            description="Configure and manage the agent's settings, connectors, and capabilities."
            name={agent.name}
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
              agent={agent}
              formData={formData}
              setFormData={setFormData}
              onSubmit={updateAgent}
              loading={loading}
              submitButtonText={isMobile ? "Save" : "Save Changes"}
              metadata={metadata}
              setAgent={setAgent}
              id={id}
              initialActiveSection={activeSection}
              onSectionChange={setActiveSection}
            />
          ) : (
            <div className="centered-loading">
              <div className="spinner-primary"></div>
              <p className="loading-text">Loading agent configuration</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default AgentSettings;
