package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gorchestrate/async"
)

type LocalResumer struct {
	r *async.Runner
}

func (gr *LocalResumer) ScheduleResume(r *async.Runner, id string) error {
	log.Print("scheduling resume")
	go func() {
		for i := 0; i < 10; i++ {
			err := r.Resume(context.Background(), time.Second*10, 10000, id)
			if err != nil {
				log.Printf("err resuming %v: %v", id, err)
				time.Sleep(time.Second * 1)
				continue
			}
			return
		}
	}()
	return nil
}

type LocalTimeoutManager struct {
	r *async.Runner
}

func (t *LocalTimeoutManager) Setup(req async.CallbackRequest) error {
	var dur time.Duration
	err := json.Unmarshal(req.Data, &dur)
	if err != nil {
		return err
	}
	go func() {
		time.Sleep(dur)
		err := t.r.OnCallback(context.Background(), req)
		if err != nil {
			panic(err)
		}
	}()
	log.Printf("timer setup")
	return nil
}

func (t *LocalTimeoutManager) Teardown(req async.CallbackRequest) error {
	log.Printf("timer teardown")
	return nil
}

type TimeoutHandler struct {
	Delay time.Duration
}

// no aciton for timeout handler
func (t *TimeoutHandler) Handle(req async.CallbackRequest, input json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

func (t *TimeoutHandler) Marshal() json.RawMessage {
	d, err := json.Marshal(t.Delay)
	if err != nil {
		panic(err)
	}
	return d
}

func (t *TimeoutHandler) Type() string {
	return "timeout"
}
