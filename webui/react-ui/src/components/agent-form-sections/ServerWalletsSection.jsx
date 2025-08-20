import React, { useEffect, useState } from "react";
import { useOutletContext } from "react-router-dom";
import ServerWalletCard from "../ServerWalletCard";
import PayLimitsModal from "../PayLimitsModal";
import { agentApi } from "../../utils/api";

/**
 * ServerWalletsSection component for the agent form
 * Displays server wallet addresses and balances for different blockchain networks
 */
const ServerWalletsSection = ({ fetchServerWallets, agent, agentId, setAgent }) => {
  const { showToast } = useOutletContext();
  const [serverWallets, setServerWallets] = useState([]);
  const [loading, setLoading] = useState(true);
  const [isPayLimitsModalOpen, setIsPayLimitsModalOpen] = useState(false);

  useEffect(() => {
    if (fetchServerWallets) {
      const fetchServerWalletsAsync = async () => {
        setLoading(true);
        try {
          const response = await agentApi.getAgentServerWallets(agentId);
          setServerWallets(response?.server_wallets || []);
        } catch (err) {
          console.error("Error fetching agent server wallets:", err);
          showToast("Failed to load agent server wallets", "error");
        } finally {
          setLoading(false);
        }
      };
      fetchServerWalletsAsync();
    }
  }, [fetchServerWallets]);

  return (
    <div className="server-wallets-section">
      <div className="section-header">
        <div>
          <h3 className="section-title">Server Wallets</h3>
          <p className="section-description">
            View server wallet addresses and balances
          </p>
        </div>
        <button
          className="btn-outline"
          onClick={() => setIsPayLimitsModalOpen(true)}
          type="button"
        >
          Edit Limits
        </button>
      </div>

      {loading ? (
        <div className="centered-loading">
          <div className="spinner-primary"></div>
        </div>
      ) : (
        serverWallets?.length > 0 && (
          <div className="wallets-grid">
            {serverWallets.map((serverWallet) => (
              <ServerWalletCard key={serverWallet.address} serverWallet={serverWallet} />
            ))}
          </div>
        )
      )}

      <PayLimitsModal
        isOpen={isPayLimitsModalOpen}
        onClose={() => setIsPayLimitsModalOpen(false)}
        agent={agent}
        agentId={agentId}
        setAgent={setAgent}
      />
    </div>
  );
};

export default ServerWalletsSection;
