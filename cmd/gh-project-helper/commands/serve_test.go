package commands

import (
	"encoding/json"
	"testing"
)

func TestHandleMCPRequest_Initialize(t *testing.T) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
	}
	resp := handleMCPRequest(req)

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}
	if resp.Error != nil {
		t.Errorf("expected no error, got %v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}

	result, ok := resp.Result.(mcpInitializeResult)
	if !ok {
		t.Fatalf("expected mcpInitializeResult, got %T", resp.Result)
	}
	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("expected protocol version 2024-11-05, got %s", result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "gh-project-helper" {
		t.Errorf("expected server name gh-project-helper, got %s", result.ServerInfo.Name)
	}
	if result.Capabilities.Tools == nil {
		t.Error("expected tools capability to be non-nil")
	}
}

func TestHandleMCPRequest_Initialized(t *testing.T) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	resp := handleMCPRequest(req)

	// Notifications should return empty response (no JSONRPC set)
	if resp.JSONRPC != "" {
		t.Errorf("expected empty jsonrpc for notification, got %s", resp.JSONRPC)
	}
}

func TestHandleMCPRequest_ToolsList(t *testing.T) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/list",
	}
	resp := handleMCPRequest(req)

	if resp.Error != nil {
		t.Errorf("expected no error, got %v", resp.Error)
	}

	result, ok := resp.Result.(mcpToolsListResult)
	if !ok {
		t.Fatalf("expected mcpToolsListResult, got %T", resp.Result)
	}
	if len(result.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result.Tools))
	}
	if result.Tools[0].Name != "apply_project_plan" {
		t.Errorf("expected tool name apply_project_plan, got %s", result.Tools[0].Name)
	}
	if len(result.Tools[0].InputSchema) == 0 {
		t.Error("expected non-empty input schema")
	}
}

func TestHandleMCPRequest_UnknownMethod(t *testing.T) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "unknown/method",
	}
	resp := handleMCPRequest(req)

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestHandleToolCall_UnknownTool(t *testing.T) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`4`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"nonexistent","arguments":{}}`),
	}
	resp := handleMCPRequest(req)

	result, ok := resp.Result.(mcpToolCallResult)
	if !ok {
		t.Fatalf("expected mcpToolCallResult, got %T", resp.Result)
	}
	if !result.IsError {
		t.Error("expected IsError to be true for unknown tool")
	}
}

func TestHandleToolCall_InvalidParams(t *testing.T) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`5`),
		Method:  "tools/call",
		Params:  json.RawMessage(`not-json`),
	}
	resp := handleMCPRequest(req)

	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected error code -32602, got %d", resp.Error.Code)
	}
}

func TestHandleToolCall_InvalidPlan(t *testing.T) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`6`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"apply_project_plan","arguments":"not-an-object"}`),
	}
	resp := handleMCPRequest(req)

	result, ok := resp.Result.(mcpToolCallResult)
	if !ok {
		t.Fatalf("expected mcpToolCallResult, got %T", resp.Result)
	}
	if !result.IsError {
		t.Error("expected IsError to be true for invalid plan JSON")
	}
}

func TestHandleMCPRequest_IDPreserved(t *testing.T) {
	// String ID
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"abc-123"`),
		Method:  "tools/list",
	}
	resp := handleMCPRequest(req)
	if string(resp.ID) != `"abc-123"` {
		t.Errorf("expected ID \"abc-123\", got %s", string(resp.ID))
	}

	// Numeric ID
	req2 := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`42`),
		Method:  "initialize",
	}
	resp2 := handleMCPRequest(req2)
	if string(resp2.ID) != `42` {
		t.Errorf("expected ID 42, got %s", string(resp2.ID))
	}
}
