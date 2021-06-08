package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gorchestrate/async"
	cloudtasks "google.golang.org/api/cloudtasks/v2beta3"
)

type CloudTaskManager struct {
	C           *cloudtasks.Service
	ProjectID   string
	LocationID  string
	QueueName   string
	ResumeURL   string
	CallbackURL string
}

func (mgr CloudTaskManager) SetResume(ctx context.Context, r async.ResumeRequest) error {
	body, err := json.Marshal(r)
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

func (mgr CloudTaskManager) SetTimeout(ctx context.Context, d time.Duration, r async.CallbackRequest) error {
	body, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	sTime := time.Now().Add(d).Format(time.RFC3339)
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
	if err != nil { // TODO: unlock on error
		return err
	}
	_, err = fs.DB.Collection(fs.Collection).Doc(id).Update(ctx,
		[]firestore.Update{
			{
				Path:  "LockTill",
				Value: 0,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("err unlocking workflow: %v", err)
	}
	return nil
}
