import { Link } from 'react-router-dom';

function MobilePlaceholder() {
  return (
    <div className="mobile-placeholder-overlay">
      <div className="logo-container">
        <div className="logo-image-container">
          <img src="/app/logo.svg" alt="Logo" className="logo-image" />
        </div>
      </div>
      
      <div className="mobile-placeholder-modal">
        <div className="mobile-placeholder-content">
          <div className="mobile-placeholder-icon">
            <i className="fas fa-mobile-screen-button"></i>
          </div>
          
          <h2 className="mobile-placeholder-title">Mobile Coming Soon</h2>
          <p className="mobile-placeholder-description">
            We're still cooking up the mobile version. 
            For now, hop on a computer to use everything. 
          </p>
        </div>
      </div>
    </div>
  );
}

export default MobilePlaceholder;
