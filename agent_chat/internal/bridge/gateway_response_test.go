package bridge

import (
	"encoding/json"
	"testing"
)

func TestDecodeGatewayResponseUnwrapsAPIData(t *testing.T) {
	var out gatewayAgentDTO
	raw := []byte(`{"code":"ok","data":{"id":"agent-1","name":"Agent One","protocol":"acp"}}`)
	if err := decodeGatewayResponse(raw, &out); err != nil {
		t.Fatalf("decodeGatewayResponse() error = %v", err)
	}
	if out.ID != "agent-1" || out.Name != "Agent One" {
		t.Fatalf("out = %#v", out)
	}
}

func TestDecodeGatewayResponseUnwrapsPageItems(t *testing.T) {
	var out []gatewaySkillDTO
	raw := []byte(`{"code":"ok","data":{"items":[{"id":"skill-1","name":"Skill One"}],"page":1,"pageSize":20,"total":1}}`)
	if err := decodeGatewayResponse(raw, &out); err != nil {
		t.Fatalf("decodeGatewayResponse() error = %v", err)
	}
	if len(out) != 1 || out[0].ID != "skill-1" {
		t.Fatalf("out = %#v", out)
	}
}

func TestDecodeGatewayResponsePreservesRawMessageData(t *testing.T) {
	var out json.RawMessage
	raw := []byte(`{"code":"ok","data":{"sessionId":"session-1"}}`)
	if err := decodeGatewayResponse(raw, &out); err != nil {
		t.Fatalf("decodeGatewayResponse() error = %v", err)
	}
	if string(out) != `{"sessionId":"session-1"}` {
		t.Fatalf("raw = %s", string(out))
	}
}
