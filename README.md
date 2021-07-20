# pizzaapp
Example app using Gorchestrate to manage pizza ordering

Deployed at https://pizzaapp-ffs2ro4uxq-uc.a.run.app


## Usage

Create Workflow
	https://pizzaapp-ffs2ro4uxq-uc.a.run.app/new/{id}

Check Workflow status
https://pizzaapp-ffs2ro4uxq-uc.a.run.app/status/{id}

Send Events
https://pizzaapp-ffs2ro4uxq-uc.a.run.app/event/{id}/{event}


```
export wfid={FILL IN YOUR ID}
curl -X POST -d '{}' https://pizzaapp-ffs2ro4uxq-uc.a.run.app/new/$wfid
curl -X POST -d '{}' https://pizzaapp-ffs2ro4uxq-uc.a.run.app/event/$wfid/clean
curl -X POST -d '{"Name":"pepperoni","Qty":2}' https://pizzaapp-ffs2ro4uxq-uc.a.run.app/event/$wfid/add
curl -X POST -d '{}' https://pizzaapp-ffs2ro4uxq-uc.a.run.app/event/$wfid/submit
curl -X POST -d '{}' https://pizzaapp-ffs2ro4uxq-uc.a.run.app/event/$wfid/confirm
curl -X POST -d '{}' https://pizzaapp-ffs2ro4uxq-uc.a.run.app/event/$wfid/confirm_payment
curl -X POST -d '{"CookName":"Chef"}' https://pizzaapp-ffs2ro4uxq-uc.a.run.app/event/$wfid/start_cooking
curl -X POST -d '{}' https://pizzaapp-ffs2ro4uxq-uc.a.run.app/event/$wfid/cooked
curl -X POST -d '{}' https://pizzaapp-ffs2ro4uxq-uc.a.run.app/event/$wfid/take_for_delivery
curl -X POST -d '{}' https://pizzaapp-ffs2ro4uxq-uc.a.run.app/event/$wfid/delivered

```







### Definition
```go
func (wf *PizzaOrderWorkflow) Definition() async.Section {
	return S(
		Step("init", func() error {
			wf.Cart = []Pizza{}
			wf.Status = "started"
			return nil
		}),

		For(true, "order not yet submitted",
			Wait("wait for user input",
				On("24h passsed", gTaskMgr.Timeout(24*3600*time.Second),
					Step("cart timed out", func() error {
						wf.Status = "timed out"
						return nil
					}),
					Return(), //stop workflow
				),
				Event("add", func(in Pizza) (PizzaOrderWorkflow, error) {
					wf.Cart = append(wf.Cart, in)
					return *wf, nil
				}),
				Event("clean", func(in Pizza) (PizzaOrderWorkflow, error) {
					wf.Cart = []Pizza{}
					return *wf, nil
				}),
				Event("submit", func(in Empty) (PizzaOrderWorkflow, error) {
					wf.Status = "submitted"
					return *wf, nil
				}, Break()),
			),
		),

		Wait("manager confirms order",
			On("10min passsed", gTaskMgr.Timeout(10*60*time.Second),
				Step("manager didn't confirm", func() error {
					wf.Status = "manager is sleeping"
					log.Printf("notify user that order won't be processed because manager did not confirm order in time")
					return nil
				}),
				Return(), //stop workflow
			),
			Event("confirm", func(in Empty) (PizzaOrderWorkflow, error) {
				wf.Status = "confirmed"
				return *wf, nil
			}),
		),

		Go("customer pays while order is cooking", S(
			Wait("customer pays money",
				Event("confirm_payment", func(in Empty) (PizzaOrderWorkflow, error) {
					wf.Paid = true
					return *wf, nil
				}),
			),
		)),

		Wait("kitchen takes order",
			On("30min passsed", gTaskMgr.Timeout(30*60*time.Second),
				Step("kitchen didn't confirm", func() error {
					wf.Status = "kitchen is sleeping"
					log.Printf("notify user that order won't be processed because kitchen is sleeping")
					return nil
				}),
				Return(), //stop workflow
			),
			Event("start_cooking", func(in CookingRecord) (PizzaOrderWorkflow, error) {
				wf.Status = "cooking"
				wf.CookName = in.CookName
				return *wf, nil
			}),
		),

		Wait("pizzas cooked",
			On("1h cook timeout", gTaskMgr.Timeout(60*60*time.Second),
				Step("kitchen didn't cook in time", func() error {
					wf.Status = "kitchen cooking is not done"
					log.Printf("notify user that order won't be processed because kitchen can't cook his pizza")
					return nil
				}),
				Return(), //stop workflow
			),
			Event("cooked", func(in Empty) (PizzaOrderWorkflow, error) {
				wf.Status = "cooked"
				return *wf, nil
			}),
		),

		Wait("taken for delivery",
			On("1h to take timeout", gTaskMgr.Timeout(60*60*time.Second),
				Step("delivery forgot about this order", func() error {
					wf.Status = "delivery is not done"
					log.Printf("notify user that order won't be processed because delivery can't be done")
					return nil
				}),
				Return(), //stop workflow
			),
			Event("take_for_delivery", func(in Empty) (PizzaOrderWorkflow, error) {
				wf.Status = "delivering"
				return *wf, nil
			}),
		),
		Wait("for delivered",
			On("1h delivery timeout", gTaskMgr.Timeout(60*60*time.Second),
				Step("delivery lost on the road", func() error {
					wf.Status = "delivery is lost"
					log.Printf("notify user that order won't be processed because delivery was lost on a road")
					return nil
				}),
				Return(), //stop workflow
			),
			Event("delivered", func(in Empty) (PizzaOrderWorkflow, error) {
				wf.Status = "delivered"
				return *wf, nil
			}),
		),
		WaitCond(wf.Paid, "wait for payment", func() {
			wf.Status = "completed"
		}),
	)
}
```