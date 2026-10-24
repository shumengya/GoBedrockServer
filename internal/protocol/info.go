package protocol

// CurrentProtocol is the MCPE protocol version for Minecraft Bedrock 1.1.x.
// Source: ProtocolInfo.php → CURRENT_PROTOCOL = 113
const CurrentProtocol = 113

// MinecraftVersionNetwork is the display version string for Bedrock 1.1.x.
const MinecraftVersionNetwork = "1.1.7"

// Packet IDs for Minecraft Bedrock Edition 1.1.x (from ProtocolInfo.php).
const (
	IDLogin                      = 0x01
	IDPlayStatus                 = 0x02
	IDServerToClientHandshake    = 0x03
	IDClientToServerHandshake    = 0x04
	IDDisconnect                 = 0x05
	IDResourcePacksInfo          = 0x06
	IDResourcePackStack          = 0x07
	IDResourcePackClientResponse = 0x08
	IDText                       = 0x09
	IDSetTime                    = 0x0a
	IDStartGame                  = 0x0b
	IDAddPlayer                  = 0x0c
	IDAddEntity                  = 0x0d
	IDRemoveEntity               = 0x0e
	IDAddItemEntity              = 0x0f
	IDAddHangingEntity           = 0x10
	IDTakeItemEntity             = 0x11
	IDMoveEntity                 = 0x12
	IDMovePlayer                 = 0x13
	IDRiderJump                  = 0x14
	IDRemoveBlock                = 0x15
	IDUpdateBlock                = 0x16
	IDAddPainting                = 0x17
	IDExplode                    = 0x18
	IDLevelSoundEvent            = 0x19
	IDLevelEvent                 = 0x1a
	IDBlockEvent                 = 0x1b
	IDEntityEvent                = 0x1c
	IDMobEffect                  = 0x1d
	IDUpdateAttributes           = 0x1e
	IDMobEquipment               = 0x1f
	IDMobArmorEquipment          = 0x20
	IDInteract                   = 0x21
	IDBlockPickRequest           = 0x22
	IDUseItem                    = 0x23
	IDPlayerAction               = 0x24
	IDEntityFall                 = 0x25
	IDHurtArmor                  = 0x26
	IDSetEntityData              = 0x27
	IDSetEntityMotion            = 0x28
	IDSetEntityLink              = 0x29
	IDSetHealth                  = 0x2a
	IDSetSpawnPosition           = 0x2b
	IDAnimate                    = 0x2c
	IDRespawn                    = 0x2d
	IDDropItem                   = 0x2e
	IDInventoryAction            = 0x2f
	IDContainerOpen              = 0x30
	IDContainerClose             = 0x31
	IDContainerSetSlot           = 0x32
	IDContainerSetData           = 0x33
	IDContainerSetContent        = 0x34
	IDCraftingData               = 0x35
	IDCraftingEvent              = 0x36
	IDAdventureSettings          = 0x37
	IDBlockEntityData            = 0x38
	IDPlayerInput                = 0x39
	IDFullChunkData              = 0x3a
	IDSetCommandsEnabled         = 0x3b
	IDSetDifficulty              = 0x3c
	IDChangeDimension            = 0x3d
	IDSetPlayerGameType          = 0x3e
	IDPlayerList                 = 0x3f
	IDSimpleEvent                = 0x40
	IDEvent                      = 0x41
	IDSpawnExperienceOrb         = 0x42
	IDClientboundMapItemData     = 0x43
	IDMapInfoRequest             = 0x44
	IDRequestChunkRadius         = 0x45
	IDChunkRadiusUpdated         = 0x46
	IDItemFrameDropItem          = 0x47
	IDReplaceItemInSlot          = 0x48
	IDGameRulesChanged           = 0x49
	IDCamera                     = 0x4a
	IDAddItem                    = 0x4b
	IDBossEvent                  = 0x4c
	IDShowCredits                = 0x4d
	IDAvailableCommands          = 0x4e
	IDCommandStep                = 0x4f
	IDCommandBlockUpdate         = 0x50
	IDUpdateTrade                = 0x51
	IDUpdateEquip                = 0x52
	IDResourcePackDataInfo       = 0x53
	IDResourcePackChunkData      = 0x54
	IDResourcePackChunkRequest   = 0x55
	IDTransfer                   = 0x56
	IDPlaySound                  = 0x57
	IDStopSound                  = 0x58
	IDSetTitle                   = 0x59
	IDAddBehaviorTree            = 0x5a
	IDStructureBlockUpdate       = 0x5b
	IDShowStoreOffer             = 0x5c
	IDPurchaseReceipt            = 0x5d

	// BatchPacket wraps all game packets (zlib-compressed).
	IDBatch = 0xfe
)

// PlayStatus codes.
const (
	PlayStatusLoginSuccess             = 0
	PlayStatusLoginFailedClient        = 1
	PlayStatusLoginFailedServer        = 2
	PlayStatusPlayerSpawn              = 3
	PlayStatusLoginFailedInvalidTenant = 4
	PlayStatusLoginFailedEduVanilla    = 5
	PlayStatusLoginFailedVanillaEdu    = 6
)

// ResourcePack client response status.
const (
	ResourcePackResponseRefused      = 1
	ResourcePackResponseSendPacks    = 2
	ResourcePackResponseHaveAllPacks = 3
	ResourcePackResponseCompleted    = 4
)

// TextPacket types.
const (
	TextTypeRaw         = 0
	TextTypeChat        = 1
	TextTypeTranslation = 2
	TextTypePopup       = 3
	TextTypeTip         = 4
	TextTypeSystem      = 5
	TextTypeWhisper     = 6
)

// MovePlayer modes.
const (
	MovePlayerModeNormal   = 0
	MovePlayerModeReset    = 1
	MovePlayerModeTeleport = 2
	MovePlayerModePitch    = 3
)

// AdventureSettings permissions.
const (
	PermissionNormal     = 0
	PermissionOperator   = 1
	PermissionHost       = 2
	PermissionAutomation = 3
	PermissionAdmin      = 4
)

// PlayerList actions.
const (
	PlayerListAdd    = 0
	PlayerListRemove = 1
)

// Game modes.
const (
	GameModeSurvival  = 0
	GameModeCreative  = 1
	GameModeAdventure = 2
	GameModeSpectator = 3
)

// Difficulty levels.
const (
	DifficultyPeaceful = 0
	DifficultyEasy     = 1
	DifficultyNormal   = 2
	DifficultyHard     = 3
)

// Dimensions.
const (
	DimensionOverworld = 0
	DimensionNether    = 1
	DimensionEnd       = 2
)

// AcceptedProtocols lists the protocol versions this server accepts.
// Source: ProtocolInfo.php → ACCEPTED_PROTOCOLS = [110, 111, 112, 113]
var AcceptedProtocols = []int{110, 111, 112, 113}
