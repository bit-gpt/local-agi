const useIsMobile = () => {
  const userAgent = typeof navigator === 'undefined' ? 'SSR' : navigator.userAgent;
  const isMobile = Boolean(userAgent.match(/Android/i)) || Boolean(userAgent.match(/iPhone|iPad|iPod/i)) || Boolean(userAgent.match(/Opera Mini/i)) || Boolean(userAgent.match(/IEMobile/i));
  return isMobile;
};

export default useIsMobile;
