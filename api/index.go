package api

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// Safely handle internal server errors
func internalServerError(w http.ResponseWriter, err error) {
	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func Handler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("WithHandler panic: %v", err)
			http.Error(w, fmt.Sprintf("internal server error: %v", err), http.StatusInternalServerError)
		}
	}()

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// Handle preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Redirect root path
	if r.URL.Path == "/" {
		http.Redirect(w, r, "https://github.com/TBXark/vercel-proxy", http.StatusMovedPermanently)
		return
	}

	// Build the target URL from the path
	re := regexp.MustCompile(`^/*(https?:)/*`)
	targetURL := re.ReplaceAllString(r.URL.Path, "$1//")
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}
	if !strings.HasPrefix(targetURL, "http") {
		http.Error(w, "invalid url: "+targetURL, http.StatusBadRequest)
		return
	}

	// Create new outbound request
	outReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		internalServerError(w, err)
		return
	}

	// Only allow and forward safe headers
	allowedHeaders := map[string]bool{
		"Authorization": true,
		"Content-Type":  true,
		"User-Agent":    true,
		"Accept":        true,
	}

	for k, v := range r.Header {
		if allowedHeaders[strings.Title(strings.ToLower(k))] {
			for _, vv := range v {
				outReq.Header.Add(k, vv)
			}
		}
	}

	// Ensure host is handled by the HTTP client
	outReq.Host = ""

	// Perform the HTTP request
	client := &http.Client{}
	resp, err := client.Do(outReq)
	if err != nil {
		internalServerError(w, err)
		return
	}
	defer resp.Body.Close()

	// Copy response headers to client
	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}

	// Set the response status code and body
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		internalServerError(w, err)
	}
}
