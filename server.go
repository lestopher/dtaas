package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	g "github.com/lestopher/gophy"
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

// Sets up our command prefix as /giphy, but really it could be whatever you wanted
const cmdPrefix = "/giphy"

// Global so that we can invoke from handler
var oauthToken string
var delTacoCounter int

var (
	local = flag.String("local", "", "serve as webserver, example: 0.0.0.0:8000")
	tcp   = flag.String("tcp", "", "serve as FCGI via TCP, example: 0.0.0.0:8000")
	unix  = flag.String("unix", "", "serve as FCGI via UNIX socket, example /tmp/myprogram.sock")
	token = flag.String("token", "", "oauth token")
)

type DeployMessage struct {
	Env      string `json:"env"`
	Status   string `json:"status"`
	Location string `json:"location"`
	RoomId   int32  `json:"room_id"`
}

// SlackResponse is returned to the caller so that it returns prints the message
// in the app
type SlackResponse struct {
	Text string `json:"text"`
}

func main() {
	var err error

	flag.Parse()
	// TODO: Determine if I still want to get it by looking at os variables
	oauthToken = os.Getenv("HC_OAUTH_TOKEN")

	if len(oauthToken) < 1 {
		if *token != "" {
			oauthToken = *token
		} else {
			log.Fatalln("Environment variable HC_OAUTH_TOKEN wasn't set")
		}
	}

	r := mux.NewRouter()
	r.HandleFunc("/deltaco", DelTacoHandler).Methods("POST")
	r.HandleFunc("/gifsearch", GifSearchHandler).Methods("POST")
	r.HandleFunc("/deploy", DeployHandler).Methods("POST")
	r.HandleFunc("/slack/gifsearch", SlackGifSearchHandler).Methods("POST")

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

/**
 * Decodes an io.Reader, usually from the http response into a RoomMessage struct
 * @param r [io.Reader] the Reader type to decode into RoomMessage
 * @return *RoomMessage decoded reader stream
 * @return error should be nil if no errors
 */
func bodyToRoomMessage(r io.Reader) (*room_message.RoomMessage, error) {
	var res room_message.RoomMessage
	decoder := json.NewDecoder(r)
	err := decoder.Decode(&res)

	if err != nil {
		log.Printf("OMG, messed up decoding\n%s", err)
		return nil, err
	}

	return &res, nil
}

/**
 * Takes care of messages posted to /deltaco. It keeps an in-memory counter
 * of how many times deltaco is mentioned. CAVEAT - deltaco is not persisted
 * @param rw [http.ResponseWriter] the stream to write to for responses
 * @param r  [*http.Request] the request that came in
 * @return none
 */
func DelTacoHandler(rw http.ResponseWriter, r *http.Request) {
	log.Println("DelTacoHandler, is handling.")

	res, err := bodyToRoomMessage(r.Body)

	if err != nil {
		log.Println("**ERROR** bodyToRoomMessage failed in DelTacoHandler")
		rw.WriteHeader(http.StatusInternalServerError)
	}

	// Every time this handler is called, we increment delTacoCounter, as the
	// webhook that responded to us, is supposed to do the heavy lifting with
	// the regex pattern
	delTacoCounter++

	n := room_message.RoomNotification{
		Color:         "yellow",
		Message:       "(deltaco) Del Taco (deltaco) has been mentioned " + strconv.Itoa(delTacoCounter) + " times.",
		MessageFormat: "text",
		Notify:        false,
	}

	// Notify the room asynchronously
	go NotifyRoom(n, res.Item.Room.Id)

	log.Printf("%v\n", res)
	rw.WriteHeader(http.StatusNoContent)
}

/**
 * Hipchat room gets the message
 * @param n [room_message.RoomNotification] imported from package gophy, data to send
 * @param id [int32] the id of the hipchat room
 * @return none
 */
func NotifyRoom(n room_message.RoomNotification, id int32) {
	roomURL := fmt.Sprintf("https://api.hipchat.com/v2/room/%d/notification?auth_token=%s", id, oauthToken)
	b, errJson := json.Marshal(n)

	if errJson != nil {
		log.Println("error marshalling data")
		return
	}

	// TODO: do we need to print this? is it important?
	log.Println(string(b))

	res, err := http.Post(roomURL, "application/json", bytes.NewBuffer(b))

	if err != nil {
		log.Println("issue posting", err)
	}

	defer res.Body.Close()
	body, errReadBody := ioutil.ReadAll(res.Body)

	if errReadBody != nil {
		log.Println("error reading response body", errReadBody)
	}
	log.Println("response: ", body)
}

/**
 * Handles request to query giphy
 * @param rw [http.ResponseWriter] the stream to write to for responses
 * @param r  [*http.Request] the request that came in
 * @return none
 */
func GifSearchHandler(rw http.ResponseWriter, r *http.Request) {
	log.Printf("Gif me.")

	rm, err := bodyToRoomMessage(r.Body)

	if err != nil {
		log.Println("**ERROR** bodyToRoomMessage failed in GifSearchHandler")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer r.Body.Close()

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

	go NotifyRoom(n, rm.Item.Room.Id)
	rw.WriteHeader(http.StatusNoContent)
}

/**
 * Tells you the status of a deploy
 * Expects:
 * {
 *  "env"      : "staging",
 *  "status"   : "starting",
 *  "location" : "bamboo", // Could also be... "staging"
 *  "room_id"  : 12345
 * }
 * @param rw [http.ResponseWriter] the stream to write to for responses
 * @param r  [*http.Request] the request that came in
 * @return none
 */
func DeployHandler(rw http.ResponseWriter, r *http.Request) {
	log.Printf("Deploy time buddy.")

	var dm DeployMessage
	var msg string
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&dm)

	if err != nil {
		log.Println("**ERROR** bodyToRoomMessage failed in DeployHandler")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer r.Body.Close()

	color := colorPicker(dm.Status)

	if dm.Status == "beginning" {
		msg = fmt.Sprintf("%s deploy in %s on %s", dm.Status, dm.Env, dm.Location)
	} else {
		msg = fmt.Sprintf("deploy in %s on %s: %s", dm.Env, dm.Location, dm.Status)
	}

	n := room_message.RoomNotification{
		Color:         color,
		Message:       msg,
		MessageFormat: "text",
		Notify:        true,
	}

	go NotifyRoom(n, dm.RoomId)
	rw.WriteHeader(http.StatusNoContent)
}

func colorPicker(s string) string {
	switch {
	case s == "success":
		return "green"
	case s == "fail":
		return "red"
	case s == "beginning":
		return "yellow"
	}
	return "yellow"
}

// SlackGifSearchHandler responds to a request for a specific giphy tag
func SlackGifSearchHandler(rw http.ResponseWriter, r *http.Request) {
	log.Printf("SlackGifSearchHandler")
	r.ParseForm()
	text := r.Form.Get("text")
	triggerWord := r.Form.Get("trigger_word")

	if len(text) < 1 {
		log.Println("**ERROR** text parameter does not exist")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(triggerWord) < 1 {
		log.Println("**ERROR** text parameter does not exist")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	msg, err := searchGiphy(text[len(triggerWord):])

	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")

	enc := json.NewEncoder(rw)
	payload := &SlackResponse{Text: msg}

	log.Println(payload)

	if err = enc.Encode(&payload); err != nil {
		log.Println("**ERROR** encoding response failed")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func searchGiphy(query string) (string, error) {
	// Our search query
	searchGiphy := fmt.Sprintf(
		"%s?q=%s&api_key=dc6zaTOxFJmzC", g.GIPHY_API, url.QueryEscape(query))

	res, err := http.Get(searchGiphy)

	if err != nil {
		log.Println(err)
		return ":finnadie:", err
	}

	defer res.Body.Close()

	giphyResp := &struct{ Data []g.GiphyGif }{}
	dec := json.NewDecoder(res.Body)

	if err := dec.Decode(giphyResp); err != nil {
		log.Println(err)
		return ":finnadie:", err
	}

	msg := ":rage2:"

	if len(giphyResp.Data) > 0 {
		msg = fmt.Sprintf("%s: %s",
			query, giphyResp.Data[rand.Intn(len(giphyResp.Data))].Images.Original.URL)
	}

	return msg, nil
}
