import { useEffect, useMemo, useState } from "react";
import { useEvmWallet } from "@/paywall/evm/context/EvmWalletContext";
import {
  generateAvailableNetworks,
  getCompatiblePaymentRequirements,
  normalizePaymentMethods,
} from "@/paywall/utils/paymentUtils";
import {
  Dropdown,
  ErrorMessage,
  NoPaymentOptions,
} from "@/paywall/components/PaywallComponents";
import { useWalletDetection } from "@/paywall/hooks/useWalletDetection";
import SolanaPaymentHandler from "@/paywall/solana/components/SolanaPaymentHandler";
import EvmPaymentHandler from "@/paywall/evm/components/EvmPaymentHandler";
import { formatAmountForDisplay } from "@/paywall/utils/amountFormatting";
import { useOutletContext } from "react-router-dom";

/**
 * Payment UI component with network/coin selection
 * and integrated payment button
 */
export default function PaymentUI({ paymentRequirements, onPaymentSuccess }) {
  const { connectedAddress: evmAddress } = useEvmWallet();
  const { isTrueEvmProvider } = useWalletDetection(evmAddress);
  const [paymentStatus, setPaymentStatus] = useState("idle");
  const [activePaymentRequirements, setActivePaymentRequirements] =
    useState(null);

  const { showToast } = useOutletContext();

  // Convert payment details to array if needed
  const paymentMethods = useMemo(
    () => normalizePaymentMethods(paymentRequirements),
    [paymentRequirements]
  );

  // Generate network and coin options from payment requirements
  const availableNetworks = useMemo(
    () => generateAvailableNetworks(paymentMethods),
    [paymentMethods]
  );

  // State for selections - initialize with empty defaults
  const [selectedNetwork, setSelectedNetwork] = useState({
    id: "",
    name: "",
    icon: "",
    coins: [],
  });

  const [selectedCoin, setSelectedCoin] = useState({
    id: "",
    name: "",
    icon: "",
  });

  const [dropdownState, setDropdownState] = useState({
    network: false,
    coin: false,
  });

  const [selectedPaymentMethodIndex, setSelectedPaymentMethodIndex] =
    useState(0);


  // Reset payment method index when network changes and update selected network
  useEffect(() => {
    setSelectedPaymentMethodIndex(0);

    // If there are available networks
    if (availableNetworks.length > 0) {
      // If the current network is not in the available networks list, select the first available one
      if (
        !availableNetworks.some((network) => network.id === selectedNetwork.id)
      ) {
        setSelectedNetwork(availableNetworks[0]);
        setSelectedCoin(availableNetworks[0].coins[0]);
      }
    }
  }, [selectedNetwork.id, availableNetworks]);

  // Get the active payment requirements based on selected coin
  useEffect(() => {
    // Get compatible methods for the selected network
    const compatibleMethods = getCompatiblePaymentRequirements(
      paymentMethods,
      selectedNetwork.id
    );

    if (compatibleMethods.length === 0) {
      console.log("[DEBUG-PAYMENT-FLOW] No compatible payment methods found");
      setActivePaymentRequirements(null);
      return;
    }

    // Find a payment method matching the selected coin
    const matchingPaymentMethod = compatibleMethods.find(
      (method) => method.tokenSymbol === selectedCoin.name
    );

    if (matchingPaymentMethod) {
      console.log(
        "[DEBUG-PAYMENT-FLOW] Found matching payment method for coin:",
        JSON.stringify(matchingPaymentMethod, null, 2)
      );
      setActivePaymentRequirements(matchingPaymentMethod);
      return;
    }

    // If no match found, use the first compatible method
    console.log(
      "[DEBUG-PAYMENT-FLOW] No exact match found, using first compatible method"
    );
    setActivePaymentRequirements(compatibleMethods[0]);

  }, [
    paymentMethods,
    selectedPaymentMethodIndex,
    selectedNetwork.id,
    selectedCoin,
  ]);

  // Event handlers
  const handlePaymentSuccess = async (paymentHeader) => {
    // Set payment status to success
    setPaymentStatus("success");

    console.log("Completing payment flow...");

    console.log("Payment header:", paymentHeader);
    console.log("Selected request ID:", activePaymentRequirements);
    
    onPaymentSuccess(paymentHeader, activePaymentRequirements.selectedRequestID);
  };

  const handlePaymentError = (error) => {
    console.error("Payment failed:", error);
    showToast(error.message, "error");
    // Could add toast notification here
  };

  const toggleDropdown = (dropdownName) => {
    setDropdownState((prev) => ({
      ...prev,
      [dropdownName]: !prev[dropdownName],
    }));
  };

  const selectNetwork = (network) => {
    setSelectedNetwork(network);
    setSelectedCoin(network.coins[0]);
    toggleDropdown("network");
  };

  const selectCoin = (coin) => {
    setSelectedCoin(coin);
    toggleDropdown("coin");
  };

  const renderPaymentButton = () => {
    if (!activePaymentRequirements) {
      return null;
    }

    const isValidMethod =
      activePaymentRequirements.namespace ===
      (selectedNetwork.id === "solana" ? "solana" : "evm");

    // Wallet compatibility is now handled by WalletSelector component in the payment handlers

    // Check if the payment method matches the selected network
    // if (!isValidMethod) {
    //   return (
    //     <ErrorMessage
    //       message="Please select a valid payment method."
    //     />
    //   );
    // }

    if (selectedNetwork.id === "solana") {
      return (
        <SolanaPaymentHandler
          amount={formatAmountForDisplay({
            amount: activePaymentRequirements.amountRequired?.toString() ?? 0,
            format:
              activePaymentRequirements.amountRequiredFormat ?? "smallestUnit",
            symbol: selectedCoin.name,
            decimals: activePaymentRequirements.tokenDecimals,
          })}
          paymentRequirements={activePaymentRequirements}
          onSuccess={handlePaymentSuccess}
          onError={handlePaymentError}
          setPaymentStatus={setPaymentStatus}
          paymentStatus={paymentStatus}
        />
      );
    } else if (selectedNetwork.id === "bsc" || selectedNetwork.id === "base") {
      return (
        <EvmPaymentHandler
          amount={formatAmountForDisplay({
            amount: activePaymentRequirements.amountRequired?.toString() ?? 0,
            format:
              activePaymentRequirements.amountRequiredFormat ?? "smallestUnit",
            symbol: selectedCoin.name,
            decimals: activePaymentRequirements.tokenDecimals,
          })}
          paymentRequirements={activePaymentRequirements}
          onSuccess={handlePaymentSuccess}
          onError={handlePaymentError}
          setPaymentStatus={setPaymentStatus}
          paymentStatus={paymentStatus}
          networkId={selectedNetwork.id}
        />
      );
    }

    return null;
  };

  return (
    <div
      className={""}
    >
      {availableNetworks.length === 0 ? (
        <NoPaymentOptions />
      ) : (
        <>
          {/* Network Selection */}
          {availableNetworks.length > 1 && (
            <Dropdown
              type="network"
              items={availableNetworks}
              selected={selectedNetwork}
              onSelect={selectNetwork}
              isOpen={dropdownState.network}
              toggleDropdown={() => toggleDropdown("network")}
            />
          )}

          {/* Coin Selection */}
          {selectedNetwork.coins.length > 0 && (
            <Dropdown
              type="coin"
              items={selectedNetwork.coins}
              selected={selectedCoin}
              onSelect={(coin) => selectCoin(coin)}
              isOpen={dropdownState.coin}
              toggleDropdown={() => toggleDropdown("coin")}
            />
          )}

          {/* Wallet Selection */}

          {/* Payment Button */}
          {renderPaymentButton()}
        </>
      )}
    </div>
  );
}