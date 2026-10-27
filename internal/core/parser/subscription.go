package parser

import (
	"bufio"
	"bytes"
	"strings"

	"subrefine/internal/core/model"
)

// ParseSubscription auto-detects subscription content (Base64 list, Clash YAML, sing-box JSON)
// and returns parsed proxy nodes.
func ParseSubscription(data []byte) ([]*model.ProxyNode, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	// 1. Check if it's sing-box JSON (starts with '{' or '[')
	if (trimmed[0] == '{' || trimmed[0] == '[') && jsonLikely(trimmed) {
		nodes, err := ParseSingbox(trimmed)
		if err == nil && len(nodes) > 0 {
			return nodes, nil
		}
	}

	// 2. Check if it's Clash YAML (contains "proxies:" or starts with YAML mapping)
	if bytes.Contains(trimmed, []byte("proxies:")) {
		nodes, err := ParseClash(trimmed)
		if err == nil && len(nodes) > 0 {
			return nodes, nil
		}
	}

	// 3. Try Base64 decoding (standard Base64 URI list)
	decoded, err := decodeBase64Safe(string(trimmed))
	if err == nil && len(decoded) > 0 {
		nodes := parseLineByLineURIs(decoded)
		if len(nodes) > 0 {
			return nodes, nil
		}
		// If base64 decoded was YAML or JSON:
		if bytes.Contains(decoded, []byte("proxies:")) {
			if clashNodes, err := ParseClash(decoded); err == nil && len(clashNodes) > 0 {
				return clashNodes, nil
			}
		}
	}

	// 4. Try raw URI list line-by-line
	nodes := parseLineByLineURIs(trimmed)
	if len(nodes) > 0 {
		return nodes, nil
	}

	// 5. Fallback attempt for Clash YAML without proxies key check
	if clashNodes, err := ParseClash(trimmed); err == nil && len(clashNodes) > 0 {
		return clashNodes, nil
	}

	return nil, nil
}

func parseLineByLineURIs(content []byte) []*model.ProxyNode {
	var nodes []*model.ProxyNode
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		node, err := ParseURI(line)
		if err == nil && node != nil {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func jsonLikely(data []byte) bool {
	return bytes.Contains(data, []byte(`"outbounds"`)) || bytes.Contains(data, []byte(`"type"`))
}
