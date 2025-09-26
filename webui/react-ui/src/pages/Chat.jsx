import { useState, useRef, useEffect, useCallback } from "react";
import { useParams, useOutletContext } from "react-router-dom";
import { useChat } from "../hooks/useChat";
import Header from "../components/Header";
import { agentApi } from "../utils/api";
import TypingIndicator from "../components/TypingIndicator";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import PaymentModal from "../paywall/components/PaymentModal";
import { SelectedWalletAccountContextProvider } from "../paywall/solana/context/SelectedWalletAccountContextProvider";
import { EvmWalletProvider } from "../paywall/evm/context/EvmWalletContext";
import { WagmiProvider } from "wagmi";
import { config } from "../paywall/evm/config/wagmi";
import { useAgent } from "../hooks/useAgent";

const PAY_LIMIT_STATUS = {
  APPROVED: "APPROVED",
  CANCELLED: "CANCELLED",
};

function Chat() {
  const { id } = useParams();
  const { showToast } = useOutletContext();
  const [message, setMessage] = useState("");
  const [agentConfig, setAgentConfig] = useState(null);
  const messagesEndRef = useRef(null);
  const [approveLoading, setApproveLoading] = useState(false);
  const [cancelLoading, setCancelLoading] = useState(false);

  const [currentStatus, setCurrentStatus] = useState(null);
  const [eventSource, setEventSource] = useState(null);
  const [requirePaymentApproval, setRequirePaymentApproval] = useState(null);
  const [paymentRequestsData, setPaymentRequestsData] = useState(null);
  const [paymentModalOpen, setPaymentModalOpen] = useState(false);
  const [hasScrolledToInitialMessages, setHasScrolledToInitialMessages] =
    useState(false);

  const getStatusMessage = (observable) => {
    if (!observable) return null;

    if (observable.completion?.error) {
      return "Error while processing. Please try again.";
    }

    const name = observable.name?.toLowerCase() || "";

    switch (name) {
      case "job":
        return "Thinking";
      case "decision":
        const completion = observable.completion;
        if (
          completion?.chat_completion_response?.choices?.[0]?.message
            ?.tool_calls
        ) {
          const toolCalls =
            completion.chat_completion_response.choices[0].message.tool_calls;
          if (Array.isArray(toolCalls) && toolCalls.length > 0) {
            let toolName =
              toolCalls[0].function?.name || toolCalls[0].name || "";

            if (toolName === "pick_tool" || toolName === "call_tool") {
              try {
                const args = JSON.parse(
                  toolCalls[0].function?.arguments || "{}"
                );
                if (args.tool) {
                  toolName = args.tool;
                }
              } catch (e) {
                console.log("Failed to parse tool arguments:", e);
              }
            }

            if (
              toolName.toLowerCase().includes("reasoning") ||
              toolName.toLowerCase().includes("reason")
            ) {
              return "Reasoning";
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
            if (toolName.toLowerCase().includes("reply")) {
              return "Preparing response";
            }
            return null;
          }
        }
        setRequirePaymentApproval(null);
        setPaymentRequestsData(null);
        return "Thinking";
      case "action":
        const actionName = observable.creation?.function_definition?.name;
        if (actionName) {
          if (actionName.includes("search_internet")) return "Searching the web";
          if (actionName.includes("browse")) return "Browsing the web page";
          if (actionName.includes("github")) return "Checking GitHub";
          if (actionName.includes("send_email"))return "Sending email";
          if (actionName.includes("shell")) return "Running command";
          if (actionName.includes("estimate_transaction_fee"))
            return "Estimating transaction fee";
          if (actionName.includes("get_server_wallet_address"))
            return "Getting server wallet address";
          if (actionName.includes("get_all_server_wallet_balances"))
            return "Getting server wallet balances";
          if (actionName.includes("get_server_wallet_balance"))
            return "Getting server wallet balance";
          if (actionName.includes("send_crypto")) return "Sending crypto";
          if (actionName.includes("wait_for_transaction_confirmation"))
            return "Waiting for transaction confirmation";
          if (actionName.includes("generate_image")) return "Generating image";
          if (actionName.includes("gmail-send-email")) return "Sending email";
          if (actionName.includes("gmail-create-draft-email")) return "Creating draft email";
          if (actionName.includes("gmail-send-draft-email")) return "Sending draft email";
          if (actionName.includes("gmail-read-email")) return "Reading email";
          if (actionName.includes("gmail-search-emails")) return "Searching emails";
          if (actionName.includes("gmail-archive-email")) return "Archiving email";
          if (actionName.includes("gmail-create-label")) return "Creating label";
          if (actionName.includes("gmail-update-label")) return "Updating label";
          if (actionName.includes("gmail-list-labels")) return "Listing labels";
          if (actionName.includes("gmail-add-label-to-email")) return "Adding label to email";
          if (actionName.includes("gmail-remove-label-from-email")) return "Removing label from email";
          if (actionName.includes("google-calendar-list-calendars")) return "Listing calendars";
          if (actionName.includes("google-calendar-list-events")) return "Listing events";
          return `Running ${actionName}`;
        }
        return "Taking action";
      case "filter":
        return "Checking permissions";
      default:
        return "Processing";
    }
  };

  useEffect(() => {
    const fetchAgentConfig = async () => {
      try {
        const config = await agentApi.getAgentConfig(id);
        setAgentConfig(config);
      } catch (error) {
        console.error("Failed to load agent config", error);
        showToast && showToast(error?.message || String(error), "error");
        setAgentConfig(null);
      }
    };
    fetchAgentConfig();
  }, []);

  useEffect(() => {
    if (!id) return;

    const sse = new EventSource(`/sse/${id}`);
    setEventSource(sse);

    sse.addEventListener("observable_update", (event) => {
      const data = JSON.parse(event.data);

      if (data.completion) {
        if (data.completion.error) {
          setCurrentStatus({
            message: "Error while processing. Please try again.",
            isError: true,
          });
        } else {
          if (data.name.toLowerCase() === "decision") {
            const statusMessage = getStatusMessage(data);
            if (statusMessage) {
              setCurrentStatus({ message: statusMessage, isError: false });
            }
          }
        }
      } else {
        const statusMessage = getStatusMessage(data);
        if (statusMessage) {
          setCurrentStatus({ message: statusMessage, isError: false });
        }
      }
    });

    sse.addEventListener("request_payment_approval", (event) => {
      console.log("request_payment_approval", event.data);
      setRequirePaymentApproval(event.data);
    });

    sse.addEventListener("request_payment_header", (event) => {
      console.log("request_payment_header", event.data);
      setPaymentRequestsData(JSON.parse(event.data));
    });

    sse.onerror = (err) => {
      console.error("SSE connection error:", err);
    };

    return () => {
      if (sse) {
        sse.close();
      }
    };
  }, [id]);

  const handleStatusCompleted = useCallback(() => {
    setCurrentStatus(null);
  }, []);

  const {
    messages,
    sending,
    error,
    isConnected,
    sendMessage,
    clearChat,
    clearError,
  } = useChat(id, agentConfig?.model, handleStatusCompleted);

  const { agent } = useAgent(id);

  useEffect(() => {
    if (messages.length > 0) {
      const lastMessage = messages[messages.length - 1];
      if (lastMessage.sender === "agent" && !lastMessage.loading) {
        setCurrentStatus(null);
      }
    }
  }, [messages]);

  useEffect(() => {
    if (agentConfig) {
      document.title = `Chat with ${agentConfig.name} - LocalAGI`;
    }
    return () => {
      document.title = "LocalAGI";
    };
  }, [agentConfig]);

  useEffect(() => {
    if (!hasScrolledToInitialMessages && messages.length > 0) {
      messagesEndRef.current?.scrollIntoView({ behavior: "instant" });
      setHasScrolledToInitialMessages(true);
    } else if (hasScrolledToInitialMessages) {
      messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [
    messages,
    currentStatus,
    requirePaymentApproval,
    paymentRequestsData,
    hasScrolledToInitialMessages,
  ]);

  const handleSend = (e) => {
    e.preventDefault();
    if (message.trim() !== "") {
      sendMessage(message);
      setMessage("");
      setCurrentStatus(null);
    }
  };

  const handleApprovePayment = async () => {
    setApproveLoading(true);
    try {
      await agentApi.updateAgentPayLimitStatus(id, PAY_LIMIT_STATUS.APPROVED);
      showToast("Payment approved successfully", "success");
      setRequirePaymentApproval(null);
    } catch (error) {
      console.error("Failed to approve payment", error);
      showToast && showToast(error?.message || String(error), "error");
    } finally {
      setApproveLoading(false);
    }
  };

  const handleCancelPayment = async () => {
    setCancelLoading(true);
    try {
      await agentApi.updateAgentPayLimitStatus(id, PAY_LIMIT_STATUS.CANCELLED);
      showToast("Payment cancelled successfully", "success");
      setRequirePaymentApproval(null);
    } catch (error) {
      console.error("Failed to cancel payment", error);
      showToast && showToast(error?.message || String(error), "error");
    } finally {
      setCancelLoading(false);
    }
  };

  const handleCancelPaymentRequest = async () => {
    setCancelLoading(true);
    try {
      await agentApi.submitPaymentHeader(id, paymentRequestsData.requestId, {
        status: "CANCELLED",
      });
      showToast("Payment request cancelled successfully", "success");
      setPaymentRequestsData(null);
    } catch (error) {
      console.error("Failed to cancel payment", error);
      showToast && showToast(error?.message || String(error), "error");
    } finally {
      setCancelLoading(false);
    }
  };

  const openPaymentModal = () => {
    setPaymentModalOpen(true);
  };

  const handlePaymentSuccess = async (paymentHeader, selectedRequestID) => {
    try {
      await agentApi.submitPaymentHeader(id, paymentRequestsData.requestId, {
        status: "APPROVED",
        paymentHeader: paymentHeader,
        selectedRequestID: selectedRequestID,
      });
      showToast("Payment request approved successfully", "success");
      setPaymentModalOpen(false);
      setPaymentRequestsData(null);
    } catch (error) {
      console.error("Failed to approve payment", error);
      showToast && showToast(error?.message || String(error), "error");
    } finally {
      setApproveLoading(false);
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
      <EvmWalletProvider>
        <SelectedWalletAccountContextProvider>
          <div className="dashboard-container">
            <div className="main-content-area">
              <div className="header-container">
                <Header
                  title="Chat with"
                  description="Send messages and interact with your agent in real time."
                  name={agentConfig.name}
                />
              </div>

              <div
                className="section-card chat-section-box"
              >
                <div className="chat-messages-container">
                  {messages.length === 0 ? (
                    <div className="chat-empty-state">
                      No messages yet. Say hello!
                    </div>
                  ) : (
                    messages.map((msg, idx) =>
                      msg.loading ? null : (
                        <div
                          key={idx}
                          className={`chat-message ${msg.sender}`}
                        >
                          {msg.sender === "user" ? (
                            <div className="chat-message-bubble user">
                              <div className="markdown-content">
                                <ReactMarkdown remarkPlugins={[remarkGfm]}>
                                  {msg.content}
                                </ReactMarkdown>
                              </div>
                            </div>
                          ) : msg.type === "error" ? (
                            <div className="chat-message-bubble error">
                              <span className="chat-error-text">
                                <div className="markdown-content">
                                  <ReactMarkdown remarkPlugins={[remarkGfm]}>
                                    Error while processing. Please try again.
                                  </ReactMarkdown>
                                </div>
                              </span>
                            </div>
                          ) : (
                            <div className="chat-message-bubble agent">
                              <div className="markdown-content">
                                <ReactMarkdown remarkPlugins={[remarkGfm]}>
                                  {msg.content}
                                </ReactMarkdown>
                              </div>
                            </div>
                          )}
                        </div>
                      )
                    )
                  )}

                  {currentStatus && (
                    <div className="chat-status-container">
                      <div
                        className={`chat-status-content ${currentStatus.isError ? 'error' : ''}`}
                      >
                        {currentStatus.isError ? null : (
                          <div className="chat-status-spinner" />
                        )}
                        <span
                          className={`chat-status-text ${currentStatus.isError ? 'error' : ''}`}
                        >
                          {requirePaymentApproval
                            ? "Waiting for your confirmation"
                            : paymentRequestsData
                            ? "Waiting for your payment"
                            : currentStatus.message}
                        </span>
                      </div>
                    </div>
                  )}

                  {requirePaymentApproval && (
                    <>
                      <div className="payment-approval-message">
                        {requirePaymentApproval}
                      </div>
                      <div className="payment-approval-container">
                        <button
                          className="btn-solid"
                          onClick={handleApprovePayment}
                          disabled={approveLoading || cancelLoading}
                        >
                          {approveLoading && <div className="btn-spinner" />}
                          Approve
                        </button>
                        <button
                          className="btn-outline"
                          onClick={handleCancelPayment}
                          disabled={cancelLoading || approveLoading}
                        >
                          {cancelLoading && (
                            <div className="btn-spinner outline" />
                          )}
                          Cancel
                        </button>
                      </div>
                    </>
                  )}

                  {paymentRequestsData && (
                    <>
                      <div className="payment-approval-message">
                        {requirePaymentApproval}
                      </div>
                      <div className="payment-approval-container">
                        <button
                          className="btn-solid"
                          onClick={openPaymentModal}
                          disabled={cancelLoading}
                        >
                          Pay now
                        </button>
                        <button
                          className="btn-outline"
                          onClick={handleCancelPaymentRequest}
                          disabled={cancelLoading}
                        >
                          {cancelLoading && (
                            <div className="btn-spinner outline" />
                          )}
                          Cancel
                        </button>
                      </div>
                    </>
                  )}

                  <div ref={messagesEndRef} />
                </div>

                <form
                  onSubmit={handleSend}
                  className="chat-form"
                  autoComplete="off"
                >
                  <div className="chat-input-container">
                    <input
                      type="text"
                      value={message}
                      onChange={(e) => setMessage(e.target.value)}
                      placeholder={
                        isConnected ? "Type your message..." : !agent?.active ? "Agent is paused..." : "Connecting..."
                      }
                      disabled={sending || !isConnected}
                      className="chat-input"
                    />
                    <button
                      type="submit"
                      disabled={sending || !isConnected || !message.trim()}
                      className="chat-send-button"
                    >
                      <i className="fas fa-paper-plane"></i>
                    </button>
                  </div>
                  {/* <button
                    type="button"
                    className="action-btn"
                    style={{
                      background: "#f6f8fa",
                      color: "#222",
                      minWidth: 120,
                    }}
                    onClick={clearChat}
                    disabled={sending || messages.length === 0}
                  >
                    <i className="fas fa-trash"></i> Clear Chat
                  </button> */}
                </form>
              </div>
            </div>
            {paymentRequestsData && paymentModalOpen && (
              <PaymentModal
                isOpen={paymentModalOpen}
                onClose={() => setPaymentModalOpen(false)}
                paymentRequirements={paymentRequestsData.paymentRequests}
                onPaymentSuccess={handlePaymentSuccess}
              />
            )}
          </div>
        </SelectedWalletAccountContextProvider>
      </EvmWalletProvider>
  );
}

export default Chat;
