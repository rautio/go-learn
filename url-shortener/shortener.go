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
	"strconv"
	"strings"
	"sync"
	"time"
)

type ShortenRequest struct {
	Url string `json:"url"`
}

type URL struct {
	Url string `json:"url"`
	Key string `json:"key"`
	Created time.Time `json:"created"`
}

type URLStorer interface {
	Set(key, url string, created time.Time)
	Get(key string) (URL, bool)
	SyncToFile()
	HydrateFromFile()
}

type URLStore struct {
	urls  map[string]URL
	mutex sync.RWMutex
}

func NewUrlStore() *URLStore {
	store := &URLStore{
		urls: make(map[string]URL),
	}
	return store
}

func (store *URLStore) Set(key, url string, created time.Time) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.urls[key] = URL{ Url: url, Key: key, Created: created}
}

func (store *URLStore) Get(key string) (URL, bool) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	url, exists := store.urls[key]
	return url, exists
}

func (store *URLStore) SyncToFile() {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	file, err := os.Create("urls.txt")
  defer file.Close()
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}

	writer := bufio.NewWriter(file)
	for key, value := range store.urls {
		// URL, Created
    _, err := writer.Write([]byte(fmt.Sprintf("%v ** %v ** %v\n", key, value.Url, value.Created.Unix())))
    if err != nil {
			log.Println(err)
			panic(err)
    }
	}
	writer.Flush()
}

func (store *URLStore) HydrateFromFile() {
	file, err := os.Open("urls.txt")
  defer file.Close()
	if err != nil {
		// File likely doesn't exist, create it
		file, err = os.Create("urls.txt")
		if err != nil { 
			log.Println("Error creating file:", err)
		}
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
			line := scanner.Text() // Get the line as a string
			splits := strings.Split(line, " ** ")
			if len(splits) == 3 {
				timestamp, err := strconv.ParseInt(splits[2], 10, 64)
				created := time.Unix(timestamp, 0)
				if err != nil {
					log.Println("Error parsing time:", err)
				} else {
					store.Set(splits[0], splits[1], created)
				}
			}
	}
}



func generateKey(length int, r io.Reader) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, length)
	// Inject bytes with random numbers
	if _, err := r.Read(bytes); err != nil {
		return "", err
	}
	// Use the random numbers to pull random characters from chars
	for i, b := range bytes {
		bytes[i] = chars[b % byte(len(chars))]
	}
	return string(bytes), nil
}

func createShortenHandler(store URLStorer) func(w http.ResponseWriter, r *http.Request) {
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
		var key string
		key_length := 6
		for key == "" {
			key_option, err := generateKey(key_length, rand.Reader)
			if err != nil {
				log.Println(err)
				http.Error(w, "Error generating shortened key", http.StatusInternalServerError)
				return
			}
			_, exists := store.Get(key_option)
			if !exists {
				key = key_option
			} else {
				log.Println("Key collision. Incrementing key length.")
				key_length += 1
			}
		}
		store.Set(key, data.Url, time.Now())
		store.SyncToFile()
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		fmt.Fprintf(w, "%s://%s/%s", scheme, r.Host, key)
	}
}

func createRedirectHandler(store URLStorer) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		url, exists := store.Get(r.PathValue("key"))
		if !exists {
			http.Error(w, "No URL found for key", http.StatusNotFound)
			return
		}
		http.Redirect(w, r, url.Url, http.StatusSeeOther)
	}
}
func main() {
	var store URLStorer = NewUrlStore()
	store.HydrateFromFile()
	fmt.Println("Engine running...")

	http.HandleFunc("/shorten", createShortenHandler(store))
	http.HandleFunc("/{key}", createRedirectHandler(store))
	http.ListenAndServe(":9000", nil)
}
