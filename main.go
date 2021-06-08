package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gorchestrate/async"
	"github.com/gorilla/mux"
	cloudtasks "google.golang.org/api/cloudtasks/v2beta3"
)

// Basic UI using jsonschema (separate module to do that? Need support subworkflows?)

// Everything is a workflow! i.e. event-sourcing approach for syncing up settings
// and other events. How to do that?
//

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
	r := async.Runner{
		DB: FirestoreStorage{
			DB:         db,
			Collection: "orders",
		},
		TaskMgr: CloudTaskManager{
			C:           cTasks,
			ProjectID:   "async-315408",
			LocationID:  "us-central1",
			QueueName:   "order",
			ResumeURL:   "https://pizzaapp-ffs2ro4uxq-uc.a.run.app/resume",
			CallbackURL: "https://pizzaapp-ffs2ro4uxq-uc.a.run.app/callback",
		},
		Workflows: map[string]async.Workflow{
			"order": func() async.WorkflowState {
				return &PizzaOrderWorkflow{}
			},
		},
	}
	mr := mux.NewRouter()
	mr.HandleFunc("/new", func(rw http.ResponseWriter, req *http.Request) {
		err = r.NewWorkflow(context.Background(), fmt.Sprint(rand.Intn(10000)), "order", PizzaOrderWorkflow{})
		if err != nil {
			panic(err)
		}
	})
	mr.HandleFunc("/resume", func(rw http.ResponseWriter, req *http.Request) {
		var resume async.ResumeRequest
		err := json.NewDecoder(req.Body).Decode(&resume)
		if err != nil {
			panic(err)
		}
		err = r.OnResume(req.Context(), resume)
		if err != nil {
			panic(err)
		}
	})
	mr.HandleFunc("/callback", func(rw http.ResponseWriter, req *http.Request) {
		var cb async.CallbackRequest
		err := json.NewDecoder(req.Body).Decode(&cb)
		if err != nil {
			panic(err)
		}
		err = r.OnCallback(req.Context(), cb)
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
	PI           int
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

var S = async.S
var Select = async.Select
var After = async.After
var Step = async.Step
var On = async.On
var For = async.For

func UserAction(input string) []async.WaitCond {
	return []async.WaitCond{}
}

func (e *PizzaOrderWorkflow) Definition() async.Section {
	return S(
		For(e.I < 100, "loodp", S(
			Step("inside Loop", func() async.ActionResult {
				log.Print("LOOP ", e.I)
				e.I++
				return async.ActionResult{Success: true}
			}),
		)),
		Step("outside wait loop", func() async.ActionResult {
			log.Print("eeee ", e.I)
			e.I = 0
			return async.ActionResult{Success: true}
		}),
		For(e.I < 10, "loodpd", S(
			Step("inside wait loopd", func() async.ActionResult {
				log.Print("LOOP2 ", e.I)
				e.I++
				return async.ActionResult{Success: true}
			}),
			async.Go("parallel", S(
				Select("wait in loop",
					async.After(time.Second*5, S()),
				),
				Step("inside wait loopd goroutine", func() async.ActionResult {
					log.Print("INSIDE LOOP! ", e.I)
					e.PI++
					return async.ActionResult{Success: true}
				}),
			), func() string { return fmt.Sprint(e.I) + "_thread" }),
		)),
		Select("wait 1 minute loop",
			async.After(time.Minute, S()),
		),
		async.Return("end"),
	)
}

// Add WAIT() condition function that is evaluated  each time after process is updated
// useful to simulate sync.WorkGroup
// can be added to Select() stmt

/*
// GCLOUD:
Context canceling?
	- Canceling is batch write operation, updates both task and canceled ctx
	- After ctx has been updated via cloud - we create cloud tasks for each workflow to update
		all processes that were affected. (for now can be done as post-action, later listen for event)
	- We also monitor writes for all processes and if they have new ctx added
		we will create cloud task for them. (for now can be done as post-action, later listen for event)


// CONCLUSION:
	- PERFECT CLIENTSIDE LIBRARY
	- WAIT FOR JUNE & CHECK HOW EVENT ARC UPDATE NOTIFYING WORKS (latency)


Sending/Receiving channels:
	JUST DO IN-PROCESS channels
	AND IN-PROCESS WAIT
*/

// Add channels? not sure we need them.
// All our process is executed in 1 thread and most of the time they are useless.
// Make them scoped only to 1 process (for now, because it's easy to screw it up)
