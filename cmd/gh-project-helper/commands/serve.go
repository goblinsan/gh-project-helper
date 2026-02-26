package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/goblinsan/gh-project-helper/pkg/engine"
	"github.com/goblinsan/gh-project-helper/pkg/github"
	"github.com/goblinsan/gh-project-helper/pkg/types"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(serveCmd)
}

// JSON-RPC 2.0 types for MCP protocol
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP protocol types
type mcpInitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    mcpCapabilities `json:"capabilities"`
	ServerInfo      mcpServerInfo   `json:"serverInfo"`
}

type mcpCapabilities struct {
	Tools *struct{} `json:"tools,omitempty"`
}

type mcpServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type mcpToolsListResult struct {
	Tools []mcpToolDef `json:"tools"`
}

type mcpToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type mcpToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type mcpToolCallResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

var applyToolSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "project": {"type": "string", "description": "The GitHub Project V2 board title"},
    "repository": {"type": "string", "description": "Owner/repo (e.g. my-org/my-repo)"},
    "milestones": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "title": {"type": "string"},
          "due_on": {"type": "string"},
          "description": {"type": "string"}
        },
        "required": ["title"]
      }
    },
    "epics": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "title": {"type": "string"},
          "body": {"type": "string"},
          "milestone": {"type": "string"},
          "status": {"type": "string"},
          "labels": {"type": "array", "items": {"type": "string"}},
          "assignees": {"type": "array", "items": {"type": "string"}},
          "children": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "title": {"type": "string"},
                "body": {"type": "string"},
                "labels": {"type": "array", "items": {"type": "string"}}
              },
              "required": ["title"]
            }
          }
        },
        "required": ["title"]
      }
    }
  },
  "required": ["project", "repository"]
}`)

func handleMCPRequest(req jsonRPCRequest) jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpInitializeResult{
				ProtocolVersion: "2024-11-05",
				Capabilities:    mcpCapabilities{Tools: &struct{}{}},
				ServerInfo:      mcpServerInfo{Name: "gh-project-helper", Version: Version},
			},
		}

	case "notifications/initialized":
		// Client acknowledgment, no response needed (notification, no ID)
		return jsonRPCResponse{}

	case "tools/list":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpToolsListResult{
				Tools: []mcpToolDef{
					{
						Name:        "apply_project_plan",
						Description: "Takes a plan defining milestones, epics, and issues and creates them in a GitHub Project V2 board.",
						InputSchema: applyToolSchema,
					},
				},
			},
		}

	case "tools/call":
		return handleToolCall(req)

	default:
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

func handleToolCall(req jsonRPCRequest) jsonRPCResponse {
	var params mcpToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32602, Message: fmt.Sprintf("invalid params: %v", err)},
		}
	}

	if params.Name != "apply_project_plan" {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpToolCallResult{
				Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("unknown tool: %s", params.Name)}},
				IsError: true,
			},
		}
	}

	var plan types.Plan
	if err := json.Unmarshal(params.Arguments, &plan); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpToolCallResult{
				Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("failed to parse plan: %v", err)}},
				IsError: true,
			},
		}
	}

	client, err := github.NewClient()
	if err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpToolCallResult{
				Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("failed to create github client: %v", err)}},
				IsError: true,
			},
		}
	}

	report, err := engine.ApplyPlan(context.Background(), client, plan, engine.Options{})
	if err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpToolCallResult{
				Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("apply failed: %v", err)}},
				IsError: true,
			},
		}
	}

	reportJSON, _ := json.Marshal(report)
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: mcpToolCallResult{
			Content: []mcpContent{{Type: "text", Text: string(reportJSON)}},
		},
	}
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the MCP server over stdio",
	Long:  `Run the MCP server to allow AI agents (Claude, Gemini, etc.) to interact with the tool via the Model Context Protocol over stdin/stdout.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		scanner := bufio.NewScanner(os.Stdin)
		// Increase buffer for large plan payloads (1 MB)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
		encoder := json.NewEncoder(os.Stdout)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var req jsonRPCRequest
			if err := json.Unmarshal(line, &req); err != nil {
				resp := jsonRPCResponse{
					JSONRPC: "2.0",
					Error:   &jsonRPCError{Code: -32700, Message: fmt.Sprintf("parse error: %v", err)},
				}
				encoder.Encode(resp)
				continue
			}

			resp := handleMCPRequest(req)
			// Notifications (no ID) don't get a response
			if resp.JSONRPC == "" {
				continue
			}
			encoder.Encode(resp)
		}

		return scanner.Err()
	},
}
