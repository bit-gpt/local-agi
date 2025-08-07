import { useState } from "react";
import { useOutletContext } from "react-router-dom";
import PrivateKeyReveal from "./PrivateKeyReveal";

const ServerWalletCard = ({ serverWallet }) => {
  const { showToast } = useOutletContext();
  const [copySuccess, setCopySuccess] = useState(false);

  const handleCopyAddress = async () => {
    try {
      await navigator.clipboard.writeText(serverWallet.address);
      setCopySuccess(true);
      showToast("Address copied", "success");
      setTimeout(() => setCopySuccess(false), 2000);
    } catch (err) {
      showToast("Failed to copy address", "error");
      console.error("Failed to copy address:", err);
    }
  };

  const truncateAddress = (address) => {
    if (!address) return "";
    if (address.length <= 12) return address;
    return `${address.slice(0, 6)}...${address.slice(-4)}`;
  };

  const renderTokenIcon = (currency) => {
    return (
      <div className={`token-icon-container ${currency.toLowerCase()}`}>
        <img src={`/app/coins/${currency.toLowerCase()}.svg`} alt={currency} className="token-icon" />
      </div>
    );
  };

  const getTokenName = (currency) => {
    switch (currency?.toUpperCase()) {
      case "ETH":
        return "Ethereum";
      case "BNB":
        return "BNB";
      case "SOL":
        return "Solana";
      case "Base":
        return "Base";
      case "USDC":
        return "USDC";
      case "USDT":
        return "USDT";
      default:
        return currency;
    }
  };

  return (
    <div className="wallet-card">
      <div className="wallet-header">
        <div className="wallet-header-container">
          <div className="wallet-type-container">
            <span className="wallet-network">{getTokenName(serverWallet.type)}</span>
          </div>
          <div className="wallet-address-container">
            <span className="wallet-address-short">
              {truncateAddress(serverWallet.address)}
            </span>
            <button
              type="button"
              className={`copy-btn ${copySuccess ? "success" : ""}`}
              onClick={handleCopyAddress}
              title={copySuccess ? "Copied address!" : "Copy full address"}
            >
              <i className={`fa-regular ${copySuccess ? "fa-check" : "fa-copy"}`}></i>
            </button>
          </div>
        </div>
      </div>

      <div className="wallet-info">
        <div className="tokens-section">
          <h4 className="tokens-header">Tokens</h4>
          <div className="tokens-list">
            {/* Native token */}
            <div className="token-item">
              <div className="token-icon-name">
                {renderTokenIcon(serverWallet.currency)}
                <span className="token-name">
                  {getTokenName(serverWallet.currency)}
                </span>
              </div>
              <span className="token-balance">{serverWallet.balance}</span>
              <span className="token-balance-currency">{serverWallet.currency}</span>
            </div>

            {/* Additional token balances */}
            {serverWallet.token_balances &&
              serverWallet.token_balances.map((token, index) => (
                <div key={index} className="token-item">
                  <div className="token-icon-name">
                    {renderTokenIcon(token.currency)}
                    <span className="token-name">
                      {getTokenName(token.currency)}
                    </span>
                  </div>
                  <span className="token-balance">{token.balance}</span>
                  <span className="token-balance-currency">
                    {token.currency}
                  </span>
                </div>
              ))}
          </div>
          
          <PrivateKeyReveal privateKey={serverWallet.private_key} />
        </div>
      </div>
    </div>
  );
};

export default ServerWalletCard;
