import { useState, useEffect } from "react";

/**
 * EVM Wallet Selector Component
 * Dynamically detects available EVM wallets instead of hardcoding them
 */
export default function EvmWalletSelector({
  chainId,
  onWalletSelect,
  selectedWallet,
  disabled = false,
}) {
  const [availableWallets, setAvailableWallets] = useState([]);

  useEffect(() => {
    const detectWallets = () => {
      const wallets = [];

      if (window.ethereum?.isMetaMask && !window.ethereum?.isPhantom) {
        wallets.push({
          id: "metamask",
          name: "MetaMask",
          icon: "/app/wallets/metamask.svg",
          type: "evm",
        });
      }

      if (window.ethereum?.isCoinbaseWallet) {
        wallets.push({
          id: "coinbase",
          name: "Coinbase Wallet",
          icon: "/app/wallets/coinbase.svg",
          type: "evm",
        });
      }

      if (window.ethereum?.isRabby) {
        wallets.push({
          id: "rabby",
          name: "Rabby",
          icon: "/app/wallets/rabby.svg",
          type: "evm",
        });
      }

      if (window.trustwallet?.isTrust) {
        wallets.push({
          id: "trust",
          name: "Trust Wallet",
          icon: "/app/wallets/trustwallet.svg",
          type: "evm",
        });
      }

      if (window.phantom && chainId !== "solana") {
        wallets.push({
          id: "phantom",
          name: "Phantom",
          icon: "/app/wallets/phantom.svg",
          type: "hybrid",
        });
      }

      wallets.push({
        id: "walletconnect",
        name: "WalletConnect",
        icon: "/app/wallets/walletConnect.svg",
        type: "evm",
      });

      const supportedWallets = wallets.filter((wallet) => {
        const chainSupport = {
          metamask: ["bsc", "base"],
          coinbase: ["bsc", "base"],
          rabby: ["bsc", "base"],
          trust: ["bsc", "base"],
          phantom: ["base"],
          walletconnect: ["bsc", "base"],
        };

        const isSupported = chainSupport[wallet.id]?.includes(chainId) || false;
        return isSupported;
      });

      setAvailableWallets(supportedWallets);
    };

    if (typeof window !== "undefined") {
      detectWallets();
    }
  }, [chainId]);

  const handleWalletSelect = (walletId) => {
    if (!disabled) {
      onWalletSelect(walletId);
    }
  };

  if (availableWallets.length === 0) {
    return (
      <div className="p-4 bg-red-50 border border-red-200 rounded-lg">
        <p className="text-red-700 text-sm">
          No supported EVM wallets found for {chainId.toUpperCase()} chain.
        </p>
      </div>
    );
  }

  return (
    <div className="mb-8">
      <div className="flex items-center justify-between">
        <div className="block text-sm font-medium text-gray-700 mb-2">
          Select EVM wallet
        </div>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
        {availableWallets.map((wallet) => (
          <div
            key={wallet.id}
            className={`
              relative flex items-center p-3 border rounded-lg cursor-pointer transition-all duration-200
              ${
                selectedWallet === wallet.id
                  ? "border-blue-500 bg-blue-50 ring-2 ring-blue-200"
                  : "border-gray-200 hover:bg-gray-50"
              }
              ${disabled ? "opacity-50 cursor-not-allowed" : ""}
            `}
            onClick={() => handleWalletSelect(wallet.id)}
          >
            <input
              type="radio"
              id={wallet.id}
              name="evm-wallet-selection"
              value={wallet.id}
              checked={selectedWallet === wallet.id}
              onChange={() => handleWalletSelect(wallet.id)}
              disabled={disabled}
              className="sr-only"
            />

            <div className="w-6 h-6 mr-3 flex-shrink-0">
              <img
                src={wallet.icon}
                alt={wallet.name}
                className="w-6 h-6 object-contain"
              />
            </div>

            <div className="text-sm font-medium text-gray-900 truncate">
              {wallet.name}
            </div>

            {selectedWallet === wallet.id && (
              <div className="absolute -top-2 -right-2 w-5 h-5 bg-blue-500 rounded-full flex items-center justify-center">
                <div className="w-2.5 h-2.5 bg-white rounded-full"></div>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
