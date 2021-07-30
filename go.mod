module github.com/gorchestrate/pizzaapp

go 1.16

replace github.com/gorchestrate/async => ../async
replace github.com/gorchestrate/gasync => ../gasync

require (
	github.com/gorchestrate/async v0.10.2
	github.com/gorchestrate/gasync v0.1.5
)
