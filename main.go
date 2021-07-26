package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/awalterschulze/gographviz"
	"github.com/goccy/go-graphviz"
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
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
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

	mr.HandleFunc("/graph", func(w http.ResponseWriter, r *http.Request) {
		wf := PizzaOrderWorkflow{}
		g := Grapher{}
		def := g.Dot(wf.Definition())
		gv := graphviz.New()
		gd, err := graphviz.ParseBytes([]byte(def))
		if err != nil {
			fmt.Fprintf(w, " %v \n %v", def, err)
			return
		}
		w.Header().Add("Content-Type", "image/jpg")
		gv.Render(gd, graphviz.JPG, w)
	})

	mr.HandleFunc("/swagger", func(w http.ResponseWriter, r *http.Request) {
		wf := PizzaOrderWorkflow{}
		definitions := map[string]interface{}{}
		endpoints := map[string]interface{}{
			"/new/{id}": map[string]interface{}{
				"post": map[string]interface{}{
					"consumes": []string{"application/json"},
					"produces": []string{"application/json"},
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"description": "workflow id",
							"required":    true,
							"type":        "string",
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "success",
						},
					},
				},
			},
		}
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
					endpoints["/event/{id}/"+v.Callback.Name] = map[string]interface{}{
						"post": map[string]interface{}{
							"consumes": []string{"application/json"},
							"produces": []string{"application/json"},
							"parameters": []map[string]interface{}{
								{
									"name":        "id",
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

type Grapher struct {
	g *gographviz.Graph
}

func (g *Grapher) Dot(s async.Stmt) string {
	g.g = gographviz.NewGraph()
	g.g.Directed = true
	ctx := GraphCtx{}
	start := ctx.node(g, "start", "circle")
	end := ctx.node(g, "end", "circle")
	ctx.Prev = []string{start}
	octx := g.Walk(s, ctx)
	g.AddEdges(octx.Prev, end)
	return g.g.String()
}

func (g *Grapher) AddEdges(from []string, to string) {
	for _, v := range from {
		_ = g.g.AddEdge(v, to, true, nil)
	}
}

func (g *Grapher) AddEdge(from string, to string) {
	if from == "" || to == "" {
		return
	}
	_ = g.g.AddEdge(from, to, true, nil)
}

type GraphCtx struct {
	Parent string
	Prev   []string
	Break  []string
}

var ncount int

func (ctx *GraphCtx) node(g *Grapher, name string, shape string) string {
	ncount++
	id := fmt.Sprint(ncount)
	_ = g.g.AddNode("", id, map[string]string{
		"label": strconv.Quote(name),
		"shape": shape,
	})
	return id
}

func (g *Grapher) Walk(s async.Stmt, ctx GraphCtx) GraphCtx {
	switch x := s.(type) {
	case nil:
		return GraphCtx{}
	case async.ReturnStmt:
		n := ctx.node(g, "end", "circle")
		g.AddEdges(ctx.Prev, n)
		return GraphCtx{}
	case async.BreakStmt:
		return GraphCtx{Break: ctx.Prev}
	case async.ContinueStmt:
		return GraphCtx{}
	case async.StmtStep:
		id := ctx.node(g, x.Name+"  ", "box")
		g.AddEdges(ctx.Prev, id)
		return GraphCtx{Prev: []string{id}}
	case async.WaitCondStmt:
		id := ctx.node(g, "wait for "+x.Name, "hexagon")
		g.AddEdges(ctx.Prev, id)
		return GraphCtx{Prev: []string{id}}
	case async.WaitEventsStmt:
		id := ctx.node(g, "wait "+x.Name, "hexagon")
		g.AddEdges(ctx.Prev, id)
		prev := []string{}
		breaks := []string{}
		for _, v := range x.Cases {
			cid := ctx.node(g, v.Callback.Name+"  ", "component")
			_ = g.g.AddEdge(id, cid, true, nil)
			octx := g.Walk(v.Stmt, GraphCtx{
				Prev: []string{cid},
			})
			prev = append(prev, octx.Prev...)
			breaks = append(breaks, octx.Break...)
		}
		return GraphCtx{Prev: prev}
	case *async.GoStmt:
		id := ctx.node(g, x.Name, "ellipse")

		for _, v := range ctx.Prev {
			_ = g.g.AddEdge(v, id, true, map[string]string{
				"style": "dashed",
				"label": "parallel",
			})
		}
		//g.AddEdges(octx.Prev, "end")
		_ = g.Walk(x.Stmt, GraphCtx{Prev: []string{id}})
		//pend := ctx.node(g, "end parallel", "circle")
		//g.AddEdges(octx.Prev, pend)
		return GraphCtx{Prev: ctx.Prev}
	case async.ForStmt:
		id := ctx.node(g, "while "+x.Name, "hexagon")
		g.AddEdges(ctx.Prev, id)
		breaks := []string{}
		curCtx := GraphCtx{Prev: []string{id}}
		for _, v := range x.Section {
			curCtx = g.Walk(v, GraphCtx{
				Prev:   curCtx.Prev,
				Parent: "sub",
			})
			breaks = append(breaks, curCtx.Break...)
		}
		g.AddEdges(curCtx.Prev, id)
		return GraphCtx{Prev: append(breaks, id)}
	case *async.SwitchStmt:
		for _, v := range x.Cases {
			g.Walk(v.Stmt, ctx)
		}
		return GraphCtx{}
	case async.Section:
		curCtx := ctx
		breaks := []string{}
		for _, v := range x {
			curCtx = g.Walk(v, GraphCtx{
				Prev:   curCtx.Prev,
				Parent: ctx.Parent,
			})
			breaks = append(breaks, curCtx.Break...)
		}
		return GraphCtx{Prev: curCtx.Prev, Break: breaks}
	default:
		panic(reflect.TypeOf(s))
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
var WaitFor = async.WaitFor
var Return = async.Return
var Break = async.Break
