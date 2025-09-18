import { useState, useEffect } from "react";
import { useNavigate, useOutletContext, useSearchParams } from "react-router-dom";
import { agentApi, templatesApi } from "../utils/api";
import AgentForm from "../components/AgentForm";
import Header from "../components/Header";

function CreateAgent() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const templateId = searchParams.get("template");

  const { showToast } = useOutletContext();
  const [loading, setLoading] = useState(false);
  const [metadata, setMetadata] = useState(null);
  const [metadataLoading, setMetadataLoading] = useState(false);
  const [formData, setFormData] = useState({});
  const [templateLoading, setTemplateLoading] = useState(false);
  const [templateError, setTemplateError] = useState(null);
  const [templateConfig, setTemplateConfig] = useState(null);
  const [activeSection, setActiveSection] = useState(null);

  useEffect(() => {
    const fetchTemplateConfig = async () => {
      try {
        setTemplateLoading(true);
        const response = await templatesApi.getTemplateConfig(templateId);
        setTemplateConfig(response.config);
      } catch(err){
        setTemplateError(err);
        setTemplateLoading(false);
      }
    }
    if(templateId) {
      fetchTemplateConfig();
    }
  }, [templateId]);

  // Fetch metadata on component mount
  useEffect(() => {
    const fetchMetadata = async () => {
      setMetadataLoading(true)
      try {
        // Fetch metadata from the dedicated endpoint
        const response = await agentApi.getAgentConfigMetadata();
        if (response) {
          setMetadata(response);
        }
        setMetadataLoading(false);
      } catch (error) {
        console.error("Error fetching metadata:", error);
        setMetadataLoading(false);
        // Continue without metadata, the form will use default fields
      }
    };

    fetchMetadata();
  }, []);

  // Initialize formData with template config or default values when metadata is loaded
  useEffect(() => {
    if (metadata && Object.keys(formData).length === 0) {
      let initialFormData = {
        // Initialize arrays for complex fields
        connectors: [],
        actions: [],
        dynamic_prompts: [],
        mcp_servers: [],
      };

      // Helper function to get default value for a field from metadata
      const getDefaultValueFromMetadata = (fieldName) => {
        const sections = [
          'BasicInfoSection',
          'ModelSettingsSection', 
          'MemorySettingsSection',
          'PromptsGoalsSection',
          'AdvancedSettingsSection'
        ];

        for (const sectionKey of sections) {
          if (metadata[sectionKey] && Array.isArray(metadata[sectionKey])) {
            const field = metadata[sectionKey].find(f => f.name === fieldName);
            if (field) {
              // If field has options array, use the first option's value
              if (field.options && Array.isArray(field.options) && field.options.length > 0) {
                return field.options[0].value;
              } else if (field.hasOwnProperty('defaultValue')) {
                return field.defaultValue;
              }
            }
          }
        }
        return undefined;
      };

      // If we have template config, use it to prepopulate the form
      if (templateConfig) {
        // Automatically map all fields from template config
        Object.keys(templateConfig).forEach(key => {
          if (key === 'actions') {
            // Special handling for actions array
            initialFormData[key] = templateConfig[key] ? templateConfig[key].map(action => ({
              config: action.config || '{}',
              name: action.name
            })) : [];
          } else if (Array.isArray(templateConfig[key])) {
            initialFormData[key] = templateConfig[key] || [];
          } else if(key === 'system_prompt' || key === 'name' || key === 'description' || key === 'model') {
            initialFormData[key] = templateConfig[key] || '';
          } else {
            initialFormData[key] = getDefaultValueFromMetadata(key);
          }
        });
      } else {
        // Use default values from metadata when no template config
        const sections = [
          'BasicInfoSection',
          'ModelSettingsSection', 
          'MemorySettingsSection',
          'PromptsGoalsSection',
          'AdvancedSettingsSection'
        ];

        sections.forEach((sectionKey) => {
          if (metadata[sectionKey] && Array.isArray(metadata[sectionKey])) {
            metadata[sectionKey].forEach((field) => {
              if (field.name) {
                const defaultValue = getDefaultValueFromMetadata(field.name);
                if (defaultValue !== undefined) {
                  initialFormData[field.name] = defaultValue;
                }
              }
            });
          }
        });
      }

      setFormData(initialFormData);
      setTemplateLoading(false)
    }
  }, [metadata, templateConfig, formData]);

  // Handle form submission
  const handleSubmit = async (data) => {
    setLoading(true);
    try {
      await agentApi.createAgent(data);
      showToast && showToast("Agent created successfully!", "success");
      navigate("/agents");
    } catch (error) {
      if(error?.message){
        showToast && showToast(error.message.charAt(0).toUpperCase() + error.message.slice(1), "error");
      } else {
        showToast && showToast("Failed to create agent", "error");
      }
      if(error.section) {
        setActiveSection(error.section);
      }
      console.log("Error creating agent:", error);
    } finally {
      setLoading(false);
    }
  };

  const backButton = (
    <button
      className="action-btn pause-resume-btn"
      onClick={() => navigate("/agents")}
    >
      <i className="fas fa-arrow-left"></i> Back to Agents
    </button>
  );

  if (templateLoading || metadataLoading) {
    return (
      <div className="loading-container">
        <div className="spinner"></div>
      </div>
    );
  }

  if (templateError) {
    return <div className="error">{templateError}</div>;
  }

  return (
    <div className="dashboard-container">
      <div className="main-content-area">
        <div className="header-container">
          <Header
            title="Create Agent"
            description="Fill out the form below to create a new agent. You can customize its configuration and capabilities."
          />
          <div className="header-right">{backButton}</div>
        </div>

        <div style={{ marginTop: 32 }}>
          <AgentForm
            metadata={metadata}
            formData={formData}
            setFormData={setFormData}
            onSubmit={handleSubmit}
            loading={loading}
            initialActiveSection={activeSection}
            onSectionChange={setActiveSection}
          />
        </div>
      </div>
    </div>
  );
}

export default CreateAgent;
