import { useCallback, useEffect, useRef, useState } from "react";
import { createPaymentHeader, h402Version } from "@bit-gpt/h402";
import { useEvmWallet } from "../context/EvmWalletContext";
import {
  formatError,
  isUnauthorizedAccount,
  getPaymentStatusFromError,
} from "../../utils/errorFormatter";
import PaymentButtonUI from "../../components/PaymentButton";
import EvmWalletSelector from "./EvmWalletSelector";

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
  networkId = "bsc",
}) {
  const [errorMessage, setErrorMessage] = useState(null);
  const [selectedWallet, setSelectedWallet] = useState(null);
  const [lastNetworkId, setLastNetworkId] = useState(networkId);

  const { walletClient, connectedAddress, connectWallet, disconnectWallet } =
    useEvmWallet();

  const paymentAttemptRef = useRef({
    attemptInProgress: false,
  });

  useEffect(() => {
    if (lastNetworkId !== networkId && connectedAddress) {
      console.log(
        "[DEBUG] Network changed from",
        lastNetworkId,
        "to",
        networkId,
        "- disconnecting wallet"
      );
      disconnectWallet();
      setSelectedWallet(null);
      setLastNetworkId(networkId);
      console.log("DISCONNECTED WALLET");
    } else if (lastNetworkId !== networkId) {
      setLastNetworkId(networkId);
      console.log("SET LAST NETWORK ID", lastNetworkId);
    }
  }, [networkId, lastNetworkId, connectedAddress, disconnectWallet]);

  const handleButtonClick = async () => {
    console.log("[DEBUG] Button clicked", {
      hasWallet: !!walletClient,
      connectedAddress: connectedAddress?.slice(0, 8),
      currentStatus: paymentStatus,
    });

    if (!connectedAddress) {
      if (!selectedWallet) {
        setErrorMessage("Please select a wallet to connect");
        return;
      }
      await handleConnectWallet(selectedWallet);
      return;
    }

    setPaymentStatus("approving");
  };

  const handleConnectWallet = async (walletId = selectedWallet) => {
    if (!walletId) {
      setErrorMessage("Please select a wallet first");
      return;
    }

    setErrorMessage(null);
    setPaymentStatus("connecting");

    try {
      if (connectedAddress || walletClient) {
        console.log(
          "[DEBUG] Disconnecting existing wallet before connecting new wallet:",
          walletId
        );
        await disconnectWallet();
      }

      console.log(
        "[DEBUG] Connecting EVM wallet:",
        walletId,
        "to network:",
        networkId
      );

      await connectWallet(walletId, networkId);

      setErrorMessage(null);
      setPaymentStatus("approving");
    } catch (err) {
      const errorInfo = formatError(err, {
        networkName: networkId.toUpperCase(),
      });
      console.error("[DEBUG] EVM wallet connection error:", errorInfo);

      setErrorMessage(errorInfo.message);
      setPaymentStatus(getPaymentStatusFromError(errorInfo.type));
      onError?.(err instanceof Error ? err : new Error(errorInfo.message));
    }
  };

  const handleWalletSelect = (walletId) => {
    setSelectedWallet(walletId);
  };

  const handlePaymentSuccess = useCallback(
    (paymentHeader) => {
      console.log("[DEBUG] Payment sent and signed");
      console.log("[DEBUG] Payment header:", paymentHeader);

      if (onSuccess) onSuccess(paymentHeader);
    },
    [onSuccess]
  );

  const handlePaymentError = useCallback(
    async (err) => {
      const errMsg = err instanceof Error ? err.message : String(err);
      console.log("[DEBUG] Payment error", { errMsg });

      const errorInfo = formatError(err, {
        networkName: networkId.toUpperCase(),
      });

      if (
        isUnauthorizedAccount(err) &&
        selectedWallet &&
        paymentStatus !== "connecting"
      ) {
        console.log(
          "[DEBUG] Unauthorized account error detected, attempting reconnection"
        );
        setPaymentStatus("connecting");
        setErrorMessage(errorInfo.message);

        try {
          await disconnectWallet();
          await handleConnectWallet(selectedWallet);
          return;
        } catch (reconnectError) {
          console.error("[DEBUG] Reconnection failed:", reconnectError);
          const reconnectErrorInfo = formatError(reconnectError);
          setPaymentStatus("error");
          setErrorMessage(
            "Failed to reconnect wallet. Please try connecting again."
          );
          paymentAttemptRef.current.attemptInProgress = false;
          if (onError)
            onError(
              reconnectError instanceof Error
                ? reconnectError
                : new Error(reconnectErrorInfo.message)
            );
          return;
        }
      }

      setPaymentStatus(getPaymentStatusFromError(errorInfo.type));
      setErrorMessage(errorInfo.message);

      paymentAttemptRef.current.attemptInProgress = false;

      if (onError) onError(err instanceof Error ? err : new Error(errMsg));
    },
    [
      onError,
      setPaymentStatus,
      selectedWallet,
      paymentStatus,
      disconnectWallet,
      handleConnectWallet,
    ]
  );

  const handlePaymentProcessing = useCallback(() => {
    setPaymentStatus("processing");
    paymentAttemptRef.current.attemptInProgress = true;
  }, [setPaymentStatus]);

  const isDisabled =
    ["connecting", "processing", "success"].includes(paymentStatus) ||
    (!connectedAddress && !selectedWallet);


    console.log('ss', paymentStatus)

  return (
    <div className="flex flex-col w-full space-y-4">
      <EvmWalletSelector
        chainId={networkId}
        onWalletSelect={handleWalletSelect}
        selectedWallet={selectedWallet}
        disabled={["approving", "connecting", "processing"].includes(paymentStatus)}
      />

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
            networkId={networkId}
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

function EvmPaymentProcessor({
  walletClient,
  connectedAddress,
  paymentRequirements,
  onSuccess,
  onError,
  onProcessing,
  paymentAttemptRef,
  networkId = "bsc",
}) {
  const hasAttemptedRef = useRef(false);

  useEffect(() => {
    if (
      hasAttemptedRef.current ||
      (paymentAttemptRef.current && paymentAttemptRef.current.attemptInProgress)
    ) {
      return;
    }

    hasAttemptedRef.current = true;

    onProcessing();

    const processPayment = async () => {
      try {
        if (!walletClient) {
          throw new Error("EVM wallet client not available");
        }

        const getChainId = (network) => {
          switch (network) {
            case "bsc":
              return "56";
            case "base":
              return "8453";
            default:
              return "56";
          }
        };

        const currentChainId = await walletClient.getChainId();
        const expectedChainId = parseInt(getChainId(networkId));

        if (currentChainId !== expectedChainId) {
          throw new Error(
            `Wallet is on wrong network. Please switch to ${networkId.toUpperCase()} network and try again.`
          );
        }

        const finalPaymentRequirements = {
          ...paymentRequirements,
          namespace: "evm",
          networkId: getChainId(networkId),
          scheme: "exact",
          resource: paymentRequirements.resource || "payment",
        };

        const paymentClients = {
          evmClient: walletClient,
        };

        const paymentHeader = await createPaymentHeader(
          paymentClients,
          h402Version,
          finalPaymentRequirements
        );
        onSuccess(paymentHeader);
      } catch (err) {
        const errorInfo = formatError(err, {
          networkName: networkId.toUpperCase(),
        });

        if (errorInfo.isUserCancellation) {
          onError(new Error(errorInfo.message));
        } else {
          onError(err instanceof Error ? err : new Error(errorInfo.message));
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
    networkId,
  ]);

  return null;
}
