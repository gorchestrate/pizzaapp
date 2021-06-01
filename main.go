package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gorchestrate/async"
	cloudtasks "google.golang.org/api/cloudtasks/v2beta3"
)

var S = async.S
var Select = async.Select
var After = async.After
var Step = async.Step
var On = async.On
var For = async.For

func main() {
	rand.Seed(time.Now().Unix())
	ctx := context.Background()
	cTasks, err := cloudtasks.NewService(ctx)
	if err != nil {
		panic(err)
	}
	db, err := firestore.NewClient(ctx, "async-315408")
	if err != nil {
		panic(err)
	}

	// TODO: separate workflows. Each one has it's own runner.
	// it's easy to do that because we have no loops

	// TODO: do less resumes. Just save data - no need to unlock
	// need to unlock only if WAITING
	// TO make this - resumes should be step-agnostic?
	// No problem resuming thread at any point of time, if no resuming is possible - just log err

	r, err := async.NewRunner(async.RunnerConfig{
		BaseURL:    "https://pizzaapp-ffs2ro4uxq-uc.a.run.app",
		ProjectID:  "async-315408",
		LocationID: "us-central1",
		Collection: "orders",
		QueueName:  "order",
		Workflows: map[string]async.Workflow{
			"order": {
				Name: "order",
				InitState: func() async.WorkflowState {
					return &PizzaOrderWorkflow{}
				},
			},
		},
	}, db, cTasks)
	if err != nil {
		panic(err)
	}
	mr := r.Router()
	mr.HandleFunc("/new", func(rw http.ResponseWriter, req *http.Request) {
		err = r.NewWorkflow(context.Background(), fmt.Sprint(rand.Intn(10000)), "order", PizzaOrderWorkflow{})
		if err != nil {
			panic(err)
		}
	})
	err = http.ListenAndServe(":8080", mr)
	if err != nil {
		panic(err)
	}
}

type PizzaOrderWorkflow struct {
	ID           string
	IsAuthorized bool
	OrderNumber  string
	Created      time.Time
	Status       string
	Request      PizzaOrderRequest
	I            int
	Wg           int
}

type Pizza struct {
	ID    string
	Name  string
	Sause string
	Qty   int
}
type PizzaOrderResponse struct {
	OrderNumber string
}

type PizzaOrderRequest struct {
	User      string
	OrderTime time.Time
	Pizzas    []Pizza
}

func (e *PizzaOrderWorkflow) Definition() async.WorkflowDefinition {
	return async.WorkflowDefinition{
		New: func(req PizzaOrderRequest) (*PizzaOrderResponse, error) {
			// POST /pizza/:id
			log.Printf("got pizza order")
			return &PizzaOrderResponse{}, nil
		},
		Body: S(
			For(e.I < 100, "loodp", S(
				Step("inside Loop", func() async.ActionResult {
					log.Print("LOOP")
					e.I++
					return async.ActionResult{Success: true}
				}),
			)),
			async.Return("end"),
		),
	}
}

// Add WAIT() condition function that is evaluated  each time after process is updated
// useful to simulate sync.WorkGroup
// can be added to Select() stmt

/*
// GCLOUD:
Firestore as storage
	- Optimistic locking
Cloud tasks to delay time.After
	- if select canceled - we try to cancel task
		- if can't cancel - no problem - it should cancel itself

Context canceling?
	- Canceling is batch write operation, updates both task and canceled ctx
	- After ctx has been updated via cloud - we create cloud tasks for each workflow to update
		all processes that were affected. (for now can be done as post-action, later listen for event)
	- We also monitor writes for all processes and if they have new ctx added
		we will create cloud task for them. (for now can be done as post-action, later listen for event)


// CONCLUSION:
	- TRY OUT FIRESTORE & Cloud Tasks time.After
	- PERFECT CLIENTSIDE LIBRARY
	- WAIT FOR JUNE & CHECK HOW EVENT ARC UPDATE NOTIFYING WORKS (latency)


Sending/Receiving channels:
	JUST DO IN-PROCESS channels
	AND IN-PROCESS WAIT
*/

// Add channels? not sure we need them.
// All our process is executed in 1 thread and most of the time they are useless.
// Make them scoped only to 1 process (for now, because it's easy to screw it up)
