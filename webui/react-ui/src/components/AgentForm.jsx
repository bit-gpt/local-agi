import React, { useState, useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";

// Import form sections
import BasicInfoSection from './agent-form-sections/BasicInfoSection';
import ConnectorsSection from './agent-form-sections/ConnectorsSection';
import ActionsSection from './agent-form-sections/ActionsSection';
import MCPServersSection from './agent-form-sections/MCPServersSection';
import MemorySettingsSection from './agent-form-sections/MemorySettingsSection';
import ModelSettingsSection from './agent-form-sections/ModelSettingsSection';
import PromptsGoalsSection from './agent-form-sections/PromptsGoalsSection';
import AdvancedSettingsSection from './agent-form-sections/AdvancedSettingsSection';
import ExportSection from './agent-form-sections/ExportSection';
import FiltersSection from './agent-form-sections/FiltersSection';
import ServerWalletsSection from './agent-form-sections/ServerWalletsSection';
import useIsMobile from "../hooks/useMobileDetect";

const AgentForm = ({
  isEdit = false,
  agent,
  formData,
  setFormData,
  onSubmit,
  loading = false,
  submitButtonText,
  isGroupForm = false,
  noFormWrapper = false,
  metadata = null,
  id,
  setAgent,
}) => {
  const navigate = useNavigate();
  const [activeSection, setActiveSection] = useState(
    isGroupForm ? "model-section" : "basic-section"
  );
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const dropdownRef = useRef(null);
  const isMobile = useIsMobile();

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target)) {
        setIsDropdownOpen(false);
      }
    };

    if (isDropdownOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => {
        document.removeEventListener('mousedown', handleClickOutside);
      };
    }
  }, [isDropdownOpen]);

  // Navigation options for easier management
  const getNavigationOptions = () => {
    const options = [];
    
    if (!isGroupForm) {
      options.push({
        id: "basic-section",
        icon: "fas fa-info-circle",
        label: isMobile ? "Basic Info" : "Basic Information"
      });
    }
    
    options.push(
      {
        id: "model-section",
        icon: "fas fa-brain",
        label: isMobile ? "Model" : "Model Settings"
      },
      {
        id: "connectors-section",
        icon: "fas fa-plug",
        label: "Connectors"
      },
      {
        id: "filters-section",
        icon: "fas fa-shield",
        label: "Filters & Triggers",
        hasTag: true
      },
      {
        id: "actions-section",
        icon: "fas fa-bolt",
        label: "Actions"
      },
      {
        id: "mcp-section",
        icon: "fas fa-server",
        label: isMobile ? "MCP" : "MCP Servers"
      },
      {
        id: "memory-section",
        icon: "fas fa-memory",
        label: isMobile ? "Memory" : "Memory Settings"
      },
      {
        id: "prompts-section",
        icon: "fas fa-comment-alt",
        label: "Prompts & Goals"
      },
      {
        id: "advanced-section",
        icon: "fas fa-cogs",
        label: isMobile ? "Advanced" : "Advanced Settings"
      }
    );

    if (isEdit && formData.server_wallets_enabled) {
      options.push({
        id: "server-wallets-section",
        icon: "fas fa-wallet",
        label: "Server Wallets"
      });
    }

    if (isEdit) {
      options.push({
        id: "export-section",
        icon: "fas fa-file-export",
        label: isMobile ? "Export" : "Export Data"
      });
    }

    return options;
  };

  // Handle input changes
  const handleInputChange = (e) => {
    const { name, value, type, checked } = e.target.name.target;

    // Convert value to number if it's a number input
    const processedValue = type === "number" ? Number(value) : value;

    if (name.includes(".")) {
      const [parent, child] = name.split(".");
      setFormData({
        ...formData,
        [parent]: {
          ...formData[parent],
          [child]: type === "checkbox" ? checked : processedValue,
        },
      });
    } else {
      setFormData({
        ...formData,
        [name]: type === "checkbox" ? checked : processedValue,
      });
    }
  };

  // Handle form submission
  const handleSubmit = async (e) => {
    e.preventDefault();
    if (onSubmit) {
      onSubmit(formData);
    }
  };

  // Handle navigation between sections
  const handleSectionChange = (section) => {
    setActiveSection(section);
  };

  // Handle connector change (simplified)
  const handleConnectorChange = (index, updatedConnector) => {
    const updatedConnectors = [...formData.connectors];
    updatedConnectors[index] = updatedConnector;
    setFormData({
      ...formData,
      connectors: updatedConnectors,
    });
  };

  // Handle adding a connector
  const handleAddConnector = () => {
    setFormData({
      ...formData,
      connectors: [...(formData.connectors || []), { type: "", config: "{}" }],
    });
  };

  // Handle removing a connector
  const handleRemoveConnector = (index) => {
    const updatedConnectors = [...formData.connectors];
    updatedConnectors.splice(index, 1);
    setFormData({
      ...formData,
      connectors: updatedConnectors,
    });
  };

  const handleAddDynamicPrompt = () => {
    setFormData({
      ...formData,
      dynamicPrompts: [
        ...(formData.dynamicPrompts || []),
        { type: "", config: "{}" },
      ],
    });
  };

  const handleRemoveDynamicPrompt = (index) => {
    const updatedDynamicPrompts = [...formData.dynamicPrompts];
    updatedDynamicPrompts.splice(index, 1);
    setFormData({
      ...formData,
      dynamicPrompts: updatedDynamicPrompts,
    });
  };

  const handleDynamicPromptChange = (index, updatedPrompt) => {
    const updatedPrompts = [...formData.dynamicPrompts];
    updatedPrompts[index] = updatedPrompt;
    setFormData({
      ...formData,
      dynamicPrompts: updatedPrompts,
    });
  };

  // Handle adding an MCP server
  const handleAddMCPServer = () => {
    setFormData({
      ...formData,
      mcp_servers: [...(formData.mcp_servers || []), { url: "", token: "" }],
    });
  };

  // Handle removing an MCP server
  const handleRemoveMCPServer = (index) => {
    const updatedMCPServers = [...formData.mcp_servers];
    updatedMCPServers.splice(index, 1);
    setFormData({
      ...formData,
      mcp_servers: updatedMCPServers,
    });
  };

  // Handle MCP server change
  const handleMCPServerChange = (index, field, value) => {
    const updatedMCPServers = [...formData.mcp_servers];
    updatedMCPServers[index] = {
      ...updatedMCPServers[index],
      [field]: value,
    };
    setFormData({
      ...formData,
      mcp_servers: updatedMCPServers,
    });
  };

  if (loading) {
    return (
      <div className="loading-container">
        <div className="spinner"></div>
      </div>
    );
  }

  const navigationOptions = getNavigationOptions();
  const currentSection = navigationOptions.find(option => option.id === activeSection);

  return (
    <div className="agent-form-container">
      {/* Mobile Dropdown Navigation */}
      {isMobile ? (
        <div className="wizard-mobile-dropdown" ref={dropdownRef}>
          <div 
            className="wizard-dropdown-trigger"
            onClick={() => setIsDropdownOpen(!isDropdownOpen)}
          >
            <div className="wizard-dropdown-trigger-content">
              <i className={currentSection?.icon || "fas fa-cogs"}></i>
              <span>{currentSection?.label || "Select Section"}</span>
            </div>
            <i className={`fas fa-chevron-${isDropdownOpen ? 'up' : 'down'} dropdown-arrow`}></i>
          </div>
          {isDropdownOpen && (
            <div className="wizard-dropdown-menu">
              {navigationOptions.map((option) => (
                <div
                  key={option.id}
                  className={`wizard-dropdown-item ${
                    activeSection === option.id ? "active" : ""
                  }`}
                  onClick={() => {
                    handleSectionChange(option.id);
                    setIsDropdownOpen(false);
                  }}
                >
                  <i className={option.icon}></i>
                  <span>{option.label}</span>
                  {option.hasTag && <span className="advanced-tag">Advanced</span>}
                </div>
              ))}
            </div>
          )}
        </div>
      ) : (
        /* Desktop Sidebar */
        <div className="wizard-sidebar">
          <ul className="wizard-nav">
            {navigationOptions.map((option) => (
              <li
                key={option.id}
                className={`wizard-nav-item ${
                  activeSection === option.id ? "active" : ""
                }`}
                onClick={() => handleSectionChange(option.id)}
              >
                <i className={option.icon}></i>
                {option.label}
                {option.hasTag && <span className="advanced-tag">Advanced</span>}
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Form Content */}
      <div className="form-content-area">
        {noFormWrapper ? (
          <div className="agent-form">
            {/* Form Sections */}
            <div
              style={{
                display: activeSection === "basic-section" ? "block" : "none",
              }}
            >
              <BasicInfoSection
                formData={formData}
                handleInputChange={handleInputChange}
                isEdit={isEdit}
                isGroupForm={isGroupForm}
                metadata={metadata}
              />
            </div>

            <div
              style={{
                display: activeSection === "model-section" ? "block" : "none",
              }}
            >
              <ModelSettingsSection
                formData={formData}
                handleInputChange={handleInputChange}
                metadata={metadata}
              />
            </div>

            <div
              style={{
                display:
                  activeSection === "connectors-section" ? "block" : "none",
              }}
            >
              <ConnectorsSection
                formData={formData}
                handleAddConnector={handleAddConnector}
                handleRemoveConnector={handleRemoveConnector}
                handleConnectorChange={handleConnectorChange}
                metadata={metadata}
              />
            </div>

            <div style={{ display: activeSection === 'filters-section' ? 'block' : 'none' }}>
              <FiltersSection formData={formData} setFormData={setFormData} metadata={metadata} />
            </div>

            <div style={{ display: activeSection === 'actions-section' ? 'block' : 'none' }}>
              <ActionsSection formData={formData} setFormData={setFormData} metadata={metadata} />
            </div>

            <div
              style={{
                display: activeSection === "mcp-section" ? "block" : "none",
              }}
            >
              <MCPServersSection
                formData={formData}
                handleAddMCPServer={handleAddMCPServer}
                handleRemoveMCPServer={handleRemoveMCPServer}
                handleMCPServerChange={handleMCPServerChange}
              />
            </div>

            <div
              style={{
                display: activeSection === "memory-section" ? "block" : "none",
              }}
            >
              <MemorySettingsSection
                formData={formData}
                handleInputChange={handleInputChange}
                metadata={metadata}
              />
            </div>

            <div
              style={{
                display: activeSection === "prompts-section" ? "block" : "none",
              }}
            >
              <PromptsGoalsSection
                formData={formData}
                handleInputChange={handleInputChange}
                isGroupForm={isGroupForm}
                metadata={metadata}
                onAddPrompt={handleAddDynamicPrompt}
                onRemovePrompt={handleRemoveDynamicPrompt}
                handleDynamicPromptChange={handleDynamicPromptChange}
              />
            </div>

            <div
              style={{
                display:
                  activeSection === "advanced-section" ? "block" : "none",
              }}
            >
              <AdvancedSettingsSection
                formData={formData}
                handleInputChange={handleInputChange}
                metadata={metadata}
              />
            </div>

            {isEdit && (
              <>
                <div
                  style={{
                    display:
                      activeSection === "server-wallets-section" ? "block" : "none",
                  }}
                >
                  <ServerWalletsSection agent={agent} fetchServerWallets={activeSection === "server-wallets-section"} agentId={id} setAgent={setAgent} />
                </div>
                <div
                  style={{
                    display:
                      activeSection === "export-section" ? "block" : "none",
                  }}
                >
                  <ExportSection id={id} />
                </div>
              </>
            )}
          </div>
        ) : (
          <form className="agent-form" onSubmit={handleSubmit} noValidate>
            {/* Form Sections */}
            <div
              style={{
                display: activeSection === "basic-section" ? "block" : "none",
              }}
            >
              <BasicInfoSection
                formData={formData}
                handleInputChange={handleInputChange}
                isEdit={isEdit}
                isGroupForm={isGroupForm}
                metadata={metadata}
              />
            </div>

            <div
              style={{
                display: activeSection === "model-section" ? "block" : "none",
              }}
            >
              <ModelSettingsSection
                formData={formData}
                handleInputChange={handleInputChange}
                metadata={metadata}
              />
            </div>

            <div
              style={{
                display:
                  activeSection === "connectors-section" ? "block" : "none",
              }}
            >
              <ConnectorsSection
                formData={formData}
                handleAddConnector={handleAddConnector}
                handleRemoveConnector={handleRemoveConnector}
                handleConnectorChange={handleConnectorChange}
                metadata={metadata}
              />
            </div>

            <div style={{ display: activeSection === 'filters-section' ? 'block' : 'none' }}>
              <FiltersSection formData={formData} setFormData={setFormData} metadata={metadata} />
            </div>

            <div style={{ display: activeSection === 'actions-section' ? 'block' : 'none' }}>
              <ActionsSection formData={formData} setFormData={setFormData} metadata={metadata} />
            </div>

            <div
              style={{
                display: activeSection === "mcp-section" ? "block" : "none",
              }}
            >
              <MCPServersSection
                formData={formData}
                handleAddMCPServer={handleAddMCPServer}
                handleRemoveMCPServer={handleRemoveMCPServer}
                handleMCPServerChange={handleMCPServerChange}
              />
            </div>

            <div
              style={{
                display: activeSection === "memory-section" ? "block" : "none",
              }}
            >
              <MemorySettingsSection
                formData={formData}
                handleInputChange={handleInputChange}
                metadata={metadata}
              />
            </div>

            <div
              style={{
                display: activeSection === "prompts-section" ? "block" : "none",
              }}
            >
              <PromptsGoalsSection
                formData={formData}
                handleInputChange={handleInputChange}
                isGroupForm={isGroupForm}
                metadata={metadata}
                onAddPrompt={handleAddDynamicPrompt}
                onRemovePrompt={handleRemoveDynamicPrompt}
                handleDynamicPromptChange={handleDynamicPromptChange}
              />
            </div>

            <div
              style={{
                display:
                  activeSection === "advanced-section" ? "block" : "none",
              }}
            >
              <AdvancedSettingsSection
                formData={formData}
                handleInputChange={handleInputChange}
                metadata={metadata}
              />
            </div>

            {isEdit && (
              <>
                <div
                  style={{
                    display:
                      activeSection === "server-wallets-section" ? "block" : "none",
                  }}
                >
                 <ServerWalletsSection agent={agent} fetchServerWallets={activeSection === "server-wallets-section"} agentId={id} setAgent={setAgent} />
                </div>
                <div
                  style={{
                    display:
                      activeSection === "export-section" ? "block" : "none",
                  }}
                >
                  <ExportSection id={id} />
                </div>
              </>
            )}

            {/* Form Controls */}
            <div
              className="form-actions"
              style={{
                display: "flex",
                gap: "1rem",
                justifyContent: "flex-end",
              }}
            >
              <button
                type="button"
                className="primary-btn"
                onClick={() => navigate("/agents")}
              >
                <i className="fas fa-times"></i> Cancel
              </button>
              <button type="submit" className="primary-btn" disabled={loading}>
                <i className="fas fa-save"></i>{" "}
                {submitButtonText || (isEdit ? "Update Agent" : "Create Agent")}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
};

export default AgentForm;
