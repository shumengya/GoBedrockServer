package packets

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go-mc-server/internal/protocol"
)

// LoginPacket (0x01) is sent by the client to begin a session.
//
// Wire format (after the 0x01 ID byte):
//   [int32 BE]               protocol version
//   [byte]                   game edition  (0 = Pocket)
//   [UnsignedVarInt + bytes] outer body string:
//     [LInt  / LE int32]     chainData length
//     [chainLen bytes]       chain JSON  ({"chain":[jwt1,jwt2,…]})
//     [LInt  / LE int32]     clientData length
//     [clientLen bytes]      client JWT
//
// Source: LoginPacket.php decode() in the PocketMine reference.
type LoginPacket struct {
	Protocol    int32
	GameEdition byte

	Username    string
	ClientUUID  string
	XUID        string
	ClientID    int64

	SkinID   string
	SkinData []byte

	DeviceOS    int
	DeviceModel string
	LanguageCode string
}

func (p *LoginPacket) ID() byte { return protocol.IDLogin }

func (p *LoginPacket) Encode(s *protocol.BinaryStream) error {
	return fmt.Errorf("LoginPacket.Encode: not implemented (server never sends this)")
}

// Decode reads the LoginPacket fields from stream (ID byte already consumed).
func (p *LoginPacket) Decode(s *protocol.BinaryStream) error {
	// 1. Protocol version (big-endian int32).
	proto, err := s.ReadInt32BE()
	if err != nil {
		return fmt.Errorf("login: read protocol: %w", err)
	}
	p.Protocol = proto

	// 2. Game edition byte.
	if p.GameEdition, err = s.ReadByte(); err != nil {
		return fmt.Errorf("login: read game edition: %w", err)
	}

	// 3. Outer body — encoded as a UnsignedVarInt-prefixed string.
	bodyStr, err := s.ReadString()
	if err != nil {
		return fmt.Errorf("login: read outer body string: %w", err)
	}
	inner := protocol.NewStream([]byte(bodyStr))

	// 4. Chain data — LE int32 length prefix + bytes.
	chainLen, err := inner.ReadInt32LE()
	if err != nil {
		return fmt.Errorf("login: read chain length: %w", err)
	}
	chainBytes, err := inner.ReadBytes(int(chainLen))
	if err != nil {
		return fmt.Errorf("login: read chain data: %w", err)
	}

	// 5. Client data — LE int32 length prefix + bytes.
	clientLen, err := inner.ReadInt32LE()
	if err != nil {
		return fmt.Errorf("login: read client data length: %w", err)
	}
	clientBytes, err := inner.ReadBytes(int(clientLen))
	if err != nil {
		return fmt.Errorf("login: read client data: %w", err)
	}

	// 6. Parse chain JWT array to extract displayName / identity / XUID.
	if err := p.parseChain(chainBytes); err != nil {
		return fmt.Errorf("login: parse chain: %w", err)
	}

	// 7. Parse client-data JWT to extract skin, device info, etc.
	if err := p.parseClientData(clientBytes); err != nil {
		return fmt.Errorf("login: parse client data: %w", err)
	}
	return nil
}

// ---- JWT parsing (offline mode: no signature verification) ----

type chainContainer struct {
	Chain []string `json:"chain"`
}

func (p *LoginPacket) parseChain(data []byte) error {
	var cc chainContainer
	if err := json.Unmarshal(data, &cc); err != nil {
		return err
	}
	for _, token := range cc.Chain {
		payload, err := jwtPayload(token)
		if err != nil {
			continue
		}
		var claims map[string]interface{}
		if err := json.Unmarshal(payload, &claims); err != nil {
			continue
		}
		if extra, ok := claims["extraData"].(map[string]interface{}); ok {
			if name, ok := extra["displayName"].(string); ok && name != "" {
				p.Username = name
			}
			if id, ok := extra["identity"].(string); ok && id != "" {
				p.ClientUUID = id
			}
			if xuid, ok := extra["XUID"].(string); ok {
				p.XUID = xuid
			}
		}
	}
	return nil
}

func (p *LoginPacket) parseClientData(token []byte) error {
	payload, err := jwtPayload(string(token))
	if err != nil {
		return err
	}
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}
	if v, ok := data["SkinId"].(string); ok {
		p.SkinID = v
	}
	if v, ok := data["SkinData"].(string); ok {
		p.SkinData, _ = base64.StdEncoding.DecodeString(v)
	}
	if v, ok := data["DeviceOS"].(float64); ok {
		p.DeviceOS = int(v)
	}
	if v, ok := data["DeviceModel"].(string); ok {
		p.DeviceModel = v
	}
	if v, ok := data["LanguageCode"].(string); ok {
		p.LanguageCode = v
	}
	if v, ok := data["ClientRandomId"].(float64); ok {
		p.ClientID = int64(v)
	}
	return nil
}

// jwtPayload returns the decoded JSON payload of a JWT token.
// We skip signature verification (offline mode).
func jwtPayload(token string) ([]byte, error) {
	parts := splitDot(token)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid JWT: expected at least 2 parts, got %d", len(parts))
	}
	return base64.RawURLEncoding.DecodeString(parts[1])
}

func splitDot(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}
