package packets

import (
	"testing"

	"go-mc-server/internal/protocol"
)

func TestUseItemRoundTrip(t *testing.T) {
	orig := &UseItemPacket{
		X: 1, Y: 4, Z: 2,
		BlockID: 2,
		Face:    1,
		FX:      0.5, FY: 1, FZ: 0.5,
		PosX: 1.5, PosY: 6, PosZ: 2.5,
		Slot: 0,
		Item: protocol.Item{ID: 1, Meta: 0, Count: 64},
	}
	raw, err := EncodePacket(orig)
	if err != nil {
		t.Fatal(err)
	}
	if raw[0] != protocol.IDUseItem {
		t.Fatalf("id 0x%02x", raw[0])
	}
	got := &UseItemPacket{}
	if err := got.Decode(protocol.NewStream(raw[1:])); err != nil {
		t.Fatal(err)
	}
	if got.X != orig.X || got.Y != orig.Y || got.Z != orig.Z || got.Face != orig.Face || got.Item.ID != orig.Item.ID {
		t.Fatalf("got %+v want %+v", got, orig)
	}
}

func TestUpdateBlockRoundTrip(t *testing.T) {
	orig := &UpdateBlockPacket{
		X: 0, Y: 5, Z: 0,
		BlockID:   1,
		BlockData: 0,
		Flags:     protocol.UpdateBlockFlagAllPriority,
	}
	raw, err := EncodePacket(orig)
	if err != nil {
		t.Fatal(err)
	}
	got := &UpdateBlockPacket{}
	if err := got.Decode(protocol.NewStream(raw[1:])); err != nil {
		t.Fatal(err)
	}
	if got.BlockID != orig.BlockID || got.Flags != orig.Flags || got.Y != orig.Y {
		t.Fatalf("got %+v want %+v", got, orig)
	}
}

func TestDefaultAttributesPocketMine11(t *testing.T) {
	attrs := DefaultAttributes()
	if len(attrs) != 11 {
		t.Fatalf("len=%d want 11", len(attrs))
	}
	if attrs[4].Name != "minecraft:health" || attrs[4].Max != 20 || attrs[4].Value != 20 {
		t.Fatalf("health slot: %+v", attrs[4])
	}
	if attrs[5].Name != "minecraft:movement" || attrs[5].Value != 0.10 {
		t.Fatalf("movement slot: %+v", attrs[5])
	}
	for _, a := range attrs {
		switch a.Name {
		case "minecraft:underwater_movement", "minecraft:luck", "minecraft:fall_damage":
			t.Fatalf("1.1 不该发送 %s", a.Name)
		}
	}
}

