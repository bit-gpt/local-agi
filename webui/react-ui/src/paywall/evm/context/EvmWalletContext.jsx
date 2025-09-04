import { createContext, useContext, useState, useCallback } from "react";
import { publicActions } from "viem";
import { connect, disconnect } from "wagmi/actions";
import { getWalletClient } from "@wagmi/core";
import { config } from "../config/wagmi";
import { getConnector, getChainConfig } from "../utils/connectorUtils";
import { handleChainSwitch } from "../utils/chainUtils";
import { 
  getWalletErrorMessage, 
  validateWalletEnvironment, 
  validateConnectionResult,
  validateWalletClient,
  validateWalletAddress,
  validateChainId
} from "../utils/errorUtils";
import { clearWalletStorage } from "../utils/storageUtils";

const EvmWalletContext = createContext({
  walletClient: null,
  connectedAddress: "",
  statusMessage: "",
  setStatusMessage: () => {},
  connectWallet: async () => {},
  disconnectWallet: async () => {},
});

export const useEvmWallet = () => useContext(EvmWalletContext);

export function EvmWalletProvider({ children }) {
  const [walletClient, setWalletClient] = useState(null);
  const [connectedAddress, setConnectedAddress] = useState("");
  const [statusMessage, setStatusMessage] = useState("");

  const connectWallet = useCallback(async (walletType, networkId = "bsc") => {
    const { targetChain, networkName } = getChainConfig(networkId);
    
    try {
      setStatusMessage("Connecting wallet...");

      // Disconnect any existing wallet first to avoid conflicts
      if (walletClient || connectedAddress) {
        setStatusMessage("Disconnecting previous wallet...");
        try {
          await disconnect(config);
          clearWalletStorage();
        } catch (disconnectError) {
          console.warn("Failed to disconnect previous wallet:", disconnectError);
        }
        // Clear local state
        setWalletClient(null);
        setConnectedAddress("");
      }

      setStatusMessage("Connecting wallet...");
      validateWalletEnvironment();

      const connector = getConnector(walletType);

      let result = await connect(config, { connector });
      validateConnectionResult(result);

      const initialClient = await getWalletClient(config, {
        account: result.accounts[0],
      });
      validateWalletClient(initialClient);

      result = await handleChainSwitch({
        initialClient,
        targetChain,
        networkName,
        setStatusMessage,
        config,
        account: result.accounts[0],
        connector,
        result
      });

      validateConnectionResult(result);

      const baseClient = await getWalletClient(config, {
        account: result.accounts[0],
        chainId: targetChain.id,
      });
      validateWalletClient(baseClient);

      const client = baseClient.extend(publicActions);

      const [address] = await client.getAddresses();
      validateWalletAddress(address);

      const chainId = await client.getChainId();
      validateChainId(chainId, targetChain.id, networkName);

      setWalletClient(client);
      setConnectedAddress(address);
      setStatusMessage("Wallet connected!");
      
    } catch (error) {
      console.error("EvmWalletContext - Connection error:", error);
      const message = getWalletErrorMessage(error, networkName);
      setStatusMessage(message);
    }
  }, []);

  const disconnectWallet = useCallback(async () => {
    try {
      disconnect(config).catch((err) => {
        console.warn("Disconnect from wagmi failed:", err);
      });

      setWalletClient(null);
      setConnectedAddress("");
      setStatusMessage("Wallet disconnected");

      clearWalletStorage();
      
    } catch (error) {
      console.error("EvmWalletContext - Disconnect error:", error);
      setWalletClient(null);
      setConnectedAddress("");
      setStatusMessage("Wallet disconnected (with errors)");
    }
  }, []);

  const contextValue = {
    walletClient,
    connectedAddress,
    statusMessage,
    setStatusMessage,
    connectWallet,
    disconnectWallet,
  };

  return (
    <EvmWalletContext.Provider value={contextValue}>
      {children}
    </EvmWalletContext.Provider>
  );
}
