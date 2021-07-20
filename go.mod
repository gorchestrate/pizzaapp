module github.com/gorchestrate/pizzaapp

go 1.16

replace github.com/gorchestrate/async => ../async

require (
	cloud.google.com/go/firestore v1.5.0
	github.com/alecthomas/jsonschema v0.0.0-20210526225647-edb03dcab7bc
	github.com/gorchestrate/async v0.9.8
	github.com/gorilla/mux v1.8.0
	github.com/xeipuuv/gojsonschema v1.2.0
	google.golang.org/api v0.50.0
)
