package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/lestopher/hipchat-webhooks/room_message"
	"log"
	"net/http"
	"os"
)

// Global so that we can invoke from handler
var oauthToken string

func main() {
	oauthToken = os.Getenv("HC_OAUTH_TOKEN")

	if len(oauthToken) < 1 {
		log.Fatalln("Envionrment variable HC_OAUTH_TOKEN wasn't set")
	}

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

	log.Printf("%v\n", res)
	rw.WriteHeader(http.StatusNoContent)
}
