package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func flushSubscribersToDisk() {
	f, err := os.Create(subscribersFileName)

	if err != nil {
		panic(err)
	}

	defer f.Close()

	w := bufio.NewWriter(f)
	for webhook := range subscribers {
		_, err = w.WriteString(fmt.Sprintf("%s\n", webhook))
		if err != nil {
			log.Printf("Failed to flush a subscriber %s", webhook)
		} else {
			log.Printf("%s is flushed", webhook)
		}
	}
	w.Flush()
}

// Subscriber : Subscriber information
type Subscriber struct {
	WebhookURL string
}

func registerSubscriber(w http.ResponseWriter, req *http.Request) {
	var subscriber Subscriber

	log.Println("Registering new subscriber")

	err := json.NewDecoder(req.Body).Decode(&subscriber)

	if err != nil {
		log.Fatalf("Registration of a new subscriber is failed. Error details: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("%+v", subscriber)

	subscribers[subscriber.WebhookURL] = true

	log.Println("Flush subscribers to disk")

	flushSubscribersToDisk()
}

const subscribersFileName = "subscribers"

var subscribers = make(map[string]bool)

func initSubscribersMap() {
	log.Println("Initializing subscribers")

	file, err := os.OpenFile(subscribersFileName, os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("Initialization of subscribers is failed. Error details: %s", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		s := scanner.Text()
		log.Println(s)
		subscribers[s] = true
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Initialization of subscribers is failed. Error details: %s", err)
	}

	log.Println("Initialization of subscribers is finished susscessfully")
}

func handlePush(w http.ResponseWriter, req *http.Request) {
	for webhook := range subscribers {
		res, err := http.Post(webhook, "application/json", bytes.NewBufferString(""))

		if err != nil {
			log.Printf("Failed to notify a subscriber '%s'", webhook)
		} else if res.Status != "200" {
			log.Printf("Subscriber '%s' responded with non 200. Response code: %s", webhook, res.Status)
		} else {
			log.Printf("Subscriber '%s' notified successfully.", webhook)
		}
	}
}

func main() {
	initSubscribersMap()

	r := mux.NewRouter()
	r.HandleFunc("/subscribers", registerSubscriber).Methods("POST")
	r.HandleFunc("/push", handlePush).Methods("POST")
	http.Handle("/", r)
	http.ListenAndServe(":8080", nil)
}
