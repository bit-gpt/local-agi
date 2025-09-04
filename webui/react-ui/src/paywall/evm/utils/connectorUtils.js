import {
  injected,
  metaMask,
  coinbaseWallet,
  walletConnect,
} from "wagmi/connectors";
import { bsc, base } from "viem/chains";

/**
 * Gets the appropriate connector based on wallet type
 * @param {string} walletType - The type of wallet to connect to
 * @returns {Object} The wagmi connector instance
 */
export function getConnector(walletType) {
  switch (walletType) {
    case "metamask":
      return metaMask();
    case "coinbase":
      return coinbaseWallet();
    case "rabby":
      return injected({ shimDisconnect: true, target: "rabby" });
    case "phantom":
      return injected({ shimDisconnect: true, target: "phantom" });
    case "trust":
    case "walletconnect":
      return walletConnect({
        projectId: "233c440b08a2b78d6b3e76370b979bed",
      });
    default:
      return injected({ shimDisconnect: true });
  }
}

/**
 * Gets chain configuration based on network ID
 * @param {string} networkId - The network identifier
 * @returns {Object} Object containing chain and network name
 */
export function getChainConfig(networkId) {
  const targetChain = networkId === "base" ? base : bsc;
  const networkName = networkId === "base" ? "Base" : "Binance Smart Chain (BSC)";
  
  return { targetChain, networkName };
}
