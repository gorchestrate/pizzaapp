package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/rs/cors"

	"cloud.google.com/go/firestore"
	"github.com/alecthomas/jsonschema"
	"github.com/gorchestrate/async"
	"github.com/gorilla/mux"
	cloudtasks "google.golang.org/api/cloudtasks/v2beta3"
)

var gTaskMgr *GTasksScheduler
var engine *FirestoreEngine

func main() {
	jsonschema.Version = ""
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
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"https://petstore.swagger.io"},
		AllowedMethods: []string{"GET"},
	})
	mr.Use(c.Handler)
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
	mr.HandleFunc("/swagger", func(w http.ResponseWriter, r *http.Request) {
		wf := PizzaOrderWorkflow{}
		definitions := map[string]interface{}{}
		endpoints := map[string]interface{}{}
		docs := map[string]interface{}{
			"definitions": definitions,
			"swagger":     "2.0",
			"info": map[string]interface{}{
				"title":   "Pizza Service",
				"version": "0.0.1",
			},
			"host":     "pizzaapp-ffs2ro4uxq-uc.a.run.app",
			"basePath": "/",
			"schemes":  []string{"https"},
			"paths":    endpoints,
		}
		var oErr error
		_, err := async.Walk(wf.Definition(), func(s async.Stmt) bool {
			switch x := s.(type) {
			case async.WaitEventsStmt:
				for _, v := range x.Cases {
					h, ok := v.Handler.(*ReflectEvent)
					if !ok {
						continue
					}
					in, out, err := h.Schemas()
					if err != nil {
						oErr = err
						panic(err)
					}
					for name, def := range in.Definitions {
						definitions[name] = def
					}
					for name, def := range out.Definitions {
						definitions[name] = def
					}
					endpoints["/event/{wfid}/"+v.Callback.Name] = map[string]interface{}{
						"post": map[string]interface{}{
							"consumes": []string{"application/json"},
							"produces": []string{"application/json"},
							"parameters": []map[string]interface{}{
								{
									"name":        "wfid",
									"in":          "path",
									"description": "workflow id",
									"required":    true,
									"type":        "string",
								},
								{
									"name":        "body",
									"in":          "body",
									"description": "event data",
									"required":    true,
									"schema": map[string]interface{}{
										"$ref": in.Ref,
									},
								},
							},
							"responses": map[string]interface{}{
								"200": map[string]interface{}{
									"description": "success",
									"schema": map[string]interface{}{
										"$ref": out.Ref,
									},
								},
							},
						},
					}
				}
			}
			return false
		})
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, err.Error())
			return
		}
		if oErr != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, oErr.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		e := json.NewEncoder(w)
		e.SetIndent("", " ")
		_ = e.Encode(docs)
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
