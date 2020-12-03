package main

import (
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/gorchestrate/async"
)

type Pizza struct {
	Name string
	Size int
}

type Order struct {
	Pizzas       []Pizza
	Phone        string
	ManagerEmail string
}

// type definition for gorchestrate core.
func (s Order) Type() *async.Type {
	// here we simply generating one using reflection
	return async.ReflectType(fmt.Sprintf("%s.Order", serviceName), s)
}

type ConfirmedOrder struct {
	Order    Order
	Approved bool
	Message  string
}

func (s ConfirmedOrder) Type() *async.Type {
	return async.ReflectType(fmt.Sprintf("%s.ConfirmedOrder", serviceName), s)
}

// this is a state of our workflow that will be persistet between callbacks(methods)
type OrderPizzaWorkflow struct {
	State   string
	DB      *bolt.DB `json:"-"`
	Order   Order
	Cancel  async.Channel
	Thread2 string
}

func (s OrderPizzaWorkflow) Type() *async.Type {
	return async.ReflectType(fmt.Sprintf("%s.OrderPizzaWorkflow", serviceName), s)
}
