import React, { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';

/**
 * Payment Settings section of the agent form
 * 
 * @param {Object} props Component props
 * @param {Object} props.formData Current form data values
 * @param {Function} props.handleInputChange Handler for input changes
 * @param {string} props.agentId The agent ID for this form
 */
const PaymentSettingsSection = ({ formData, handleInputChange, agentId }) => {
  const [searchParams] = useSearchParams();
  const [binancePayStatus, setBinancePayStatus] = useState('disconnected'); // 'disconnected', 'connected'
  
  // Mock Binance Pay connection data
  const [binanceData, setBinanceData] = useState({
    merchantCode: '',
    walletId: '',
    connectedAt: null
  });

  // Check if we're returning from Binance authorization
  useEffect(() => {
    const authCode = searchParams.get('code');
    const status = searchParams.get('status');
    
    if (authCode && status === 'success') {
      // Simulate successful connection
      setBinancePayStatus('connected');
      setBinanceData({
        merchantCode: 'BITGPT',
        walletId: 'wallet_' + Date.now(),
        connectedAt: new Date().toISOString()
      });
      
      // Clear URL params
      window.history.replaceState({}, document.title, window.location.pathname);
    }
  }, [searchParams]);

  const handleConnectBinancePay = () => {
    const baseUrl = window.location.origin;
    const returnUrl = `${baseUrl}/app/settings/${agentId}`;
    const authUrl = `/app/binance-auth?merchantCode=BITGPT&returnUrl=${encodeURIComponent(returnUrl)}`;
    
    window.location.href = authUrl;
  };

  const handleDisconnectBinancePay = () => {
    if (window.confirm('Are you sure you want to disconnect Binance Pay? This will disable payment processing for this agent.')) {
      setBinancePayStatus('disconnected');
      setBinanceData({
        merchantCode: '',
        walletId: '',
        connectedAt: null
      });
    }
  };

  return (
    <div id="payment-section">
      <h3 className="section-title">Payment Settings</h3>
      <p className="section-description">
        Configure payment methods for your agent. Enable paid services and premium features.
      </p>
      
      {/* Binance Pay Integration */}
      <div className="mb-4">
        <h4 style={{ 
          fontSize: '1.1rem', 
          fontWeight: '600', 
          marginBottom: '1rem',
          color: '#1f2937'
        }}>
          Binance Pay Integration
        </h4>
        
        <div style={{
          background: '#f9fafb',
          border: '1px solid #e5e7eb',
          borderRadius: '8px',
          padding: '20px'
        }}>
          {binancePayStatus === 'disconnected' ? (
            <>
              <div style={{ marginBottom: '16px' }}>
                <p style={{ 
                  color: '#6b7280', 
                  fontSize: '0.9rem',
                  marginBottom: '12px'
                }}>
                  Connect your Binance Pay account to enable cryptocurrency payments for premium agent features.
                </p>
              </div>
              
              <button
                type="button"
                onClick={handleConnectBinancePay}
                style={{
                  marginTop: '16px',
                  padding: '10px 20px',
                  background: '#f0b90b',
                  color: '#000',
                  border: 'none',
                  borderRadius: '6px',
                  fontSize: '0.9rem',
                  fontWeight: '600',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px',
                  transition: 'background-color 0.2s ease'
                }}
                onMouseOver={(e) => e.target.style.background = '#e6a50b'}
                onMouseOut={(e) => e.target.style.background = '#f0b90b'}
              >
                <span style={{
                  background: '#000',
                  color: '#f0b90b',
                  width: '20px',
                  height: '20px',
                  borderRadius: '4px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: '12px',
                  fontWeight: 'bold'
                }}>
                  B
                </span>
                Connect Binance Pay
              </button>
            </>
          ) : (
            <>
              <div style={{
                background: '#d1fae5',
                border: '1px solid #10b981',
                borderRadius: '6px',
                padding: '12px',
                marginBottom: '16px'
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <div style={{
                    width: '20px',
                    height: '20px',
                    background: '#10b981',
                    borderRadius: '50%',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: '12px',
                    color: 'white'
                  }}>
                    ✓
                  </div>
                  <span style={{ fontSize: '0.9rem', color: '#065f46', fontWeight: '600' }}>
                    Binance Pay Connected Successfully
                  </span>
                </div>
              </div>
              
              <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ fontSize: '0.85rem', color: '#6b7280' }}>Merchant Code:</span>
                  <span style={{ fontSize: '0.85rem', fontFamily: 'monospace', color: '#1f2937' }}>
                    {binanceData.merchantCode}
                  </span>
                </div>
                
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ fontSize: '0.85rem', color: '#6b7280' }}>Wallet ID:</span>
                  <span style={{ fontSize: '0.85rem', fontFamily: 'monospace', color: '#1f2937' }}>
                    {binanceData.walletId}
                  </span>
                </div>
                
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ fontSize: '0.85rem', color: '#6b7280' }}>Connected:</span>
                  <span style={{ fontSize: '0.85rem', color: '#1f2937' }}>
                    {binanceData.connectedAt ? new Date(binanceData.connectedAt).toLocaleDateString() : 'Unknown'}
                  </span>
                </div>
              </div>
              
              <div style={{ display: 'flex', gap: '12px', marginTop: '16px' }}>
                <button
                  type="button"
                  onClick={handleDisconnectBinancePay}
                  style={{
                    padding: '8px 16px',
                    background: '#fff',
                    color: '#dc2626',
                    border: '1px solid #fca5a5',
                    borderRadius: '6px',
                    fontSize: '0.85rem',
                    fontWeight: '500',
                    cursor: 'pointer',
                    transition: 'all 0.2s ease'
                  }}
                  onMouseOver={(e) => {
                    e.target.style.background = '#fef2f2';
                    e.target.style.borderColor = '#ef4444';
                  }}
                  onMouseOut={(e) => {
                    e.target.style.background = '#fff';
                    e.target.style.borderColor = '#fca5a5';
                  }}
                >
                  Disconnect
                </button>
                
                <button
                  type="button"
                  style={{
                    padding: '8px 16px',
                    background: '#f3f4f6',
                    color: '#6b7280',
                    border: '1px solid #d1d5db',
                    borderRadius: '6px',
                    fontSize: '0.85rem',
                    fontWeight: '500',
                    cursor: 'pointer',
                    transition: 'all 0.2s ease'
                  }}
                  onMouseOver={(e) => {
                    e.target.style.background = '#e5e7eb';
                    e.target.style.color = '#374151';
                  }}
                  onMouseOut={(e) => {
                    e.target.style.background = '#f3f4f6';
                    e.target.style.color = '#6b7280';
                  }}
                >
                  View Transactions
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
};

export default PaymentSettingsSection; 