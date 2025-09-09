import { createContext, useContext, useState, useCallback } from "react";
import { publicActions } from "viem";
import { connect, disconnect } from "wagmi/actions";
import { getWalletClient } from "@wagmi/core";
import { config } from "../config/wagmi";
import { getConnector, getChainConfig } from "../utils/connectorUtils";
import { handleChainSwitch } from "../utils/chainUtils";
import {
  validateWalletEnvironment,
  validateConnectionResult,
  validateWalletClient,
  validateWalletAddress,
  validateChainId,
} from "../utils/errorUtils";
import { formatError } from "../../utils/errorFormatter";
import { clearWalletStorage } from "../utils/storageUtils";

const EvmWalletContext = createContext({
  walletClient: null,
  connectedAddress: "",
  connectWallet: async () => {},
  disconnectWallet: async () => {},
});

export const useEvmWallet = () => useContext(EvmWalletContext);

export function EvmWalletProvider({ children }) {
  const [walletClient, setWalletClient] = useState(null);
  const [connectedAddress, setConnectedAddress] = useState("");

  const connectWallet = useCallback(async (walletType, networkId = "bsc") => {
    const { targetChain, networkName } = getChainConfig(networkId);
    console.log("Start Connecting", targetChain, networkName);

    try {
      if (walletClient || connectedAddress) {
        try {
          await disconnect(config);
          clearWalletStorage();
        } catch (disconnectError) {
          console.warn(
            "Failed to disconnect previous wallet:",
            disconnectError
          );
        }
        setWalletClient(null);
        setConnectedAddress("");
      }

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
          config,
          account: result.accounts[0],
          connector,
          result,
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
    } catch (error) {
      const errorInfo = formatError(error, { networkName });
      throw errorInfo.message;
    }
  }, []);

  const disconnectWallet = useCallback(async () => {
    try {
      disconnect(config).catch((err) => {
        console.warn("Disconnect from wagmi failed:", err);
      });

      setWalletClient(null);
      setConnectedAddress("");

      clearWalletStorage();
    } catch (error) {
      setWalletClient(null);
      setConnectedAddress("");
      throw error;
    }
  }, []);

  const contextValue = {
    walletClient,
    connectedAddress,
    connectWallet,
    disconnectWallet,
  };

  return (
    <EvmWalletContext.Provider value={contextValue}>
      {children}
    </EvmWalletContext.Provider>
  );
}
