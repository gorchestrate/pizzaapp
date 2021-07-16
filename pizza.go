package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorchestrate/async"
	"github.com/gorilla/mux"
)

type PizzaOrderWorkflow struct {
	Status string
	Body   map[string]string
}

func (wf *PizzaOrderWorkflow) Definition() async.Section {
	return S(
		Step("step1", func() error {
			wf.Status = "started"
			return nil
		}),
		Select("wait for event 20 seconds",
			On("20 seconds passsed", gTaskMgr.Timeout(time.Second*20),
				Step("set timed out", func() error {
					wf.Status = "timed out"
					return nil
				}),
			),
			On("myEvent", &SimpleEvent{Handler: func(body map[string]string) (map[string]string, error) {
				body["processed"] = "true"
				wf.Body = body
				wf.Status = "got event"
				return body, nil
			}}),
		),
		Select("wait some time", On("10 seconds passed", gTaskMgr.Timeout(time.Second*10))),
		Step("step2", func() error {
			wf.Status = "finished"
			return nil
		}),
		//async.Return(),
	)
}

// This is an example of how to create your custom events
type SimpleEvent struct {
	Handler func(body map[string]string) (map[string]string, error)
}

// code that will be executed when event is received
func (t *SimpleEvent) Handle(ctx context.Context, req async.CallbackRequest, input interface{}) (interface{}, error) {
	in := map[string]string{}
	err := json.Unmarshal(input.([]byte), &input)
	if err != nil {
		return nil, err
	}
	out, err := t.Handler(in)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// when we will start listening for this event - Setup() will be called for us to setup this event on external services
func (t *SimpleEvent) Setup(ctx context.Context, req async.CallbackRequest) (json.RawMessage, error) {
	// we will receive event via http call, no setup is needed
	return nil, nil
}

// when we will stop listening for this event - Teardown() will be called for us to remove this event on external services
func (t *SimpleEvent) Teardown(ctx context.Context, req async.CallbackRequest, handled bool) error {
	// we will receive event via http call, no teardown is needed
	return nil
}

// Receive event and forward it to workflow engine
func SimpleEventHandler(w http.ResponseWriter, r *http.Request) {
	d, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, err.Error())
		return
	}
	out, err := engine.HandleEvent(r.Context(), mux.Vars(r)["id"], mux.Vars(r)["event"], d)
	if err != nil {
		w.WriteHeader(400)
		fmt.Fprintf(w, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
