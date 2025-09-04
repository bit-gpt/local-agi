import BscNetwork from "/coins/bnb.svg";
import SolanaNetwork from "/coins/sol.svg";
import SolCoin from "/coins/sol.svg";
import BnbCoin from "/coins/bnb.svg";
import UsdcCoin from "/coins/usdc.svg";
import UsdtCoin from "/coins/usdt.svg";
import EthCoin from "/coins/eth.svg";
import BaseNetwork from "/coins/base.svg";

/**
 * Get the appropriate coin icon based on token symbol
 */
function getCoinIcon(tokenSymbol) {
  switch (tokenSymbol) {
    case "BNB":
      return BnbCoin;
    case "SOL":
      return SolCoin;
    case "USDC":
      return UsdcCoin;
    case "USDT":
      return UsdtCoin;
    case "ETH":
      return EthCoin;
    default:
      return "";
  }
}

/**
 * Create a coin object from a payment requirement
 */
function createCoinFromRequirement(requirement) {
  const tokenSymbol = requirement.tokenSymbol || "Missing token metadata";
  
  return {
    id: requirement.tokenAddress || "",
    name: tokenSymbol,
    icon: getCoinIcon(tokenSymbol),
    paymentMethod: requirement,
  };
}

/**
 * Converts payment details to array if needed
 */
export function normalizePaymentMethods(paymentRequirements) {
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
) {
  if (!paymentRequirements.length) {
    return [];
  }

  const compatibleMethods = paymentRequirements.filter((requirement) => {
    if (networkId === "solana") {
      return requirement.namespace === "solana";
    } else if (networkId === "bsc") {
      return requirement.networkId === "56"; 
    } else if (networkId === "base") {
      return requirement.networkId === "8453"; 
    }
    return false;
  });
  return compatibleMethods;
}

/**
 * Generate network and coin options from payment requirements
 */
export function generateAvailableNetworks(paymentRequirements) {
  const networkGroups = {};

  paymentRequirements.forEach((requirement) => {
    const networkId = requirement.namespace || "";
    if (!networkGroups[networkId]) {
      networkGroups[networkId] = [];
    }
    networkGroups[networkId].push(requirement);
  });

  const networks = [];
  let containsBSC = false;
  let containsBASE = false;

  if (networkGroups["evm"] && networkGroups["evm"].length > 0) {
    networkGroups["evm"].forEach((requirement) => {
      if (
        containsBSC === false &&
        (requirement.networkId === "56")
      ) {
        containsBSC = true;
      }

      if (
        containsBASE === false &&
        (requirement.networkId === "8453")
      ) {
        containsBASE = true;
      }
    });

    if (containsBSC) {
      const bscCoins = networkGroups["evm"]
        .filter((requirement) => 
          requirement.networkId === "56"
        )
        .map(createCoinFromRequirement);

      networks.push({
        id: "bsc",
        name: "Binance Smart Chain",
        icon: BscNetwork,
        coins: bscCoins,
      });
    }

    if (containsBASE) {
      const baseCoins = networkGroups["evm"]
        .filter((requirement) => 
          requirement.networkId === "8453"
        )
        .map(createCoinFromRequirement);

      networks.push({
        id: "base",
        name: "Base",
        icon: BaseNetwork,
        coins: baseCoins,
      });
    }
  }

  if (networkGroups["solana"] && networkGroups["solana"].length > 0) {
    const solanaCoins = networkGroups["solana"].map((requirement) => {
      const isNative = requirement.tokenAddress === "11111111111111111111111111111111";
      const tokenSymbol = requirement.tokenSymbol || (isNative ? "SOL" : "Missing token metadata");
      
      const modifiedRequirement = {
        ...requirement,
        tokenSymbol: tokenSymbol
      };
      
      return createCoinFromRequirement(modifiedRequirement);
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
