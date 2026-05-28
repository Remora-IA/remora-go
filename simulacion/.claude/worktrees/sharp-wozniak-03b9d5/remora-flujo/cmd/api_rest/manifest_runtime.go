package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"channel/manifest"
)

type manifestRuntime struct {
	Command     string `json:"command"`
	Cwd         string `json:"cwd"`
	Mode        string `json:"mode"`
	BuildOutput string `json:"build_output,omitempty"`
	Freshness   string `json:"freshness,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

func resolveManifestRuntime(rootDir string, m *manifest.Manifest) manifestRuntime {
	rt := manifestRuntime{}
	if m == nil {
		return rt
	}
	rt.Command = m.Binary.Command
	rt.Cwd = manifestRuntimeCwd(rootDir, m)
	rt.BuildOutput = manifestBuildOutput(m)
	rt.Mode = "manifest_binary"
	rt.Freshness = "unknown"

	if rt.Command == "" {
		rt.Reason = "binary.command vacío"
		return rt
	}
	if filepath.IsAbs(rt.Command) {
		rt.Freshness = "explicit"
		return rt
	}

	outPath := ""
	if rt.BuildOutput != "" {
		outPath = filepath.Join(rt.Cwd, rt.BuildOutput)
	}
	_, canGoRun := manifestBuildGoRunArgs(m)
	isGoRun := rt.Command == "go" && len(m.Binary.ArgsPrefix) > 0 && m.Binary.ArgsPrefix[0] == "run"

	switch {
	case isGoRun:
		if outPath == "" {
			rt.Mode = "go_run"
			rt.Freshness = "no_build_output"
			return rt
		}
		if binaryFresh, reason := manifestBuildOutputFresh(rt.Cwd, outPath); binaryFresh {
			rt.Command = "./" + filepath.Base(outPath)
			rt.Mode = "built_binary"
			rt.Freshness = "fresh"
			rt.Reason = reason
			return rt
		}
		rt.Mode = "go_run"
		rt.Freshness = "stale_or_missing"
		if canGoRun {
			rt.Reason = "build output ausente o stale; se usa go run"
			return rt
		}
		rt.Reason = "build output ausente o stale"
		return rt
	case strings.HasPrefix(rt.Command, "./") || strings.HasPrefix(rt.Command, "../"):
		if outPath == "" || filepath.Base(rt.Command) != filepath.Base(outPath) {
			if stat, err := os.Stat(filepath.Join(rt.Cwd, rt.Command)); err == nil && !stat.IsDir() {
				rt.Freshness = "implicit"
			}
			return rt
		}
		if binaryFresh, reason := manifestBuildOutputFresh(rt.Cwd, outPath); binaryFresh {
			rt.Freshness = "fresh"
			rt.Reason = reason
			return rt
		}
		if canGoRun {
			rt.Command = "go"
			rt.Mode = "go_run_fallback"
			rt.Freshness = "stale_or_missing"
			rt.Reason = "build output ausente o stale; se usa go run"
			return rt
		}
		rt.Freshness = "stale_or_missing"
		rt.Reason = "build output ausente o stale"
		return rt
	default:
		rt.Freshness = "explicit"
		return rt
	}
}

func (r manifestRuntime) FullArgs(commandArgs []string, m *manifest.Manifest) []string {
	prefix := append([]string{}, manifestRuntimeArgsPrefix(r, m)...)
	prefix = append(prefix, commandArgs...)
	return prefix
}

func manifestRuntimeArgsPrefix(r manifestRuntime, m *manifest.Manifest) []string {
	if m == nil {
		return nil
	}
	if r.Mode == "built_binary" && m.Binary.Command == "go" {
		return nil
	}
	if r.Command == "go" {
		if goRunArgs, ok := manifestBuildGoRunArgs(m); ok && (r.Mode == "go_run" || r.Mode == "go_run_fallback") {
			return goRunArgs
		}
	}
	return append([]string{}, m.Binary.ArgsPrefix...)
}

func manifestRuntimeCwd(rootDir string, m *manifest.Manifest) string {
	cwdRel := ""
	if m != nil {
		cwdRel = m.Cwd
		if cwdRel == "" {
			cwdRel = "framework-" + m.Name
		}
	}
	return filepath.Join(rootDir, cwdRel)
}

func manifestBuildOutput(m *manifest.Manifest) string {
	if m == nil {
		return ""
	}
	for i, arg := range m.Build.Args {
		if arg == "-o" && i+1 < len(m.Build.Args) {
			return m.Build.Args[i+1]
		}
	}
	return ""
}

func manifestBuildGoRunArgs(m *manifest.Manifest) ([]string, bool) {
	if m == nil || m.Build.Command != "go" || len(m.Build.Args) == 0 || m.Build.Args[0] != "build" {
		return nil, false
	}
	args := []string{"run"}
	packageArgs := []string{}
	for i := 1; i < len(m.Build.Args); i++ {
		arg := m.Build.Args[i]
		if arg == "-o" && i+1 < len(m.Build.Args) {
			i++
			continue
		}
		if strings.HasPrefix(arg, "-o=") {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			args = append(args, arg)
			continue
		}
		packageArgs = append(packageArgs, arg)
	}
	if len(packageArgs) == 0 {
		return nil, false
	}
	args = append(args, packageArgs...)
	return args, true
}

func manifestBuildOutputFresh(cwd, outPath string) (bool, string) {
	info, err := os.Stat(outPath)
	if err != nil || info.IsDir() {
		return false, "build output ausente"
	}
	sourceMod, ok := manifestRuntimeSourceModTime(cwd)
	if !ok {
		return true, "sin fuentes observables más nuevas"
	}
	if info.ModTime().Before(sourceMod) {
		return false, "fuentes más nuevas que el binario"
	}
	return true, "binario alineado con las fuentes"
}

func manifestRuntimeSourceModTime(cwd string) (time.Time, bool) {
	latest := time.Time{}
	seen := false
	candidates := []string{
		filepath.Join(cwd, "framework.manifest.json"),
		filepath.Join(cwd, "go.mod"),
		filepath.Join(cwd, "go.sum"),
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if !seen || info.ModTime().After(latest) {
				latest = info.ModTime()
				seen = true
			}
		}
	}
	for _, dirName := range []string{"cmd", "internal"} {
		root := filepath.Join(cwd, dirName)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			if !seen || info.ModTime().After(latest) {
				latest = info.ModTime()
				seen = true
			}
			return nil
		})
	}
	matches, _ := filepath.Glob(filepath.Join(cwd, "*.go"))
	for _, path := range matches {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if !seen || info.ModTime().After(latest) {
				latest = info.ModTime()
				seen = true
			}
		}
	}
	return latest, seen
}
