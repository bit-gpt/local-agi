import React, { useMemo } from "react";
import ActionForm from "../ActionForm";
import PlatformOAuthButtons from "../PlatformOAuthButtons";
import { PLATFORM_INFO } from "../../utils/platformActions";
import { useOutletContext } from "react-router-dom";

/**
 * ActionsSection component for the agent form
 */
const ActionsSection = ({ formData, setFormData, metadata }) => {
  const { showToast } = useOutletContext();

  // Memoize actions array to prevent unnecessary re-renders
  const actions = useMemo(() => formData.actions || [], [formData.actions]);

  // Handle action change
  const handleActionChange = (index, updatedAction) => {
    const updatedActions = [...(formData.actions || [])];
    updatedActions[index] = updatedAction;
    setFormData({
      ...formData,
      actions: updatedActions,
    });
  };

  // Handle action removal
  const handleActionRemove = (index) => {
    const updatedActions = [...(formData.actions || [])].filter(
      (_, i) => i !== index
    );
    setFormData({
      ...formData,
      actions: updatedActions,
    });
  };

  // Handle adding an action
  const handleAddAction = () => {
    setFormData({
      ...formData,
      actions: [...(formData.actions || []), { name: "", config: "{}" }],
    });
  };

  const handleOAuthChange = (platform, status) => {
    console.log(`OAuth status changed for ${platform}:`, status);
    if(status.connected){
      showToast(`${PLATFORM_INFO[platform]?.name} connected successfully`, "success");
    } else {
      showToast(`${PLATFORM_INFO[platform]?.name} disconnected successfully`, "success");
    }
    // Optionally trigger a refresh or update UI state
  };

  return (
    <div className="actions-section">
      <h3 className="section-title">Actions</h3>
      <p className="section-description">
        Configure actions that the agent can perform.
      </p>

      <PlatformOAuthButtons
        actions={actions}
        onOAuthChange={handleOAuthChange}
      />

      <ActionForm
        actions={actions}
        onChange={handleActionChange}
        onRemove={handleActionRemove}
        onAdd={handleAddAction}
        fieldGroups={metadata?.actions || []}
      />

    </div>
  );
};

export default ActionsSection;
