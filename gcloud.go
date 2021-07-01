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
	"google.golang.org/api/cloudtasks/v2"
)

type CloudTasksResumer struct {
	r          *async.Runner
	C          *cloudtasks.Service
	ProjectID  string
	LocationID string
	QueueName  string
	ResumeURL  string
}

type ResumeRequest struct {
	ID string
}

func (mgr *CloudTasksResumer) ResumeHandler(w http.ResponseWriter, r *http.Request) {
	var req ResumeRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Printf("err: %v", err)
		return
	}
	err = mgr.r.Resume(r.Context(), time.Second*10, 10000, req.ID)
	if err != nil {
		log.Printf("err: %v", err)
		w.WriteHeader(500)
		return
	}
}

func (mgr *CloudTasksResumer) ScheduleResume(r *async.Runner, id string) error {
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
	r           *async.Runner
	C           *cloudtasks.Service
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
	err = mgr.r.OnCallback(r.Context(), req)
	if err != nil {
		log.Printf("err: %v", err)
		w.WriteHeader(500)
		return
	}
}

func (mgr *GTasksTimeoutMgr) Setup(req async.CallbackRequest) error {
	var dur time.Duration
	err := json.Unmarshal(req.Data, &dur)
	if err != nil {
		return err
	}
	body, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}
	sTime := time.Now().Add(dur).Format(time.RFC3339)
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

type FirestoreStorage struct {
	Collection string
	DB         *firestore.Client
}

func (fs FirestoreStorage) Get(ctx context.Context, id string) (*async.State, error) {
	d, err := fs.DB.Collection(fs.Collection).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	var s async.State
	err = d.DataTo(s)
	return &s, err
}

func (fs FirestoreStorage) Create(ctx context.Context, id string, s *async.State) error {
	_, err := fs.DB.Collection(fs.Collection).Doc(id).Create(ctx, s)
	return err
}

func (fs FirestoreStorage) RunLocked(ctx context.Context, id string, f async.UpdaterFunc) error {
	var err error
	var doc *firestore.DocumentSnapshot
	var s async.State
	for i := 0; ; i++ {
		doc, err = fs.DB.Collection(fs.Collection).Doc(id).Get(ctx)
		if err != nil {
			return err
		}
		err = doc.DataTo(&s)
		if err != nil {
			return fmt.Errorf("err unmarshaling workflow: %v", err)
		}
		if time.Since(s.LockTill) < 0 {
			if i > 50 {
				return fmt.Errorf("workflow is locked. can't unlock with 50 retries")
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
			return fmt.Errorf("err locking workflow: %v", err)
		}
		break
	}

	err = f(ctx, &s, func() error {
		_, err = fs.DB.Collection(fs.Collection).Doc(id).Update(ctx,
			[]firestore.Update{
				{
					Path:  "State",
					Value: s.State,
				},
				{
					Path:  "Status",
					Value: s.Status,
				},
				{
					Path:  "Output",
					Value: s.Output,
				},
				{
					Path:  "Threads",
					Value: s.Threads,
				},
				{
					Path:  "PC",
					Value: s.PC,
				},
			},
		)
		return err
	})

	// always unlock, even if previous err != nil
	_, unlockErr := fs.DB.Collection(fs.Collection).Doc(id).Update(ctx,
		[]firestore.Update{
			{
				Path:  "LockTill",
				Value: time.Time{},
			},
		},
	)
	if err != nil {
		return fmt.Errorf("err during workflow processing: %v", err)
	}
	if unlockErr != nil {
		return fmt.Errorf("err unlocking workflow: %v", err)
	}
	return nil
}
