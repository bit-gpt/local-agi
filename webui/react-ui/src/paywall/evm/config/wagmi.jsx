import { createConfig, http } from "wagmi";
import { bsc, base } from "viem/chains";
import {
  injected,
  coinbaseWallet,
  metaMask,
  walletConnect,
} from "wagmi/connectors";

const config = createConfig({
  chains: [bsc, base],
  connectors: [
    metaMask(),
    coinbaseWallet({ appName: "BitGPT Agents" }),
    walletConnect({ projectId: "3fbb6bba6f1de962d911bb5b5c9dba88" }),
    injected({ shimDisconnect: true }),
  ],
  transports: {
    [bsc.id]: http(),
    [base.id]: http(),
  },
});

export { config };
