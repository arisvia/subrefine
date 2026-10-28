package parser

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/arisvia/subrefine/internal/core/model"
)

// ParseSubscription auto-detects subscription content (Base64 encoded payload,
// Clash YAML, sing-box JSON or a plain line-separated URI list) and returns
// the parsed proxy nodes.
func ParseSubscription(data []byte) ([]*model.ProxyNode, error) {
	return parseSubscription(data, 2)
}

func parseSubscription(data []byte, depth int) ([]*model.ProxyNode, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	// 1. sing-box JSON: full config ({"outbounds":[...]}) or bare outbound array.
	if trimmed[0] == '{' || trimmed[0] == '[' {
		if nodes, err := ParseSingbox(trimmed); err == nil && len(nodes) > 0 {
			return nodes, nil
		}
	}

	// 2. Clash / Mihomo YAML.
	if bytes.Contains(trimmed, []byte("proxies:")) {
		if nodes, err := ParseClash(trimmed); err == nil && len(nodes) > 0 {
			return nodes, nil
		}
	}

	// 3. Plain line-separated URI list.
	if nodes := parseLineByLineURIs(trimmed); len(nodes) > 0 {
		return nodes, nil
	}

	// 4. Base64-wrapped payload of any of the above.
	if depth > 0 {
		if decoded, err := decodeBase64Safe(string(trimmed)); err == nil {
			if !bytes.Equal(bytes.TrimSpace(decoded), trimmed) {
				if nodes, err := parseSubscription(decoded, depth-1); err == nil && len(nodes) > 0 {
					return nodes, nil
				}
			}
		}
	}

	// 5. Last resort: Clash YAML that does not carry a literal "proxies:" key
	// (e.g. a single-line flow mapping list).
	if nodes, err := ParseClash(trimmed); err == nil && len(nodes) > 0 {
		return nodes, nil
	}

	return nil, nil
}

func parseLineByLineURIs(content []byte) []*model.ProxyNode {
	var nodes []*model.ProxyNode
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
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
