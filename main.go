package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	_ "embed"

	"github.com/gorchestrate/async"
	"github.com/gorchestrate/gasync"
)

/*

Workflows
	- Order
		- order1
		- order2
		- order3
	- Delivery
	- Pizza

ACL:
	Workflows: a,b,c
		A:
			View:
			[{
				Query - which workflows we can see
					{
						equal: {
							key:"",
							value:""
						},
						not-equal: {
							key:"",
							value:""
						},
						contains: {
							key: "",
							value: ""
						},
						range:
							key:
							ge:
							le:
							lte:
							gte:
					}
					Expr Match
				PrefixFilter - what parts of workflows we can see
				{
					"include": *
					"exclude": *.secrets
					"events": * - what events are allowed
				}
			}]





Filter-based access levels:
	- ReadQuery  -> allow to see workflows matching query   (added on query level)
	- ReadSubquery  -> allow to see parts of workflows mathing query  (filtered after query)
	- New -> allow to create new workflow
	- Delete -> allow to delete workflow
	- Event -> allow to delete workflow
	- Change -> allow to update parts of the workflow matching query

Problem: how to make this filtering logic unified across databases?
	[
		"query": {
			filter: "Firestore-compatible filter",

		},
		"include": ...,
		"exclude": ...,
	]
Using this logic - Frontend can query whatever they want from backend.
??? query language ???






*/
// ReadFilter ->
//   -

// F
// RW roles => operation:  Read, Event

//go:embed ui/build/*
var ui embed.FS

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
	sub, err := fs.Sub(ui, "ui/build")
	if err != nil {
		log.Fatal(err)
	}
	gs.Router.PathPrefix("/ui/").Handler(http.StripPrefix("/ui/", http.FileServer(http.FS(sub))))
	log.Fatal(http.ListenAndServe(":8080", gs.Router))
}
