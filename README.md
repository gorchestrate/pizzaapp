# pizzaapp
Example app using Gorchestrate to manage pizza ordering

This shows how synchronous calls via Rest(UI) can be combined with gorchestrate async processing.

## FAQ

#### How does gorchestrate differs from other Workflow engines?
Gorchestrate separates workflow **execution and decision-making** from **communication**. 

With typical workflows engines - you use them to model your workflow logic and use code only to execute some actions. 

With Gorchstrate - you model and execute your workflow logic using code, and only use Gorchestrate for communication.


#### Is it possible to use languages other than Go to work with Gorchestrate?
**async** library is mostly syntatic sugar to model processes in a "Go way" without too much efforts. 

You can create **your** library for **your** programming language to model processes the way **you** want.
You may even integrate existing workflow engines to communicate with Gorchestrate.

#### Why we have to define callbacks for each step in our workflow? Can we implement workflows in a simpler way?
The main reason for **async** library API design is simplicity. FSM-based design allows people to adapt to framework quickly.

Another reason is maintenance. Having separated callbacks allows modifying and fixing workflow on the fly.

It's possible to use any other way of workflow definition - it's just **async** is a default gorchestrate framework to model workflows in Go.