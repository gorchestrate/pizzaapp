# pizzaapp
Example app using Gorchestrate to manage pizza ordering

## Usage
Run service in a separate terminal:
```
go run .
```

Create new PizzaOrderProcess
```
curl -X POST http://localhost:8080/order/1 -d '{"ManagerEmail": "YourEmailAddress@gmail.com","Phone": "+12345678","Pizzas": [{"Name":"Pepperoni", "Size":1}]}'
```

Check Process status:
```
curl -X GET http://localhost:8080/order/1
{"Output":null,"Status":"Running"}
```

You will receive mail that is asking to approve your order (it may go into Spam). You can either approve it by replying with "Approve" or reject by typing "Reject" or you can cancel order by calling
```
curl -X POST http://localhost:8080/order/1/cancel
curl -X GET http://localhost:8080/order/1
{"Output":{"Order":{"Pizzas":[{"Name":"Pepperoni","Size":1}],"Phone":"+375298468489","ManagerEmail":"artem.gladkikh@idt.net"},"Approved":false,"Message":"order was canceled by user"},"Status":"Finished"}
```

## FAQ

#### How does gorchestrate differs from other Workflow engines?
Gorchestrate separates workflow **execution and decision-making** from **communication**. 

With typical workflows engines - you use them to model your workflow logic and use code only to execute some actions. 

With Gorchstrate - you model and execute your workflow logic using code, and only use Gorchestrate for communication.


#### What are guarantees related to process execution?
* **Linearized consistency** for all communications between all processes within single Gorchestrate Core instance.
* **Strong consistency** for all write operations. Any read following write will return up-to-date data.
* **Atomic** update to process. All Recv/Send/Call operations are either succeed or fail.
* **Exactly Once** workflow execution semantics. Callbacks are called in the order they were triggered. Only one callback gets unblocked in a single point of time (except callback is taking longer time for execution than expected)

#### Is it possible to use languages other than Go to work with Gorchestrate?
**async** library is mostly syntatic sugar to model processes in a "Go way" without too much efforts. 

You can create **your** library for **your** programming language to model processes the way **you** want.
You may even integrate existing workflow engines to communicate with Gorchestrate.

#### What performance I can expect from Gorchestrate?
Throughtput:
* With small size of workflows state you can expect continious >10,000 req/sec performance on a typical server
* Bigger workflows states make API thoughput go down propotionally to their size

Latency:
* Latencies between event happening and callback being called are 10-20ms
* Increasing throughput should not increase latency up to a certain point. After that latencies will go up, limited by max thoughput server can sustain.

Stability:
* Max throughput decreases with database size. From latest benchmarks it drops 2x after 1 billion of records written.

#### Why we have to define callbacks for each step in our workflow? Can we implement workflows in a simpler way?
The main reason for **async** library API design is simplicity. FSM-based design allows people to adapt to framework quickly.

Another reason is maintenance. Having separated callbacks allows modifying and fixing workflow on the fly.

It's possible to use any other way of workflow definition - it's just **async** is a default gorchestrate framework to model workflows in Go.
