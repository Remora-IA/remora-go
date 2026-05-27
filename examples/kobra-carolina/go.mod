module github.com/remora-go/examples/kobra-carolina

go 1.24.0

require (
	github.com/remora-go/framework-agent v0.0.0
	github.com/remora-go/framework-channels v0.0.0
	github.com/remora-go/framework-llm v0.0.0
	github.com/remora-go/framework-paladin v0.0.0
	github.com/remora-go/framework-store v0.0.0
)

replace (
	github.com/remora-go/framework-agent => ../../framework-agent
	github.com/remora-go/framework-channels => ../../framework-channels
	github.com/remora-go/framework-llm => ../../framework-llm
	github.com/remora-go/framework-paladin => ../../framework-paladin
	github.com/remora-go/framework-store => ../../framework-store
)
