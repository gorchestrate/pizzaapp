package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorchestrate/async"
	"github.com/gorchestrate/mail-plugin"
)

func (s *OrderPizzaWorkflow) Main(p *async.P, order Order) error {
	//you have full control over current state of the workflow
	//Everything assigned to workflow struct will be automatically persisted by Gorchestrate
	s.Order = order

	//you can use this to manage statuses in a way that works for you
	s.State = "Started"

	// you can execute any code you want in the process. For example save record to DB
	_ = s.DB.Update(func(t *bolt.Tx) error {
		b, _ := t.CreateBucketIfNotExists([]byte("orders"))
		return b.Put([]byte(order.Phone), []byte("saved order body"))
	})

	// you can have any custom logic inside your code. No need to draw diagrams
	if len(s.Order.Pizzas) < 100 {
		// you can call other workflows inside your workflow
		mail.SendAndWaitAnswer(p, mail.Message{
			To:      order.ManagerEmail,
			CC:      []string{},
			Subject: "Please approve pizza order",
			Message: fmt.Sprintf("Please approve order (reply with 'Approved'): \n\n Pizzas: %v Phone: %v", order.Pizzas, order.Phone),
		}).To(s.Answered)
	} else {
		// you can finish your workflow at any time
		p.Finish(ConfirmedOrder{
			Order:    s.Order,
			Approved: false,
			Message:  "We don't accept such big orders",
		})
		return nil
	}

	// you can run multiple goroutines in you workflow
	p.Go("Thread2", func(p *async.P) {
		// you can manage concurrency same way you do it in Go - using channels, timeouts and etc...
		s.Cancel = p.MakeChan(order.Type(), 0)
		p.Select().
			After(time.Second * 1800).To(s.Aborted).
			Recv(s.Cancel).To(s.Canceled)
	})
	return nil
}

func (s *OrderPizzaWorkflow) Answered(p *async.P, resp mail.Message) error {
	if strings.Contains(resp.Message, "Approved") {
		s.State = "Approved"
		p.Finish(ConfirmedOrder{
			Order:    s.Order,
			Approved: true,
			Message:  resp.Message,
		})
		return nil
	}
	mail.SendAndWaitAnswer(p, mail.Message{
		To:      s.Order.ManagerEmail,
		CC:      []string{},
		Subject: "Please approve pizza order again",
		Message: fmt.Sprintf("Please approve order (reply with 'Approved'): \n\n Pizzas: %v Phone: %v", s.Order.Pizzas, s.Order.Phone),
	}).To(s.Answered)
	return nil
}

func (s *OrderPizzaWorkflow) Aborted(p *async.P) error {
	s.State = "Aborted"
	p.Finish(ConfirmedOrder{
		Order:    s.Order,
		Approved: false,
		Message:  "order was not confirmed in time",
	})
	return nil
}

func (s *OrderPizzaWorkflow) Canceled(p *async.P) error {
	s.State = "Canceled"
	p.Finish(ConfirmedOrder{
		Order:    s.Order,
		Approved: false,
		Message:  "order was canceled by user",
	})
	return nil
}
