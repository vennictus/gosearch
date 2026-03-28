// Package search provides filesystem traversal.
package search

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/vennictus/gosearch/internal/config"
	"github.com/vennictus/gosearch/internal/ignore"
)

// WalkFiles walks the filesystem and sends file paths to the jobs channel.
func WalkFiles(ctx context.Context, cfg config.Config, jobs chan<- string, stderr io.Writer, metrics *Metrics) error {
	visited := make(map[string]struct{})
	rootAbs, _ := filepath.Abs(cfg.RootPath)
	if cfg.FollowSymlinks {
		if resolved, err := filepath.EvalSymlinks(rootAbs); err == nil {
			visited[resolved] = struct{}{}
		}
	}
	return walkDirectory(ctx, cfg, cfg.RootPath, 0, nil, visited, jobs, stderr, metrics)
}

func walkDirectory(
	ctx context.Context,
	cfg config.Config,
	currentDir string,
	depth int,
	inheritedRules []ignore.Rule,
	visited map[string]struct{},
	jobs chan<- string,
	stderr io.Writer,
	metrics *Metrics,
) error {
	if cfg.MaxDepth >= 0 && depth > cfg.MaxDepth {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	rules, err := ignore.LoadRules(currentDir, inheritedRules)
	if err != nil {
		fmt.Fprintln(stderr, err)
	}

	entries, err := os.ReadDir(currentDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fullPath := filepath.Join(currentDir, entry.Name())
		entryType := entry.Type()
		isSymlink := entryType&os.ModeSymlink != 0
		isDir := entry.IsDir()

		if ignore.ShouldIgnore(cfg.DefaultIgnoreDirs, rules, fullPath, isDir) {
			continue
		}

		if isSymlink {
			if !cfg.FollowSymlinks {
				continue
			}
			targetInfo, statErr := os.Stat(fullPath)
			if statErr != nil {
				fmt.Fprintln(stderr, statErr)
				continue
			}
			isDir = targetInfo.IsDir()

			if ignore.ShouldIgnore(cfg.DefaultIgnoreDirs, rules, fullPath, isDir) {
				continue
			}
		}

		if isDir {
			if _, blocked := cfg.DefaultIgnoreDirs[strings.ToLower(entry.Name())]; blocked {
				continue
			}
			if isSymlink {
				resolved, resolveErr := filepath.EvalSymlinks(fullPath)
				if resolveErr != nil {
					fmt.Fprintln(stderr, resolveErr)
					continue
				}
				if _, seen := visited[resolved]; seen {
					continue
				}
				visited[resolved] = struct{}{}
			}
			if err := walkDirectory(ctx, cfg, fullPath, depth+1, rules, visited, jobs, stderr, metrics); err != nil {
				if errors.Is(err, context.Canceled) {
					return err
				}
			}
			continue
		}

		if len(cfg.Extensions) > 0 {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if _, ok := cfg.Extensions[ext]; !ok {
				continue
			}
		}

		if cfg.MaxSizeBytes > 0 {
			entryInfo, infoErr := entry.Info()
			if infoErr != nil {
				fmt.Fprintln(stderr, infoErr)
				continue
			}
			if entryInfo.Size() > cfg.MaxSizeBytes {
				continue
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case jobs <- fullPath:
			metrics.FilesEnqueued.Add(1)
		}
	}

	return nil
}
