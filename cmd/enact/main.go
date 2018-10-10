package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/voutasaurus/wait/env"
)

var (
	version = "dev"
)

func main() {
	(&server{
		addr: env.Get("ENACT_ADDR").WithDefault(":8080"),
		log:  log.New(os.Stderr, "enact[v="+version+"]", log.LstdFlags|log.LUTC|log.Llongfile),
		client: &client{
			remote: env.Get("ENACT_REMOTE").WithDefault("http://localhost:8090"),
			token:  env.Get("ENACT_REMOTE_TOKEN").WithDefault(""), // TODO: make token mandatory
			http: &http.Client{
				Timeout: time.Duration(env.Get("ENACT_REMOTE_TIMEOUT").WithDefaultInt(0, nil)) * time.Second,
			},
		},
	}).Serve()
}

type server struct {
	addr   string
	log    *log.Logger
	client *client
	state  *state
}

func (s *server) Serve() {
	m := http.NewServeMux()
	m.HandleFunc("/", s.serveRoot)
	s.log.Fatalf("server exited: %v", http.ListenAndServe(s.addr, m))
}

type task struct {
	ID string `json:"id"`
}

var errEmptyID = errors.New("task has empty ID")

func (t task) validate() error {
	if t.ID == "" {
		return errEmptyID
	}
	return nil
}

func (s *server) serveRoot(w http.ResponseWriter, r *http.Request) {
	var in task
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		s.jsonError(w, r, "bad request body, invalid json object", fmt.Sprintf("serveRoot: json decode error: %v", err), 400)
	}
	if err := in.validate(); err != nil {
		s.jsonError(w, r, "bad request body, task is invalid", fmt.Sprintf("serveRoot: validation error: %v", err), 400)
	}
	go s.background(in)
	// respond early
}

// error writes an error to both the request's span and to the response writer.
func (s *server) jsonError(w http.ResponseWriter, r *http.Request, msg, detail string, code int) {
	errID := time.Now().UTC().Format(time.RFC3339Nano)
	s.log.Printf("api error id=%q,path=%q,code=%d,err=%q,detail=%q", errID, r.URL.Path, code, msg, detail)
	w.Header().Add("X-Errid", errID)
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(&struct {
		Err   string `json:"err"`
		ErrID string `json:"err_id"`
	}{
		Err:   msg,
		ErrID: errID,
	}); err != nil {
		s.log.Printf("jsonError: couldn't encode: %v", err)
	}
}

func (s *server) background(in task) {
	err := s.client.process(in)
	if final(err) || err == nil {
		s.state.update(in, err)
		return
	}
	s.log.Printf("temporary error processing task: %v", err)
}

type client struct {
	http   *http.Client
	remote string
	token  string
}

func (cc *client) process(in task) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	body := bytes.NewReader(b)
	req, err := http.NewRequest("POST", cc.remote, body)
	if err != nil {
		return err
	}
	req.Header.Add("Authentication", "Bearer "+cc.token)
	res, err := cc.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		// TODO: read body for error message
		return fmt.Errorf("status: %v", res.StatusCode)
	}
	// TODO: read body for response detail and record them?
	return nil
}

func final(err error) bool {
	// TODO: add logic to determine errors that should halt retries
	return false
}

type state struct {
	db string
}

func (s *state) update(in task, err error) error {
	// TODO: add logic to update persistent record with task status
	return nil
}
