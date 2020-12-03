package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/boltdb/bolt"
	"github.com/gorchestrate/async"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var serviceName string

func main() {
	serviceName = os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		log.Fatal("please specify unique SERVICE_NAME to avoid collision on a demo server")
	}

	// init GRPC connection
	ctx := context.Background()
	coreAddr := os.Getenv("CORE_ADDR")
	if coreAddr == "" {
		coreAddr = "51.158.73.58:9090" // default demo server
		serviceName = os.Getenv("SERVICE_NAME")
	}
	conn, err := grpc.Dial(coreAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := async.NewRuntimeClient(conn)

	// Setup HTTP handlers
	s := Service{
		c: client,
	}
	r := mux.NewRouter()
	r.HandleFunc("/order/{id}/cancel", s.CancelOrderHandler).Methods("POST")
	r.HandleFunc("/order/{id}", s.NewOrderHandler).Methods("POST")
	r.HandleFunc("/order/{id}", s.GetOrderHandler).Methods("GET")
	go func() {
		err := http.ListenAndServe(":8080", r)
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	db, err := bolt.Open("orders.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	service := async.Service{
		Name: fmt.Sprintf("%s", serviceName),
		Types: []*async.Type{
			Order{}.Type(),
			ConfirmedOrder{}.Type(),
			OrderPizzaWorkflow{}.Type(),
		},
		APIs: []async.API{
			{
				NewProcState: func() interface{} {
					return &OrderPizzaWorkflow{
						DB: db,
					}
				},
				API: &async.WorkflowAPI{
					Name:        fmt.Sprintf("%s.OrderPizza()", serviceName),
					Description: "Order new pizza",
					Input:       fmt.Sprintf("%s.Order", serviceName),
					Output:      fmt.Sprintf("%s.ConfirmedOrder", serviceName),
					State:       fmt.Sprintf("%s.OrderPizzaWorkflow", serviceName),
					Service:     fmt.Sprintf("%s", serviceName),
				},
			},
		},
	}

	// Listen for processes that need to be processed.
	// This method will get the update, call appropriate callback and save updated process on the server.
	err = async.Manage(ctx, client, service)
	if err != nil {
		log.Fatal(err)
	}
}
