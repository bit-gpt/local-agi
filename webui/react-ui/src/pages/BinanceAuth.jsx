import { useState, useEffect } from "react";
import { useSearchParams, useNavigate } from "react-router-dom";

function BinanceAuth() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [isAuthorizing, setIsAuthorizing] = useState(false);
  const [authStep, setAuthStep] = useState('initial'); // 'initial', 'authorizing', 'success'
  
  // Mock merchant and app details
  const returnUrl = searchParams.get('returnUrl');
  const appName = "BitGPT";

  useEffect(() => {
    document.title = "Binance Pay Authorization - Binance";
  }, []);

  const handleAuthorize = () => {
    setIsAuthorizing(true);
    setAuthStep('authorizing');
    
    // Simulate authorization process
    setTimeout(() => {
      setAuthStep('success');
      setIsAuthorizing(false);
      
      // Auto redirect after success
      setTimeout(() => {
        // In real implementation, this would redirect with an authorization code
        const authCode = 'mock_auth_' + Date.now();
        window.location.href = `${returnUrl}?code=${authCode}&status=success`;
      }, 2000);
    }, 3000);
  };

  const handleDeny = () => {
    window.location.href = `${returnUrl}?error=access_denied&status=cancelled`;
  };

  return (
    <div style={{
      minHeight: '100vh',
      background: 'linear-gradient(180deg, #181A20 0%, #1E2329 100%)',
      fontFamily: 'Arial, sans-serif',
      color: '#EAECEF',
      margin: 0,
      padding: 0
    }}>
      {/* Binance Header */}
      <div style={{
        background: '#181A20',
        borderBottom: '1px solid #2B3139',
        padding: '16px 0'
      }}>
        <div style={{
          maxWidth: '1200px',
          margin: '0 auto',
          padding: '0 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <div style={{
              background: '#F0B90B',
              width: '32px',
              height: '32px',
              borderRadius: '4px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontWeight: 'bold',
              color: '#000',
              fontSize: '18px'
            }}>
              B
            </div>
            <span style={{ fontSize: '24px', fontWeight: '600', color: '#EAECEF' }}>
              Binance
            </span>
          </div>
          <div style={{ fontSize: '14px', color: '#848E9C' }}>
            Pay Authorization
          </div>
        </div>
      </div>

      {/* Main Content */}
      <div style={{
        maxWidth: '480px',
        margin: '60px auto',
        padding: '0 24px'
      }}>
        {/* Authorization Card */}
        <div style={{
          background: '#1E2329',
          borderRadius: '8px',
          border: '1px solid #2B3139',
          padding: '32px',
          boxShadow: '0 8px 24px rgba(0, 0, 0, 0.4)'
        }}>
          {authStep === 'initial' && (
            <>
              {/* App Icon and Title */}
              <div style={{ textAlign: 'center', marginBottom: '32px' }}>
                <img
                  src="/app/logo_1.png"
                  alt="BitGPT Network"
                  style={{
                    width: '64px',
                    height: '64px',
                    borderRadius: '12px',
                    margin: '0 auto 16px',
                    display: 'block'
                  }}
                />
                <h2 style={{
                  fontSize: '24px',
                  fontWeight: '600',
                  color: '#EAECEF',
                  margin: '0 0 8px 0'
                }}>
                  {appName} wants to access your Binance Pay
                </h2>
              </div>

              {/* Permissions */}
              <div style={{ marginBottom: '32px' }}>
                <h3 style={{
                  fontSize: '16px',
                  fontWeight: '600',
                  color: '#EAECEF',
                  marginBottom: '16px'
                }}>
                  This app will be able to:
                </h3>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                  {[
                    'Access your merchant account information',
                    'Process payments on your behalf',
                    'View transaction history',
                    'Generate payment requests'
                  ].map((permission, index) => (
                    <div key={index} style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '12px',
                      padding: '12px',
                      background: '#0D1421',
                      borderRadius: '6px',
                      border: '1px solid #1E2329'
                    }}>
                      <div style={{
                        width: '20px',
                        height: '20px',
                        background: '#0ECB81',
                        borderRadius: '50%',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        fontSize: '12px',
                        color: 'white'
                      }}>
                        ✓
                      </div>
                      <span style={{ fontSize: '14px', color: '#EAECEF' }}>
                        {permission}
                      </span>
                    </div>
                  ))}
                </div>
              </div>

              {/* Action Buttons */}
              <div style={{ display: 'flex', gap: '12px' }}>
                <button
                  onClick={handleDeny}
                  style={{
                    flex: 1,
                    padding: '14px 24px',
                    background: 'transparent',
                    border: '1px solid #474D57',
                    borderRadius: '4px',
                    color: '#EAECEF',
                    fontSize: '14px',
                    fontWeight: '500',
                    cursor: 'pointer',
                    transition: 'all 0.2s ease'
                  }}
                  onMouseOver={(e) => {
                    e.target.style.background = '#2B3139';
                    e.target.style.borderColor = '#848E9C';
                  }}
                  onMouseOut={(e) => {
                    e.target.style.background = 'transparent';
                    e.target.style.borderColor = '#474D57';
                  }}
                >
                  Deny
                </button>
                <button
                  onClick={handleAuthorize}
                  disabled={isAuthorizing}
                  style={{
                    flex: 1,
                    padding: '14px 24px',
                    background: '#F0B90B',
                    border: 'none',
                    borderRadius: '4px',
                    color: '#000',
                    fontSize: '14px',
                    fontWeight: '600',
                    cursor: isAuthorizing ? 'not-allowed' : 'pointer',
                    transition: 'all 0.2s ease',
                    opacity: isAuthorizing ? 0.6 : 1
                  }}
                  onMouseOver={(e) => {
                    if (!isAuthorizing) {
                      e.target.style.background = '#E6A50B';
                    }
                  }}
                  onMouseOut={(e) => {
                    if (!isAuthorizing) {
                      e.target.style.background = '#F0B90B';
                    }
                  }}
                >
                  {isAuthorizing ? 'Authorizing...' : 'Authorize'}
                </button>
              </div>
            </>
          )}

          {authStep === 'authorizing' && (
            <div style={{ textAlign: 'center', padding: '40px 0' }}>
              <div style={{
                width: '48px',
                height: '48px',
                border: '4px solid #2B3139',
                borderTop: '4px solid #F0B90B',
                borderRadius: '50%',
                margin: '0 auto 24px',
                animation: 'spin 1s linear infinite'
              }}></div>
              <h3 style={{
                fontSize: '20px',
                fontWeight: '600',
                color: '#EAECEF',
                marginBottom: '12px'
              }}>
                Authorizing Application
              </h3>
              <p style={{
                fontSize: '14px',
                color: '#848E9C',
                margin: 0
              }}>
                Please wait while we process your authorization...
              </p>
            </div>
          )}

          {authStep === 'success' && (
            <div style={{ textAlign: 'center', padding: '40px 0' }}>
              <div style={{
                width: '64px',
                height: '64px',
                background: '#0ECB81',
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
                fontSize: '20px',
                fontWeight: '600',
                color: '#EAECEF',
                marginBottom: '12px'
              }}>
                Authorization Successful
              </h3>
              <p style={{
                fontSize: '14px',
                color: '#848E9C',
                margin: '0 0 24px 0'
              }}>
                {appName} has been authorized to access your Binance Pay. You will be redirected shortly.
              </p>
              <div style={{
                background: '#0D1421',
                borderRadius: '6px',
                padding: '12px',
                border: '1px solid #1E2329'
              }}>
                <div style={{ fontSize: '12px', color: '#0ECB81', fontWeight: '600' }}>
                  ✓ AUTHORIZATION GRANTED
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div style={{
          textAlign: 'center',
          marginTop: '32px',
          padding: '16px'
        }}>
          <p style={{
            fontSize: '12px',
            color: '#474D57',
            margin: '0 0 8px 0'
          }}>
            By authorizing this application, you agree to Binance's Terms of Service
          </p>
          <div style={{
            display: 'flex',
            justifyContent: 'center',
            gap: '24px',
            fontSize: '12px'
          }}>
            <a href="#" style={{ color: '#848E9C', textDecoration: 'none' }}>Privacy Policy</a>
            <a href="#" style={{ color: '#848E9C', textDecoration: 'none' }}>Terms of Service</a>
            <a href="#" style={{ color: '#848E9C', textDecoration: 'none' }}>Support</a>
          </div>
        </div>
      </div>

      {/* Add CSS Animation */}
      <style jsx>{`
        @keyframes spin {
          0% { transform: rotate(0deg); }
          100% { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
}

export default BinanceAuth; 