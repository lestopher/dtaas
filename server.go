package main

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/lestopher/hipchat-webhooks/room_message"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
)

// Global so that we can invoke from handler
var oauthToken string
var delTacoCounter int
var room_notification string

func main() {
	oauthToken = os.Getenv("HC_OAUTH_TOKEN")

	if len(oauthToken) < 1 {
		log.Fatalln("Envionrment variable HC_OAUTH_TOKEN wasn't set")
	}

	room_notification = "https://api.hipchat.com/v2/room/447199/notification?auth_token=" + oauthToken
	r := mux.NewRouter()
	r.HandleFunc("/deltaco", DelTacoHandler).Methods("POST")
	http.Handle("/", r)
	log.Println("Listening on http://localhost:8888")
	http.ListenAndServe(":8888", nil)
}

func DelTacoHandler(rw http.ResponseWriter, r *http.Request) {
	log.Println("Yay, I got a request.")

	decoder := json.NewDecoder(r.Body)
	var res room_message.RoomMessage

	err := decoder.Decode(&res)

	if err != nil {
		log.Fatalf("OMG, messed up decoding\n%s", err)
	}

	delTacoCounter++
	go NotifyRoom()

	log.Printf("%v\n", res)
	rw.WriteHeader(http.StatusNoContent)
}

func NotifyRoom() {
	type RoomNotification struct {
		Color         string `json:"color"`
		Message       string `json:"message"`
		MessageFormat string `json:"message_format"`
	}

	n := RoomNotification{
		Color:         "yellow",
		Message:       "(deltaco) Del Taco (deltaco) has been mentioned " + strconv.Itoa(delTacoCounter) + " times.",
		MessageFormat: "text",
	}

	b, errJson := json.Marshal(n)

	if errJson != nil {
		log.Println("error marshalling data")
		return
	}

	log.Println(string(b))

	res, err := http.Post(room_notification, "application/json", bytes.NewBuffer(b))
	defer res.Body.Close()

	if err != nil {
		log.Println("issue posting", err)
	}

	body, errReadBody := ioutil.ReadAll(res.Body)

	if errReadBody != nil {
		log.Println("error reading response body", errReadBody)
	}
	log.Println("response: ", body)
}
