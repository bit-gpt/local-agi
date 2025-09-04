import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {useWalletAccountTransactionSendingSigner, useWalletAccountTransactionSigner} from "@solana/react";
import {useConnect} from "@wallet-standard/react";
import {createPaymentHeader, h402Version} from "@bit-gpt/h402";
import PaymentButtonUI from "../../components/PaymentButton";
import SolanaWalletSelector from "./SolanaWalletSelector";
import { useSolanaWallets } from "../hooks/useSolanaWallets";

/**
 * Solana-specific payment handler
 * Uses wallet-standard/react hooks for Solana wallet integration
 */
export default function SolanaPaymentHandler({
  amount,
  paymentRequirements,
  onSuccess,
  onError,
  paymentStatus,
  setPaymentStatus,
  className = "",
}) {
  const [errorMessage, setErrorMessage] = useState(null);
  
  // Use the custom hook for Solana wallet management
  const { selectedWallet, selectedWalletAccount, setSelectedWalletAccount } = useSolanaWallets();

  // Simplified ref to track payment attempts
  const paymentAttemptRef = useRef({attemptInProgress: false});

  console.log("[DEBUG] selectedWalletSSSSSSSS", selectedWallet);

  // Handle wallet selection
  const handleWalletSelect = (wallet) => {
    // Set the first account from the selected wallet
    const firstAccount = wallet?.accounts?.[0];
    setSelectedWalletAccount(firstAccount);
  };

  // Handle button click for unified experience
  // If no account, connect first
  // If already connected, start processing payment
  const handleButtonClick = async () => {
    console.log("[DEBUG] Button clicked", {
      selectedAccountAddress: selectedWalletAccount?.address,
      currentStatus: paymentStatus,
    });
    // If not connected, prompt user to select a wallet
    if (!selectedWalletAccount) {
      if (!selectedWallet) {
        setErrorMessage("Please select a wallet to connect");
        return;
      }
      // If wallet is selected but not connected, try to connect
      await handleConnectWallet();
      return;
    }
    // If already connected, start processing payment
    setPaymentStatus("approving");
  };

  // Connect wallet handler
  const handleConnectWallet = async () => {
    setErrorMessage(null);
    setPaymentStatus("connecting");

    try {
      console.log("[DEBUG] Connecting wallet");
      // Use existing accounts if available, otherwise connect to get accounts
      const accounts =
        selectedWallet?.accounts?.length > 0 ? selectedWallet.accounts : await connectFunction();
      console.log("[DEBUG] Retrieved accounts", accounts?.length);
      if (!accounts || accounts.length === 0) {
        throw new Error("No accounts available");
      }
      setSelectedWalletAccount(accounts[0]);
      // Set the status to approving to show tx approval in wallet
      setPaymentStatus("approving");
    } catch (err) {
      console.error("[DEBUG] Wallet connection error:", err);
      setPaymentStatus("error");
      const errMsg = err instanceof Error ? err.message : String(err);
      setErrorMessage(errMsg);
      onError?.(err instanceof Error ? err : new Error(errMsg));
    }
  };

  // Update payment status callbacks
  const handlePaymentSuccess = useCallback(
    (paymentHeader) => {
      console.log("[DEBUG] Payment signed and maybe sent");
      console.log("[DEBUG] Payment header:", paymentHeader);

      // Keep the processing state - we'll let the parent component set success
      // after facilitator verifies the transaction

      // Call onSuccess immediately - the parent will handle facilitator verification
      // Pass setStatus so the parent can update our status after facilitator verification
      if (onSuccess) onSuccess(paymentHeader);
    },
    [onSuccess]
  );

  const handlePaymentError = useCallback(
    (err) => {
      const errMsg = err instanceof Error ? err.message : String(err);
      console.log("[DEBUG] Payment error", {errMsg});

      // Check if this is a user cancellation
      const isUserCancellation =
        errMsg.includes("cancelled by user") ||
        errMsg.includes("User rejected");

      // Check if this is a facilitator error using the custom property
      const isFacilitatorError = err.isFacilitatorError === true;

      if (isUserCancellation) {
        console.log("[DEBUG] User cancelled payment");
        // Set status to error so the error message is displayed
        setPaymentStatus("error");
        setErrorMessage("Transaction cancelled by user");
      } else if (isFacilitatorError) {
        console.log("[DEBUG] Facilitator service unavailable");
        // Use the specific facilitator_error status
        setPaymentStatus("facilitator_error");
        setErrorMessage(
          "Payment verification service is currently unavailable. Please try again later."
        );
      } else {
        setPaymentStatus("error");
        setErrorMessage(errMsg);
      }

      // Always reset payment tracking
      paymentAttemptRef.current.attemptInProgress = false;

      if (onError) onError(err instanceof Error ? err : new Error(errMsg));
    },
    [onError]
  );

  const handlePaymentProcessing = useCallback(() => {
    console.log("[DEBUG] Payment processing started");
    setPaymentStatus("processing");
    paymentAttemptRef.current.attemptInProgress = true;
  }, []);

  // Determine if the button is disabled
  const isDisabled =
    ["processing", "success"].includes(paymentStatus) ||
    (!selectedWalletAccount && !selectedWallet); // Disable if no wallet selected and not connected

  return (
    <div className="flex flex-col w-full space-y-4">
      {/* Wallet Selector - Always visible */}
      <SolanaWalletSelector
        onWalletSelect={handleWalletSelect}
        selectedWallet={selectedWallet}
        disabled={paymentStatus === "connecting"}
      />

      {/* Error Message */}
      {/* {errorMessage && (
        <div className="p-3 bg-red-50 border border-red-200 rounded-lg">
          <p className="text-red-700 text-sm">{errorMessage}</p>
        </div>
      )} */}

      {/* Wallet Connection Manager - conditionally rendered */}
      {selectedWallet && (
        <WalletConnectionManager
          wallet={selectedWallet}
        />
      )}

      {/* Payment processor component that watches for account and status */}
      {selectedWalletAccount &&
        paymentStatus === "approving" &&
        !paymentAttemptRef.current.attemptInProgress && (
          <SolanaPaymentProcessor
            account={selectedWalletAccount}
            paymentRequirements={paymentRequirements}
            onSuccess={handlePaymentSuccess}
            onError={handlePaymentError}
            onProcessing={handlePaymentProcessing}
            paymentAttemptRef={paymentAttemptRef}
          />
        )}

      <PaymentButtonUI
        paymentStatus={paymentStatus}
        amount={amount}
        errorMessage={errorMessage}
        onClick={handleButtonClick}
        disabled={isDisabled}
        className={className}
      />
    </div>
  );
}

