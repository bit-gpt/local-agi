import { useCallback, useEffect, useRef, useState } from "react";
import {
  useWalletAccountTransactionSendingSigner,
  useWalletAccountTransactionSigner,
} from "@solana/react";
import { useConnect } from "@wallet-standard/react";
import { createPaymentHeader, h402Version } from "@bit-gpt/h402";
import PaymentButtonUI from "../../components/PaymentButton";
import SolanaWalletSelector from "./SolanaWalletSelector";
import { useSolanaWallets } from "../hooks/useSolanaWallets";
import {
  formatError,
  getPaymentStatusFromError,
} from "../../utils/errorFormatter";
import { useOutletContext } from "react-router-dom";

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
  const [selectedWallet, setSelectedWallet] = useState(null);

  const { selectedWalletAccount, setSelectedWalletAccount } =
    useSolanaWallets();

  const paymentAttemptRef = useRef({ attemptInProgress: false });

  const handleWalletSelect = (wallet) => {
    setSelectedWallet(wallet);
    if (selectedWalletAccount) {
      setSelectedWalletAccount(null);
    }
  };

  const handleButtonClick = async () => {
    if (selectedWallet && !selectedWalletAccount) {
      setPaymentStatus("connecting");
      return;
    }

    setPaymentStatus("approving");
  };

  const handlePaymentSuccess = useCallback(
    (paymentHeader) => {
      if (onSuccess) onSuccess(paymentHeader);
    },
    [onSuccess]
  );

  const handlePaymentError = useCallback(
    (err) => {
      const errMsg = err instanceof Error ? err.message : String(err);

      const errorInfo = formatError(err, { networkName: "Solana" });

      setPaymentStatus(getPaymentStatusFromError(errorInfo.type));
      setErrorMessage(errorInfo.message);

      paymentAttemptRef.current.attemptInProgress = false;

      if (onError) onError(err instanceof Error ? err : new Error(errMsg));
    },
    [onError]
  );

  const handlePaymentProcessing = useCallback(() => {
    setPaymentStatus("processing");
    paymentAttemptRef.current.attemptInProgress = true;
  }, []);

  const handleConnectionError = (err) => {
    console.error(
      "[DEBUG] Connection error from WalletConnectionManager:",
      err
    );
    const errorInfo = formatError(err, { networkName: "Solana" });
    setPaymentStatus(getPaymentStatusFromError(errorInfo.type));
    setErrorMessage(errorInfo.message);
  };

  const isDisabled =
    ["processing", "success"].includes(paymentStatus) ||
    (!selectedWalletAccount && !selectedWallet);

  return (
    <div className="flex flex-col w-full space-y-4">
      <SolanaWalletSelector
        onWalletSelect={handleWalletSelect}
        selectedWallet={selectedWallet}
        disabled={paymentStatus === "connecting"}
      />
      {selectedWallet && paymentStatus === "connecting" && (
        <WalletConnectionManager
          wallet={selectedWallet}
          onConnectionError={handleConnectionError}
          paymentStatus={paymentStatus}
          setSelectedWalletAccount={setSelectedWalletAccount}
          setPaymentStatus={setPaymentStatus}
        />
      )}
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
  const transactionSigner = useWalletAccountTransactionSigner(
    account,
    "solana:mainnet"
  );

  const hasAttemptedRef = useRef(false);

  useEffect(() => {
    if (
      hasAttemptedRef.current ||
      (paymentAttemptRef.current && paymentAttemptRef.current.attemptInProgress)
    ) {
      return;
    }

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

        const signTransactionFn = transactionSigner?.modifyAndSignTransactions;

        const paymentClients = {
          solanaClient: {
            publicKey: account.address,
            signAndSendTransaction: signAndSendTransactionFn,
            signTransaction: signTransactionFn,
          },
        };

        const paymentHeader = await createPaymentHeader(
          paymentClients,
          h402Version,
          paymentRequirements
        );
        onProcessing();
        onSuccess(paymentHeader);
      } catch (err) {
        const errorInfo = formatError(err, { networkName: "Solana" });

        if (errorInfo.isFacilitatorError) {
          const facilitatorError = new Error(errorInfo.message);
          Object.defineProperty(facilitatorError, "isFacilitatorError", {
            value: true,
            enumerable: true,
          });
          onError(facilitatorError);
        } else {
          onError(err instanceof Error ? err : new Error(errorInfo.message));
        }
      }
    };

    processPayment();

    return () => {
      paymentAttemptRef.current.attemptInProgress = false;
    };
  }, [
    account,
    paymentRequirements,
    transactionSendingSigner,
    onSuccess,
    onError,
    onProcessing,
    paymentAttemptRef,
    transactionSigner?.modifyAndSignTransactions,
  ]);

  return null;
}

function WalletConnectionManager({
  wallet,
  onConnectionError,
  setSelectedWalletAccount,
  setPaymentStatus,
}) {
  const { showToast } = useOutletContext();
  const [isConnecting, connect] = useConnect(wallet);
  const connectionAttemptedRef = useRef(false);
  const walletAccountSelectionRef = useRef(false);
  const { wallets } = useSolanaWallets();
  const [connectionSuccess, setConnectionSuccess] = useState(false);

  useEffect(() => {
    if (!isConnecting && !connectionAttemptedRef.current) {
      connectionAttemptedRef.current = true;

      const handleConnection = async () => {
        try {
          await connect();
          setConnectionSuccess(true);
        } catch (err) {
          const errorInfo = formatError(err);
          showToast(errorInfo.message, "error");
          setConnectionSuccess(false);
          if (onConnectionError) {
            onConnectionError(err);
          }
        }
      };

      handleConnection();
    }
  }, [isConnecting, connect, onConnectionError, setConnectionSuccess]);

  useEffect(() => {
    if (
      wallets.length > 0 &&
      connectionAttemptedRef.current &&
      connectionSuccess &&
      !walletAccountSelectionRef.current
    ) {
      walletAccountSelectionRef.current = true;
      const updatedWallet = wallets.find((w) => w.name === wallet.name);

      if (updatedWallet?.accounts?.length > 0) {
        setSelectedWalletAccount(updatedWallet.accounts[0]);
        setPaymentStatus("approving");
      } else {
        throw new Error("No accounts available after connection");
      }
    }
  }, [
    wallet,
    wallets,
    setSelectedWalletAccount,
    setPaymentStatus,
    walletAccountSelectionRef,
    connectionSuccess,
  ]);

  return null;
}
