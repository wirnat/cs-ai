package cs_ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func Request(url string, method string, reqBody map[string]interface{}, setHeader func(*http.Request)) (result map[string]interface{}, err error) {
	// Marshal request body menjadi JSON dengan format indented (pretty-print)
	jsonData, err := json.MarshalIndent(reqBody, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	// Buat request
	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	setHeader(req)

	// Print LOG REQUEST
	fmt.Println("\n========== REQUEST ===============")
	fmt.Printf("URL: %s\nMethod: %s\n", url, method)
	fmt.Println("Headers:")
	for key, values := range req.Header {
		fmt.Printf("  %s: %s\n", key, values)
	}
	fmt.Println("Body:")
	fmt.Println(string(jsonData)) // Pretty-print body
	fmt.Println("===================================\n")

	// Kirim request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Baca response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Print LOG RESPONSE
	fmt.Println("\n========== RESPONSE ===============")
	fmt.Printf("Status Code: %d\n", resp.StatusCode)
	fmt.Println("Headers:")
	for key, values := range resp.Header {
		fmt.Printf("  %s: %s\n", key, values)
	}
	fmt.Println("Body:")

	// Coba decode response JSON dengan indented format
	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, bodyBytes, "", "  ")
	if err != nil {
		fmt.Printf("Raw Response: %s\n", string(bodyBytes)) // Jika gagal parse, tampilkan response mentah
		return nil, fmt.Errorf("failed to parse JSON: %v, response: %s", err, string(bodyBytes))
	}

	fmt.Println(prettyJSON.String()) // Pretty-print JSON response
	fmt.Println("===================================\n")

	// Decode response JSON ke result
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v, response: %s", err, string(bodyBytes))
	}

	return result, nil
}
