import { useState, useRef, useEffect, useCallback } from "react";
import { useParams, useOutletContext } from "react-router-dom";
import { useChat } from "../hooks/useChat";
import Header from "../components/Header";
import { agentApi } from "../utils/api";
import TypingIndicator from "../components/TypingIndicator";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

function Chat() {
  const { id } = useParams();
  const { showToast } = useOutletContext();
  const [message, setMessage] = useState("");
  const [agentConfig, setAgentConfig] = useState(null);
  const messagesEndRef = useRef(null);
  
  // Observable status tracking
  const [currentStatus, setCurrentStatus] = useState(null);
  const [eventSource, setEventSource] = useState(null);
  
  // Helper function to map observable data to user-friendly status messages
  const getStatusMessage = (observable) => {
    if (!observable) return null;
    
    // Check for errors first
    if (observable.completion?.error) {
      return 'Error while processing. Please try again.';
    }
    
    const name = observable.name?.toLowerCase() || '';
    
    // Map different observable types to user-friendly messages
    switch (name) {
      case 'job':
        return 'Thinking';
      case 'decision':
        // Check for tool calls in completion to provide more specific status
        const completion = observable.completion;
        if (completion?.chat_completion_response?.choices?.[0]?.message?.tool_calls) {
          const toolCalls = completion.chat_completion_response.choices[0].message.tool_calls;
          if (Array.isArray(toolCalls) && toolCalls.length > 0) {
            let toolName = toolCalls[0].function?.name || toolCalls[0].name || '';
            
            // Try to extract actual tool from arguments if function name is generic (like "pick_tool")
            if (toolName === 'pick_tool' || toolName === 'call_tool') {
              try {
                const args = JSON.parse(toolCalls[0].function?.arguments || '{}');
                if (args.tool) {
                  toolName = args.tool;
                }
              } catch (e) {
                // If parsing fails, keep the original toolName
                console.log('Failed to parse tool arguments:', e);
              }
            }
            
            if (toolName.toLowerCase().includes('reasoning') || toolName.toLowerCase().includes('reason')) {
              return 'Reasoning';
            }
            // if (toolName.toLowerCase().includes('search')) {
            //   return 'Searching the web';
            // }
            // if (toolName.toLowerCase().includes('browse')) {
            //   return 'Browsing the web';
            // }
            // if (toolName.toLowerCase().includes('github')) {
            //   return 'Checking GitHub';
            // }
            // if (toolName.toLowerCase().includes('email') || toolName.toLowerCase().includes('mail')) {
            //   return 'Composing email';
            // }
            // if (toolName.toLowerCase().includes('shell') || toolName.toLowerCase().includes('command')) {
            //   return 'Running command';
            // }
            // if (toolName.toLowerCase().includes('write') || toolName.toLowerCase().includes('create')) {
            //   return 'Writing content';
            // }
            // if (toolName.toLowerCase().includes('read') || toolName.toLowerCase().includes('get')) {
            //   return 'Reading content';
            // }
            if (toolName.toLowerCase().includes('reply')) {
              return 'Preparing response';
            }
            // Generic tool call message
            return null;
          }
        }
        return 'Thinking';
      case 'action':
        // Try to get more specific message based on action type
        const actionName = observable.creation?.function_definition?.name;
        if (actionName) {
          if (actionName.includes('search')) return 'Searching the web';
          if (actionName.includes('browse')) return 'Browsing the web page';
          if (actionName.includes('github')) return 'Checking GitHub';
          if (actionName.includes('email') || actionName.includes('mail')) return 'Sending email';
          if (actionName.includes('shell')) return 'Running command';
          return `Running ${actionName}`;
        }
        return 'Taking action';
      case 'filter':
        return 'Checking permissions';
      default:
        return 'Processing';
    }
  };

  // Helper function to check if an observable has an error
  const hasError = (observable) => {
    return observable?.completion?.error;
  };

  const [paymentStatus, setPaymentStatus] = useState('pending'); // 'pending', 'processing', 'success'
  const [showPaymentModal, setShowPaymentModal] = useState(false);
  const [injectedPayment, setInjectedPayment] = useState(false);

  // Fetch agent config on mount
  useEffect(() => {
    const fetchAgentConfig = async () => {
      try {
        const config = await agentApi.getAgentConfig(id);
        setAgentConfig(config);
        // setIsOpenRouter(config.model.split("/")[0] === "openrouter");
      } catch (error) {
        console.error("Failed to load agent config", error);
        showToast && showToast(error?.message || String(error), "error");
        setAgentConfig(null);
      }
    };
    fetchAgentConfig();
  }, []);

  // Setup SSE connection for observable updates
  useEffect(() => {
    if (!id) return;

    const sse = new EventSource(`/sse/${id}`);
    setEventSource(sse);

    sse.addEventListener('observable_update', (event) => {
      const data = JSON.parse(event.data);
      
      if (data.completion) {
        // Observable is completed
        if (data.completion.error) {
          setCurrentStatus({ message: 'Error while processing. Please try again.', isError: true });
        } else {
          if(data.name.toLowerCase() === 'decision') {
            const statusMessage = getStatusMessage(data);
            if (statusMessage) {
              setCurrentStatus({ message: statusMessage, isError: false });
            }
          }
        }
      } else {
        // Observable is active, show its status
        const statusMessage = getStatusMessage(data);
        if (statusMessage) {
          setCurrentStatus({ message: statusMessage, isError: false });
        }
      }
    });

    sse.onerror = (err) => {
      console.error('SSE connection error:', err);
    };

    return () => {
      if (sse) {
        sse.close();
      }
    };
  }, [id]);

  // Callback to clear status when chat processing is completed
  const handleStatusCompleted = useCallback(() => {
    setCurrentStatus(null);
  }, []);

  // Use our custom chat hook with model from agent config
  const {
    messages,
    sending,
    error,
    isConnected,
    sendMessage,
    clearChat,
    clearError,
  } = useChat(id, agentConfig?.model, handleStatusCompleted);

  // Add demo messages for screenshot if chat is empty (only once)
  const [demoInjected, setDemoInjected] = useState(false);
  useEffect(() => {
    if (!demoInjected && messages.length === 0 && agentConfig) {
      messages.push(
        {
          sender: 'user',
          type: 'user',
          content: 'Give me a detailed report on the most important AI model releases from the last 6 months, including their capabilities, benchmarks, and key differences.',
          timestamp: new Date(Date.now() - 300000)
        },
        {
          sender: 'agent',
          type: 'agent',
          content: 'I can help you with that! However, this advanced data analysis feature requires pro plan.',
          timestamp: new Date(Date.now() - 240000)
        },
        {
          sender: 'system',
          type: 'system',
          content: 'payment_required',
          timestamp: new Date(Date.now() - 180000),
          paymentDetails: {
            service: 'Pro Plan',
            price: '20',
            currency: 'USDT',
          }
        }
      );
      setDemoInjected(true);
    }
  }, [messages, demoInjected, agentConfig]);


  // Clear status when we receive a new assistant message
  useEffect(() => {
    if (messages.length > 0) {
      const lastMessage = messages[messages.length - 1];
      if (lastMessage.sender === 'agent' && !lastMessage.loading) {
        setCurrentStatus(null);
      }
    }
  }, [messages]);

  // Detect payment_required message
  const paymentMessage = messages.find(
    (msg) => (msg.type === 'system' || msg.sender === 'system') && msg.content === 'payment_required'
  );

  useEffect(() => {
    if (paymentMessage && paymentStatus !== 'success') {
      setShowPaymentModal(true);
    } else {
      setShowPaymentModal(false);
    }
  }, [paymentMessage, paymentStatus]);

  // Payment handlers
  const handlePayment = () => {
    setPaymentStatus('processing');
    setTimeout(() => {
      setPaymentStatus('success');
      setTimeout(() => setShowPaymentModal(false), 2000);
    }, 3000);
  };
  const handleDeclinePayment = () => setShowPaymentModal(false);

  // Inject payment_required message after user asks for 'analyze' (demo only)
  useEffect(() => {
    if (!injectedPayment && messages.length > 0) {
      const last = messages[messages.length - 1];
      if (last.sender === 'user' && /analyze/i.test(last.content)) {
        // Only inject if not already present
        if (!messages.some(m => (m.type === 'system' || m.sender === 'system') && m.content === 'payment_required')) {
          messages.push({
            sender: 'system',
            type: 'system',
            content: 'payment_required',
            paymentDetails: {
              service: 'Pro Plan',
              price: '2.50',
              currency: 'USDT',
            }
          });
          setInjectedPayment(true);
        }
      }
    }
  }, [messages, injectedPayment]);


  useEffect(() => {
    if (agentConfig) {
      document.title = `Chat with ${agentConfig.name} - LocalAGI`;
    }
    return () => {
      document.title = "LocalAGI";
    };
  }, [agentConfig]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, currentStatus]);

  // useEffect(() => {
  //   if (error) {
  //     showToast && showToast(error?.message || String(error), "error");
  //     clearError();
  //   }
  // }, [error, showToast, clearError]);

  const handleSend = (e) => {
    e.preventDefault();
    if (message.trim() !== "") {
      sendMessage(message);
      setMessage("");
      // Clear any existing status when sending a new message
      setCurrentStatus(null);
    }
  };

  if (!agentConfig) {
    return (
      <div className="dashboard-container">
        <div className="main-content-area">
          <div className="loading-container">
            <div className="spinner"></div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="dashboard-container">
      <div className="main-content-area">
        <div className="header-container">
          <Header
            title="Chat with"
            description="Send messages and interact with your agent in real time."
            name={agentConfig.name}
          />
          {/* No right content for chat header */}
        </div>

        {/* Chat Window */}
        <div
          className="section-box chat-section-box"
          style={{
            width: "100%",
            height: "calc(100vh - 300px)",
            display: "flex",
            flexDirection: "column",
            margin: 0,
            maxWidth: "none",
          }}
        >
          <div
            style={{
              flex: 1,
              overflowY: "auto",
            }}
          >
            {messages.length === 0 ? (
              <div
                style={{
                  color: "var(--text-light)",
                  textAlign: "center",
                  marginTop: 48,
                }}
              >
                No messages yet. Say hello!
              </div>
            ) : (
              messages.map((msg, idx) => (
                msg.loading ? (
                  null
                ) : (
                  <div
                    key={idx}
                    style={{
                      marginBottom: 12,
                      display: "flex",
                      flexDirection:
                        msg.sender === "user" ? "row-reverse" : "row",
                    }}
                  >
                    {
                      msg.sender === "user" ? (
                        <div
                          style={{
                            background: "#e0e7ff",
                            color: "#222",
                            borderRadius: 18,
                            padding: "12px 18px",
                            maxWidth: "70%",
                            fontSize: "1rem",
                            boxShadow: "0 2px 6px rgba(0,0,0,0.04)",
                            alignSelf: "flex-end",
                          }}
                        >
                          <div className="markdown-content">
                            <ReactMarkdown remarkPlugins={[remarkGfm]}>{msg.content}</ReactMarkdown>
                          </div>
                        </div>
                      ) : (
                        // Check if this is an error message
                        msg.type === "error" ? (
                          <div
                            style={{
                              color: "#991b1b",
                              padding: "12px 0",
                              maxWidth: "70%",
                              fontSize: "1rem",
                              alignSelf: "flex-start",
                              display: "flex",
                              alignItems: "center",
                              gap: "8px",
                            }}
                          >
                            <span style={{ fontSize: "16px", fontWeight: 400 }}>
                              <div className="markdown-content">
                                <ReactMarkdown remarkPlugins={[remarkGfm]}>Error while processing. Please try again.</ReactMarkdown>
                              </div>
                            </span>
                          </div>
                        ) : (
                          // Payment required card for system message
                          msg.type === 'system' && msg.content === 'payment_required' ? (
                            <div style={{
                              background: '#fef3c7',
                              border: '1px solid #fde68a',
                              borderRadius: '16px',
                              padding: '18px 20px',
                              maxWidth: '70%',
                              alignSelf: 'flex-start',
                              display: 'flex',
                              flexDirection: 'column',
                              gap: '8px',
                              boxShadow: '0 2px 6px rgba(251,191,36,0.08)',
                              minWidth: '320px'
                            }}>
                              <div style={{ 
                                display: 'flex', 
                                alignItems: 'center', 
                                gap: '8px',
                                marginBottom: '8px',
                                fontWeight: 600,
                                color: '#92400e',
                                fontSize: '16px'
                              }}>
                                <span>💳</span>
                                Payment Required
                              </div>
                              <div style={{ fontSize: '13px', color: '#78350f', marginBottom: 8 }}>
                                This action requires a <b>{msg.paymentDetails?.service}</b>
                              </div>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: 8 }}>
                                <span style={{ fontWeight: 600, fontSize: '15px' }}>Amount:</span>
                                <span style={{ fontWeight: 700, fontSize: '18px', color: '#1f2937', display: 'flex', alignItems: 'center', gap: '6px' }}>
                                  {msg.paymentDetails?.price} {msg.paymentDetails?.currency}
                                </span>
                              </div>
                              {paymentStatus !== 'success' ? (
                                <button
                                  onClick={() => setShowPaymentModal(true)}
                                  style={{
                                    marginTop: 4,
                                    padding: '10px 18px',
                                    background: '#f0b90b',
                                    border: 'none',
                                    borderRadius: '8px',
                                    color: '#000',
                                    fontSize: '15px',
                                    fontWeight: 600,
                                    cursor: 'pointer',
                                    display: 'flex',
                                    width: 'max-content',
                                    alignItems: 'center',
                                    gap: '8px',
                                    boxShadow: '0 1px 2px rgba(0,0,0,0.03)'
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
                              ) : (
                                <span style={{
                                  marginTop: 4,
                                  background: '#10b981',
                                  color: 'white',
                                  borderRadius: '6px',
                                  padding: '4px 10px',
                                  fontWeight: 600,
                                  fontSize: '13px',
                                  alignSelf: 'flex-start'
                                }}>
                                  Payment completed
                                </span>
                              )}
                            </div>
                          ) : (
                            <div
                              style={{
                                background: "transparent",
                                color: "#222",
                                padding: "12px 0",
                                maxWidth: "70%",
                                fontSize: "1rem",
                                alignSelf: "flex-start",
                                position: "relative",
                              }}
                            >
                              <div className="markdown-content">
                                <ReactMarkdown remarkPlugins={[remarkGfm]}>{msg.content}</ReactMarkdown>
                              </div>
                            </div>
                          )
                        )
                      )
                    }
                  </div>
                )
              ))
            )}
            
            {/* Show current status as a temporary message */}
            {currentStatus && (
              <div
                style={{
                  marginBottom: 12,
                  display: "flex",
                  flexDirection: "row",
                }}
              >
                <div
                  style={{
                    color: currentStatus.isError ? "#991b1b" : "#6b7280",
                    padding: "12px 0",
                    maxWidth: "70%",
                    fontSize: "1rem",
                    alignSelf: "flex-start",
                    display: "flex",
                    alignItems: "center",
                    gap: "8px",
                  }}
                >
                  {currentStatus.isError ? (
                    null
                  ) : (
                    <div
                      style={{
                        width: "16px",
                        height: "16px",
                        border: "2px solid #e5e7eb",
                        borderTop: "2px solid #6b7280",
                        borderRadius: "50%",
                        animation: "spin 1s linear infinite",
                        flexShrink: 0,
                      }}
                    />
                  )}
                  <span style={{ fontSize: "16px", fontWeight: currentStatus.isError ? 400 : 500 }}>{currentStatus.message}</span>
                </div>
              </div>
            )}
            
            <div ref={messagesEndRef} />
          </div>

          {/* Payment Modal Overlay */}
          {showPaymentModal && paymentMessage && (
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
                          {paymentMessage.paymentDetails?.service}
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
                          {paymentMessage.paymentDetails?.price} {paymentMessage.paymentDetails?.currency}
                        </span>
                      </div>
                    </div>
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
          <form
            onSubmit={handleSend}
            style={{ display: "flex", gap: 12, alignItems: "center" }}
            autoComplete="off"
          >
            <input
              type="text"
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              placeholder={
                isConnected
                  ? "Type your message..."
                  : "Connecting..."
              }
              disabled={showPaymentModal || sending || !isConnected}
              style={{
                flex: 1,
                padding: "12px 16px",
                border: "1px solid #e5e7eb",
                borderRadius: 8,
                fontSize: "1rem",
                background:
                  showPaymentModal || sending || !isConnected
                    ? "#f3f4f6"
                    : "#fff",
                color: "#222",
                outline: "none",
                transition: "border-color 0.15s",
              }}
            />
            <button
              type="submit"
              className="action-btn"
              style={{ minWidth: 120 }}
              disabled={showPaymentModal || sending || !isConnected}
            >
              <i className="fas fa-paper-plane"></i> Send
            </button>
            <button
              type="button"
              className="action-btn"
              style={{ background: "#f6f8fa", color: "#222", minWidth: 120 }}
              onClick={clearChat}
              disabled={showPaymentModal || sending || messages.length === 0}
            >
              <i className="fas fa-trash"></i> Clear Chat
            </button>
          </form>
        </div>
        {/* CSS Animation */}
        <style jsx>{`
          @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
          }
        `}</style>
      </div>
    </div>
  );
}

export default Chat;
