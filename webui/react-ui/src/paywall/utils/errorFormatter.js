/**
 * Comprehensive error formatting utility for payment handlers
 * Provides standardized error type detection and user-friendly message formatting
 */

export const ERROR_TYPES = {
  USER_CANCELLATION: 'user_cancellation',
  FACILITATOR_ERROR: 'facilitator_error',
  NETWORK_ERROR: 'network_error',
  WALLET_CONNECTION: 'wallet_connection',
  UNAUTHORIZED_ACCOUNT: 'unauthorized_account',
  INSUFFICIENT_FUNDS: 'insufficient_funds',
  TRANSACTION_FAILED: 'transaction_failed',
  CHAIN_MISMATCH: 'chain_mismatch',
  UNKNOWN: 'unknown'
};

const ERROR_PATTERNS = {
  [ERROR_TYPES.USER_CANCELLATION]: [
    'cancelled by user',
    'User rejected',
    'User rejected the request',
    'user denied',
    'user cancelled',
    'transaction was cancelled'
  ],
  [ERROR_TYPES.FACILITATOR_ERROR]: [
    'Facilitator service unavailable',
    'Payment verification service is currently unavailable',
    'fetch failed',
    'facilitator error',
    'verification service error'
  ],
  [ERROR_TYPES.NETWORK_ERROR]: [
    'network error',
    'connection failed',
    'timeout',
    'network timeout',
    'request timeout',
    'failed to fetch'
  ],
  [ERROR_TYPES.WALLET_CONNECTION]: [
    'No Ethereum provider',
    'No Ethereum wallet detected',
    'wallet not found',
    'extension not installed',
    'provider not available',
    'wallet connection failed'
  ],
  [ERROR_TYPES.UNAUTHORIZED_ACCOUNT]: [
    'UnauthorizedProviderError',
    'not been authorized by the user',
    'unauthorized account',
    'account not authorized',
    'authorization required'
  ],
  [ERROR_TYPES.INSUFFICIENT_FUNDS]: [
    'insufficient funds',
    'insufficient balance',
    'not enough balance',
    'balance too low',
    'insufficient SOL',
    'insufficient ETH'
  ],
  [ERROR_TYPES.TRANSACTION_FAILED]: [
    'transaction failed',
    'transaction reverted',
    'execution reverted',
    'transaction rejected',
    'simulation failed'
  ],
  [ERROR_TYPES.CHAIN_MISMATCH]: [
    'chain mismatch',
    'wrong network',
    'Please switch to',
    'Wallet is on wrong network',
    'chain of the connector',
    'approve the network switch'
  ]
};

const ERROR_MESSAGES = {
  [ERROR_TYPES.USER_CANCELLATION]: "Transaction cancelled by user",
  [ERROR_TYPES.FACILITATOR_ERROR]: "Payment verification service is currently unavailable. Please try again later.",
  [ERROR_TYPES.NETWORK_ERROR]: "Network connection error. Please check your internet connection and try again.",
  [ERROR_TYPES.WALLET_CONNECTION]: "Wallet connection failed. Please ensure your wallet is installed and try again.",
  [ERROR_TYPES.UNAUTHORIZED_ACCOUNT]: "Account changed in wallet. Reconnecting...",
  [ERROR_TYPES.INSUFFICIENT_FUNDS]: "Insufficient funds to complete the transaction.",
  [ERROR_TYPES.TRANSACTION_FAILED]: "Transaction failed. Please try again.",
  [ERROR_TYPES.CHAIN_MISMATCH]: "Please switch to the correct network in your wallet.",
  [ERROR_TYPES.UNKNOWN]: "An unexpected error occurred. Please try again."
};

/**
 * Detects the type of error based on error message patterns
 * @param {Error|string} error - The error object or message
 * @returns {string} Error type from ERROR_TYPES
 */
export function detectErrorType(error) {
  const errorMessage = (error instanceof Error ? error.message : String(error)).toLowerCase();
  
  if (error && typeof error === 'object') {
    if (error.isFacilitatorError === true) {
      return ERROR_TYPES.FACILITATOR_ERROR;
    }
    if (error.type && ERROR_TYPES[error.type.toUpperCase()]) {
      return ERROR_TYPES[error.type.toUpperCase()];
    }
  }
  
  for (const [errorType, patterns] of Object.entries(ERROR_PATTERNS)) {
    if (patterns.some(pattern => errorMessage.includes(pattern.toLowerCase()))) {
      return errorType;
    }
  }
  
  return ERROR_TYPES.UNKNOWN;
}

/**
 * Formats an error into a user-friendly message
 * @param {Error|string} error - The error object or message
 * @param {Object} options - Formatting options
 * @param {string} options.networkName - Network name for context
 * @param {boolean} options.includeOriginal - Whether to include original error message
 * @returns {Object} Formatted error information
 */
export function formatError(error, options = {}) {
  const { networkName, includeOriginal = false } = options;
  
  const errorType = detectErrorType(error);
  const originalMessage = error instanceof Error ? error.message : String(error);
  
  let formattedMessage = ERROR_MESSAGES[errorType];
  
  if (errorType === ERROR_TYPES.CHAIN_MISMATCH && networkName) {
    formattedMessage = `Please switch to ${networkName} network in your wallet.`;
  }
  
  if (errorType === ERROR_TYPES.WALLET_CONNECTION && networkName) {
    formattedMessage = `Failed to connect wallet to ${networkName}. Please ensure your wallet supports this network.`;
  }
  
  if ((errorType === ERROR_TYPES.CHAIN_MISMATCH || errorType === ERROR_TYPES.WALLET_CONNECTION) && 
      originalMessage.includes('switch to') || originalMessage.includes('Please')) {
    formattedMessage = originalMessage;
  }
  
  return {
    type: errorType,
    message: formattedMessage,
    originalMessage: includeOriginal ? originalMessage : undefined,
    isUserCancellation: errorType === ERROR_TYPES.USER_CANCELLATION,
    isFacilitatorError: errorType === ERROR_TYPES.FACILITATOR_ERROR,
    isUnauthorizedAccount: errorType === ERROR_TYPES.UNAUTHORIZED_ACCOUNT,
    isRetryable: ![ERROR_TYPES.USER_CANCELLATION, ERROR_TYPES.INSUFFICIENT_FUNDS].includes(errorType)
  };
}

/**
 * Utility function to check specific error types
 */
export const isUnauthorizedAccount = (error) => detectErrorType(error) === ERROR_TYPES.UNAUTHORIZED_ACCOUNT;

/**
 * Gets payment status based on error type
 * @param {string} errorType - Error type from ERROR_TYPES
 * @returns {string} Payment status
 */
export function getPaymentStatusFromError(errorType) {
  switch (errorType) {
    case ERROR_TYPES.FACILITATOR_ERROR:
      return 'facilitator_error';
    case ERROR_TYPES.USER_CANCELLATION:
    case ERROR_TYPES.NETWORK_ERROR:
    case ERROR_TYPES.INSUFFICIENT_FUNDS:
    case ERROR_TYPES.TRANSACTION_FAILED:
    case ERROR_TYPES.CHAIN_MISMATCH:
      return 'error';
    case ERROR_TYPES.UNAUTHORIZED_ACCOUNT:
      return 'connecting'; 
    default:
      return 'error';
  }
}
