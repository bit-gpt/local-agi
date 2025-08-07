import { useState, useRef, useEffect } from "react";
import { useOutletContext } from "react-router-dom";

const PrivateKeyReveal = ({ privateKey }) => {
  const { showToast } = useOutletContext();
  const [isHolding, setIsHolding] = useState(false);
  const [progress, setProgress] = useState(0);
  const [isRevealed, setIsRevealed] = useState(false);
  const [copySuccess, setCopySuccess] = useState(false);
  const holdTimerRef = useRef(null);
  const progressTimerRef = useRef(null);
  const startTimeRef = useRef(null);

  const HOLD_DURATION = 5000; // 5 seconds

  const startHolding = () => {
    if (isRevealed) return;
    
    setIsHolding(true);
    startTimeRef.current = Date.now();
    
    // Start progress animation
    progressTimerRef.current = setInterval(() => {
      const elapsed = Date.now() - startTimeRef.current;
      const newProgress = Math.min((elapsed / HOLD_DURATION) * 100, 100);
      setProgress(newProgress);
    }, 16); // ~60fps

    // Set timer to reveal private key
    holdTimerRef.current = setTimeout(() => {
      setIsRevealed(true);
      setIsHolding(false);
      setProgress(100);
      showToast("Private key revealed", "success");
    }, HOLD_DURATION);
  };

  const stopHolding = () => {
    if (isRevealed) return;
    
    setIsHolding(false);
    setProgress(0);
    
    if (holdTimerRef.current) {
      clearTimeout(holdTimerRef.current);
      holdTimerRef.current = null;
    }
    
    if (progressTimerRef.current) {
      clearInterval(progressTimerRef.current);
      progressTimerRef.current = null;
    }
  };

  const handleCopyPrivateKey = async () => {
    try {
      await navigator.clipboard.writeText(privateKey);
      setCopySuccess(true);
      showToast("Private key copied", "success");
      setTimeout(() => setCopySuccess(false), 2000);
    } catch (err) {
      showToast("Failed to copy private key", "error");
      console.error("Failed to copy private key:", err);
    }
  };

  const truncatePrivateKey = (key) => {
    if (!key) return "";
    if (key.length <= 20) return key;
    return `${key.slice(0, 10)}...${key.slice(-6)}`;
  };

  // Cleanup timers on unmount
  useEffect(() => {
    return () => {
      if (holdTimerRef.current) {
        clearTimeout(holdTimerRef.current);
      }
      if (progressTimerRef.current) {
        clearInterval(progressTimerRef.current);
      }
    };
  }, []);

  if (isRevealed) {
    return (
      <div className="private-key-section">
        <div className="private-key-header">
          <i className="fas fa-key"></i>
          <span>Private Key</span>
        </div>
        <div className="private-key-container">
          <code className="private-key-value">{truncatePrivateKey(privateKey)}</code>
          <button
            type="button"
            className={`copy-btn ${copySuccess ? "success" : ""}`}
            onClick={handleCopyPrivateKey}
            title={copySuccess ? "Copied private key!" : "Copy private key"}
          >
            <i className={`fa-regular ${copySuccess ? "fa-check" : "fa-copy"}`}></i>
          </button>
        </div>
        <div className="private-key-warning">
          <i className="fas fa-exclamation-triangle"></i>
          <span>Keep this private key secure. Never share it with anyone.</span>
        </div>
      </div>
    );
  }

  return (
    <div className="private-key-reveal-section">
      <button
        className={`hold-to-reveal-btn ${isHolding ? "holding" : ""}`}
        onMouseDown={startHolding}
        onMouseUp={stopHolding}
        onMouseLeave={stopHolding}
        onTouchStart={startHolding}
        onTouchEnd={stopHolding}
        disabled={isRevealed}
        onClick={(e) => {
          e.preventDefault();
          e.stopPropagation();
        }}
      >
        <div className="hold-progress-bg">
          <div 
            className="hold-progress-fill" 
            style={{ width: `${progress}%` }}
          ></div>
        </div>
        <div className="hold-btn-content">
          <i className="fas fa-key"></i>
          <span>{isHolding ? `Hold ${Math.ceil((HOLD_DURATION - (Date.now() - (startTimeRef.current || Date.now()))) / 1000)}s` : "Hold to reveal private key"}</span>
        </div>
      </button>
    </div>
  );
};

export default PrivateKeyReveal;