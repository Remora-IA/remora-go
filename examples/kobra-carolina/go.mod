module github.com/Remora-IA/remora-go/examples/kobra-carolina

go 1.24.0

require (
	github.com/Remora-IA/remora-go/framework-agent v0.0.0-00010101000000-000000000000
	github.com/Remora-IA/remora-go/framework-channels v0.0.0-00010101000000-000000000000
	github.com/Remora-IA/remora-go/framework-llm v0.0.0-00010101000000-000000000000
	github.com/Remora-IA/remora-go/framework-paladin v0.0.0-00010101000000-000000000000
	github.com/Remora-IA/remora-go/framework-store v0.0.0-00010101000000-000000000000
)

replace (
	github.com/Remora-IA/remora-go/framework-agent => ../../framework-agent
	github.com/Remora-IA/remora-go/framework-channels => ../../framework-channels
	github.com/Remora-IA/remora-go/framework-llm => ../../framework-llm
	github.com/Remora-IA/remora-go/framework-paladin => ../../framework-paladin
	github.com/Remora-IA/remora-go/framework-store => ../../framework-store
)
