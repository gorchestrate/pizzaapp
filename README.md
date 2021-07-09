# pizzaapp
Example app using Gorchestrate to manage pizza ordering

Deployed at https://pizzaapp-ffs2ro4uxq-uc.a.run.app


## Usage

Create Workflow
https://pizzaapp-ffs2ro4uxq-uc.a.run.app/new/{id}

Check Workflow status
https://pizzaapp-ffs2ro4uxq-uc.a.run.app/status/{id}

Send Events 
https://pizzaapp-ffs2ro4uxq-uc.a.run.app/simpleevent/{id}/{event}


### Definition
```Go
func (wf *PizzaOrderWorkflow) Definition() async.Section {
	return S(
		Step("step1", func() error {
			wf.Status = "started"
			return nil
		}),
		Wait("wait for event 20 seconds",
			On("20 seconds passsed", gTaskMgr.Timeout(time.Second*20),
				Step("set timed out", func() error {
					wf.Status = "timed out"
					return nil
				}),
			),
			On("myEvent", &SimpleEvent{Handler: func(body map[string]string) (map[string]string, error) {
				body["processed"] = "true"
				wf.Body = body
				wf.Status = "got event"
				return body, nil
			}}),
		),
		Wait("wait some time", On("10 seconds passed", gTaskMgr.Timeout(time.Second*10))),
		Step("step2", func() error {
			wf.Status = "finished"
			return nil
		}),
		async.Return(),
	)
}
```