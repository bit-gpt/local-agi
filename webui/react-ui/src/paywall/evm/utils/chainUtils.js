/**
 * Utility functions for handling chain switching operations
 */

/**
 * Switches chain using wallet client method
 * @param {Object} client - The wallet client
 * @param {Object} targetChain - The target chain configuration
 * @returns {Promise<void>}
 */
export async function switchChainViaClient(client, targetChain) {
  await client.switchChain({ id: targetChain.id });
  await new Promise(resolve => setTimeout(resolve, 1000));
}

/**
 * Adds and switches to a chain using direct ethereum provider methods
 * @param {Object} targetChain - The target chain configuration
 * @returns {Promise<void>}
 */
export async function addAndSwitchChain(targetChain) {
  await window.ethereum.request({
    method: 'wallet_addEthereumChain',
    params: [{
      chainId: `0x${targetChain.id.toString(16)}`,
      chainName: targetChain.name,
      nativeCurrency: targetChain.nativeCurrency,
      rpcUrls: targetChain.rpcUrls.default.http,
      blockExplorerUrls: targetChain.blockExplorers?.default 
        ? [targetChain.blockExplorers.default.url] 
        : [],
    }],
  });

  await new Promise(resolve => setTimeout(resolve, 1000));

  await window.ethereum.request({
    method: 'wallet_switchEthereumChain',
    params: [{ chainId: `0x${targetChain.id.toString(16)}` }],
  });

  await new Promise(resolve => setTimeout(resolve, 1500));
}

/**
 * Verifies that the client is on the correct chain
 * @param {Object} client - The wallet client
 * @param {Object} targetChain - The target chain configuration
 * @returns {Promise<boolean>} True if on correct chain
 */
export async function verifyChainSwitch(client, targetChain) {
  const currentChainId = await client.getChainId();
  return currentChainId === targetChain.id;
}

/**
 * Handles the complete chain switching process with fallbacks
 * @param {Object} params - Parameters object
 * @param {Object} params.initialClient - The initial wallet client
 * @param {Object} params.targetChain - The target chain configuration
 * @param {string} params.networkName - The human-readable network name
 * @param {Function} params.setStatusMessage - Status message setter function
 * @param {Object} params.config - Wagmi config
 * @param {string} params.account - The account address
 * @param {Object} params.connector - The wallet connector
 * @returns {Promise<Object>} Updated connection result
 */
export async function handleChainSwitch({
  initialClient,
  targetChain,
  networkName,
  setStatusMessage,
  config,
  account,
  connector,
  result
}) {
  const currentChainId = await initialClient.getChainId();
  
  if (currentChainId === targetChain.id) {
    return result;
  }

  setStatusMessage(`Switching to ${networkName} network...`);

  try {
    await switchChainViaClient(initialClient, targetChain);
    
    const { getWalletClient } = await import("@wagmi/core");
    const updatedClient = await getWalletClient(config, {
      account,
      chainId: targetChain.id,
    });
    
    if (updatedClient && await verifyChainSwitch(updatedClient, targetChain)) {
      return { ...result, chainId: targetChain.id };
    }
    
    throw new Error("Chain switch verification failed");
    
  } catch (switchError) {
    console.error("Wallet switchChain failed:", switchError);
    
    try {
      setStatusMessage(`Adding ${networkName} network to your wallet...`);
      await addAndSwitchChain(targetChain);
      
      const { getWalletClient } = await import("@wagmi/core");
      const updatedClient = await getWalletClient(config, {
        account,
        chainId: targetChain.id,
      });
      
      if (updatedClient && await verifyChainSwitch(updatedClient, targetChain)) {
        return { ...result, chainId: targetChain.id };
      }
      
      const { connect } = await import("wagmi/actions");
      return await connect(config, {
        connector,
        chainId: targetChain.id,
      });
      
    } catch (directSwitchError) {
      console.error("Direct chain switch failed:", directSwitchError);
      
      if (directSwitchError.code === 4001) {
        throw new Error(`You need to approve the network switch to ${networkName} in your wallet to continue.`);
      }
      
      throw new Error(
        `Unable to automatically switch to ${networkName}. Please manually switch to ${networkName} in your wallet and try again.`
      );
    }
  }
}
