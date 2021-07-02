package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gorchestrate/async"
	cloudtasks "google.golang.org/api/cloudtasks/v2beta3"
)

type Scheduler struct {
	engine     *Firestore
	C          *cloudtasks.Service
	Collection string
	ProjectID  string
	LocationID string
	QueueName  string
	ResumeURL  string
}

type ResumeRequest struct {
	ID string
}

func (s *Scheduler) ResumeHandler(w http.ResponseWriter, r *http.Request) {
	var req ResumeRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Printf("err: %v", err)
		return
	}
	err = s.engine.OnResume(r.Context(), req.ID)
	if err != nil {
		log.Printf("err: %v", err)
		w.WriteHeader(500)
		return
	}
}

func (mgr *Scheduler) Schedule(id string) error {
	body, err := json.Marshal(ResumeRequest{
		ID: id,
	})
	if err != nil {
		panic(err)
	}
	sTime := time.Now().Add(time.Millisecond * 100).Format(time.RFC3339)
	_, err = mgr.C.Projects.Locations.Queues.Tasks.Create(
		fmt.Sprintf("projects/%v/locations/%v/queues/%v",
			mgr.ProjectID, mgr.LocationID, mgr.QueueName),
		&cloudtasks.CreateTaskRequest{
			Task: &cloudtasks.Task{
				ScheduleTime: sTime,
				HttpRequest: &cloudtasks.HttpRequest{
					Url:        mgr.ResumeURL,
					HttpMethod: "POST",
					Body:       base64.StdEncoding.EncodeToString(body),
				},
			},
		}).Do()
	return err
}

type GTasksTimeoutMgr struct {
	engine      *Firestore
	C           *cloudtasks.Service
	Collection  string
	ProjectID   string
	LocationID  string
	QueueName   string
	CallbackURL string
}

func (mgr *GTasksTimeoutMgr) TimeoutHandler(w http.ResponseWriter, r *http.Request) {
	var req async.CallbackRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Printf("err: %v", err)
		return
	}
	// TODO: pass input to callback!?
	var input interface{}
	_, err = mgr.engine.OnCallback(r.Context(), req, input)
	if err != nil {
		log.Printf("err: %v", err)
		w.WriteHeader(500)
		return
	}
}

func (mgr *GTasksTimeoutMgr) Setup(req async.CallbackRequest) error {
	t, ok := req.Handler.(TimeoutHandler)
	if !ok {
		return fmt.Errorf("invalid timeout handler: %v", req)
	}
	body, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}
	sTime := time.Now().Add(t.Delay).Format(time.RFC3339)
	_, err = mgr.C.Projects.Locations.Queues.Tasks.Create(
		fmt.Sprintf("projects/%v/locations/%v/queues/%v",
			mgr.ProjectID, mgr.LocationID, mgr.QueueName),
		&cloudtasks.CreateTaskRequest{
			Task: &cloudtasks.Task{
				ScheduleTime: sTime,
				HttpRequest: &cloudtasks.HttpRequest{
					Url:        mgr.CallbackURL,
					HttpMethod: "POST",
					Body:       base64.StdEncoding.EncodeToString(body),
				},
			},
		}).Do()
	return err
}

func (t *GTasksTimeoutMgr) Teardown(req async.CallbackRequest) error {
	log.Printf("TODO: gtasks teardown, delete task")
	return nil
}

type Firestore struct {
	scheduler  Scheduler
	r          async.Runner
	DB         *firestore.Client
	Collection string
}

type DBWorkflow struct {
	Meta     async.Meta
	State    interface{} // json body of workflow state
	Input    interface{} // json input of the workflow
	Output   interface{} // json output of the finished workflow. Valid only if Status = Finished
	LockTill time.Time   // optimistic locking
}

