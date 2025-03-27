import { useState, useCallback, useEffect, useRef } from 'react';
import { chatApi } from '../utils/api';
import { useSSE } from './useSSE';

/**
 * Custom hook for chat functionality
 * @param {string} agentName - Name of the agent to chat with
 * @returns {Object} - Chat state and functions
 */
export function useChat(agentName) {
  const [messages, setMessages] = useState([]);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState(null);
  const processedMessageIds = useRef(new Set());
  
  // Use SSE hook to receive real-time messages
  const { messages: sseMessages, statusUpdates, errorMessages, isConnected } = useSSE(agentName);
  
  // Process SSE messages into chat messages
  useEffect(() => {
    if (!sseMessages || sseMessages.length === 0) return;
    
    // Process the latest SSE message
    const latestMessage = sseMessages[sseMessages.length - 1];
    
    // Skip if we've already processed this message
    if (processedMessageIds.current.has(latestMessage.id)) {
      return;
    }
    
    // Handle JSON messages
    if (latestMessage.type === 'json_message') {
      try {
        // The message should already be a parsed JSON object
        const messageData = latestMessage.content;
        
        // Skip if we've already processed this message ID
        if (processedMessageIds.current.has(messageData.id)) {
          return;
        }
        
        // Add to processed set to avoid duplicates
        processedMessageIds.current.add(messageData.id);
        
        // Add the message to our state
        setMessages(prev => [...prev, {
          id: messageData.id,
          sender: messageData.sender,
          content: messageData.content,
          timestamp: messageData.timestamp,
        }]);
      } catch (err) {
        console.error('Error processing JSON message:', err);
      }
    }
  }, [sseMessages]);
  
  // Process status updates
  useEffect(() => {
    if (!statusUpdates || statusUpdates.length === 0) return;
    
    const latestStatus = statusUpdates[statusUpdates.length - 1];
    
    // Handle status updates
    if (latestStatus.type === 'status') {
      try {
        // The status should be a parsed JSON object
        const statusData = latestStatus.content;
        
        if (statusData.status === 'processing') {
          setSending(true);
        } else if (statusData.status === 'completed') {
          setSending(false);
        }
      } catch (err) {
        console.error('Error processing status update:', err);
      }
    }
  }, [statusUpdates]);
  
  // Process error messages
  useEffect(() => {
    if (!errorMessages || errorMessages.length === 0) return;
    
    const latestError = errorMessages[errorMessages.length - 1];
    
    try {
      // The error should be a parsed JSON object
      const errorData = latestError.content;
      
      if (errorData.error) {
        setError(errorData.error);
      }
    } catch (err) {
      console.error('Error processing error message:', err);
    }
  }, [errorMessages]);

  // Send a message to the agent
  const sendMessage = useCallback(async (content) => {
    if (!agentName || !content) return false;
    
    setSending(true);
    setError(null);
    
    // Add user message to the local state immediately for better UX
    const messageId = `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
    
    const userMessage = {
      id: messageId,
      sender: 'user',
      content,
      timestamp: new Date().toISOString(),
    };
    
    setMessages(prev => [...prev, userMessage]);
    
    try {
      // Use the JSON-based API endpoint
      await chatApi.sendMessage(agentName, content);
      return true;
    } catch (err) {
      setError(err.message || 'Failed to send message');
      console.error('Error sending message:', err);
      setSending(false);
      return false;
    }
  }, [agentName]);

  // Clear chat history
  const clearChat = useCallback(() => {
    setMessages([]);
    processedMessageIds.current.clear();
  }, []);

  return {
    messages,
    sending,
    error,
    isConnected,
    sendMessage,
    clearChat,
  };
}
