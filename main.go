package main

import (
	"log"
	"net/http"

	"github.com/gorchestrate/async"
	"github.com/gorchestrate/gasync"
)

var gs *gasync.Server

func main() {
	var err error
	gs, err = gasync.NewServer(gasync.Config{
		GCloudProjectID:      "async-315408",
		GCloudLocationID:     "us-central1",
		GCloudTasksQueueName: "order",
		BasePublicURL:        "https://pizzaapp-ffs2ro4uxq-uc.a.run.app",
		CORS:                 true,
		Collection:           "workflows",
		SignSecret:           "{Secret to sign callbacks sent over public API from Google Cloud Tasks}",
	}, map[string]func() async.WorkflowState{
		"pizza": func() async.WorkflowState {
			return &PizzaOrderWorkflow{}
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(http.ListenAndServe(":8080", gs.Router))
}
