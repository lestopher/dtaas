package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

type RoomMessage struct {
	Event string `json:"event"`
	Item  Item   `json:"item"`
}

type Item struct {
	Message       Message `json:"message"`
	OauthClientId string  `json:"oauth_client_id"`
	WebhookId     string  `json:"webhook_id"`
}

type Message struct {
	Date string `json:"date"`
	File string `json:"file"`
	From struct {
		Id    int32 `json:"id"`
		Links struct {
			Self string `json:"self"`
		} `json:"links"`
		MentionName string `json:"mention_name"`
		Name        string `json:"name"`
	} `json:"from"`
	Id       int32 `json:"id"`
	Mentions []struct {
		Mention Mention
	} `json:"mentions"`
	Message string `json:"message"`
}

type Mention struct {
	Id    int32 `json:"id"`
	Links struct {
		Self string `json:"self"`
	}
	MentionName string `json:"mention_name"`
	Name        string `json:"name"`
}

type Room struct {
	Id    int32 `json:"id"`
	Links struct {
		Members  string `json:"members"`
		Self     string `json:"self"`
		Webhooks string `json:"webhooks"`
	} `json:"links"`
	Name string `json:"name"`
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/deltaco", DelTacoHandler).Methods("POST")
	http.Handle("/", r)
	log.Println("Listening on http://localhost:8888")
	http.ListenAndServe(":8888", nil)
}

func DelTacoHandler(rw http.ResponseWriter, r *http.Request) {
	log.Println("Yay, I got a request.")

	decoder := json.NewDecoder(r.Body)
	var res RoomMessage

	err := decoder.Decode(&res)

	if err != nil {
		log.Fatalf("OMG, messed up decoding\n%s", err)
	}

	log.Printf("%v\n", res)
	rw.WriteHeader(http.StatusNoContent)
}
