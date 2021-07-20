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

var gTaskMgr *GTasksScheduler
var engine *FirestoreEngine

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
	mr := mux.NewRouter()
	engine = &FirestoreEngine{
		DB:         db,
		Collection: "workflows",
		Workflows: map[string]func() async.WorkflowState{
			"pizzaOrder": func() async.WorkflowState {
				return &PizzaOrderWorkflow{}
			},
		},
	}

	s := &GTasksScheduler{
		Engine:     engine,
		C:          cTasks,
		ProjectID:  "async-315408",
		LocationID: "us-central1",
		QueueName:  "order",
		ResumeURL:  "https://pizzaapp-ffs2ro4uxq-uc.a.run.app/resume",
	}
	mr.HandleFunc("/resume", s.ResumeHandler)
	engine.Scheduler = s
	gTaskMgr = &GTasksScheduler{
		Engine:      engine,
		C:           cTasks,
		ProjectID:   "async-315408",
		LocationID:  "us-central1",
		QueueName:   "order",
		CallbackURL: "https://pizzaapp-ffs2ro4uxq-uc.a.run.app/callback/timeout",
	}
	mr.HandleFunc("/callback/timeout", gTaskMgr.TimeoutHandler)

	mr.HandleFunc("/new/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := engine.ScheduleAndCreate(r.Context(), mux.Vars(r)["id"], "pizzaOrder", &PizzaOrderWorkflow{
			Status: "created",
		})
		if err != nil {
			w.WriteHeader(400)
			fmt.Fprintf(w, err.Error())
			return
		}
		// after callback is handled - we wait for resume process
		err = engine.Resume(r.Context(), mux.Vars(r)["id"])
		if err != nil {
			log.Printf("resume err: %v", err)
			w.WriteHeader(500)
			return
		}
	})
	mr.HandleFunc("/status/{id}", func(w http.ResponseWriter, r *http.Request) {
		wf, err := engine.Get(r.Context(), mux.Vars(r)["id"])
		if err != nil {
			w.WriteHeader(400)
			fmt.Fprintf(w, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(wf)
	})
	mr.HandleFunc("/definition", func(w http.ResponseWriter, r *http.Request) {
		wf := PizzaOrderWorkflow{}
		w.Header().Set("Content-Type", "application/json")
		e := json.NewEncoder(w)
		e.SetIndent("", " ")
		_ = e.Encode(wf.Definition())
	})

	mr.HandleFunc("/event/{id}/{event}", SimpleEventHandler)

	err = http.ListenAndServe(":8080", mr)
	if err != nil {
		panic(err)
	}
}

var S = async.S
var If = async.If
var Switch = async.Switch
var Case = async.Case
var Step = async.Step
var For = async.For
var On = async.On
var Go = async.Go
var Wait = async.Wait
var WaitCond = async.WaitCond
var Return = async.Return
var Break = async.Break
