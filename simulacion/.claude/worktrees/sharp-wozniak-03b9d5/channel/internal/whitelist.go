package internal

// Whitelist de comandos permitidos (Axioma 7)
// Solo comandos hardcodeados. No hay wildcards, no hay escapes.
var allowedCommands = map[string]bool{
	// Utilidades de sistema
	"ls":        true,
	"cat":       true,
	"head":      true,
	"tail":      true,
	"wc":        true,
	"echo":      true,
	"date":      true,
	"whoami":    true,
	"pwd":       true,
	"mkdir":     true,
	"cp":        true,
	"stat":      true,
	"find":      true,
	"grep":      true,
	"sort":      true,
	"uniq":      true,
	"cut":       true,
	"tr":        true,
	"base64":    true,
	"md5sum":    true,
	"sha256sum": true,
	"realpath":  true,
	"readlink":  true,
	"sleep":     true,
	"seq":       true,
	"printf":    true,

	// Git (limitado)
	"git": true,

	// Go
	"go": true,

	// Docker (solo lectura)
	"docker": true,

	// Node/npm
	"node": true,
	"npm":  true,
	"npx":  true,

	// Python
	"python3": true,
	"pip3":    true,

	// Misc
	"curl":   true,
	"wget":   true,
	"tar":    true,
	"gzip":   true,
	"gunzip": true,

	// Binarios de frameworks que se invocan vía manifest.binary.command.
	// Cada uno vive en su propio framework-*/ con cwd correspondiente.
	// Convención: ./framework<nombre> (compilado). Channel ejecuta solo
	// estos paths exactos; los frameworks que se ejecutan vía `go run`
	// (echo, alfa) usan el comando "go" arriba y no necesitan entrada propia.
	"./frameworksabio":      true,
	"./frameworkindexa":     true,
	"./frameworkauditor":    true,
	"./frameworkmecanico":   true,
	"./frameworkhosting":    true,
	"./foco":                true,
	"./frameworkradar":      true,
	"./frameworkmensajero":  true,
	"./frameworkarquitecto": true,
	"./frameworkcritico":    true,
	"./frameworkpaladin":    true,
}

// DestructiveCommands son comandos que NUNCA se ejecutan (Axioma 4.5)
var destructiveCommands = map[string]bool{
	"rm":       true,
	"sudo":     true,
	"chmod":    true,
	"chown":    true,
	"dd":       true,
	"mkfs":     true,
	"mount":    true,
	"umount":   true,
	"reboot":   true,
	"shutdown": true,
	"halt":     true,
	"poweroff": true,
	"init":     true,
	"kill":     true,
	"killall":  true,
}

// IsCommandAllowed verifica si un comando está en la whitelist (Axioma 7)
func IsCommandAllowed(cmd string) bool {
	return allowedCommands[cmd]
}

// IsDestructiveCommand verifica si un comando es destructivo (Axioma 4.5)
func IsDestructiveCommand(cmd string) bool {
	return destructiveCommands[cmd]
}
