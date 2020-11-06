package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorchestrate/async"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var service = async.Service{
	// Name of the service.
	// Two different service instances won't affect each other's performance
	// Service Name is also used to filter and consume event history just for your service.
	Name: "pizza",

	// Each service is like a Go package - defines it's types and methods(APIs)
	// All requests to gorchestrate core will validate calls to comply with types defined here
	// If someone sends a request with invalid type - request will be rejected before it reaches your service
	Types: []*async.Type{
		async.ReflectType("pizza.Order", &Order{}),
		async.ReflectType("pizza.ConfirmedOrder", &ConfirmedOrder{}),
		async.ReflectType("pizza.OrderPizzaProcess", &OrderPizzaProcess{}),
	},

	// APIs define processes(workflows) that this service supports
	APIs: []async.API{
		{
			// This is a constructor for process struct to which current process(workflow) state will be unmarshalled to
			// You may want to set here external connections, db clients, and other data you may need inside process(workflow) definition callbacks
			NewProcState: func() interface{} {
				return &OrderPizzaProcess{}
			},
			API: &async.ProcessAPI{
				// Process name should be in format serviceName.ProcessName()
				Name:        "pizza.OrderPizza()",
				Description: "Order new pizza",

				// Input type is used to validate request that want to create new process
				// So that this process receives correct inputs
				Input: "pizza.Order",
				// Output type is used to validate output of this process.
				// So that clients who receive result of this process get expected result
				Output: "pizza.ConfirmedOrder",

				// Description of the process state. This is used to make sure that all updates to the process are type-safe.
				// It also makes sure that all changes to process state type are backward-compatible and clients
				// who listening for process updated events receive correct type-safe stream of data
				State: "pizza.OrderPizzaProcess",

				Service: "pizza", // Duplicated field yo be removed

			},
		},
	},
}

func main() {
	// init GRPC connection
	ctx := context.Background()
	coreAddr := os.Getenv("CORE_ADDR")
	if coreAddr == "" {
		coreAddr = "51.158.73.58:9090" // default demo server
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

	// Listen for processes that need to be processed.
	// This method will get the update, call appropriate callback and save updated process on the server.
	err = async.Manage(ctx, client, service)
	if err != nil {
		log.Fatal(err)
	}
}
