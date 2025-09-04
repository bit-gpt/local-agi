/**
 * Utility functions for handling wallet connection errors
 */

/**
 * Maps error messages to user-friendly messages
 * @param {Error} error - The error object
 * @param {string} networkName - The network name for context
 * @returns {string} User-friendly error message
 */
export function getWalletErrorMessage(error, networkName = "the network") {
  if (!(error instanceof Error)) {
    return "Failed to connect wallet";
  }

  const message = error.message;

  if (message.includes("Unsupported chain")) {
    return `This wallet doesn't support ${networkName}. Please use MetaMask or another compatible wallet.`;
  }
  
  if (message.includes("User rejected") || message.includes("rejected")) {
    return "Connection rejected. Please try again.";
  }
  
  if (message.includes("already pending")) {
    return "Connection already pending. Check your wallet.";
  }
  
  if (message.includes("No Ethereum provider") || message.includes("No Ethereum wallet detected")) {
    return "No Ethereum provider found. Please install a wallet extension.";
  }
  
  if (
    message.includes("chain of the connector") ||
    message.includes("chain mismatch") ||
    message.includes("approve the network switch") ||
    message.includes("manually switch") ||
    message.includes("Please switch to")
  ) {
    return message;
  }

  return message || "Failed to connect wallet";
}

/**
 * Validates that the required wallet environment is available
 * @throws {Error} If wallet environment is not available
 */
export function validateWalletEnvironment() {
  if (typeof window === "undefined" || !window.ethereum) {
    throw new Error(
      "No Ethereum wallet detected. Please install MetaMask or another wallet extension."
    );
  }
}

/**
 * Validates wallet connection result
 * @param {Object} result - The connection result
 * @throws {Error} If validation fails
 */
export function validateConnectionResult(result) {
  if (!result.accounts?.[0]) {
    throw new Error("Please select an account in your wallet");
  }
}

/**
 * Validates wallet client
 * @param {Object} client - The wallet client
 * @throws {Error} If client is invalid
 */
export function validateWalletClient(client) {
  if (!client) {
    throw new Error("Failed to get wallet client");
  }
}

/**
 * Validates wallet address
 * @param {string} address - The wallet address
 * @throws {Error} If address is invalid
 */
export function validateWalletAddress(address) {
  if (!address) {
    throw new Error("Cannot access wallet account");
  }
}

/**
 * Validates chain ID matches target
 * @param {number} currentChainId - Current chain ID
 * @param {number} targetChainId - Target chain ID
 * @param {string} networkName - Network name for error message
 * @throws {Error} If chain IDs don't match
 */
export function validateChainId(currentChainId, targetChainId, networkName) {
  if (currentChainId !== targetChainId) {
    throw new Error(`Please switch to ${networkName} network`);
  }
}
