package protocol

import "fmt"

// PacketName returns a human-readable name for a MCPE packet ID.
func PacketName(id byte) string {
	if n, ok := packetNames[id]; ok {
		return n
	}
	return "Unknown"
}

// PacketLabel returns "Name(0xNN)" for logs.
func PacketLabel(id byte) string {
	return fmt.Sprintf("%s(0x%02X)", PacketName(id), id)
}

const (
	UpdateBlockFlagNeighbors   = 0x1
	UpdateBlockFlagNetwork     = 0x2
	UpdateBlockFlagNoGraphic   = 0x4
	UpdateBlockFlagPriority    = 0x8
	UpdateBlockFlagAll         = UpdateBlockFlagNeighbors | UpdateBlockFlagNetwork
	UpdateBlockFlagAllPriority = UpdateBlockFlagAll | UpdateBlockFlagPriority
	ContainerInventory         = 0
	ContainerCreative          = 0x79 // PocketMine SPECIAL_CREATIVE
)

var packetNames = map[byte]string{
	IDLogin:                      "Login",
	IDPlayStatus:                 "PlayStatus",
	IDServerToClientHandshake:    "ServerToClientHandshake",
	IDClientToServerHandshake:    "ClientToServerHandshake",
	IDDisconnect:                 "Disconnect",
	IDResourcePacksInfo:          "ResourcePacksInfo",
	IDResourcePackStack:          "ResourcePackStack",
	IDResourcePackClientResponse: "ResourcePackClientResponse",
	IDText:                       "Text",
	IDSetTime:                    "SetTime",
	IDStartGame:                  "StartGame",
	IDAddPlayer:                  "AddPlayer",
	IDAddEntity:                  "AddEntity",
	IDRemoveEntity:               "RemoveEntity",
	IDAddItemEntity:              "AddItemEntity",
	IDAddHangingEntity:           "AddHangingEntity",
	IDTakeItemEntity:             "TakeItemEntity",
	IDMoveEntity:                 "MoveEntity",
	IDMovePlayer:                 "MovePlayer",
	IDRiderJump:                  "RiderJump",
	IDRemoveBlock:                "RemoveBlock",
	IDUpdateBlock:                "UpdateBlock",
	IDAddPainting:                "AddPainting",
	IDExplode:                    "Explode",
	IDLevelSoundEvent:            "LevelSoundEvent",
	IDLevelEvent:                 "LevelEvent",
	IDBlockEvent:                 "BlockEvent",
	IDEntityEvent:                "EntityEvent",
	IDMobEffect:                  "MobEffect",
	IDUpdateAttributes:           "UpdateAttributes",
	IDMobEquipment:               "MobEquipment",
	IDMobArmorEquipment:          "MobArmorEquipment",
	IDInteract:                   "Interact",
	IDBlockPickRequest:           "BlockPickRequest",
	IDUseItem:                    "UseItem",
	IDPlayerAction:               "PlayerAction",
	IDEntityFall:                 "EntityFall",
	IDHurtArmor:                  "HurtArmor",
	IDSetEntityData:              "SetEntityData",
	IDSetEntityMotion:            "SetEntityMotion",
	IDSetEntityLink:              "SetEntityLink",
	IDSetHealth:                  "SetHealth",
	IDSetSpawnPosition:           "SetSpawnPosition",
	IDAnimate:                    "Animate",
	IDRespawn:                    "Respawn",
	IDDropItem:                   "DropItem",
	IDInventoryAction:            "InventoryAction",
	IDContainerOpen:              "ContainerOpen",
	IDContainerClose:             "ContainerClose",
	IDContainerSetSlot:           "ContainerSetSlot",
	IDContainerSetData:           "ContainerSetData",
	IDContainerSetContent:        "ContainerSetContent",
	IDCraftingData:               "CraftingData",
	IDCraftingEvent:              "CraftingEvent",
	IDAdventureSettings:          "AdventureSettings",
	IDBlockEntityData:            "BlockEntityData",
	IDPlayerInput:                "PlayerInput",
	IDFullChunkData:              "FullChunkData",
	IDSetCommandsEnabled:         "SetCommandsEnabled",
	IDSetDifficulty:              "SetDifficulty",
	IDChangeDimension:            "ChangeDimension",
	IDSetPlayerGameType:          "SetPlayerGameType",
	IDPlayerList:                 "PlayerList",
	IDSimpleEvent:                "SimpleEvent",
	IDEvent:                      "Event",
	IDSpawnExperienceOrb:         "SpawnExperienceOrb",
	IDClientboundMapItemData:     "ClientboundMapItemData",
	IDMapInfoRequest:             "MapInfoRequest",
	IDRequestChunkRadius:         "RequestChunkRadius",
	IDChunkRadiusUpdated:         "ChunkRadiusUpdated",
	IDItemFrameDropItem:          "ItemFrameDropItem",
	IDReplaceItemInSlot:          "ReplaceItemInSlot",
	IDGameRulesChanged:           "GameRulesChanged",
	IDCamera:                     "Camera",
	IDAddItem:                    "AddItem",
	IDBossEvent:                  "BossEvent",
	IDShowCredits:                "ShowCredits",
	IDAvailableCommands:          "AvailableCommands",
	IDCommandStep:                "CommandStep",
	IDCommandBlockUpdate:         "CommandBlockUpdate",
	IDUpdateTrade:                "UpdateTrade",
	IDUpdateEquip:                "UpdateEquip",
	IDResourcePackDataInfo:       "ResourcePackDataInfo",
	IDResourcePackChunkData:      "ResourcePackChunkData",
	IDResourcePackChunkRequest:   "ResourcePackChunkRequest",
	IDTransfer:                   "Transfer",
	IDPlaySound:                  "PlaySound",
	IDStopSound:                  "StopSound",
	IDSetTitle:                   "SetTitle",
	IDAddBehaviorTree:            "AddBehaviorTree",
	IDStructureBlockUpdate:       "StructureBlockUpdate",
	IDShowStoreOffer:             "ShowStoreOffer",
	IDPurchaseReceipt:            "PurchaseReceipt",
	IDBatch:                      "Batch",
}
