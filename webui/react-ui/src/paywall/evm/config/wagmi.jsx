import { createConfig, http } from "wagmi";
import { bsc, base } from "viem/chains";
import { injected, coinbaseWallet, metaMask,walletConnect } from "wagmi/connectors";

const config = createConfig({
  chains: [bsc, base],
  connectors: [
    metaMask(),
    coinbaseWallet({ appName: "BitGPT Agents" }),
    walletConnect({ projectId: "233c440b08a2b78d6b3e76370b979bed" }),
    injected({ shimDisconnect: true })
  ],
  transports: {
    [bsc.id]: http(),
    [base.id]: http(),
  },
});

export { config };