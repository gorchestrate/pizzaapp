package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gorchestrate/async"
)

type LocalResumer struct {
}

func (gr *LocalResumer) ScheduleExecution(r *async.Runner, id string) error {
	log.Print("scheduling resume")
	go func() {
		// for i := 0; i < 10; i++ {
		// 	err := r.Execute(context.Background(), time.Second*10, 10000, id)
		// 	if err != nil {
		// 		log.Printf("err resuming %v: %v", id, err)
		// 		time.Sleep(time.Second * 1)
		// 		continue
		// 	}
		// 	return
		// }
	}()
	return nil
}

type LocalTimeoutManager struct {
}

func (t *LocalTimeoutManager) Setup(req async.CallbackRequest) error {
	h, ok := req.Handler.(TimeoutHandler)
	if !ok {
		return fmt.Errorf("invalid timeout handler: %v", req)
	}
	go func() {
		time.Sleep(h.Delay)
		// err := t.r.OnCallback(context.Background(), req)
		// if err != nil {
		// 	log.Printf("err on callback: %v", err)
		// }
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
func (t *TimeoutHandler) Handle(req async.CallbackRequest, input interface{}) (interface{}, error) {
	return nil, nil
}

func (t *TimeoutHandler) Type() string {
	return "timeout"
}
