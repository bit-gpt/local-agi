import { createContext, useState } from "react";

export const SelectedWalletAccountContext = createContext([
  undefined,
  function setSelectedWalletAccount() {
  },
]);

export function SelectedWalletAccountProvider({ children }) {
  const [selectedWalletAccount, setSelectedWalletAccount] = useState(undefined);

  return (
    <SelectedWalletAccountContext.Provider
      value={[selectedWalletAccount, setSelectedWalletAccount]}
    >
      {children}
    </SelectedWalletAccountContext.Provider>
  );
}
