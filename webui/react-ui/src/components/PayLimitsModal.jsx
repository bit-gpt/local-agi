import React, { useState, useEffect } from "react";
import { useOutletContext } from "react-router-dom";
import { agentApi } from "../utils/api";

const PayLimitsModal = ({ isOpen, onClose, agent, agentId, setAgent }) => {
  const { showToast } = useOutletContext();
  const [payLimits, setPayLimits] = useState({});
  const [loading, setLoading] = useState(false);
  const [errors, setErrors] = useState({});

  const tokenInfo = {
    ETH: { min: 0.0001, max: 0.02 },
    BNB: { min: 0.0001, max: 0.2 },
    SOL: { min: 0.0001, max: 1 },
    USDC: { min: 0.1, max: 100 },
    USDT: { min: 0.1, max: 100 },
  };

  useEffect(() => {
    if (isOpen && agent) {
      const currentPayLimits = agent.pay_limits || {};
      const initialLimits = {};
      Object.keys(tokenInfo).forEach(token => {
        initialLimits[token] = currentPayLimits[token] || "";
      });
      setPayLimits(initialLimits);
      setErrors({});
    }
  }, [isOpen, agent]);

  const handleInputChange = (token, value) => {
    setPayLimits(prev => ({
      ...prev,
      [token]: value
    }));
    
    if (errors[token]) {
      setErrors(prev => ({
        ...prev,
        [token]: null
      }));
    }
  };

  const validateLimits = () => {
    const newErrors = {};
    
    Object.entries(payLimits).forEach(([token, value]) => {
      if (value !== "" && value !== null && value !== undefined) {
        const numValue = parseFloat(value);
        const tokenLimits = tokenInfo[token];
        
        if (isNaN(numValue)) {
          newErrors[token] = "Please enter a valid numeric value";
        } else if (numValue < 0) {
          newErrors[token] = "Value must be positive";
        } else if (numValue > 0 && numValue < tokenLimits.min) {
          newErrors[token] = `Minimum allowed value is ${tokenLimits.min}`;
        } else if (numValue > tokenLimits.max) {
          newErrors[token] = `Maximum allowed value is ${tokenLimits.max}`;
        }
      }
    });
    
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSave = async () => {
    
    if (!validateLimits()) {
      return;
    }

    setLoading(true);
    try {
      const limitsToSave = {};
      Object.entries(payLimits).forEach(([token, value]) => {
        if (value !== "" && value !== null && value !== undefined) {
          const numValue = parseFloat(value);
          if (!isNaN(numValue) && numValue > 0) {
            limitsToSave[token] = numValue;
          }
        }
      });

      await agentApi.updateAgentPayLimits(agentId, limitsToSave);
      showToast("Pay limits updated successfully", "success");
      setAgent(prevAgent => ({
        ...prevAgent,
        pay_limits: limitsToSave
      }));
      showToast("Pay limits updated successfully", "success");
      onClose();
    } catch (err) {
      showToast(err?.message || "Failed to update pay limits", "error");
    } finally {
      setLoading(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Edit Pay Limits</h3>
          <button className="modal-close" onClick={onClose}>
            ×
          </button>
        </div>

        <div className="modal-body">
          <p className="modal-description">
            Agent will not spend more than the limits you set.
          </p>

          <div className="pay-limits-grid">
            {Object.entries(tokenInfo).map(([token, info]) => (
              <div key={token} className="pay-limit-field">
                <div className="pay-limit-field-header">
                  <img 
                    src={`/app/coins/${token.toLowerCase()}.svg`} 
                    alt={token}
                    className="token-icon"
                    onError={(e) => {
                      e.target.style.display = 'none';
                    }}
                  />
                  <label htmlFor={`pay-limit-${token.toLowerCase()}`} className="field-label">
                    {token}
                  </label>
                </div>
                <input
                  id={`pay-limit-${token.toLowerCase()}`}
                  type="number"
                  value={payLimits[token] || ""}
                  onChange={(e) => handleInputChange(token, e.target.value)}
                  className={`form-input ${errors[token] ? 'error' : ''}`}
                />
                {errors[token] && (
                  <span className="error-text">{errors[token]}</span>
                )}
              </div>
            ))}
          </div>
        </div>

        <div className="modal-footer">
          <div className="modal-actions">
            <button 
              type="button"
              className="action-btn" 
              onClick={onClose}
              disabled={loading}
            >
              <i className="fas fa-times"></i> 
              Cancel
            </button>
            <button 
              type="button"
              className="action-btn" 
              onClick={handleSave}
              disabled={loading}
            >
              <i className="fas fa-save"></i>{" "}
              {loading ? "Saving..." : "Save Changes"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default PayLimitsModal;
