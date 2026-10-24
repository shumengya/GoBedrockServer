package protocol

import "testing"

func TestCommandPacketIDsMatchPocketMine113(t *testing.T) {
	if IDAddItem != 0x4b {
		t.Fatalf("AddItem=0x%02x want 0x4b", IDAddItem)
	}
	if IDAvailableCommands != 0x4e {
		t.Fatalf("AvailableCommands=0x%02x want 0x4e", IDAvailableCommands)
	}
	if IDCommandStep != 0x4f {
		t.Fatalf("CommandStep=0x%02x want 0x4f", IDCommandStep)
	}
}
