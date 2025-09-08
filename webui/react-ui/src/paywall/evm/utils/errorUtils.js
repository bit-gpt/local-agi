/**
 * Utility functions for handling wallet connection errors
 */

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
