package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gorchestrate/async"
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
		Wait("wait for event 20 seconds",
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
		Wait("wait some time", On("10 seconds passed", gTaskMgr.Timeout(time.Second*10))),
		Step("step2", func() error {
			wf.Status = "finished"
			return nil
		}),
		async.Return(),
	)
}

type SimpleEvent struct {
	Handler func(body map[string]string) (map[string]string, error)
}

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

func (t *SimpleEvent) Setup(ctx context.Context, req async.CallbackRequest) (json.RawMessage, error) {
	return nil, nil
}

func (t *SimpleEvent) Teardown(ctx context.Context, req async.CallbackRequest) error {
	return nil
}
