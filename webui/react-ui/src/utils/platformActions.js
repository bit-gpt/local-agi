export const PLATFORM_INFO = {
  gmail: {
    name: 'Gmail',
    icon: '/app/logos/gmail-logo.svg',
    color: '#EA4335',
  },
  ["google-calendar"]: {
    name: 'Google Calendar',
    icon: '/app/logos/google-calendar-logo.svg',
    color: '#4285F4',
  }
};

/**
 * Extract platforms needed based on selected actions
 * @param {Array} platformActions - Array of all platform actions available
 * @param {Array} actions - Array of action objects with 'name' property
 * @returns {Array} Array of platform names that need OAuth
 */
export function extractRequiredPlatforms(actions) {

  const targetPrefixes = ['gmail', 'google-calendar'];
  const result = targetPrefixes.filter(prefix => 
    actions.some(action => action.name.startsWith(prefix + '-'))
  );

  console.log(result)
  
  return result;
}

