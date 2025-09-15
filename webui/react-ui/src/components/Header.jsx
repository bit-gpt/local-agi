/**
 * Header component for page titles and descriptions
 *
 * @param {string} icon - FontAwesome icon class (e.g., "fa-comments")
 * @param {string} title - The main title text
 * @param {string} description - Descriptive text below the title
 * @param {string} name - Optional name to be highlighted (e.g., agent name)
 * @returns {JSX.Element} Header component
 */
const Header = ({
  title = "Chat with",
  description = "Send messages and interact with your agent in real time.",
  name = "",
  titleExtra = null,
}) => {
  const isName = title === "Agent Settings" || title === "Agent Status" || title === "Chat with";
  return (
    <div className="header-content">
      {!isName ? (
        <div className={`header-title`}>
          {title}
          {titleExtra && (
            <span className="header-title-extra">{titleExtra}</span>
          )}
        </div>
      ) : (
        <div className={`header-title-name`}>
          <div className="text-sm text-gray-500 mb-3">{title}</div>
          <div className="text-2xl sm:text-3xl font-semibold">{name}</div>
        </div>
      )}
      <div className="text-gray-500">{description}</div>
    </div>
  );
};

export default Header;
