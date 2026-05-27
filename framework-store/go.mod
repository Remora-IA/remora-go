module github.com/remora-go/framework-store

go 1.24

require (
	github.com/remora-go/framework-agent v0.0.0
	github.com/remora-go/framework-llm v0.0.0
	github.com/remora-go/framework-paladin v0.0.0
)

replace (
	github.com/remora-go/framework-agent => ../framework-agent
	github.com/remora-go/framework-llm => ../framework-llm
	github.com/remora-go/framework-paladin => ../framework-paladin
)
