package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gorchestrate/async"
	"github.com/gorchestrate/gcloud-plugins"
	"github.com/gorilla/mux"
	cloudtasks "google.golang.org/api/cloudtasks/v2beta3"
)

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
	r := async.Runner{}
	mr := mux.NewRouter()
	engine := &gcloud.FirestoreEngine{
		DB:         db,
		Collection: "workflows",
		Workflows: map[string]func() async.WorkflowState{
			"pizzaOrder": func() async.WorkflowState {
				return &PizzaOrderWorkflow{}
			},
		},
	}

	s := &gcloud.GTasksScheduler{
		Engine:     engine,
		C:          cTasks,
		ProjectID:  "async-315408",
		LocationID: "us-central1",
		QueueName:  "order",
		ResumeURL:  "https://pizzaapp-ffs2ro4uxq-uc.a.run.app/resume",
	}
	mr.HandleFunc("/resume", s.ResumeHandler)
	engine.Scheduler = s

	gTaskMgr := &gcloud.TimeoutMgr{
		Engine:      engine,
		C:           cTasks,
		ProjectID:   "async-315408",
		LocationID:  "us-central1",
		QueueName:   "order",
		CallbackURL: "https://pizzaapp-ffs2ro4uxq-uc.a.run.app/callback/timeout",
	}
	mr.HandleFunc("/callback/timeout", gTaskMgr.TimeoutHandler)

	r.CallbackManagers = map[string]async.CallbackManager{
		gcloud.TimeoutTypeName: gTaskMgr,
	}

	mr.HandleFunc("/new", func(rw http.ResponseWriter, req *http.Request) {
		id := fmt.Sprint(rand.Intn(10000))
		engine.ScheduleAndCreate(req.Context(), id, "pizzaOrder", &PizzaOrderWorkflow{})
	})

	err = http.ListenAndServe(":8080", mr)
	if err != nil {
		panic(err)
	}
}

var S = async.S
var Wait = async.Wait
var If = async.If
var Switch = async.Switch
var Case = async.Case
var Step = async.Step
var For = async.For
var On = async.On
