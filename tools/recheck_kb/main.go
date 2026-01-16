package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type KnowledgeProvider struct {
	Id           int64
	Name         string
	ProviderType string
	Config       string
}

func (KnowledgeProvider) TableName() string {
	return "knowledge_provider"
}

type CloudflareConfig struct {
	AccountID      string  `json:"account_id"`
	RAGName        string  `json:"rag_name"`
	APIToken       string  `json:"api_token"`
	ScoreThreshold float64 `json:"score_threshold"`
}

func main() {
	dbPath := "n9e.db"
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to open db: %v", err)
	}

	var providers []KnowledgeProvider
	if err := db.Where("enabled = ?", true).Find(&providers).Error; err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	for _, p := range providers {
		if p.ProviderType == "cloudflare_autorag" {
			checkCloudflare(p)
		}
	}
}

func checkCloudflare(p KnowledgeProvider) {
	var config CloudflareConfig
	if err := json.Unmarshal([]byte(p.Config), &config); err != nil {
		log.Printf("Failed to parse config: %v", err)
		return
	}

	fmt.Printf("Checking Cloudflare Provider: %s\n", p.Name)
	
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/autorag/rags/%s/search", config.AccountID, config.RAGName)
	
	reqBody := map[string]interface{}{
		"query": "测试环境 固定地址", 
		"max_num_results": 5,
		"ranking_options": map[string]interface{}{
			"score_threshold": 0.3,
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return
	}

	req.Header.Set("Authorization", "Bearer "+config.APIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %s\n", resp.Status)
	
	os.WriteFile("kb_content_check.json", body, 0644)
	fmt.Println("Content saved to kb_content_check.json")
}
