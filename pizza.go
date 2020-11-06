package main

import (
	"fmt"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorchestrate/async"
	"github.com/gorchestrate/mail-plugin"
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
type OrderPizzaProcess struct {
	DB      *bolt.DB `json:"-"`
	Order   Order
	Cancel  async.Channel
	Thread2 string
}

func (s OrderPizzaProcess) Type() *async.Type {
	return async.ReflectType(fmt.Sprintf("%s.OrderPizzaProcess", serviceName), s)
}

// This is main() function for our process   (To be renamed to Main() in future)
func (s *OrderPizzaProcess) Start(p *async.P, order Order) error {
	s.Order = order                        // store order in workflow state for future use
	s.Cancel = p.MakeChan(order.Type(), 0) // create channel to cancel pizza order

	log.Print("You can execute arbitrary code in the callback. For example saving order in DB")
	err := s.DB.Update(func(t *bolt.Tx) error {
		b, err := t.CreateBucketIfNotExists([]byte("orders"))
		if err != nil {
			return err
		}
		return b.Put([]byte(order.Phone), []byte("saved order body"))
	})
	if err != nil {
		return err
		// returning error from Async process means process has failed to execute.
		// it does not change the state of the process - callback will be retried after some time
		// all execution/connectivity errors are handled here. All business-level errors should be returned via p.Finish()
	}

	p.Go("Thread2", func(p *async.P) { // create new thread in our workflow(process) that will manage cancellation
		p.After(time.Second * 1800).To(s.Aborted)
		p.Recv(s.Cancel).To(s.Canceled)
	})
	// this is equivalent to
	// go func() {
	//	 select {
	//	   case <-time.After(time.Second * 1800):
	//	      s.Aborted()
	//	   case <-s.Cancel:
	//	      s.Canceled()
	//	}
	//}()

	p.Call("mail.Approve()", mail.ApprovalRequest{
		Message: fmt.Sprintf("Please approve order (reply with 'Approved'): \n\n Pizzas: %v Phone: %v", order.Pizzas, order.Phone),
		To:      []string{order.ManagerEmail},
	}).To(s.Done)
	// gorchestrate ensures that s.Done() callback expects same type that the called method returns
	// if s.Done() expects different type than mail.Approve() returns - gorchestrate will refuse to make a call
	// This type-safety is ensured API calls and channels operations.

	// After callback has finished - current workflow state and blocked conditions (calls, sends,recvs) will be sent to Gorchestrate Core, validated and saved.
	return nil
}

// Callback defines the type it's expecting
func (s *OrderPizzaProcess) Done(p *async.P, resp mail.ApprovalResponse) error {
	// mark this process as finished
	// if process was doing recv/send on channel - this select will be aborted (if it wasn't triggered already)
	p.Finish(ConfirmedOrder{
		Order:    s.Order,
		Approved: resp.Approved,
		Message:  resp.Comments,
	})
	return nil
}

func (s *OrderPizzaProcess) Aborted(p *async.P) error {
	p.Finish(ConfirmedOrder{
		Order:    s.Order,
		Approved: false,
		Message:  "order was not confirmed in time",
	})
	return nil
}

func (s *OrderPizzaProcess) Canceled(p *async.P) error {
	p.Finish(ConfirmedOrder{
		Order:    s.Order,
		Approved: false,
		Message:  "order was canceled by user",
	})
	return nil
}