func (fs Firestore) Lock(ctx context.Context, id string) (DBWorkflow, error) {
	for i := 0; ; i++ {
		doc, err := fs.DB.Collection(fs.Collection).Doc(id).Get(ctx)
		if err != nil {
			return DBWorkflow{}, err
		}
		var wf DBWorkflow
		err = doc.DataTo(&wf)
		if err != nil {
			return DBWorkflow{}, fmt.Errorf("err unmarshaling workflow: %v", err)
		}
		if time.Since(wf.LockTill) < 0 {
			if i > 50 {
				return DBWorkflow{}, fmt.Errorf("workflow is locked. can't unlock with 50 retries")
			} else {
				log.Printf("workflow is locked, waiting and trying again...")
				time.Sleep(time.Millisecond * 100 * time.Duration(i))
				continue
			}
		}
		_, err = fs.DB.Collection(fs.Collection).Doc(id).Update(ctx,
			[]firestore.Update{
				{
					Path:  "LockTill",
					Value: time.Now().Add(time.Minute),
				},
			},
			firestore.LastUpdateTime(doc.UpdateTime),
		)
		if err != nil && strings.Contains(err.Error(), "FailedPrecondition") {
			log.Printf("workflow was locked concurrently, waiting and trying again...")
			continue
		}
		if err != nil {
			return DBWorkflow{}, fmt.Errorf("err locking workflow: %v", err)
		}
		break
	}
	return DBWorkflow{}, fmt.Errorf("unexpexted")
}

func (fs Firestore) Unlock(ctx context.Context, id string) error {
	// always unlock, even if previous err != nil
	_, unlockErr := fs.DB.Collection(fs.Collection).Doc(id).Update(ctx,
		[]firestore.Update{
			{
				Path:  "LockTill",
				Value: time.Time{},
			},
		},
	)
	if unlockErr != nil {
		return fmt.Errorf("err unlocking workflow: %v", unlockErr)
	}
	return nil
}

func (fs Firestore) Checkpoint(ctx context.Context, wf DBWorkflow, s async.WorkflowState) func(bool) error {
	return func(resume bool) error {
		if resume {
			err := fs.scheduler.Schedule(wf.Meta.ID)
			if err != nil {
				return err
			}
		}
		_, err := fs.DB.Collection(fs.Collection).Doc(wf.Meta.ID).Update(ctx,
			[]firestore.Update{
				{
					Path:  "Meta",
					Value: wf.Meta,
				},
				{
					Path:  "State",
					Value: s,
				},
			},
		)
		return err
	}
}

func (fs Firestore) OnCallback(ctx context.Context, req async.CallbackRequest, input interface{}) (interface{}, error) {
	wf, err := fs.Lock(ctx, req.WorkflowID)
	if err != nil {
		return nil, err
	}
	state := &PizzaOrderWorkflow{}
	out, err := fs.r.OnCallback(ctx, req, state, &wf.Meta, input, fs.Checkpoint(ctx, wf, state))
	if err != nil {
		return out, fmt.Errorf("err during workflow processing: %v", err)
	}
	return out, fs.Unlock(ctx, req.WorkflowID)
}

func (fs Firestore) OnResume(ctx context.Context, id string) error {
	wf, err := fs.Lock(ctx, id)
	if err != nil {
		return err
	}
	state := &PizzaOrderWorkflow{}
	err = fs.r.OnResume(ctx, state, &wf.Meta, fs.Checkpoint(ctx, wf, state))
	if err != nil {
		return fmt.Errorf("err during workflow processing: %v", err)
	}
	return fs.Unlock(ctx, id)
}

// func (fs Firestore) Get(ctx context.Context, id string) (*async.State, error) {
// 	d, err := fs.DB.Collection(fs.Collection).Doc(id).Get(ctx)
// 	if err != nil {
// 		return nil, err
// 	}
// 	var s async.State
// 	err = d.DataTo(s)
// 	return &s, err
// }

// func (fs Firestore) ScheduleAndCreate(ctx context.Context, id string, s *async.State) error {
// 	err := fs.ScheduleExecution(id)
// 	if err != nil {
// 		return err
// 	}
// 	_, err = fs.DB.Collection(fs.Collection).Doc(id).Create(ctx, s)
// 	return err
// }
