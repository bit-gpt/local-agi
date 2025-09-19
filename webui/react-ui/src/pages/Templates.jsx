import React, { useState, useEffect, useRef, useMemo, useCallback } from "react";
import { useNavigate, useOutletContext } from "react-router-dom";
import { templatesApi } from "../utils/api";
import useIsMobile from "../hooks/useMobileDetect";
import { toDisplayFormat } from "../utils/helpers";
import Header from "../components/Header";

const scratchTemplate = {
  id: "scratch",
  name: "Start from Scratch",
  description: "Create a custom agent with your own configuration",
  category: "scratch",
  icon: "scratch",
};

const Templates = () => {
  const navigate = useNavigate();
  const { showToast } = useOutletContext();
  const [templates, setTemplates] = useState([]);
  const [categories, setCategories] = useState([]);
  const [activeCategory, setActiveCategory] = useState("all");
  const [loading, setLoading] = useState(true);
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const dropdownRef = useRef(null);
  const isMobile = useIsMobile();

  useEffect(() => {
    document.title = "Templates - LocalAGI";
    return () => {
      document.title = "LocalAGI";
    };
  }, []);

  useEffect(() => {
    const handleClickOutside = (event) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target)) {
        setIsDropdownOpen(false);
      }
    };

    if (isDropdownOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      return () => {
        document.removeEventListener("mousedown", handleClickOutside);
      };
    }
  }, [isDropdownOpen]);

  useEffect(() => {
    const fetchTemplates = async () => {
      setLoading(true);
      try {
        const response = await templatesApi.getTemplates();
        if (response.success) {
          setTemplates(response.templates);

          const uniqueCategories = [
            ...new Set(response.templates.map((t) => t.category)),
          ];
          setCategories(uniqueCategories);
        } else {
          showToast && showToast("Failed to load templates", "error");
        }
      } catch (error) {
        console.error("Error fetching templates:", error);
        showToast && showToast("Failed to load templates", "error");
      } finally {
        setLoading(false);
      }
    };

    fetchTemplates();
  }, []); 

  const navigationOptions = useMemo(() => {
    const options = [
      {
        id: "all",
        icon: "fas fa-th-large",
        label: "All",
      },
    ];

    categories.forEach((category) => {
      options.push({
        id: category,
        label: toDisplayFormat(category),
      });
    });

    return options;
  }, [categories]);

  const handleCategoryChange = useCallback((categoryId) => {
    setActiveCategory(categoryId);
    setIsDropdownOpen(false);
  }, []);

  const handleTemplateSelect = useCallback(async (template) => {
    if(template.id === "scratch") {
      navigate("/create");
      return;
    }
    navigate(`/create?template=${template.id}`);
  }, [navigate]);

  const filteredTemplates = useMemo(() => {
    if (activeCategory === "all") {
      return [scratchTemplate, ...templates];
    } else {
      const categoryTemplates = templates.filter(
        (template) => template.category === activeCategory
      );
      return [scratchTemplate, ...categoryTemplates];
    }
  }, [activeCategory, templates]);

  const currentCategory = useMemo(() => 
    navigationOptions.find((option) => option.id === activeCategory),
    [navigationOptions, activeCategory]
  );

  const handleToggleDropdown = useCallback(() => {
    setIsDropdownOpen(prev => !prev);
  }, []);

  const handleCategoryClick = useCallback((event) => {
    const categoryId = event.currentTarget.dataset.categoryId;
    handleCategoryChange(categoryId);
  }, [handleCategoryChange]);

  const handleTemplateClick = useCallback((event) => {
    const templateId = event.currentTarget.dataset.templateId;
    const template = filteredTemplates.find(t => t.id === templateId);
    if (template) {
      handleTemplateSelect(template);
    }
  }, [handleTemplateSelect, filteredTemplates]);

  if (loading) {
    return (
      <div className="loading-container">
        <div className="spinner"></div>
      </div>
    );
  }

  return (
    <div className="dashboard-container">
      <div className="main-content-area">
        <div className="header-container">
          <Header
            title="Templates"
            description="Select a template to create a new agent."
          />
        </div>
        {isMobile && (
          <div className="wizard-mobile-dropdown" ref={dropdownRef}>
            <div
              className="wizard-dropdown-trigger"
              onClick={handleToggleDropdown}
            >
              <div className="wizard-dropdown-trigger-content">
                <span>{currentCategory?.label || "All Templates"}</span>
              </div>
              <i
                className={`fas fa-chevron-${
                  isDropdownOpen ? "up" : "down"
                } dropdown-arrow`}
              ></i>
            </div>
            {isDropdownOpen && (
              <div className="wizard-dropdown-menu">
                {navigationOptions.map((option) => (
                  <div
                    key={option.id}
                    className={`wizard-dropdown-item ${
                      activeCategory === option.id ? "active" : ""
                    }`}
                    data-category-id={option.id}
                    onClick={handleCategoryClick}
                  >
                    <span>{option.label}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
        <div className="section-box">
          <div className="agent-form-container">
            {/* Mobile Dropdown Navigation */}
            {isMobile ? null : (
              /* Desktop Sidebar */
              <div className="templates-sidebar">
                <div className="sidebar-title">Categories</div>
                <div className="templates-sidebar-content">
                  <ul className="wizard-nav">
                    {navigationOptions.map((option) => (
                      <li
                        key={option.id}
                        className={`wizard-nav-item ${
                          activeCategory === option.id ? "active" : ""
                        }`}
                        data-category-id={option.id}
                        onClick={handleCategoryClick}
                      >
                        <div className="nav-circle">
                          <div className="circle-fill"></div>
                        </div>
                        <span className="nav-label">{option.label}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            )}

            {/* Templates Grid */}
            <div className="templates-content">
              <div className="sidebar-title">
                {filteredTemplates.length} templates
              </div>
              {filteredTemplates.length && (
                <div className="templates-grid">
                  {filteredTemplates.map((template) => (
                    <div
                      key={template.id}
                      className="template-card"
                      data-template-id={template.id}
                      onClick={handleTemplateClick}
                    >
                      <div className="template-icon">
                        <img src={`/app/templates/${template.icon}.svg`} alt={template.name} />
                      </div>
                      <div className="template-name">{template.name}</div>
                      <p className="template-description">
                        {template.description}
                      </p>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Templates;
