package pipeline

import (
	"fmt"
	"regexp"

	"github.com/arisvia/subrefine/internal/storage"
)

// knownTypes mirrors the operators applyPipeline dispatches on. A step outside
// this set is dropped at delivery time, so it must be rejected at save time
// rather than silently delivering unprocessed nodes.
var knownTypes = map[string]bool{
	"rename":      true,
	"filter":      true,
	"deduplicate": true,
	"decorate":    true,
	"sort":        true,
}

// ValidateSteps reports pipeline configuration that would silently do something
// other than what the operator wrote. Compilers are checked here because a bad
// regex is otherwise ignored at delivery time, which turns a typo into a
// subscription that leaks nodes it was supposed to hide.
func ValidateSteps(steps []storage.PipelineStep) error {
	for i, step := range steps {
		if !knownTypes[step.Type] {
			return fmt.Errorf("pipeline step %d: unknown operator %q (supported: rename, filter, deduplicate, decorate, sort)", i+1, step.Type)
		}
		at := func(err error) error {
			return fmt.Errorf("pipeline step %d (%s): %w", i+1, step.Type, err)
		}
		switch step.Type {
		case "rename":
			if err := compileIfSet(step.Pattern); err != nil {
				return at(fmt.Errorf("invalid pattern: %w", err))
			}
		case "filter":
			if err := compileIfSet(step.Include); err != nil {
				return at(fmt.Errorf("invalid include pattern: %w", err))
			}
			if err := compileIfSet(step.Exclude); err != nil {
				return at(fmt.Errorf("invalid exclude pattern: %w", err))
			}
			if err := validateTypes(step.Types); err != nil {
				return at(err)
			}
		case "sort":
			switch step.By {
			case "", "tag", "protocol":
			default:
				return at(fmt.Errorf("unknown sort key %q (supported: tag, protocol)", step.By))
			}
			switch step.Order {
			case "", "asc", "desc":
			default:
				return at(fmt.Errorf("unknown sort order %q (supported: asc, desc)", step.Order))
			}
		}
	}
	return nil
}

func compileIfSet(pattern string) error {
	if pattern == "" {
		return nil
	}
	_, err := regexp.Compile(pattern)
	return err
}

// validateTypes rejects a protocol allow-list that matches nothing. Unknown
// entries alone are tolerated (Filter drops them), but a list where every
// entry is unrecognizable would fail open and deliver every node.
func validateTypes(types []string) error {
	if len(types) == 0 {
		return nil
	}
	for _, t := range types {
		if _, ok := normalizeProtocol(t); ok {
			return nil
		}
	}
	return fmt.Errorf("no recognizable protocol in %v (accepted names: ss, vmess, vless, trojan, hysteria2, wireguard, ...)", types)
}
