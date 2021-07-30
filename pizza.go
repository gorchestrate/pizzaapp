package main

import (
	"log"
	"time"

	"github.com/gorchestrate/async"
	"github.com/gorchestrate/gasync"
)

// syntax sugar to make workflow definition more readable and less repetitive
var S = async.S
var If = async.If
var Switch = async.Switch
var Case = async.Case
var Step = async.Step
var For = async.For
var On = async.On
var Go = async.Go
var Wait = async.Wait
var WaitFor = async.WaitFor
var Return = async.Return
var Break = async.Break

type Empty struct {
}

type Pizza struct {
	Name string
	Qty  int
}

type CookingRecord struct {
	CookName string
}

type PizzaOrderWorkflow struct {
	Cart     []Pizza
	Status   string
	Paid     bool
	CookName string
}

func (wf *PizzaOrderWorkflow) Definition() async.Section {
	return S(
		Step("setup", func() error {
			wf.Status = "setup"
			return nil
		}),
		For("order not yet submitted", wf.Status != "submitted",
			Wait("for user input",
				On("24h passsed", gs.Timeout(24*3600*time.Second),
					Step("cart timed out1", func() error {
						wf.Status = "timed out"
						return nil
					}),
					Step("cart timed out2", func() error {
						wf.Status = "timed out"
						return nil
					}),
					Step("cart timed out3", func() error {
						wf.Status = "timed out"
						return nil
					}),
					Return(), //stop workflow
				),
				gasync.Event("add", func(in Pizza) (PizzaOrderWorkflow, error) {
					wf.Cart = append(wf.Cart, in)
					return *wf, nil
				}),
				gasync.Event("clean", func(in Empty) (PizzaOrderWorkflow, error) {
					wf.Cart = []Pizza{}
					return *wf, nil
				}),
				gasync.Event("submit", func(in Empty) (PizzaOrderWorkflow, error) {
					wf.Status = "submitted"
					return *wf, nil
				}),
			),
		),

		Wait("manager to confirm order",
			On("10min passsed", gs.Timeout(10*60*time.Second),
				Step("manager didn't confirm", func() error {
					wf.Status = "manager is sleeping"
					log.Printf("notify user that order won't be processed because manager did not confirm order in time")
					return nil
				}),
				Return(), //stop workflow
			),
			gasync.Event("confirm", func(in Empty) (PizzaOrderWorkflow, error) {
				wf.Status = "confirmed"
				return *wf, nil
			}),
		),

		Go("customer pays while order is cooking", S(
			Wait("customer pays money",
				gasync.Event("confirm_payment", func(in Empty) (PizzaOrderWorkflow, error) {
					wf.Paid = true
					return *wf, nil
				}),
			),
		)),

		Wait("for kitchen to take order",
			On("30min passsed", gs.Timeout(30*60*time.Second),
				Step("kitchen didn't confirm", func() error {
					wf.Status = "kitchen is sleeping"
					log.Printf("notify user that order won't be processed because kitchen is sleeping")
					return nil
				}),
				Return(), //stop workflow
			),
			gasync.Event("start_cooking", func(in CookingRecord) (PizzaOrderWorkflow, error) {
				wf.Status = "cooking"
				wf.CookName = in.CookName
				return *wf, nil
			}),
		),

		Wait("pizzas to be cooked",
			On("1h cook timeout", gs.Timeout(60*60*time.Second),
				Step("kitchen didn't cook in time", func() error {
					wf.Status = "kitchen cooking is not done"
					log.Printf("notify user that order won't be processed because kitchen can't cook his pizza")
					return nil
				}),
				Return(), //stop workflow
			),
			gasync.Event("cooked", func(in Empty) (PizzaOrderWorkflow, error) {
				wf.Status = "cooked"
				return *wf, nil
			}),
		),

		Wait("to be taken for delivery",
			On("1h to take timeout", gs.Timeout(60*60*time.Second),
				Step("delivery forgot about this order", func() error {
					wf.Status = "delivery is not done"
					log.Printf("notify user that order won't be processed because delivery can't be done")
					return nil
				}),
				Return(), //stop workflow
			),
			gasync.Event("take_for_delivery", func(in Empty) (PizzaOrderWorkflow, error) {
				wf.Status = "delivering"
				return *wf, nil
			}),
		),
		Wait("for delivered",
			On("1h delivery timeout", gs.Timeout(60*60*time.Second),
				Step("delivery lost on the road", func() error {
					wf.Status = "delivery is lost"
					log.Printf("notify user that order won't be processed because delivery was lost on a road")
					return nil
				}),
				Return(), //stop workflow
			),
			gasync.Event("delivered", func(in Empty) (PizzaOrderWorkflow, error) {
				wf.Status = "delivered"
				return *wf, nil
			}),
		),
		WaitFor("payment", wf.Paid, func() {
			wf.Status = "completed"
		}),
	)
}
