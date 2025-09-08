import { useEffect, useMemo, useState } from "react";
import { useEvmWallet } from "@/paywall/evm/context/EvmWalletContext";
import {
  generateAvailableNetworks,
  getCompatiblePaymentRequirements,
  normalizePaymentMethods,
} from "@/paywall/utils/paymentUtils";
import {
  Dropdown,
  NoPaymentOptions,
} from "@/paywall/components/PaywallComponents";
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
  const [paymentStatus, setPaymentStatus] = useState("idle");
  const [activePaymentRequirements, setActivePaymentRequirements] =
    useState(null);

  const { showToast } = useOutletContext();

  const paymentMethods = useMemo(
    () => normalizePaymentMethods(paymentRequirements),
    [paymentRequirements]
  );

  const availableNetworks = useMemo(
    () => generateAvailableNetworks(paymentMethods),
    [paymentMethods]
  );

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

  useEffect(() => {
    setSelectedPaymentMethodIndex(0);

    if (availableNetworks.length > 0) {
      if (
        !availableNetworks.some((network) => network.id === selectedNetwork.id)
      ) {
        setSelectedNetwork(availableNetworks[0]);
        setSelectedCoin(availableNetworks[0].coins[0]);
      }
    }
  }, [selectedNetwork.id, availableNetworks]);

  useEffect(() => {
    const compatibleMethods = getCompatiblePaymentRequirements(
      paymentMethods,
      selectedNetwork.id
    );

    if (compatibleMethods.length === 0) {
      setActivePaymentRequirements(null);
      return;
    }

    const matchingPaymentMethod = compatibleMethods.find(
      (method) => method.tokenSymbol === selectedCoin.name
    );

    if (matchingPaymentMethod) {
      setActivePaymentRequirements(matchingPaymentMethod);
      return;
    }

    setActivePaymentRequirements(compatibleMethods[0]);
  }, [
    paymentMethods,
    selectedPaymentMethodIndex,
    selectedNetwork.id,
    selectedCoin,
  ]);

  const handlePaymentSuccess = async (paymentHeader) => {
    setPaymentStatus("success");

    onPaymentSuccess(
      paymentHeader,
      activePaymentRequirements.selectedRequestID
    );
  };

  const handlePaymentError = (error) => {
    console.error("Payment failed:", error);
    showToast(error.message, "error");
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
    <div className={""}>
      {availableNetworks.length === 0 ? (
        <NoPaymentOptions />
      ) : (
        <>
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

          {renderPaymentButton()}
        </>
      )}
    </div>
  );
}
