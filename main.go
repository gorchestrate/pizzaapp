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
	cloudtasks "google.golang.org/api/cloudtasks/v2beta3"
)

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
		Step("start", func() error {
			log.Print("eeee ")
			return nil
		}),
		If(!e.IsAuthorized,
			Step("do auth", func() error {
				log.Print("Do AUTH ")
				return nil
			}),
		),
		Wait("timeout select",
			On("timeout1", &TimeoutHandler{Delay: time.Second * 3},
				Step("start3", func() error {
					log.Print("eeee ")
					return nil
				}),
				Step("start4", func() error {
					log.Print("eeee222 ")
					return nil
				}),
			),
			On("timeout2", &TimeoutHandler{Delay: time.Second * 3},
				Step("start113", func() error {
					log.Print("222eeee ")
					return nil
				}),
				Step("start3334", func() error {
					log.Print("222eeee222 ")
					return nil
				}),
			)),
		Step("start2", func() error {
			log.Print("tttttt ")
			return nil
		}),
		async.Return("end"),
	)
}

// Basic UI using jsonschema (separate module to do that? Need support subworkflows?)

// Everything is a workflow! i.e. event-sourcing approach for syncing up settings
// and other events. How to do that?
//

// Add WAIT() condition function that is evaluated  each time after parallel thread is updated
// useful to simulate sync.WorkGroup ???
// can be added to Select() stmt

/*
// GCLOUD:
Context canceling - separate CallbackManager & Handler.
	WaitCtx():
		SETUP:() add to DB (conditionally)
			If ctx is already canceled - create task for cancellation
		Teardown(): remove from DB
		When canceling Ctx - mark ctx as canceled(intermediate status) and create task to cancel each of Setup() processes


Sending/Receiving channels - separate service/library for that.
*/

// Add channels? not sure we need them.
// All our process is executed in 1 thread and most of the time they are useless.
// Make them scoped only to 1 process (for now, because it's easy to screw it up)