func TestSetEntityDataFlagsUseVarLongAndAir(t *testing.T) {
	p := &SetEntityDataPacket{
		EntityRuntimeID: 1,
		SendFlags:       true,
		Flags:           DefaultPlayerFlags(),
	}
	raw, err := EncodePacket(p)
	if err != nil {
		t.Fatal(err)
	}
	s := protocol.NewStream(raw[1:])
	eid, err := s.ReadEntityID()
	if err != nil || eid != 1 {
		t.Fatalf("eid=%d err=%v", eid, err)
	}
	count, err := s.ReadUnsignedVarInt()
	if err != nil || count != 3 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	key, err := s.ReadUnsignedVarInt()
	if err != nil {
		t.Fatal(err)
	}
	typ, err := s.ReadUnsignedVarInt()
	if err != nil {
		t.Fatal(err)
	}
	flags, err := s.ReadVarLong()
	if err != nil {
		t.Fatal(err)
	}
	if key != EntityDataFlags || typ != EntityDataTypeLong {
		t.Fatalf("flags entry key=%d type=%d", key, typ)
	}
	if uint64(flags)&EntityFlagBreathing == 0 {
		t.Fatal("missing DATA_FLAG_BREATHING bit 33")
	}
	if _, err = s.ReadUnsignedVarInt(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadUnsignedVarInt(); err != nil {
		t.Fatal(err)
	}
	air, err := s.ReadInt16LE()
	if err != nil || air != DefaultAirTicks {
		t.Fatalf("air=%d err=%v", air, err)
	}
}

func TestAddEntityEncodeHasTwoEntityIDsAndMetadata(t *testing.T) {
	p := &AddEntityPacket{
		EntityUniqueID:  5,
		EntityRuntimeID: 5,
		EntityType:      32,
		X:               1, Y: 2, Z: 3,
		Metadata: DefaultEntityMetadata(""),
	}
	raw, err := EncodePacket(p)
	if err != nil {
		t.Fatal(err)
	}
	if raw[0] != protocol.IDAddEntity {
		t.Fatalf("id 0x%02x", raw[0])
	}
	s := protocol.NewStream(raw[1:])
	unique, err := s.ReadEntityID()
	if err != nil || unique != 5 {
		t.Fatalf("unique=%d err=%v", unique, err)
	}
	runtime, err := s.ReadEntityID()
	if err != nil || runtime != 5 {
		t.Fatalf("runtime=%d err=%v", runtime, err)
	}
	typ, err := s.ReadUnsignedVarInt()
	if err != nil || typ != 32 {
		t.Fatalf("type=%d err=%v", typ, err)
	}
	if _, _, _, err = s.ReadVector3f(); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = s.ReadVector3f(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadLFloat(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadLFloat(); err != nil {
		t.Fatal(err)
	}
	attrs, err := s.ReadUnsignedVarInt()
	if err != nil || attrs != 0 {
		t.Fatalf("attrs=%d err=%v", attrs, err)
	}
	meta, err := s.ReadUnsignedVarInt()
	if err != nil || meta != 6 {
		t.Fatalf("metadata count=%d err=%v", meta, err)
	}
}

func TestLevelEventRoundTrip(t *testing.T) {
	orig := &LevelEventPacket{
		EventID: LevelEventBlockStartBreak,
		X:       1, Y: 4, Z: 2,
		Data: 4369,
	}
	raw, err := EncodePacket(orig)
	if err != nil {
		t.Fatal(err)
	}
	got := &LevelEventPacket{}
	if err := got.Decode(protocol.NewStream(raw[1:])); err != nil {
		t.Fatal(err)
	}
	if got.EventID != orig.EventID || got.Y != orig.Y || got.Data != orig.Data {
		t.Fatalf("got %+v want %+v", got, orig)
	}
}

func TestInteractMouseoverDecode(t *testing.T) {
	// wire: action=4 (MOUSEOVER), target entity id 0
	p := &InteractPacket{}
	if err := p.Decode(protocol.NewStream([]byte{0x04, 0x00})); err != nil {
		t.Fatal(err)
	}
	if p.Action != InteractMouseover || p.Target != 0 {
		t.Fatalf("got action=%d target=%d", p.Action, p.Target)
	}
}

func TestLevelSoundEventDecodeFromClientHex(t *testing.T) {
	// From log: 19010000e84100000042000082c204020000 — HIT at ~ (29,32,-65)
	body := []byte{0x01, 0x00, 0x00, 0xe8, 0x41, 0x00, 0x00, 0x00, 0x42, 0x00, 0x00, 0x82, 0xc2, 0x04, 0x02, 0x00, 0x00}
	p := &LevelSoundEventPacket{}
	if err := p.Decode(protocol.NewStream(body)); err != nil {
		t.Fatal(err)
	}
	if p.SoundID != SoundHit {
		t.Fatalf("sound=%d want HIT", p.SoundID)
	}
	if p.X < 28.9 || p.X > 29.1 || p.Z > -64.9 || p.Z < -65.1 {
		t.Fatalf("pos=(%v,%v,%v)", p.X, p.Y, p.Z)
	}
	if SoundName(p.SoundID) != "HIT" {
		t.Fatalf("name=%s", SoundName(p.SoundID))
	}
}

func TestContainerSetSlotDecodeFromClientHex(t *testing.T) {
	// From log: 32001000467e0000000000 — window0 slot8 wool×63
	body := []byte{0x00, 0x10, 0x00, 0x46, 0x7e, 0x00, 0x00, 0x00, 0x00, 0x00}
	p := &ContainerSetSlotPacket{}
	if err := p.Decode(protocol.NewStream(body)); err != nil {
		t.Fatal(err)
	}
	if p.WindowID != 0 || p.Slot != 8 {
		t.Fatalf("window=%d slot=%d", p.WindowID, p.Slot)
	}
	if p.Item.ID != 35 || p.Item.Count != 63 {
		t.Fatalf("item id=%d count=%d", p.Item.ID, p.Item.Count)
	}
	if p.HotbarSlot != 0 {
		t.Fatalf("hotbar=%d (client often sends 0; do not treat as HeldSlot)", p.HotbarSlot)
	}
}

func TestContainerSetContentPlayerInventoryEncode(t *testing.T) {
	inv := make([]protocol.Item, protocol.PlayerInventorySize)
	inv[0] = protocol.Item{ID: 1, Count: 64}
	p := &ContainerSetContentPacket{
		WindowID:  protocol.ContainerInventory,
		TargetEID: 1,
		Slots:     protocol.NetworkPlayerInventory(inv),
		Hotbar:    protocol.DefaultHotbarMapping(),
	}
	raw, err := EncodePacket(p)
	if err != nil {
		t.Fatal(err)
	}
	got := &ContainerSetContentPacket{}
	if err := got.Decode(protocol.NewStream(raw[1:])); err != nil {
		t.Fatal(err)
	}
	if len(got.Slots) != 45 {
		t.Fatalf("slots=%d want 45", len(got.Slots))
	}
	if got.Slots[0].ID != 1 || got.Slots[0].Count != 64 {
		t.Fatalf("slot0 %+v", got.Slots[0])
	}
	if !got.Slots[36].IsAir() {
		t.Fatal("dummy slot 36 must be air")
	}
	if len(got.Hotbar) != 9 || got.Hotbar[0] != 9 || got.Hotbar[8] != 17 {
		t.Fatalf("hotbar=%v", got.Hotbar)
	}
}

func TestCommandStepCommandLineFromJSONArray(t *testing.T) {
	p := &CommandStepPacket{
		Command:   "give",
		InputJSON: `["Steve","1",64]`,
	}
	if got := p.CommandLine(); got != "give Steve 1 64" {
		t.Fatalf("line=%q", got)
	}
}

func TestCommandStepCommandLineFromJSONObjectKeepsOrder(t *testing.T) {
	p := &CommandStepPacket{
		Command:   "give",
		InputJSON: `{"player":"@s","item":"stone","amount":10}`,
	}
	if got := p.CommandLine(); got != "give @s stone 10" {
		t.Fatalf("line=%q", got)
	}
}

func TestAddItemEntityRoundTrip(t *testing.T) {
	orig := &AddItemEntityPacket{
		EntityUniqueID:  9,
		EntityRuntimeID: 9,
		Item:            protocol.Item{ID: 1, Count: 32},
		X:               1, Y: 2, Z: 3,
		SpeedY: 0.2,
	}
	raw, err := EncodePacket(orig)
	if err != nil {
		t.Fatal(err)
	}
	if raw[0] != protocol.IDAddItemEntity {
		t.Fatalf("id 0x%02x", raw[0])
	}
	got := &AddItemEntityPacket{}
	if err := got.Decode(protocol.NewStream(raw[1:])); err != nil {
		t.Fatal(err)
	}
	if got.EntityUniqueID != 9 || got.EntityRuntimeID != 9 || got.Item.ID != 1 || got.Item.Count != 32 || got.Y != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestTakeAndRemoveEntityEncode(t *testing.T) {
	take := &TakeItemEntityPacket{TargetEntityID: 7, CollectorEID: 1}
	raw, err := EncodePacket(take)
	if err != nil {
		t.Fatal(err)
	}
	got := &TakeItemEntityPacket{}
	if err := got.Decode(protocol.NewStream(raw[1:])); err != nil {
		t.Fatal(err)
	}
	if got.TargetEntityID != 7 || got.CollectorEID != 1 {
		t.Fatalf("take %+v", got)
	}
	rm := &RemoveEntityPacket{EntityUniqueID: 7}
	raw, err = EncodePacket(rm)
	if err != nil {
		t.Fatal(err)
	}
	gotRm := &RemoveEntityPacket{}
	if err := gotRm.Decode(protocol.NewStream(raw[1:])); err != nil {
		t.Fatal(err)
	}
	if gotRm.EntityUniqueID != 7 {
		t.Fatalf("remove %+v", gotRm)
	}
}

func TestDropItemDecode(t *testing.T) {
	w := protocol.NewWriter()
	_ = w.WriteByte(0)
	w.WriteSlot(protocol.Item{ID: 4, Count: 8})
	p := &DropItemPacket{}
	if err := p.Decode(protocol.NewStream(w.Bytes())); err != nil {
		t.Fatal(err)
	}
	if p.Item.ID != 4 || p.Item.Count != 8 {
		t.Fatalf("drop %+v", p.Item)
	}
}

func TestMoveEntityRoundTrip(t *testing.T) {
	orig := &MoveEntityPacket{
		EntityRuntimeID: 9,
		X:               1, Y: 2.125, Z: 3,
		Pitch:    90,
		HeadYaw:  45,
		Yaw:      45,
		OnGround: true,
	}
	raw, err := EncodePacket(orig)
	if err != nil {
		t.Fatal(err)
	}
	if raw[0] != protocol.IDMoveEntity {
		t.Fatalf("id 0x%02x", raw[0])
	}
	got := &MoveEntityPacket{}
	if err := got.Decode(protocol.NewStream(raw[1:])); err != nil {
		t.Fatal(err)
	}
	if got.EntityRuntimeID != 9 || got.Y != 2.125 || !got.OnGround {
		t.Fatalf("got %+v", got)
	}
}

func TestSetPlayerGameTypeRoundTrip(t *testing.T) {
	orig := &SetPlayerGameTypePacket{GameMode: protocol.GameModeCreative}
	raw, err := EncodePacket(orig)
	if err != nil {
		t.Fatal(err)
	}
	if raw[0] != protocol.IDSetPlayerGameType {
		t.Fatalf("id 0x%02x", raw[0])
	}
	got := &SetPlayerGameTypePacket{}
	if err := got.Decode(protocol.NewStream(raw[1:])); err != nil {
		t.Fatal(err)
	}
	if got.GameMode != protocol.GameModeCreative {
		t.Fatalf("got %+v", got)
	}
}

func TestSetEntityMotionRoundTrip(t *testing.T) {
	orig := &SetEntityMotionPacket{EntityRuntimeID: 9, SpeedX: 0.1, SpeedY: -0.2, SpeedZ: 0.3}
	raw, err := EncodePacket(orig)
	if err != nil {
		t.Fatal(err)
	}
	if raw[0] != protocol.IDSetEntityMotion {
		t.Fatalf("id 0x%02x", raw[0])
	}
	got := &SetEntityMotionPacket{}
	if err := got.Decode(protocol.NewStream(raw[1:])); err != nil {
		t.Fatal(err)
	}
	if got.EntityRuntimeID != 9 || got.SpeedY != -0.2 {
		t.Fatalf("got %+v", got)
	}
}

func TestChangeDimensionRoundTrip(t *testing.T) {
	orig := &ChangeDimensionPacket{
		Dimension: protocol.DimensionNether,
		X:         8.5, Y: 65, Z: 8.5,
		Respawn: false,
	}
	raw, err := EncodePacket(orig)
	if err != nil {
		t.Fatal(err)
	}
	if raw[0] != protocol.IDChangeDimension {
		t.Fatalf("id 0x%02x", raw[0])
	}
	got := &ChangeDimensionPacket{}
	if err := got.Decode(protocol.NewStream(raw[1:])); err != nil {
		t.Fatal(err)
	}
	if got.Dimension != protocol.DimensionNether || got.Y != 65 || got.Respawn {
		t.Fatalf("got %+v", got)
	}
}

func TestContainerOpenRoundTrip(t *testing.T) {
	orig := &ContainerOpenPacket{
		WindowID: protocol.WindowChest,
		Type:     protocol.InventoryTypeContainer,
		X:        1, Y: 4, Z: 2,
		EntityID: -1,
	}
	raw, err := EncodePacket(orig)
	if err != nil {
		t.Fatal(err)
	}
	got := &ContainerOpenPacket{}
	if err := got.Decode(protocol.NewStream(raw[1:])); err != nil {
		t.Fatal(err)
	}
	if got.WindowID != orig.WindowID || got.Type != orig.Type || got.X != orig.X || got.EntityID != orig.EntityID {
		t.Fatalf("got %+v want %+v", got, orig)
	}
}

func TestBlockEntityDataEncodeKeepsNBT(t *testing.T) {
	orig := &BlockEntityDataPacket{X: 1, Y: 5, Z: 2, NBT: []byte{0x0a, 0x00, 0x00}}
	raw, err := EncodePacket(orig)
	if err != nil {
		t.Fatal(err)
	}
	got := &BlockEntityDataPacket{}
	if err := got.Decode(protocol.NewStream(raw[1:])); err != nil {
		t.Fatal(err)
	}
	if got.Y != 5 || string(got.NBT) != string(orig.NBT) {
		t.Fatalf("got %+v", got)
	}
}
