package main

import (
	"fmt"
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

type LogRecord struct {
	Time time.Time
	Log  string
}

type ConfirmRecord struct {
	ManagerName string
	Amount      float64
}

type PizzaOrderWorkflow struct {
	Cart        []Pizza
	Status      string
	PaidAmount  float64
	Amount      float64
	Location    string
	CookName    string
	ManagerName string
	Logs        []LogRecord
}

type PaymentRecord struct {
	PaidAmount      float64
	DeliveryAddress string
}

func (wf *PizzaOrderWorkflow) Log(format string, params ...interface{}) {
	wf.Logs = append(wf.Logs, LogRecord{
		Time: time.Now(),
		Log:  fmt.Sprintf(format, params...),
	})
}

// TODO: WaitCond should be within Wait stmt

func (wf *PizzaOrderWorkflow) Definition() async.Section {
	return S(
		Step("setup", func() error {
			wf.Status = "setup"
			wf.Log("started workflow")
			return nil
		}),
		For("order not yet submitted", wf.Status != "submitted",
			Wait("for user input",
				gs.Timeout("cart timed out", 24*3600*time.Second, S(
					Return(), //stop workflow
				)),
				gasync.Event("Customer", "AddToCart", func(in Pizza) (PizzaOrderWorkflow, error) {
					wf.Cart = append(wf.Cart, in)
					return *wf, nil
				}),
				gasync.Event("Customer", "EmptyCart", func(in Empty) (PizzaOrderWorkflow, error) {
					wf.Cart = []Pizza{}
					return *wf, nil
				}),
				gasync.Event("Customer", "SubmitCart", func(in Empty) (PizzaOrderWorkflow, error) {
					wf.Status = "submitted"
					wf.Log("cart submitted")
					return *wf, nil
				}),
			),
		),

		Wait("manager to confirm order",
			gasync.Event("Manager", "ConfirmOrder", func(in ConfirmRecord) (PizzaOrderWorkflow, error) {
				wf.Status = "confirmed"
				wf.ManagerName = in.ManagerName
				wf.Amount = in.Amount
				wf.Log("manager confirmed order")
				return *wf, nil
			}),
		),

		Go("customer pays while order is cooking", S(
			Wait("customer pays money",
				gasync.Event("Manager", "ConfirmPayment", func(in PaymentRecord) (PizzaOrderWorkflow, error) {
					if in.PaidAmount+0.01 < wf.Amount {
						return *wf, fmt.Errorf("amount paid should be %v", wf.Amount)
					}
					wf.PaidAmount = in.PaidAmount
					wf.Location = in.DeliveryAddress
					wf.Log("customer paid $%v ", in.PaidAmount)
					return *wf, nil
				}),
			),
		)),

		Wait("for kitchen to take order",
			gasync.Event("Cook", "StartCooking", func(in CookingRecord) (PizzaOrderWorkflow, error) {
				wf.Status = "cooking"
				wf.CookName = in.CookName
				wf.Log("started cooking")
				return *wf, nil
			}),
		),

		Wait("pizzas to be cooked",
			gasync.Event("Cook", "Cooked", func(in Empty) (PizzaOrderWorkflow, error) {
				wf.Status = "cooked"
				wf.Log("cooked order")
				return *wf, nil
			}),
		),

		Wait("to be taken for delivery",
			gasync.Event("Delivery", "TakeForDelivery", func(in Empty) (PizzaOrderWorkflow, error) {
				wf.Status = "delivering"
				wf.Log("taken for delivery")
				return *wf, nil
			}),
		),
		Wait("for delivered",
			gasync.Event("Delivery", "Delivered", func(in Empty) (PizzaOrderWorkflow, error) {
				wf.Status = "delivered"
				wf.Log("delivered")
				return *wf, nil
			}),
		),
		WaitFor("payment", wf.PaidAmount > 0, func() {
			wf.Status = "completed"
			wf.Log("completed")
		}),
	)
}
