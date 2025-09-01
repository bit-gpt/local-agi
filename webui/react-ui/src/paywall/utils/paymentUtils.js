
import BscNetwork from "/coins/bnb.svg";
import SolanaNetwork from "/coins/sol.svg";
import SolCoin from "/coins/sol.svg";
import BnbCoin from "/coins/bnb.svg";
import UsdcCoin from "/coins/usdc.svg";
import UsdtCoin from "/coins/usdt.svg";

/**
 * Converts payment details to array if needed
 */
export function normalizePaymentMethods(
  paymentRequirements
) {
  if (!paymentRequirements) return [];

  return Array.isArray(paymentRequirements)
    ? paymentRequirements
    : [paymentRequirements];
}

/**
 * Get compatible payment methods for the selected network
 */
export function getCompatiblePaymentRequirements(
  paymentRequirements,
  networkId
){
  if (!paymentRequirements.length) {
    return [];
  }

  // Filter methods by matching networkId
  const compatibleMethods = paymentRequirements.filter((requirement) => {
    if (networkId === "solana") {
      return requirement.namespace === "solana";
    } else if (networkId === "bsc") {
      return requirement.networkId === "56"; // BSC mainnet chain ID
    }
    return false;
  });
  return compatibleMethods;
}

/**
 * Generate network and coin options from payment requirements
 */
export function generateAvailableNetworks(
  paymentRequirements
){
  // Group payment methods by network
  const networkGroups = {};

  paymentRequirements.forEach((requirement) => {
    const networkId = requirement.namespace || "";
    if (!networkGroups[networkId]) {
      networkGroups[networkId] = [];
    }
    networkGroups[networkId].push(requirement);
  });

  // Map to network structure
  const networks = [];
  let icon;

  // Handle EVM networks
  if (networkGroups["evm"] && networkGroups["evm"].length > 0) {
    const evmCoins = networkGroups["evm"].map((requirement) => {
      // Determine coin type from token type and address
      const isNative =
        requirement.tokenAddress ===
        "0x0000000000000000000000000000000000000000";
      const tokenSymbol =
        requirement.tokenSymbol ||
        (isNative ? "BNB" : "Missing token metadata");

      switch (tokenSymbol) {
        case "BNB":
          icon = BnbCoin;
          break;
        case "SOL":
          icon = SolCoin;
          break;
        case "USDC":
          icon = UsdcCoin;
          break;
        case "USDT":
          icon = UsdtCoin;
          break;
        default:
          icon = "";
      }

      return {
        id: requirement.tokenAddress || "",
        name: tokenSymbol,
        icon: icon,
        paymentMethod: requirement, // Store the original payment method for reference
      };
    });

    networks.push({
      id: "bsc",
      name: "Binance Smart Chain",
      icon: BscNetwork,
      coins: evmCoins,
    });
  }

  // Handle Solana networks
  if (networkGroups["solana"] && networkGroups["solana"].length > 0) {
    const solanaCoins = networkGroups["solana"].map((requirement) => {
      // Determine coin type from token type and address
      const isNative =
        requirement.tokenAddress === "11111111111111111111111111111111";
      const tokenSymbol =
        requirement.tokenSymbol ||
        (isNative ? "SOL" : "Missing token metadata");

      switch (tokenSymbol) {
        case "BNB":
          icon = BnbCoin;
          break;
        case "SOL":
          icon = SolCoin;
          break;
        case "USDC":
          icon = UsdcCoin;
          break;
        case "USDT":
          icon = UsdtCoin;
          break;
        default:
          icon = "";
      }

      return {
        id: requirement.tokenAddress || "",
        name: tokenSymbol,
        icon: icon,
        paymentMethod: requirement, // Store the original payment method for reference
      };
    });

    networks.push({
      id: "solana",
      name: "Solana",
      icon: SolanaNetwork,
      coins: solanaCoins,
    });
  }

  return networks;
}