import {useCallback, useEffect, useRef, useState} from "react";
import {createPaymentHeader, h402Version} from "@bit-gpt/h402";
import {useEvmWallet} from "../context/EvmWalletContext";
import PaymentButtonUI from "../../components/PaymentButton";

/**
 * EVM-specific payment handler
 * Uses the EvmWalletContext for wallet integration
 */
export default function EvmPaymentHandler({
                                            amount,
                                            paymentRequirements,
                                            onSuccess,
                                            onError,
                                            paymentStatus,
                                            setPaymentStatus,
                                            className = "",
                                          }) {
  // State for the payment flow
  const [errorMessage, setErrorMessage] = useState(null);

  // Get EVM wallet context
  const {walletClient, connectedAddress, connectWallet} = useEvmWallet();

  // Simplified ref to track payment attempts
  const paymentAttemptRef = useRef({
    attemptInProgress: false,
  });

  // Handle button click for unified experience
  const handleButtonClick = async () => {
    console.log("[DEBUG] Button clicked", {
      hasWallet: !!walletClient,
      connectedAddress: connectedAddress?.slice(0, 8),
      currentStatus: paymentStatus,
    });

    // If not connected, connect first
    if (!connectedAddress) {
      await handleConnectWallet();
      return;
    }

    // If already connected, process payment
    setPaymentStatus("approving");
  };

  // Connect wallet handler
  const handleConnectWallet = async () => {
    setErrorMessage(null);
    setPaymentStatus("connecting");

    try {
      console.log("[DEBUG] Connecting EVM wallet");

      // Default to MetaMask
      await connectWallet("metamask");

      // If we get here, connection was successful
      setPaymentStatus("approving"); // Go directly to payment approval
    } catch (err) {
      console.error("[DEBUG] EVM wallet connection error:", err);
      setPaymentStatus("error");
      const errMsg = err instanceof Error ? err.message : String(err);
      setErrorMessage(errMsg);
      onError?.(err instanceof Error ? err : new Error(errMsg));
    }
  };

  // Update payment status callbacks
  const handlePaymentSuccess = useCallback(
    (paymentHeader) => {
      console.log("[DEBUG] Payment sent and signed");
      console.log("[DEBUG] Payment header:", paymentHeader);

      // Call onSuccess immediately - the parent will handle facilitator verification
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

      if (isUserCancellation) {
        console.log("[DEBUG] User cancelled payment");
        // Set status to error so the error message is displayed
        setPaymentStatus("error");
        setErrorMessage("Transaction cancelled by user");
      } else {
        setPaymentStatus("error");
        setErrorMessage(errMsg);
      }

      // Always reset payment tracking
      paymentAttemptRef.current.attemptInProgress = false;

      if (onError) onError(err instanceof Error ? err : new Error(errMsg));
    },
    [onError, setPaymentStatus]
  );

  const handlePaymentProcessing = useCallback(() => {
    console.log("[DEBUG] Payment processing started");
    setPaymentStatus("processing");
    // Mark payment as in progress
    paymentAttemptRef.current.attemptInProgress = true;
  }, [setPaymentStatus]);

  // Determine if the button is disabled
  const isDisabled = ["connecting", "processing", "success"].includes(
    paymentStatus
  );

  return (
    <div className="flex flex-col w-full">
      {/* Payment processor component that watches for wallet connection and status */}
      {connectedAddress &&
        walletClient &&
        paymentStatus === "approving" &&
        !paymentAttemptRef.current.attemptInProgress && (
          <EvmPaymentProcessor
            walletClient={walletClient}
            connectedAddress={connectedAddress}
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
        onClick={handleButtonClick}
        disabled={isDisabled}
        className={className}
      />
    </div>
  );
}

// Payment processor component for EVM wallets
function EvmPaymentProcessor({
                               walletClient,
                               connectedAddress,
                               paymentRequirements,
                               onSuccess,
                               onError,
                               onProcessing,
                               paymentAttemptRef,
                             }) {
  // Debug - Track if this component has already attempted a payment
  const hasAttemptedRef = useRef(false);

  // Process payment on mount
  useEffect(() => {
    console.log("[DEBUG] EvmPaymentProcessor mounted", {
      connectedAddress: connectedAddress?.slice(0, 8),
      hasAttempted: hasAttemptedRef.current,
      attemptInProgress: paymentAttemptRef.current?.attemptInProgress,
    });

    // Guard against multiple attempts
    if (
      hasAttemptedRef.current ||
      (paymentAttemptRef.current && paymentAttemptRef.current.attemptInProgress)
    ) {
      console.log("[DEBUG] Payment already in progress or attempted, skipping");
      return;
    }

    // Mark as attempted at the component level
    hasAttemptedRef.current = true;

    // Call processing callback - this will update the parent's ref
    onProcessing();

    const processPayment = async () => {
      try {
        if (!walletClient) {
          throw new Error("EVM wallet client not available");
        }

        // Set the appropriate namespace and networkId for EVM
        const finalPaymentRequirements = {
          ...paymentRequirements,
          namespace: "evm",
          networkId: "56", // BSC mainnet
          scheme: "exact",
          resource: paymentRequirements.resource || "payment", // Use provided resource or default
        };

        console.log("[DEBUG] Final EVM payment details:", {
          namespace: finalPaymentRequirements.namespace,
          networkId: finalPaymentRequirements.networkId,
          scheme: finalPaymentRequirements.scheme,
          tokenAddress: finalPaymentRequirements.tokenAddress,
          amountRequired: finalPaymentRequirements.amountRequired,
        });

        const paymentClients = {
          evmClient: walletClient,
        };

        console.log("[DEBUG] Calling createPayment for EVM");

        // Create payment using the h402 payment library
        const paymentHeader = await createPaymentHeader(
          paymentClients,
          h402Version,
        finalPaymentRequirements,
        );
        console.log("[DEBUG] createPayment succeeded");
        onSuccess(paymentHeader);
      } catch (err) {
        console.error("[DEBUG] Payment error:", err);

        // Check if this is a user rejection
        const errorMessage = err instanceof Error ? err.message : String(err);
        console.log("[DEBUG] Error message:", errorMessage);

        const isUserRejection = errorMessage.includes(
          "User rejected the request"
        );
        console.log("[DEBUG] Is user rejection:", isUserRejection);

        if (isUserRejection) {
          console.log(
            "[DEBUG] Transaction was rejected by user, handling gracefully"
          );
          onError(new Error("Transaction cancelled by user"));
        } else {
          onError(err instanceof Error ? err : new Error(String(err)));
        }
      }
    };

    processPayment();
  }, [
    connectedAddress,
    walletClient,
    paymentRequirements,
    onSuccess,
    onError,
    onProcessing,
    paymentAttemptRef,
  ]);

  // This component doesn't render anything
  return null;
}