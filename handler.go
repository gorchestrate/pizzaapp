package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorchestrate/async"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

type Service struct {
	c async.RuntimeClient
}

func (s Service) NewOrderHandler(w http.ResponseWriter, r *http.Request) {
	var o Order
	err := json.NewDecoder(r.Body).Decode(&o)
	if err != nil {
		fmt.Fprintf(w, "error creating process: %v", err)
		return
	}
	if o.ManagerEmail == "" {
		fmt.Fprintf(w, "missing manager email")
		return
	}
	if o.Phone == "" {
		fmt.Fprintf(w, "missing phone number")
		return
	}
	if len(o.Pizzas) == 0 {
		fmt.Fprintf(w, "at least 1 pizza required")
		return
	}
	for i, p := range o.Pizzas {
		if p.Name == "" {
			fmt.Fprintf(w, "pizza  %v name missing", i)
			return
		}
		if p.Size == 0 {
			fmt.Fprintf(w, "pizza  %v size missing", i)
			return
		}
	}
	body, _ := json.Marshal(o)
	_, err = s.c.NewProcess(context.Background(), &async.NewProcessReq{
		Call: &async.Call{
			Id:    mux.Vars(r)["id"],
			Name:  fmt.Sprintf("%s.OrderPizza()", serviceName),
			Input: body,
		},
	})
	if err != nil {
		fmt.Fprintf(w, "error creating process: %v", err)
	}
}

func (s Service) CancelOrderHandler(w http.ResponseWriter, r *http.Request) {
	p, err := s.c.GetProcess(r.Context(), &async.GetProcessReq{
		Id: mux.Vars(r)["id"],
	})
	if err != nil {
		fmt.Fprintf(w, "error creating process: %v", err)
		return
	}
	if p.Status != async.Process_Running {
		fmt.Fprintf(w, "canceling process that is not running")
		return
	}
	var op OrderPizzaProcess
	err = json.Unmarshal(p.State, &op)
	if err != nil {
		panic(err)
	}
	if op.Cancel.Id == "" {
		fmt.Fprintf(w, "process can't be canceled now")
		return
	}
	_, err = s.c.CloseChan(context.Background(), &async.CloseChanReq{
		Ids: []string{op.Cancel.Id},
	})
	if err != nil {
		fmt.Fprintf(w, "error closing chan: %v", err)
	}
}

func (s Service) GetOrderHandler(w http.ResponseWriter, r *http.Request) {
	p, err := s.c.GetProcess(r.Context(), &async.GetProcessReq{
		Id: mux.Vars(r)["id"],
	})
	if err != nil {
		fmt.Fprintf(w, "error creating process: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"Status": p.Status.String(),
		"Output": json.RawMessage(p.Output),
	})

}
