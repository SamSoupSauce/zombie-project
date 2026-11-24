package paths

import (
	"os"
	"path/filepath"
)

// EnsureProjectRoot detects the project root directory (containing maps/ and scripts/)
// and changes the process current working directory to it if necessary.
func EnsureProjectRoot() string {
	if hasAssets(".") {
		if abs, err := filepath.Abs("."); err == nil {
			return abs
		}
		return "."
	}

	// 1. Check directory containing executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if hasAssets(exeDir) {
			_ = os.Chdir(exeDir)
			return exeDir
		}
		parent := filepath.Dir(exeDir)
		if hasAssets(parent) {
			_ = os.Chdir(parent)
			return parent
		}
	}

	// 2. Check parent directories of CWD
	if cwd, err := os.Getwd(); err == nil {
		curr := cwd
		for i := 0; i < 4; i++ {
			parent := filepath.Dir(curr)
			if parent == curr {
				break
			}
			if hasAssets(parent) {
				_ = os.Chdir(parent)
				return parent
			}
			curr = parent
		}
	}

	return "."
}

func hasAssets(dir string) bool {
	_, errMaps := os.Stat(filepath.Join(dir, "maps"))
	_, errScripts := os.Stat(filepath.Join(dir, "scripts"))
	return errMaps == nil && errScripts == nil
}

// Resolve resolves a relative asset path by checking CWD, executable dir, and parent dirs.
func Resolve(relPath string) string {
	if relPath == "" {
		return ""
	}
	if filepath.IsAbs(relPath) {
		return relPath
	}
	if _, err := os.Stat(relPath); err == nil {
		return relPath
	}

	var candidates []string
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, relPath),
			filepath.Join(exeDir, "..", relPath),
		)
	}

	candidates = append(candidates,
		filepath.Join("..", relPath),
		filepath.Join("..", "..", relPath),
	)

	for _, cand := range candidates {
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}

	return relPath
}
