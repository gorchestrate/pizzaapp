package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gorchestrate/async"
)

type FirestoreEngine struct {
	Scheduler  *GTasksScheduler
	DB         *firestore.Client
	Collection string
	Workflows  map[string]func() async.WorkflowState
}

type DBWorkflow struct {
	Meta     async.State
	State    interface{} // json body of workflow state
	LockTill time.Time   // optimistic locking
}

func (fs FirestoreEngine) Lock(ctx context.Context, id string) (DBWorkflow, error) {
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
		return wf, nil
	}
}

func (fs FirestoreEngine) Unlock(ctx context.Context, id string) error {
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

func (fs FirestoreEngine) Checkpoint(ctx context.Context, wf DBWorkflow, s async.WorkflowState) func(bool) error {
	return func(resume bool) error {
		if resume {
			err := fs.Scheduler.Schedule(ctx, wf.Meta.ID)
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

func (fs FirestoreEngine) HandleCallback(ctx context.Context, id string, cb async.CallbackRequest, input interface{}) (interface{}, error) {
	wf, err := fs.Lock(ctx, id)
	if err != nil {
		return nil, err
	}
	defer fs.Unlock(ctx, id)
	w, ok := fs.Workflows[wf.Meta.Workflow]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %v", err)
	}
	state := w()
	d, err := json.Marshal(wf.State)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(d, &state)
	if err != nil {
		return nil, err
	}
	out, err := async.HandleCallback(ctx, cb, state, &wf.Meta, input, fs.Checkpoint(ctx, wf, state))
	if err != nil {
		return out, fmt.Errorf("err during workflow processing: %v", err)
	}
	return out, nil
}
func (fs FirestoreEngine) HandleEvent(ctx context.Context, id string, name string, input interface{}) (interface{}, error) {
	wf, err := fs.Lock(ctx, id)
	if err != nil {
		return nil, err
	}
	defer fs.Unlock(ctx, id)
	w, ok := fs.Workflows[wf.Meta.Workflow]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %v", err)
	}
	state := w()
	d, err := json.Marshal(wf.State)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(d, &state)
	if err != nil {
		return nil, err
	}
	out, err := async.HandleEvent(ctx, name, state, &wf.Meta, input, fs.Checkpoint(ctx, wf, state))
	if err != nil {
		return out, fmt.Errorf("err during workflow processing: %v", err)
	}
	return out, nil
}

func (fs FirestoreEngine) Resume(ctx context.Context, id string) error {
	wf, err := fs.Lock(ctx, id)
	if err != nil {
		return err
	}
	defer fs.Unlock(ctx, id)
	w, ok := fs.Workflows[wf.Meta.Workflow]
	if !ok {
		return fmt.Errorf("workflow not found: %v", err)
	}
	state := w()
	d, err := json.Marshal(wf.State)
	if err != nil {
		return err
	}
	err = json.Unmarshal(d, &state)
	if err != nil {
		return err
	}
	err = async.Resume(ctx, state, &wf.Meta, fs.Checkpoint(ctx, wf, state))
	if err != nil {
		return fmt.Errorf("err during workflow processing: %v", err)
	}
	return nil
}

func (fs FirestoreEngine) Get(ctx context.Context, id string) (*DBWorkflow, error) {
	d, err := fs.DB.Collection(fs.Collection).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	var wf DBWorkflow
	err = d.DataTo(&wf)
	return &wf, err
}

func (fs FirestoreEngine) ScheduleAndCreate(ctx context.Context, id, name string, state interface{}) error {
	err := fs.Scheduler.Schedule(ctx, id)
	if err != nil {
		return err
	}
	wf := DBWorkflow{
		Meta:  async.NewState(id, name),
		State: state,
	}
	_, err = fs.DB.Collection(fs.Collection).Doc(id).Create(ctx, wf)
	return err
}
