import React from "react";
import Paywall from "./Paywall";

const PaymentModal = ({ isOpen, onClose, paymentRequirements }) => {
  if (!isOpen) return null;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Pay from your Wallet</h3>
          <button className="modal-close" onClick={onClose}>
            ×
          </button>
        </div>

        <div className="modal-body">
          <Paywall paymentRequirements={paymentRequirements} />
        </div>

        {/* <div className="modal-footer">
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
        </div> */}
      </div>
    </div>
  );
};

export default PaymentModal;
