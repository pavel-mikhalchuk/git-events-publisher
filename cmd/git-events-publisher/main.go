package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var mutext sync.Mutex

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
			log.Fatalf("Failed to flush a subscriber %s", webhook)
		} else {
			log.Infof("%s is flushed", webhook)
		}
	}
	w.Flush()
}

// Subscriber : Subscriber information
type Subscriber struct {
	WebhookURL string
}

func registerSubscriber(w http.ResponseWriter, req *http.Request) {
	mutext.Lock()
	defer mutext.Unlock()

	var subscriber Subscriber

	log.Println("Registering new subscriber")

	err := json.NewDecoder(req.Body).Decode(&subscriber)

	if err != nil {
		log.Warnf("Registration of a new subscriber is failed. Error details: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Infof("%+v", subscriber)

	subscribers[subscriber.WebhookURL] = true

	log.Infof("Flush subscribers to disk")

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

	log.Info("Initialization of subscribers is finished susscessfully")
}

func handlePush(w http.ResponseWriter, req *http.Request) {
	mutext.Lock()
	defer mutext.Unlock()

	processedWhs := []string{}

	whToDelete := []string{}

	for webhook := range subscribers {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

		res, err := http.Post(webhook, "application/json", bytes.NewBufferString(""))

		if err != nil {
			log.Warnf("Failed to notify a subscriber '%s'", webhook)
			whToDelete = append(whToDelete, webhook)
		} else if res.StatusCode != 200 {
			log.Warnf("Subscriber '%s' responded with non 200. Response code: %s", webhook, res.StatusCode)
			whToDelete = append(whToDelete, webhook)
		} else {
			log.Infof("Subscriber '%s' notified successfully.", webhook)
		}

		processedWhs = append(processedWhs, webhook)
	}

	for _, webhook := range whToDelete {
		delete(subscribers, webhook)
	}

	sort.Strings(processedWhs)

	w.Write([]byte(strings.Join(processedWhs, "\n")))
}

func main() {
	initSubscribersMap()

	r := mux.NewRouter()
	r.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {}).Methods("GET")
	r.HandleFunc("/subscribers", registerSubscriber).Methods("POST")
	r.HandleFunc("/push", handlePush).Methods("POST")
	http.Handle("/", r)
	http.ListenAndServe(":8080", nil)
}
