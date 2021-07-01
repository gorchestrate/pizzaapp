package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gorchestrate/async"
	"github.com/gorilla/mux"
	"google.golang.org/api/cloudtasks/v2"
	// cloudtasks "google.golang.org/api/cloudtasks/v2beta3"
)

// Basic UI using jsonschema (separate module to do that? Need support subworkflows?)

// Everything is a workflow! i.e. event-sourcing approach for syncing up settings
// and other events. How to do that?
//

var S = async.S
var Wait = async.Wait
var If = async.If
var Switch = async.Switch
var Case = async.Case
var Step = async.Step
var For = async.For
var On = async.On

func main() {
	rand.Seed(time.Now().Unix())
	ctx := context.Background()
	db, err := firestore.NewClient(ctx, "async-315408")
	if err != nil {
		panic(err)
	}
	cTasks, err := cloudtasks.NewService(ctx)
	if err != nil {
		panic(err)
	}

	r := async.Runner{
		DB: FirestoreStorage{
			DB:         db,
			Collection: "orders",
		},
		Workflows: map[string]async.Workflow{
			"order": func() async.WorkflowState {
				return &PizzaOrderWorkflow{}
			},
		},
	}

	mr := mux.NewRouter()
	if os.Getenv("GOOGLE_RUN") != "" {
		cr := &CloudTasksResumer{
			r:          &r,
			C:          cTasks,
			ProjectID:  "async-315408",
			LocationID: "us-central1",
			QueueName:  "order",
			ResumeURL:  "https://pizzaapp-ffs2ro4uxq-uc.a.run.app/resume",
		}
		r.Resumer = cr
		mr.HandleFunc("/resume", cr.ResumeHandler)

		gTaskMgr := &GTasksTimeoutMgr{
			r:           &r,
			C:           cTasks,
			ProjectID:   "async-315408",
			LocationID:  "us-central1",
			QueueName:   "order",
			CallbackURL: "https://pizzaapp-ffs2ro4uxq-uc.a.run.app/callback/timeout",
		}
		mr.HandleFunc("/callback/timeout", gTaskMgr.TimeoutHandler)
		r.CallbackManagers = map[string]async.CallbackManager{
			"timeout": gTaskMgr,
		}
	} else {
		r.Resumer = &LocalResumer{}
		r.CallbackManagers = map[string]async.CallbackManager{
			"timeout": &LocalTimeoutManager{r: &r},
		}
	}

	mr.HandleFunc("/new", func(rw http.ResponseWriter, req *http.Request) {
		id := fmt.Sprint(rand.Intn(10000))
		err = r.NewWorkflow(context.Background(), id, "order", PizzaOrderWorkflow{})
		if err != nil {
			panic(err)
		}
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

func (e *PizzaOrderWorkflow) Definition() async.Section {
	return S(
		Step("start", func() async.ActionResult {
			log.Print("eeee ")
			return async.ActionResult{Success: true}
		}),
		If(!e.IsAuthorized,
			Step("do auth", func() async.ActionResult {
				log.Print("Do AUTH ")
				return async.ActionResult{Success: true}
			}),
		),
		Wait("timeout select",
			On("timeout1", &TimeoutHandler{Delay: time.Second * 3},
				Step("start3", func() async.ActionResult {
					log.Print("eeee ")
					return async.ActionResult{Success: true}
				}),
				Step("start4", func() async.ActionResult {
					log.Print("eeee222 ")
					return async.ActionResult{Success: true}
				}),
			),
			On("timeout2", &TimeoutHandler{Delay: time.Second * 3},
				Step("start113", func() async.ActionResult {
					log.Print("222eeee ")
					return async.ActionResult{Success: true}
				}),
				Step("start3334", func() async.ActionResult {
					log.Print("222eeee222 ")
					return async.ActionResult{Success: true}
				}),
			)),
		Step("start2", func() async.ActionResult {
			log.Print("tttttt ")
			return async.ActionResult{Success: true}
		}),
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
