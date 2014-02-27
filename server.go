package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	g "github.com/lestopher/giphy"
	"github.com/lestopher/hipchat-webhooks/room_message"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/fcgi"
	"net/url"
	"os"
	"strconv"
)

const cmdPrefix = "/giphy"

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
	r.HandleFunc("/gifsearch", GifSearchHandler).Methods("POST")

	// The following is ripped from http://www.dav-muz.net/blog/2013/09/how-to-use-go-and-fastcgi/
	if *local != "" {
		err = http.ListenAndServe(*local, r)
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

func bodyToRoomMessage(r io.Reader) (*room_message.RoomMessage, error) {
	decoder := json.NewDecoder(r)
	var res room_message.RoomMessage

	err := decoder.Decode(&res)

	if err != nil {
		log.Printf("OMG, messed up decoding\n%s", err)
		return nil, err
	}

	return &res, nil
}

func DelTacoHandler(rw http.ResponseWriter, r *http.Request) {
	log.Println("DelTacoHandler, is handling.")

	res, err := bodyToRoomMessage(r.Body)

	if err != nil {
		log.Println("**ERROR** bodyToRoomMessage failed in DelTacoHandler")
		rw.WriteHeader(http.StatusInternalServerError)
	}

	delTacoCounter++

	n := room_message.RoomNotification{
		Color:         "yellow",
		Message:       "(deltaco) Del Taco (deltaco) has been mentioned " + strconv.Itoa(delTacoCounter) + " times.",
		MessageFormat: "text",
		Notify:        false,
	}

	go NotifyRoom(n)

	log.Printf("%v\n", res)
	rw.WriteHeader(http.StatusNoContent)
}

func NotifyRoom(n room_message.RoomNotification) {

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

func GifSearchHandler(rw http.ResponseWriter, r *http.Request) {
	log.Printf("Gif me.")

	rm, err := bodyToRoomMessage(r.Body)

	if err != nil {
		log.Println("**ERROR** bodyToRoomMessage failed in GifSearchHandler")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Our search query
	q := rm.Item.Message.Message[len(cmdPrefix):]
	searchGiphy := fmt.Sprintf("%s?q=%s&api_key=dc6zaTOxFJmzC", g.GIPHY_API, url.QueryEscape(q))

	res, err := http.Get(searchGiphy)
	if err != nil {
		log.Println(err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer res.Body.Close()
	giphyResp := &struct{ Data []g.GiphyGif }{}
	dec := json.NewDecoder(res.Body)
	if err := dec.Decode(giphyResp); err != nil {
		log.Println(err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	msg := "NO RESULTS. THAT'S RACIST."
	if len(giphyResp.Data) > 0 {
		msg = fmt.Sprintf("%s: %s", q, giphyResp.Data[rand.Intn(len(giphyResp.Data))].Images.Original.URL)
	}

	n := room_message.RoomNotification{
		Color:         "purple",
		Message:       msg,
		MessageFormat: "text",
		Notify:        true,
	}

	go NotifyRoom(n)
	rw.WriteHeader(http.StatusNoContent)
}
