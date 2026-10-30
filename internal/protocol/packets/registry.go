package packets

import "go-mc-server/internal/protocol"

var constructors = map[byte]func() Packet{}

func Register(id byte, ctor func() Packet) {
	constructors[id] = ctor
}

// Lookup returns a new empty packet for id, or nil if this server has no codec for it.
func Lookup(id byte) Packet {
	if c, ok := constructors[id]; ok {
		return c()
	}
	return nil
}

func init() {
	Register(protocol.IDLogin, func() Packet { return &LoginPacket{} })
	Register(protocol.IDPlayStatus, func() Packet { return &PlayStatusPacket{} })
	Register(protocol.IDDisconnect, func() Packet { return &DisconnectPacket{} })
	Register(protocol.IDResourcePacksInfo, func() Packet { return &ResourcePacksInfoPacket{} })
	Register(protocol.IDResourcePackStack, func() Packet { return &ResourcePackStackPacket{} })
	Register(protocol.IDResourcePackClientResponse, func() Packet { return &ResourcePackClientResponsePacket{} })
	Register(protocol.IDText, func() Packet { return &TextPacket{} })
	Register(protocol.IDSetTime, func() Packet { return &SetTimePacket{} })
	Register(protocol.IDStartGame, func() Packet { return &StartGamePacket{} })
	Register(protocol.IDAddPlayer, func() Packet { return &AddPlayerPacket{} })
	Register(protocol.IDAddEntity, func() Packet { return &AddEntityPacket{} })
	Register(protocol.IDRemoveEntity, func() Packet { return &RemoveEntityPacket{} })
	Register(protocol.IDAddItemEntity, func() Packet { return &AddItemEntityPacket{} })
	Register(protocol.IDTakeItemEntity, func() Packet { return &TakeItemEntityPacket{} })
	Register(protocol.IDMoveEntity, func() Packet { return &MoveEntityPacket{} })
	Register(protocol.IDMovePlayer, func() Packet { return &MovePlayerPacket{} })
	Register(protocol.IDLevelSoundEvent, func() Packet { return &LevelSoundEventPacket{} })
	Register(protocol.IDLevelEvent, func() Packet { return &LevelEventPacket{} })
	Register(protocol.IDRemoveBlock, func() Packet { return &RemoveBlockPacket{} })
	Register(protocol.IDUpdateBlock, func() Packet { return &UpdateBlockPacket{} })
	Register(protocol.IDUpdateAttributes, func() Packet { return &UpdateAttributesPacket{} })
	Register(protocol.IDMobEquipment, func() Packet { return &MobEquipmentPacket{} })
	Register(protocol.IDMobArmorEquipment, func() Packet { return &MobArmorEquipmentPacket{} })
	Register(protocol.IDInteract, func() Packet { return &InteractPacket{} })
	Register(protocol.IDEntityEvent, func() Packet { return &EntityEventPacket{} })
	Register(protocol.IDSetHealth, func() Packet { return &SetHealthPacket{} })
	Register(protocol.IDRespawn, func() Packet { return &RespawnPacket{} })
	Register(protocol.IDUseItem, func() Packet { return &UseItemPacket{} })
	Register(protocol.IDPlayerAction, func() Packet { return &PlayerActionPacket{} })
	Register(protocol.IDEntityFall, func() Packet { return &EntityFallPacket{} })
	Register(protocol.IDSetEntityData, func() Packet { return &SetEntityDataPacket{} })
	Register(protocol.IDSetEntityMotion, func() Packet { return &SetEntityMotionPacket{} })
	Register(protocol.IDSetSpawnPosition, func() Packet { return &SetSpawnPositionPacket{} })
	Register(protocol.IDAnimate, func() Packet { return &AnimatePacket{} })
	Register(protocol.IDContainerOpen, func() Packet { return &ContainerOpenPacket{} })
	Register(protocol.IDContainerSetSlot, func() Packet { return &ContainerSetSlotPacket{} })
	Register(protocol.IDContainerSetContent, func() Packet { return &ContainerSetContentPacket{} })
	Register(protocol.IDContainerSetData, func() Packet { return &ContainerSetDataPacket{} })
	Register(protocol.IDContainerClose, func() Packet { return &ContainerClosePacket{} })
	Register(protocol.IDBlockEntityData, func() Packet { return &BlockEntityDataPacket{} })
	Register(protocol.IDDropItem, func() Packet { return &DropItemPacket{} })
	Register(protocol.IDAdventureSettings, func() Packet { return &AdventureSettingsPacket{} })
	Register(protocol.IDSetPlayerGameType, func() Packet { return &SetPlayerGameTypePacket{} })
	Register(protocol.IDFullChunkData, func() Packet { return &FullChunkDataPacket{} })
	Register(protocol.IDChangeDimension, func() Packet { return &ChangeDimensionPacket{} })
	Register(protocol.IDPlayerList, func() Packet { return &PlayerListPacket{} })
	Register(protocol.IDRequestChunkRadius, func() Packet { return &RequestChunkRadiusPacket{} })
	Register(protocol.IDChunkRadiusUpdated, func() Packet { return &ChunkRadiusUpdatedPacket{} })
	Register(protocol.IDCommandStep, func() Packet { return &CommandStepPacket{} })
	Register(protocol.IDAvailableCommands, func() Packet { return &AvailableCommandsPacket{} })
	Register(protocol.IDCraftingEvent, func() Packet { return &CraftingEventPacket{} })
}
