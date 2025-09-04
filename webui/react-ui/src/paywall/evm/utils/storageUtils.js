/**
 * Utility functions for handling wallet-related storage operations
 */

/**
 * Clears wallet-related items from localStorage
 * This helps ensure complete wallet disconnection
 */
export function clearWalletStorage() {
  if (typeof window === "undefined") {
    return;
  }

  const keysToRemove = [];
  
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i);
    if (key && (key.includes("wagmi") || key.includes("wallet"))) {
      keysToRemove.push(key);
    }
  }

  keysToRemove.forEach((key) => {
    localStorage.removeItem(key);
  });
}
