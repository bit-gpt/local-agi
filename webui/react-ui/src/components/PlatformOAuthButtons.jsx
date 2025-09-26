import React, { useState, useEffect, useMemo, useCallback } from "react";
import {
  extractRequiredPlatforms,
  PLATFORM_INFO,
} from "../utils/platformActions";
import { oauthApi } from "../utils/api";

const PlatformOAuthButtons = ({ actions, onOAuthChange }) => {
  const [platformStatuses, setPlatformStatuses] = useState({});
  const [loading, setLoading] = useState({});

  const requiredPlatforms = useMemo(() => {
    return extractRequiredPlatforms(actions || []);
  }, [actions]);

  // Fetch OAuth status for all required platforms
  useEffect(() => {
    const fetchStatuses = async () => {
      const statuses = {};
      for (const platform of requiredPlatforms) {
        setPlatformStatuses((prev) => ({
          ...prev,
          [platform]: { loading: true },
        }));
        try {
          const status = await oauthApi.getPlatformStatus(platform);
          statuses[platform] = {
            ...status,
            loading: false,
          };
        } catch (error) {
          console.error(`Failed to get ${platform} status:`, error);
          statuses[platform] = {
            connected: false,
            error: error.message,
            loading: false,
          };
        }
      }
      setPlatformStatuses(statuses);
    };

    if (requiredPlatforms.length > 0) {
      fetchStatuses();
    }
  }, [requiredPlatforms]);

  // Listen for OAuth completion messages from popup
  const handleMessage = useCallback(
    (event) => {
      // Verify origin for security
      if (event.origin !== window.location.origin) {
        return;
      }

      if (event.data.type === "OAUTH_COMPLETE") {
        const { platform, success, status, message } = event.data;

        if (success) {
          setPlatformStatuses((prev) => ({ ...prev, [platform]: status }));
          onOAuthChange?.(platform, status);
        } else {
          // Handle error case
          console.error(`OAuth failed for ${platform}:`, message);
          alert(`Failed to connect ${platform}: ${message}`);
        }

        setLoading((prev) => ({ ...prev, [platform]: false }));
      }
    },
    [onOAuthChange]
  );

  useEffect(() => {
    window.addEventListener("message", handleMessage);
    return () => window.removeEventListener("message", handleMessage);
  }, [handleMessage]);

  const handleConnect = useCallback(async (platform) => {
    setLoading((prev) => ({ ...prev, [platform]: true }));

    try {
      const response = await oauthApi.initiatePlatformAuth(platform);
      if (response.success && response.auth_url) {
        // Open in new tab - postMessage will handle the response
        window.open(response.auth_url, "_blank");
      }
    } catch (error) {
      console.error(`Failed to connect ${platform}:`, error);
      alert(`Failed to connect ${platform}: ${error.message}`);
      setLoading((prev) => ({ ...prev, [platform]: false }));
    }
  }, []);

  const handleDisconnect = useCallback(
    async (platform) => {
      if (
        !confirm(
          `Are you sure you want to disconnect ${PLATFORM_INFO[platform]?.name}?`
        )
      ) {
        return;
      }

      setLoading((prev) => ({ ...prev, [platform]: true }));

      try {
        await oauthApi.disconnectPlatform(platform);
        setPlatformStatuses((prev) => ({
          ...prev,
          [platform]: { connected: false },
        }));
        onOAuthChange?.(platform, { connected: false });
      } catch (error) {
        console.error(`Failed to disconnect ${platform}:`, error);
        alert(`Failed to disconnect ${platform}: ${error.message}`);
      } finally {
        setLoading((prev) => ({ ...prev, [platform]: false }));
      }
    },
    [onOAuthChange]
  );

  if (requiredPlatforms.length === 0) {
    return null;
  }

  return (
    <div className="platform-oauth-section">
      <div className="platform-buttons">
        {requiredPlatforms.map((platform) => {
          const info = PLATFORM_INFO[platform];
          const status = platformStatuses[platform];
          const isLoading = loading[platform];
          const isConnected = status?.connected;

          return (
            <div key={platform} className="platform-card">
              <div className="platform-header">
                <img
                  className="platform-icon"
                  src={info?.icon}
                  alt={info?.name}
                />
                <div className="platform-info">
                  <h5 className="platform-name">{info?.name}</h5>
                </div>
              </div>

              <div className="platform-status">
                {status?.loading ? (
                    <div className="spinner-primary-sm"></div>
                ) : isConnected ? (
                  <div className="connected-status">
                    <span className="status-text">
                      Connected {status.email && `(${status.email})`}
                    </span>
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDisconnect(platform);
                      }}
                      disabled={isLoading}
                      className="disconnect-btn"
                    >
                      {isLoading ? "Disconnecting..." : "Disconnect"}
                    </button>
                  </div>
                ) : (
                  <div className="disconnected-status">
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleConnect(platform);
                      }}
                      disabled={isLoading}
                      className="connect-btn"
                    >
                      {isLoading ? "Connecting..." : `Connect`}
                    </button>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default PlatformOAuthButtons;
