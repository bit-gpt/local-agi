import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';

function ChatPaymentDemo() {
  const navigate = useNavigate();
  const [paymentStatus, setPaymentStatus] = useState('pending'); // 'pending', 'processing', 'success', 'failed'
  const [showPaymentModal, setShowPaymentModal] = useState(true);

  // Mock chat messages
  const [messages] = useState([
    {
      id: 1,
      type: 'user',
      content: 'Can you analyze this large dataset and generate a detailed report?',
      timestamp: new Date(Date.now() - 300000)
    },
    {
      id: 2,
      type: 'agent',
      content: 'I can help you with that! However, this advanced data analysis feature requires premium access.',
      timestamp: new Date(Date.now() - 240000)
    },
    {
      id: 3,
      type: 'system',
      content: 'payment_required',
      timestamp: new Date(Date.now() - 180000),
      paymentDetails: {
        service: 'Advanced Data Analysis',
        price: '2.50',
        currency: 'USDT',
        description: 'This feature uses advanced AI models and significant computational resources.'
      }
    }
  ]);

  const handlePayment = () => {
    setPaymentStatus('processing');
    
    // Simulate payment processing
    setTimeout(() => {
      setPaymentStatus('success');
      setTimeout(() => {
        setShowPaymentModal(false);
      }, 2000);
    }, 3000);
  };

  const handleDeclinePayment = () => {
    setShowPaymentModal(false);
  };

  const formatTime = (date) => {
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  return (
    <div style={{
      minHeight: '100vh',
      background: '#f9fafb',
      fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'
    }}>
      {/* Header */}
      <div style={{
        background: '#1e54bf',
        color: 'white',
        padding: '16px 24px',
        borderBottom: '1px solid #e5e7eb'
      }}>
        <div style={{
          maxWidth: '1200px',
          margin: '0 auto',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <button
              onClick={() => navigate('/agents')}
              style={{
                background: 'rgba(255, 255, 255, 0.1)',
                border: 'none',
                color: 'white',
                padding: '8px',
                borderRadius: '4px',
                cursor: 'pointer'
              }}
            >
              ← Back
            </button>
            <h1 style={{ fontSize: '20px', margin: 0 }}>Chat with Data Analyst Agent</h1>
          </div>
          <div style={{
            background: 'rgba(255, 255, 255, 0.1)',
            padding: '6px 12px',
            borderRadius: '16px',
            fontSize: '14px'
          }}>
            Binance Pay Enabled
          </div>
        </div>
      </div>

      {/* Chat Container */}
      <div style={{
        maxWidth: '800px',
        margin: '0 auto',
        height: 'calc(100vh - 80px)',
        display: 'flex',
        flexDirection: 'column',
        background: 'white',
        borderLeft: '1px solid #e5e7eb',
        borderRight: '1px solid #e5e7eb'
      }}>
        {/* Messages */}
        <div style={{
          flex: 1,
          padding: '24px',
          overflowY: 'auto',
          display: 'flex',
          flexDirection: 'column',
          gap: '16px'
        }}>
          {messages.map((message) => (
            <div key={message.id} style={{
              display: 'flex',
              flexDirection: message.type === 'user' ? 'row-reverse' : 'row',
              alignItems: 'flex-start',
              gap: '12px'
            }}>
              {/* Avatar */}
              <div style={{
                width: '40px',
                height: '40px',
                borderRadius: '50%',
                background: message.type === 'user' ? '#1e54bf' : 
                           message.type === 'agent' ? '#10b981' : '#f59e0b',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: 'white',
                fontSize: '14px',
                fontWeight: 'bold',
                flexShrink: 0
              }}>
                {message.type === 'user' ? 'U' : 
                 message.type === 'agent' ? 'A' : '$'}
              </div>

              {/* Message Content */}
              <div style={{
                maxWidth: '70%',
                display: 'flex',
                flexDirection: 'column',
                gap: '4px'
              }}>
                <div style={{
                  background: message.type === 'user' ? '#1e54bf' : 
                             message.type === 'agent' ? '#f3f4f6' : '#fef3c7',
                  color: message.type === 'user' ? 'white' : '#1f2937',
                  padding: '12px 16px',
                  borderRadius: '18px',
                  fontSize: '14px',
                  lineHeight: '1.5'
                }}>
                  {message.type === 'system' && message.content === 'payment_required' ? (
                    <div>
                      <div style={{ 
                        display: 'flex', 
                        alignItems: 'center', 
                        gap: '8px',
                        marginBottom: '8px',
                        fontWeight: '600',
                        color: '#92400e'
                      }}>
                        <span>💳</span>
                        Payment Required
                      </div>
                      <div style={{ fontSize: '13px', color: '#78350f' }}>
                        This action requires a premium feature
                      </div>
                    </div>
                  ) : (
                    message.content
                  )}
                </div>
                <div style={{
                  fontSize: '11px',
                  color: '#6b7280',
                  textAlign: message.type === 'user' ? 'right' : 'left'
                }}>
                  {formatTime(message.timestamp)}
                </div>
              </div>
            </div>
          ))}
        </div>

        {/* Payment Modal Overlay */}
        {showPaymentModal && (
          <div style={{
            position: 'fixed',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            background: 'rgba(0, 0, 0, 0.5)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000
          }}>
            <div style={{
              background: 'white',
              borderRadius: '12px',
              padding: '32px',
              width: '90%',
              maxWidth: '480px',
              boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.25)'
            }}>
              {paymentStatus === 'pending' && (
                <>
                  {/* Header */}
                  <div style={{ textAlign: 'center', marginBottom: '24px' }}>
                    <div style={{
                      width: '64px',
                      height: '64px',
                      background: 'linear-gradient(135deg, #f0b90b 0%, #e6a50b 100%)',
                      borderRadius: '50%',
                      margin: '0 auto 16px',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      fontSize: '24px'
                    }}>
                      💳
                    </div>
                    <h3 style={{
                      fontSize: '20px',
                      fontWeight: '600',
                      color: '#1f2937',
                      margin: '0 0 8px 0'
                    }}>
                      Premium Feature Payment
                    </h3>
                    <p style={{
                      fontSize: '14px',
                      color: '#6b7280',
                      margin: 0
                    }}>
                      Complete payment to access this feature
                    </p>
                  </div>

                  {/* Service Details */}
                  <div style={{
                    background: '#f9fafb',
                    border: '1px solid #e5e7eb',
                    borderRadius: '8px',
                    padding: '16px',
                    marginBottom: '24px'
                  }}>
                    <div style={{ marginBottom: '12px' }}>
                      <div style={{ fontSize: '12px', color: '#6b7280', marginBottom: '4px' }}>
                        SERVICE
                      </div>
                      <div style={{ fontSize: '16px', fontWeight: '600', color: '#1f2937' }}>
                        {messages[2].paymentDetails.service}
                      </div>
                    </div>
                    
                    <div style={{ marginBottom: '12px' }}>
                      <div style={{ fontSize: '12px', color: '#6b7280', marginBottom: '4px' }}>
                        DESCRIPTION
                      </div>
                      <div style={{ fontSize: '14px', color: '#1f2937' }}>
                        {messages[2].paymentDetails.description}
                      </div>
                    </div>

                    <div style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                      paddingTop: '12px',
                      borderTop: '1px solid #e5e7eb'
                    }}>
                      <span style={{ fontSize: '14px', color: '#6b7280' }}>Total Amount:</span>
                      <span style={{ 
                        fontSize: '20px', 
                        fontWeight: '700', 
                        color: '#1f2937',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '8px'
                      }}>
                        {messages[2].paymentDetails.price} {messages[2].paymentDetails.currency}
                        <img 
                          src="data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPGNpcmNsZSBjeD0iMTIiIGN5PSIxMiIgcj0iMTEiIGZpbGw9IiMxNEI4NzkiLz4KPGV4dCB4PSI4IiB5PSIxNSIgZmlsbD0id2hpdGUiIGZvbnQtZmFtaWx5PSJBcmlhbCIgZm9udC1zaXplPSI2IiBmb250LXdlaWdodD0iYm9sZCI+VVNEVDwvdGV4dD4KPC9zdmc+" 
                          alt="USDT" 
                          style={{ width: '20px', height: '20px' }}
                        />
                      </span>
                    </div>
                  </div>

                  {/* Payment Method */}
                  <div style={{
                    background: '#181a20',
                    borderRadius: '8px',
                    padding: '16px',
                    marginBottom: '24px'
                  }}>
                    <div style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '12px',
                      marginBottom: '8px'
                    }}>
                      <div style={{
                        background: '#f0b90b',
                        width: '24px',
                        height: '24px',
                        borderRadius: '4px',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        fontSize: '14px',
                        fontWeight: 'bold',
                        color: '#000'
                      }}>
                        B
                      </div>
                      <span style={{ color: '#eaecef', fontWeight: '600' }}>Binance Pay</span>
                      <span style={{
                        background: '#0ecb81',
                        color: 'white',
                        fontSize: '10px',
                        padding: '2px 6px',
                        borderRadius: '4px',
                        fontWeight: '600'
                      }}>
                        CONNECTED
                      </span>
                    </div>
                    <div style={{ fontSize: '12px', color: '#848e9c' }}>
                      Merchant: LOCALAGI001 • Fast & Secure
                    </div>
                  </div>

                  {/* Action Buttons */}
                  <div style={{ display: 'flex', gap: '12px' }}>
                    <button
                      onClick={handleDeclinePayment}
                      style={{
                        flex: 1,
                        padding: '12px',
                        background: '#f3f4f6',
                        border: '1px solid #d1d5db',
                        borderRadius: '8px',
                        color: '#6b7280',
                        fontSize: '14px',
                        fontWeight: '500',
                        cursor: 'pointer'
                      }}
                    >
                      Cancel
                    </button>
                    <button
                      onClick={handlePayment}
                      style={{
                        flex: 2,
                        padding: '12px',
                        background: '#f0b90b',
                        border: 'none',
                        borderRadius: '8px',
                        color: '#000',
                        fontSize: '14px',
                        fontWeight: '600',
                        cursor: 'pointer',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        gap: '8px'
                      }}
                    >
                      <span style={{
                        background: '#000',
                        color: '#f0b90b',
                        width: '16px',
                        height: '16px',
                        borderRadius: '4px',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        fontSize: '10px',
                        fontWeight: 'bold'
                      }}>
                        B
                      </span>
                      Pay with Binance Pay
                    </button>
                  </div>
                </>
              )}

              {paymentStatus === 'processing' && (
                <div style={{ textAlign: 'center', padding: '20px 0' }}>
                  <div style={{
                    width: '48px',
                    height: '48px',
                    border: '4px solid #f3f4f6',
                    borderTop: '4px solid #f0b90b',
                    borderRadius: '50%',
                    margin: '0 auto 24px',
                    animation: 'spin 1s linear infinite'
                  }}></div>
                  <h3 style={{
                    fontSize: '18px',
                    fontWeight: '600',
                    color: '#1f2937',
                    marginBottom: '8px'
                  }}>
                    Processing Payment
                  </h3>
                  <p style={{
                    fontSize: '14px',
                    color: '#6b7280',
                    margin: 0
                  }}>
                    Please wait while we process your payment via Binance Pay...
                  </p>
                </div>
              )}

              {paymentStatus === 'success' && (
                <div style={{ textAlign: 'center', padding: '20px 0' }}>
                  <div style={{
                    width: '64px',
                    height: '64px',
                    background: '#10b981',
                    borderRadius: '50%',
                    margin: '0 auto 24px',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: '32px',
                    color: 'white'
                  }}>
                    ✓
                  </div>
                  <h3 style={{
                    fontSize: '18px',
                    fontWeight: '600',
                    color: '#1f2937',
                    marginBottom: '8px'
                  }}>
                    Payment Successful!
                  </h3>
                  <p style={{
                    fontSize: '14px',
                    color: '#6b7280',
                    margin: 0
                  }}>
                    Your premium feature is now available. The agent will continue processing your request.
                  </p>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Input Area */}
        <div style={{
          padding: '16px 24px',
          borderTop: '1px solid #e5e7eb',
          background: '#f9fafb'
        }}>
          <div style={{
            display: 'flex',
            gap: '12px',
            alignItems: 'center'
          }}>
            <input
              type="text"
              placeholder="Type your message..."
              disabled={showPaymentModal}
              style={{
                flex: 1,
                padding: '12px 16px',
                border: '1px solid #d1d5db',
                borderRadius: '24px',
                fontSize: '14px',
                outline: 'none',
                background: showPaymentModal ? '#f3f4f6' : 'white',
                color: showPaymentModal ? '#9ca3af' : '#1f2937'
              }}
            />
            <button
              disabled={showPaymentModal}
              style={{
                padding: '12px 16px',
                background: showPaymentModal ? '#d1d5db' : '#1e54bf',
                color: 'white',
                border: 'none',
                borderRadius: '24px',
                fontSize: '14px',
                fontWeight: '500',
                cursor: showPaymentModal ? 'not-allowed' : 'pointer'
              }}
            >
              Send
            </button>
          </div>
        </div>
      </div>

      {/* CSS Animation */}
      <style jsx>{`
        @keyframes spin {
          0% { transform: rotate(0deg); }
          100% { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
}

export default ChatPaymentDemo; 