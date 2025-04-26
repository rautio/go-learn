package main

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

type ShortenRequest struct {
	Url string `json:"url"`
}

type URLStore struct {
	urls  map[string]string
	mutex sync.RWMutex
}

func NewUrlStore() *URLStore {
	store := &URLStore{
		urls: make(map[string]string),
	}
	return store
}

func (s *URLStore) Set(key, url string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.urls[key] = url
}

func (s *URLStore) Get(key string) (string, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	url, exists := s.urls[key]
	return url, exists
}

func (s *URLStore) SyncToFile() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	file, err := os.Create("urls.txt")
  defer file.Close()
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}

	writer := bufio.NewWriter(file)
	for key, value := range s.urls {
    _, err := writer.Write([]byte(fmt.Sprintf("%v: %v\n", key, value)))
    if err != nil {
			log.Println(err)
			panic(err)
    }
	}
	writer.Flush()
	return
}

func (s *URLStore) HydrateFromFile() {
	file, err := os.Open("urls.txt")
  defer file.Close()
	if err != nil {
		log.Println(err)
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
			line := scanner.Text() // Get the line as a string
			splits := strings.Split(line, ": ")
			if len(splits) == 2 {
				s.Set(splits[0], splits[1])
			}
	}
	return 
}



func generateKey(length int) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, length)
	// Inject bytes with random numbers
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// Use the random numbers to pull random characters from chars
	for i, b := range bytes {
		bytes[i] = chars[b % byte(len(chars))]
	}
	return string(bytes), nil
}

func createShortenHandler(s *URLStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Error 1 - method not allowed
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		// Error 2 - unable to parse body
		if err != nil {
			log.Println(err)
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()
		var data ShortenRequest
		err = json.Unmarshal(body, &data)
		// Error 3 - parseable but invalid body (doesnt contain the right fields)
		if err != nil {
			log.Println(err)
			http.Error(w, "Error parsing request body", http.StatusBadRequest)
			return
		}
		// generate key
		key, err := generateKey(6)
		if err != nil {
			log.Println(err)
			http.Error(w, "Error generating shortened key", http.StatusInternalServerError)
			return
		}
		s.Set(key, data.Url)
		s.SyncToFile()
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		fmt.Fprintf(w, "%s://%s/%s", scheme, r.Host, key)
	}
}

func createRedirectHandler(s *URLStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		url, exists := s.Get(r.PathValue("key"))
		if !exists {
			http.Error(w, "No URL found for key", http.StatusNotFound)
			return
		}
		http.Redirect(w, r, url, http.StatusSeeOther)
	}
}
func main() {
	store := NewUrlStore()
	store.HydrateFromFile()
	fmt.Println("Engine running...")

	http.HandleFunc("/shorten", createShortenHandler(store))
	http.HandleFunc("/{key}", createRedirectHandler(store))
	http.ListenAndServe(":9000", nil)
}
