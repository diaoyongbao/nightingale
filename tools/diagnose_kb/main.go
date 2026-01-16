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
	Enabled      bool
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
		fmt.Printf("Checking Provider: %s (Type: %s)\n", p.Name, p.ProviderType)

		if p.ProviderType == "cloudflare_autorag" {
			fixCloudflare(db, p)
			// Re-fetch to check
			var updatedP KnowledgeProvider
			db.First(&updatedP, p.Id)
			checkCloudflare(updatedP)
		}
	}
}

func fixCloudflare(db *gorm.DB, p KnowledgeProvider) {
	var config CloudflareConfig
	if err := json.Unmarshal([]byte(p.Config), &config); err != nil {
		log.Printf("Failed to parse config for fix: %v", err)
		return
	}

	if config.ScoreThreshold > 0.4 {
		fmt.Printf("  High threshold detected: %f. Updating to 0.3...\n", config.ScoreThreshold)
		config.ScoreThreshold = 0.3
		newConfigBytes, _ := json.Marshal(config)
		
		// Update DB
		if err := db.Model(&p).Update("config", string(newConfigBytes)).Error; err != nil {
			log.Printf("Failed to update config: %v", err)
		} else {
			fmt.Println("  Updated successfully.")
		}
	}
}

func checkCloudflare(p KnowledgeProvider) {
	var config CloudflareConfig
	if err := json.Unmarshal([]byte(p.Config), &config); err != nil {
		log.Printf("Failed to parse config: %v", err)
		return
	}

	fmt.Printf("  AccountID: %s\n", config.AccountID)
	fmt.Printf("  RAGName: %s\n", config.RAGName)
	fmt.Printf("  Configured ScoreThreshold: %f\n", config.ScoreThreshold)
	// Mask token
	maskedToken := ""
	if len(config.APIToken) > 4 {
		maskedToken = config.APIToken[:4] + "..."
	}
	fmt.Printf("  Token: %s\n", maskedToken)

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/autorag/rags/%s/search", config.AccountID, config.RAGName)
	
	reqBody := map[string]interface{}{
		"query": "测试环境 固定地址",
		"max_num_results": 5,
		"ranking_options": map[string]interface{}{
			"score_threshold": 0.3,
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+config.APIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("  Request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("  Response Status: %s\n", resp.Status)
	os.WriteFile("diagnose_result.json", body, 0644)
	fmt.Printf("  Response Body written to diagnose_result.json\n")
}
