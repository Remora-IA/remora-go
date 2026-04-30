module remora-flujo

go 1.24.0

require (
	channel v0.0.0
	github.com/gorilla/mux v1.8.1
	github.com/remora-go/framework-paladin v0.0.0
)

replace github.com/remora-go/framework-paladin => ../framework-paladin

replace channel => ../channel
