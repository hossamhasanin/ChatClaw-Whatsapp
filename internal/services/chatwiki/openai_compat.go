package chatwiki

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const chatWikiModelCatalogCacheTTL = 2 * time.Minute

type cachedOpenAIModelCatalog struct {
	catalog   *ModelCatalog
	expiresAt time.Time
}

var openAIModelCatalogCache sync.Map

func ResetOpenAIModelCatalogCacheForTest() {
	openAIModelCatalogCache = sync.Map{}
}

func ResolveSelfOwnedModelConfigID(apiKey, apiEndpoint, modelID, modelType string) (int, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return 0, fmt.Errorf("chatwiki model_id is required")
	}

	catalog, err := loadModelCatalogForOpenAI(apiKey, apiEndpoint)
	if err != nil {
		return 0, err
	}

	for _, item := range catalogItemsByType(catalog, modelType) {
		if strings.TrimSpace(item.ModelID) != modelID {
			continue
		}
		if item.SelfOwnedModelConfigID <= 0 {
			return 0, fmt.Errorf("chatwiki model %q missing self_owned_model_config_id", modelID)
		}
		return item.SelfOwnedModelConfigID, nil
	}

	return 0, fmt.Errorf("chatwiki model %q not found in model catalog", modelID)
}

func loadModelCatalogForOpenAI(apiKey, apiEndpoint string) (*ModelCatalog, error) {
	// Disable external request to chatwiki endpoints for fetching models.
	return &ModelCatalog{
		LoadedAtUnix: time.Now().Unix(),
	}, nil
}

func normalizeManagementBaseURL(apiEndpoint string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(apiEndpoint), "/")
	switch {
	case strings.HasSuffix(baseURL, "/chatclaw/v1"):
		return strings.TrimSuffix(baseURL, "/chatclaw/v1")
	case strings.HasSuffix(baseURL, "/openapi/chatclaw/v1"):
		return strings.TrimSuffix(baseURL, "/chatclaw/v1")
	default:
		return baseURL
	}
}

func catalogItemsByType(catalog *ModelCatalog, modelType string) []ModelCatalogItem {
	switch strings.ToLower(strings.TrimSpace(modelType)) {
	case "embedding":
		return catalog.EmbeddingModels
	case "rerank":
		return catalog.RerankModels
	default:
		return catalog.LLMModels
	}
}
