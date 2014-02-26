package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"github.com/gorilla/mux"
	"github.com/lestopher/hipchat-webhooks/room_message"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"strconv"
)

// Global so that we can invoke from handler
var oauthToken string
var delTacoCounter int
var room_notification string

var (
	local = flag.String("local", "", "serve as webserver, example: 0.0.0.0:8000")
	tcp   = flag.String("tcp", "", "serve as FCGI via TCP, example: 0.0.0.0:8000")
	unix  = flag.String("unix", "", "serve as FCGI via UNIX socket, example /tmp/myprogram.sock")
	token = flag.String("token", "", "oauth token")
)

func main() {
	var err error

	flag.Parse()
	oauthToken = os.Getenv("HC_OAUTH_TOKEN")

	if len(oauthToken) < 1 {
		if *token != "" {
			oauthToken = *token
		} else {
			log.Fatalln("Environment variable HC_OAUTH_TOKEN wasn't set")
		}
	}

	room_notification = "https://api.hipchat.com/v2/room/447199/notification?auth_token=" + oauthToken
	r := mux.NewRouter()
	r.HandleFunc("/deltaco", DelTacoHandler).Methods("POST")

	// The following is ripped from http://www.dav-muz.net/blog/2013/09/how-to-use-go-and-fastcgi/
	if *local != "" {
		err = http.ListenAndServe(*local, r)
		// Not sure if we need the following for http anymore
		// http.Handle("/", r)
	} else if *tcp != "" {
		listener, err := net.Listen("tcp", *tcp)
		if err != nil {
			log.Fatal(err)
		}
		defer listener.Close()

		err = fcgi.Serve(listener, r)
	} else if *unix != "" { // Run as FCGI via UNIX socket
		listener, err := net.Listen("unix", *unix)
		if err != nil {
			log.Fatal(err)
		}
		defer listener.Close()

		err = fcgi.Serve(listener, r)
	} else { // Run as FCGI via standard I/O
		err = fcgi.Serve(nil, r)
	}
	if err != nil {
		log.Fatal(err)
	}
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