// Payment processor component that can use the hook with a valid account
function SolanaPaymentProcessor({
  account,
  paymentRequirements,
  onSuccess,
  onError,
  onProcessing,
  paymentAttemptRef,
}) {
  const transactionSendingSigner = useWalletAccountTransactionSendingSigner(
    account,
    "solana:mainnet"
  );
  const transactionSigner = useWalletAccountTransactionSigner(account, "solana:mainnet")

  // Debug - Track if this component has already attempted a payment
  const hasAttemptedRef = useRef(false);

  // Process payment on mount
  useEffect(() => {
    // Guard against multiple attempts
    if (
      hasAttemptedRef.current ||
      (paymentAttemptRef.current && paymentAttemptRef.current.attemptInProgress)
    ) {
      console.log(
        "[DEBUG-PAYMENT-FLOW] Payment already in progress or attempted, skipping"
      );
      return;
    }

    // Mark as attempted at the component level
    hasAttemptedRef.current = true;

    const processPayment = async () => {
      try {
        if (!transactionSendingSigner) {
          console.error(
            "[SolanaPaymentHandler] Transaction signer not available"
          );
          throw new Error("Solana transaction signer not available");
        }

        const signAndSendTransactionFn =
          transactionSendingSigner?.signAndSendTransactions;

        const signTransactionFn =
          transactionSigner?.modifyAndSignTransactions;

        const paymentClients = {
          solanaClient: {
            publicKey: account.address,
            signAndSendTransaction: signAndSendTransactionFn,
            signTransaction: signTransactionFn,
          },
        };

        // Create payment using the h402 payment library
        // At this point the user will be prompted to approve the transaction
        console.log("[DEBUG Solana Payment Handler] Calling createPayment now...");
        console.log("[DEBUG Solana Payment Handler] Payment requirements:", paymentRequirements.newtworkId);
        const paymentHeader = await createPaymentHeader(
          paymentClients,
          h402Version,
          paymentRequirements,
        );
        console.log("[DEBUG Solana Payment Handler] createPayment completed successfully");
        // If we get here, it means the user has approved the transaction
        // Now we can set the processing state as we wait for confirmation
        onProcessing();
        onSuccess(paymentHeader);
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : String(err);
        // Check for facilitator unavailability
        const isFacilitatorUnavailable =
          errorMessage.includes("Facilitator service unavailable") ||
          errorMessage.includes(
            "Payment verification service is currently unavailable"
          ) ||
          errorMessage.includes("fetch failed");

        const isUserRejection = errorMessage.includes(
          "User rejected the request"
        );

        if (isUserRejection) {
          onError(new Error("Transaction cancelled by user"));
        } else if (isFacilitatorUnavailable) {
          // Create a custom error with a special type property for the parent component to identify
          const facilitatorError = new Error(
            "Payment verification service is currently unavailable. Please try again later."
          );
          // Add a custom property to identify this as a facilitator error
          Object.defineProperty(facilitatorError, "isFacilitatorError", {
            value: true,
            enumerable: true,
          });
          onError(facilitatorError);
        } else {
          onError(err instanceof Error ? err : new Error(String(err)));
        }
      }
    };

    processPayment();

    // Clean up function
    return () => {
      paymentAttemptRef.current.attemptInProgress = false;
    };
  }, [account, paymentRequirements, transactionSendingSigner, onSuccess, onError, onProcessing, paymentAttemptRef, transactionSigner?.modifyAndSignTransactions]);

  // This component doesn't render anything
  return null;
}

// Separate component to handle wallet connection hook
function WalletConnectionManager({ wallet }) {
  const [isConnecting, connect] = useConnect(wallet);
  
  // This component doesn't render anything
  return null;
}