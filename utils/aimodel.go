package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

type SimilarityResponse struct {
	SimilarProjects []map[string]interface{} `json:"similar_projects"`
	TotalProjects   int                      `json:"total_similar_projects"`
}

func CheckProjectSimilarity(name, description string, similarityThreshold float64) (*SimilarityResponse, error) {
	// Option 1: Use environment variable (recommended)
	pythonServiceURL := os.Getenv("PYTHON_SERVICE_URL")
	if pythonServiceURL == "" {
		// Fallback to Docker network IP
		pythonServiceURL = "http://python_app:5000/detect_similarities"
	}
	fmt.Print(pythonServiceURL)

	requestBody, err := json.Marshal(map[string]interface{}{
		"project_name":         name,
		"project_description":  description,
		"similarity_threshold": similarityThreshold,
	})
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(
		pythonServiceURL,
		"application/json",
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch similarity results from API")
	}

	var similarityResp SimilarityResponse
	err = json.NewDecoder(resp.Body).Decode(&similarityResp)
	if err != nil {
		return nil, err
	}

	return &similarityResp, nil
}
