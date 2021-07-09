package main

import (
	"log"
	"time"

	"github.com/gorchestrate/async"
)

type PizzaOrderWorkflow struct {
	ID           string
	IsAuthorized bool
	OrderNumber  string
	Created      time.Time
	Status       string
	Request      PizzaOrderRequest
	I            int
	PI           int
	Wg           int
}

type Pizza struct {
	ID    string
	Name  string
	Sause string
	Qty   int
}
type PizzaOrderResponse struct {
	OrderNumber string
}

type PizzaOrderRequest struct {
	User      string
	OrderTime time.Time
	Pizzas    []Pizza
}

func (e *PizzaOrderWorkflow) Definition() async.Section {
	return S(
		Step("start", func() error {
			log.Print("eeee ")
			return nil
		}),
		If(!e.IsAuthorized,
			Step("do auth", func() error {
				log.Print("Do AUTH ")
				return nil
			}),
		),
		Wait("timeout select",
			On("timeout1", gTaskMgr.Timeout(time.Second*3),
				Step("start3", func() error {
					log.Print("eeee ")
					return nil
				}),
				Step("start4", func() error {
					log.Print("eeee222 ")
					return nil
				}),
			),
			On("timeout2", gTaskMgr.Timeout(time.Second*3),
				Step("start113", func() error {
					log.Print("222eeee ")
					return nil
				}),
				Step("start3334", func() error {
					log.Print("222eeee222 ")
					return nil
				}),
			)),
		Step("start2", func() error {
			log.Print("tttttt ")
			return nil
		}),
		Step("notify parent process", func() error {
			log.Print("tttttt ")
			return nil
		}),
		async.Return(),
	)
}
