module github.com/Remora-IA/remora-go/framework-agent

go 1.24

require (
	github.com/Remora-IA/remora-go/framework-llm v0.0.0-00010101000000-000000000000
	github.com/Remora-IA/remora-go/framework-paladin v0.0.0-00010101000000-000000000000
)

replace (
	github.com/Remora-IA/remora-go/framework-llm => ../framework-llm
	github.com/Remora-IA/remora-go/framework-paladin => ../framework-paladin
)
