module github.com/gorchestrate/pizzaapp

go 1.16

replace github.com/gorchestrate/async => ../async

require (
	cloud.google.com/go/firestore v1.5.0
	github.com/gorchestrate/async v0.5.3
	github.com/gorilla/mux v1.8.0
	google.golang.org/api v0.47.0
)
