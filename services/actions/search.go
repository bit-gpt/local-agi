package actions

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
	"github.com/tmc/langchaingo/tools/duckduckgo"
	"mvdan.cc/xurls/v2"
)

const (
	MetadataUrls = "urls"
)

func NewSearch(config map[string]string) *SearchAction {
	results := config["results"]
	intResult := 1

	// decode int from string
	if results != "" {
		_, err := fmt.Sscanf(results, "%d", &intResult)
		if err != nil {
			fmt.Printf("error: %v", err)
		}
	}

	slog.Info("Search action with results: ", "results", intResult)
	return &SearchAction{results: intResult}
}

type SearchAction struct{ results int }

func (a *SearchAction) Run(ctx context.Context, sharedState *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	result := struct {
		Query string `json:"query"`
	}{}
	err := params.Unmarshal(&result)
	if err != nil {
		fmt.Printf("error: %v", err)

		return types.ActionResult{}, err
	}
	ddg, err := duckduckgo.New(a.results, "LocalAGI")
	if err != nil {
		fmt.Printf("error: %v", err)

		return types.ActionResult{}, err
	}
	res, err := ddg.Call(ctx, result.Query)
	if err != nil {
		fmt.Printf("error: %v", err)

		return types.ActionResult{}, err
	}

	rxStrict := xurls.Strict()
	urls := rxStrict.FindAllString(res, -1)

	results := []string{}
	for _, u := range urls {
		// remove //duckduckgo.com/l/?uddg= from the url
		u = strings.ReplaceAll(u, "//duckduckgo.com/l/?uddg=", "")
		// remove everything with &rut=.... at the end
		u = strings.Split(u, "&rut=")[0]
		results = append(results, u)
	}

	// Enhance the result to include extracted URLs for easy browsing
	enhancedResult := res
	if len(results) > 0 {
		enhancedResult += "\n\nEXTRACTED URLS FOR BROWSING:\n"
		for i, url := range results {
			if i >= 5 { // Limit to first 5 URLs to avoid overwhelming
				break
			}
			enhancedResult += fmt.Sprintf("- %s\n", url)
		}
		enhancedResult += "\nCONSIDER: Use the 'browse' or 'scrape' actions to visit these URLs and get detailed information if the user requested specific content."
	}

	return types.ActionResult{Result: enhancedResult, Metadata: map[string]interface{}{MetadataUrls: results}}, nil
}

func (a *SearchAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "search_internet",
		Description: "Search the internet for something.",
		Properties: map[string]jsonschema.Definition{
			"query": {
				Type:        jsonschema.String,
				Description: "The query to search for.",
			},
		},
		Required: []string{"query"},
	}
}

func (a *SearchAction) Plannable() bool {
	return true
}

// SearchConfigMeta returns the metadata for Search action configuration fields
func SearchConfigMeta() []config.Field {
	return []config.Field{
		{
			Name:         "results",
			Label:        "Number of Results",
			Type:         config.FieldTypeNumber,
			DefaultValue: 1,
			Min:          1,
			Max:          100,
			Step:         1,
			HelpText:     "Number of search results to return",
		},
	}
}
