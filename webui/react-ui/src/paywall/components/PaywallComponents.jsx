export const Dropdown = ({
  type,
  items,
  selected,
  onSelect,
  isOpen,
  toggleDropdown,
}) => {
  if (!selected.id) return null;
  return (
    <div className="mb-[1.5rem]">
      <label className={"block text-sm font-medium mb-2 text-gray-700"}>
        {type === "network" ? "Network" : "Coin"}
      </label>
      <div className="relative">
        <button
          className={
            "w-full flex items-center justify-between border rounded-lg px-4 py-2.5 text-left focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white border-gray-300 text-gray-800"
          }
          onClick={toggleDropdown}
          type="button"
        >
          <div className="flex items-center">
            <div className="w-6 h-6 max-w-6 max-h-6 mr-2 flex-shrink-0">
              <img
                src={selected.icon}
                alt={selected.name}
                className="object-contain"
                width={24}
                height={24}
              />
            </div>
            <span>{selected.name}</span>
          </div>
          <svg
            className="w-5 h-5 text-gray-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            xmlns="http://www.w3.org/2000/svg"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2"
              d="M19 9l-7 7-7-7"
            ></path>
          </svg>
        </button>

        {isOpen && (
          <div
            className={
              "absolute z-10 w-full mt-1 border rounded-lg shadow-lg bg-white border-gray-300"
            }
          >
            {items.map((item) => (
              <button
                key={item.id}
                className={
                  "w-full flex items-center px-4 py-2.5 text-left hover:bg-gray-100 text-gray-800"
                }
                onClick={() => onSelect(item)}
              >
                <div className="w-6 h-6 max-w-6 max-h-6 mr-2 flex-shrink-0">
                  <img
                    src={item.icon}
                    alt={item.name}
                    className="object-contain"
                    width={24}
                    height={24}
                  />
                </div>
                <span>{item.name}</span>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export const ErrorMessage = ({ message }) => (
  <div
    className={
      "text-yellow-500 text-center p-4 border rounded-lg bg-yellow-50 border-yellow-200"
    }
  >
    {message}
  </div>
);

export const NoPaymentOptions = () => (
  <div
    className={
      "text-center p-4 border rounded-lg bg-gray-50 border-gray-200 text-gray-700"
    }
  >
    <p>No payment options available for this resource.</p>
  </div>
);
