import React from "react";
import Paywall from "./Paywall";

const PaymentModal = ({
  isOpen,
  onClose,
  paymentRequirements,
  onPaymentSuccess,
}) => {
  if (!isOpen) return null;

  return (
    <div className="modal-overlay">
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Pay from your Wallet</h3>
          <button className="modal-close" onClick={onClose}>
            ×
          </button>
        </div>

        <div className="modal-body">
          <Paywall
            paymentRequirements={paymentRequirements}
            onPaymentSuccess={onPaymentSuccess}
          />
        </div>
      </div>
    </div>
  );
};

export default PaymentModal;
