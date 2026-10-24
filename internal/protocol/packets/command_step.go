package packets

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go-mc-server/internal/protocol"
)

// CommandStepPacket (0x4f) is sent when the client submits a command UI step.
type CommandStepPacket struct {
	Command     string
	Overload    string
	UnknownUV   uint32
	CurrentStep uint32
	Done        bool
	ClientID    uint64
	InputJSON   string
	OutputJSON  string
}

func (p *CommandStepPacket) ID() byte { return protocol.IDCommandStep }

func (p *CommandStepPacket) Encode(s *protocol.BinaryStream) error {
	return fmt.Errorf("CommandStepPacket.Encode: server never sends this")
}

func (p *CommandStepPacket) Decode(s *protocol.BinaryStream) error {
	var err error
	if p.Command, err = s.ReadString(); err != nil {
		return err
	}
	if p.Overload, err = s.ReadString(); err != nil {
		return err
	}
	if p.UnknownUV, err = s.ReadUnsignedVarInt(); err != nil {
		return err
	}
	if p.CurrentStep, err = s.ReadUnsignedVarInt(); err != nil {
		return err
	}
	if p.Done, err = s.ReadBool(); err != nil {
		return err
	}
	if p.ClientID, err = s.ReadUnsignedVarLong(); err != nil {
		return err
	}
	if p.InputJSON, err = s.ReadString(); err != nil {
		return err
	}
	p.OutputJSON, err = s.ReadString()
	return err
}

// CommandLine builds "give Steve 1 64" style text (no leading slash).
func (p *CommandStepPacket) CommandLine() string {
	parts := []string{strings.TrimSpace(p.Command)}
	parts = append(parts, inputJSONArgs(p.InputJSON)...)
	return strings.TrimSpace(strings.Join(parts, " "))
}

func inputJSONArgs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var arr []any
	if err := json.Unmarshal([]byte(raw), &arr); err == nil {
		out := make([]string, 0, len(arr))
		for _, v := range arr {
			if s := jsonArgString(v); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return objectValuesInOrder(raw)
}

// objectValuesInOrder keeps JSON object key order (PHP foreach), not alphabetical.
func objectValuesInOrder(raw string) []string {
	dec := json.NewDecoder(strings.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil
	}
	d, ok := tok.(json.Delim)
	if !ok || d != '{' {
		return nil
	}
	var out []string
	for dec.More() {
		if _, err := dec.Token(); err != nil {
			return out
		}
		var val any
		if err := dec.Decode(&val); err != nil {
			return out
		}
		if s := jsonArgString(val); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func jsonArgString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case json.Number:
		return t.String()
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}
