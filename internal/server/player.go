package server

import (
	"fmt"
	"math"
	"sync"
	"time"

	"go-mc-server/internal/network"
	"go-mc-server/internal/protocol"
	"go-mc-server/internal/protocol/packets"
	"go-mc-server/internal/storage"
)

// Player represents a connected player in the game world.
type Player struct {
	mu      sync.RWMutex
	Session *network.Session

	EntityRuntimeID int64
	Username        string
	UUID            string
	XUID            string

	X, Y, Z    float32
	Yaw, Pitch float32
	OnGround   bool

	// RemoteIP 是进服时记下的客户端 IP，封 IP 时用来找出同 IP 的在线玩家。
	RemoteIP string

	GameMode int32

	WorldName string
	Dimension int32

	// CoordsShown 控制快捷栏上方的实时坐标显示，默认开启。
	CoordsShown bool

	ViewRadius int32
	// viewEpoch 视野代数，重置视野（出生、换世界）时加一。worker 做好的区块包带着下单时的代数，
	// 回来对不上就是给旧视野的，丢掉。chunkInflight 是此刻在 worker 手里的区块数。都受 mu 保护。
	viewEpoch       uint64
	chunkInflight   int
	SentChunks      map[string]struct{}
	ChunkQueue      [][2]int32
	chunkQueued     map[string]struct{}
	SpawnChunksSent int
	NeedSpawnStatus bool

	// Visible：已在当前世界对其他玩家 AddPlayer；切世界时先清掉再重建。
	Visible bool
	// JoinAnnounced：发过加入提示，离开时才发离开提示，切世界不重复发。
	JoinAnnounced bool

	OpenWindow             byte
	OpenWindowType         byte
	OpenTX, OpenTY, OpenTZ int32

	Inventory []protocol.Item
	HeldSlot  byte
	HeldItem  protocol.Item

	// EnderChest 玩家的末影箱，没用过时是 nil，用到时才补满 27 格。受 mu 保护。
	EnderChest []protocol.Item

	// Armor 身上的盔甲，下标见 armorSlotHelmet 等。零值是全空。受 mu 保护。
	Armor [protocol.ArmorSize]protocol.Item

	combat combatState
	food   foodState
	mining miningState

	// sneaking 是客户端最近一次上报的潜行状态（START_SNEAK / STOP_SNEAK）。
	sneaking bool

	// 移动校验用的上一次位置与时间，回拉时也拿这里的位置当基准。
	lastMoveX, lastMoveY, lastMoveZ float32
	lastMoveAt                      time.Time
	hasLastMove                     bool

	// 操作被拒提示的节流状态：同一条提示在间隔内只发一次。
	noticeText string
	noticeAt   time.Time
}

func chunkCoordKey(cx, cz int32) string {
	return fmt.Sprintf("%d,%d", cx, cz)
}

func newPlayer(sess *network.Session, entityID int64) *Player {
	inv := make([]protocol.Item, protocol.PlayerInventorySize)
	hotbar := []protocol.Item{
		{ID: 1, Count: 64},  // stone
		{ID: 2, Count: 64},  // grass
		{ID: 3, Count: 64},  // dirt
		{ID: 4, Count: 64},  // cobble
		{ID: 5, Count: 64},  // planks
		{ID: 12, Count: 64}, // sand
		{ID: 17, Count: 64}, // log
		{ID: 20, Count: 64}, // glass
		{ID: 35, Count: 64}, // wool
	}
	copy(inv, hotbar)
	return &Player{
		Session:         sess,
		EntityRuntimeID: entityID,
		Username:        sess.Username,
		UUID:            sess.ClientUUID,
		XUID:            sess.XboxID,
		X:               0,
		Y:               5,
		Z:               0,
		GameMode:        0,
		WorldName:       "",
		Dimension:       0,
		CoordsShown:     true,
		ViewRadius:      4,
		SentChunks:      make(map[string]struct{}),
		chunkQueued:     make(map[string]struct{}),
		Inventory:       inv,
		HeldItem:        inv[0],
		combat:          combatState{Health: maxPlayerHealth, AirTicks: airMax},
		food:            newFoodState(),
	}
}

func (p *Player) UpdatePosition(x, y, z, yaw, pitch float32, onGround bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.X, p.Y, p.Z = x, y, z
	p.Yaw, p.Pitch = yaw, pitch
	p.OnGround = onGround
}

func (p *Player) Position() (x, y, z float32) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.X, p.Y, p.Z
}

func (p *Player) Look() (yaw, pitch float32) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Yaw, p.Pitch
}

// CoordsEnabled 报告快捷栏上方是否正在显示实时坐标。
func (p *Player) CoordsEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.CoordsShown
}

func (p *Player) SetCoordsEnabled(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.CoordsShown = enabled
}

func (p *Player) Snapshot(worldName string) storage.PlayerData {
	p.mu.RLock()
	defer p.mu.RUnlock()
	wn := p.WorldName
	if wn == "" {
		wn = worldName
	}
	return storage.PlayerData{
		UUID:       p.UUID,
		Name:       p.Username,
		XUID:       p.XUID,
		World:      wn,
		Dimension:  p.Dimension,
		X:          p.X,
		Y:          p.Y,
		Z:          p.Z,
		Yaw:        p.Yaw,
		Pitch:      p.Pitch,
		GameMode:   p.GameMode,
		Inventory:  storage.SlotsFromItems(p.Inventory),
		HeldSlot:   p.HeldSlot,
		EnderChest: enderChestSlots(p.EnderChest),
		Armor:      armorToSlots(p.Armor),
		Food:       p.food.save(),
	}
}

// enderChestSlots 末影箱全空就不写，省得每个存档都带 27 个空格。
func enderChestSlots(items []protocol.Item) []storage.SlotData {
	var index int = 0
	for index = 0; index < len(items); index++ {
		if !items[index].IsAir() {
			return storage.SlotsFromItems(items)
		}
	}
	return nil
}

func (p *Player) ApplySave(d storage.PlayerData) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if d.Name != "" {
		p.Username = d.Name
	}
	if d.XUID != "" {
		p.XUID = d.XUID
	}
	p.WorldName = d.World
	p.Dimension = d.Dimension
	p.X, p.Y, p.Z = d.X, d.Y, d.Z
	p.Yaw, p.Pitch = d.Yaw, d.Pitch
	p.GameMode = d.GameMode
	if len(d.Inventory) > 0 {
		p.Inventory = storage.ItemsFromSlots(d.Inventory, protocol.PlayerInventorySize)
	}
	p.HeldSlot = d.HeldSlot
	if int(p.HeldSlot) < len(p.Inventory) {
		p.HeldItem = p.Inventory[p.HeldSlot]
	}
	if len(d.EnderChest) > 0 {
		p.EnderChest = storage.ItemsFromSlots(d.EnderChest, protocol.ChestSize)
	}
	p.Armor = armorFromSlots(d.Armor)
	p.food.restore(d.Food)
}

func (p *Player) ApplyInventorySlot(slot int, item protocol.Item) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if slot < 0 || slot >= len(p.Inventory) {
		return false
	}
	p.Inventory[slot] = item
	if slot == int(p.HeldSlot) {
		p.HeldItem = item
	}
	return true
}

const maxStackByte = 64

// stackLimit 工具按耐久区分，必须一格一个，否则一叠镐子共用一个损耗值。
func stackLimit(item protocol.Item) byte {
	if toolInfoFor(item).MaxDurability > 0 {
		return 1
	}
	var _, isArmor = armorInfoFor(item)
	if isArmor {
		return 1
	}
	return maxStackByte
}

// AddItem merges into existing stacks then empty slots. leftover is what did not fit.
// ok is true when at least one item was placed.
func (p *Player) AddItem(item protocol.Item) (leftover protocol.Item, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.addItemLocked(item)
}

// addItemLocked 是 AddItem 的内核，调用方必须已持有 p.mu。
// 吃东西留碗、合成产物入包这类要和别的改动在同一把锁里完成的场景用它。
func (p *Player) addItemLocked(item protocol.Item) (leftover protocol.Item, ok bool) {
	if item.IsAir() || item.Count == 0 {
		return protocol.Air(), false
	}
	remain := item.Count
	var limit byte = stackLimit(item)
	// Merge into matching stacks first.
	for i := 0; i < len(p.Inventory) && remain > 0; i++ {
		slot := p.Inventory[i]
		if slot.IsAir() || slot.ID != item.ID || slot.Meta != item.Meta {
			continue
		}
		if slot.Count >= limit {
			continue
		}
		var space byte = limit - slot.Count
		add := remain
		if add > space {
			add = space
		}
		slot.Count += add
		p.Inventory[i] = slot
		if i == int(p.HeldSlot) {
			p.HeldItem = slot
		}
		remain -= add
		ok = true
	}
	// Then empty slots.
	for i := 0; i < len(p.Inventory) && remain > 0; i++ {
		if !p.Inventory[i].IsAir() {
			continue
		}
		var placeCount byte = min(remain, limit)
		placed := protocol.Item{ID: item.ID, Meta: item.Meta, Count: placeCount, NBT: item.NBT}
		p.Inventory[i] = placed
		if i == int(p.HeldSlot) {
			p.HeldItem = placed
		}
		remain -= placeCount
		ok = true
	}
	if remain == 0 {
		return protocol.Air(), ok
	}
	out := item
	out.Count = remain
	return out, ok
}

// CanAddItem reports whether the full stack would fit (PocketMine canAddItem).
func (p *Player) CanAddItem(item protocol.Item) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if item.IsAir() || item.Count == 0 {
		return false
	}
	need := int(item.Count)
	var limit byte = stackLimit(item)
	space := 0
	for i := 0; i < len(p.Inventory); i++ {
		slot := p.Inventory[i]
		if slot.IsAir() {
			space += int(limit)
		} else if slot.ID == item.ID && slot.Meta == item.Meta && slot.Count < limit {
			space += int(limit - slot.Count)
		}
		if space >= need {
			return true
		}
	}
	return false
}

// HasItem 报告背包里是否有至少 count 个匹配的物品。创造模式永远为真。
func (p *Player) HasItem(target protocol.Item, count int) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if target.IsAir() || count <= 0 {
		return false
	}
	if p.GameMode == int32(protocol.GameModeCreative) {
		return true
	}
	return countMatching(p.Inventory, target.ID, target.Meta) >= count
}

// RemoveItem subtracts a matching stack from inventory (held slot first).
// Creative mode succeeds without changing slots.
func (p *Player) RemoveItem(item protocol.Item) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if item.IsAir() || item.Count == 0 {
		return false
	}
	if p.GameMode == int32(protocol.GameModeCreative) {
		return true
	}
	need := int(item.Count)
	if countMatching(p.Inventory, item.ID, item.Meta) < need {
		return false
	}
	held := int(p.HeldSlot)
	need = takeMatching(p, held, item.ID, item.Meta, need)
	for i := 0; i < len(p.Inventory) && need > 0; i++ {
		if i == held {
			continue
		}
		need = takeMatching(p, i, item.ID, item.Meta, need)
	}
	return need == 0
}

func countMatching(inv []protocol.Item, id, meta int32) int {
	n := 0
	for _, slot := range inv {
		if slot.IsAir() || slot.ID != id || slot.Meta != meta {
			continue
		}
		n += int(slot.Count)
	}
	return n
}

func takeMatching(p *Player, slot int, id, meta int32, need int) int {
	if slot < 0 || slot >= len(p.Inventory) || need <= 0 {
		return need
	}
	it := p.Inventory[slot]
	if it.IsAir() || it.ID != id || it.Meta != meta {
		return need
	}
	take := need
	if take > int(it.Count) {
		take = int(it.Count)
	}
	if take >= int(it.Count) {
		it = protocol.Air()
	} else {
		it.Count -= byte(take)
	}
	p.Inventory[slot] = it
	if slot == int(p.HeldSlot) {
		p.HeldItem = it
	}
	return need - take
}

// ConsumeHeld removes n from the selected hotbar stack. Creative mode does not consume.
func (p *Player) ConsumeHeld(n byte) (slot int, item protocol.Item, changed bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	slot = int(p.HeldSlot)
	if p.GameMode == int32(protocol.GameModeCreative) {
		return slot, p.HeldItem, false
	}
	if slot < 0 || slot >= len(p.Inventory) {
		return slot, protocol.Air(), false
	}
	it := p.Inventory[slot]
	if it.IsAir() || n == 0 {
		return slot, it, false
	}
	if it.Count > n {
		it.Count -= n
	} else {
		it = protocol.Air()
	}
	p.Inventory[slot] = it
	p.HeldItem = it
	return slot, it, true
}

// IsBodyColumn is true when the place target is the feet cell, one up, or one down.
func IsBodyColumn(px, py, pz float32, bx, by, bz int32) bool {
	feetX := int32(math.Floor(float64(px)))
	feetY := int32(math.Floor(float64(py)))
	feetZ := int32(math.Floor(float64(pz)))
	if bx != feetX || bz != feetZ {
		return false
	}
	return by == feetY || by == feetY+1 || by == feetY-1
}

// SyncInventory 把整个背包重新下发给玩家自己。死亡掉落清空背包后调用。
func (p *Player) SyncInventory() {
	p.mu.RLock()
	var sess *network.Session = p.Session
	var raw []protocol.Item = make([]protocol.Item, len(p.Inventory))
	copy(raw, p.Inventory)
	p.mu.RUnlock()
	if sess == nil {
		return
	}
	sess.SendPacket(&packets.ContainerSetContentPacket{
		WindowID:  0,
		TargetEID: p.EntityRuntimeID,
		Slots:     raw,
	})
}

// takeAllItems 清空背包并返回被清出来的物品，死亡掉落用。
// 创造模式由调用方判断，这里不做模式判断。
func (p *Player) takeAllItems() []protocol.Item {
	p.mu.Lock()
	defer p.mu.Unlock()
	var dumped []protocol.Item = nil
	var index int = 0
	for index = 0; index < len(p.Inventory); index++ {
		var item protocol.Item = p.Inventory[index]
		if item.IsAir() || item.Count == 0 {
			continue
		}
		dumped = append(dumped, item)
		p.Inventory[index] = protocol.Air()
	}
	var slot int = 0
	for slot = 0; slot < len(p.Armor); slot++ {
		if p.Armor[slot].IsAir() {
			continue
		}
		dumped = append(dumped, p.Armor[slot])
		p.Armor[slot] = protocol.Air()
	}
	p.HeldItem = p.Inventory[p.HeldSlot]
	return dumped
}

// claimNotice 判断这条提示现在能不能发。
// 同一条提示在 interval 内只发一次：玩家按住不放会持续触发拒绝，
// 不节流的话聊天栏会被刷爆。换了内容则立刻放行，方便看到新原因。
func (p *Player) claimNotice(text string, now time.Time, interval time.Duration) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.noticeText == text && now.Sub(p.noticeAt) < interval {
		return false
	}
	p.noticeText = text
	p.noticeAt = now
	return true
}

// horizontalPosition 只取 X/Z。击退、水平距离这类判定不需要 Y，
// 单独给一个函数，省得每个调用点都要处理三元返回里的垂直分量。
func (p *Player) horizontalPosition() (posX float32, posZ float32) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.X, p.Z
}

// onGroundSnapshot 读服务端记录的落地状态。
func (p *Player) onGroundSnapshot() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.OnGround
}

// moveRecord 是上一次通过校验的位置与时间，移动校验拿它当基准。
type moveRecord struct {
	PosX  float32
	PosY  float32
	PosZ  float32
	At    time.Time
	Known bool
}

// movementSnapshot 返回上次记录的位置与时间，供移动校验算速度。
func (p *Player) movementSnapshot() moveRecord {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return moveRecord{
		PosX:  p.lastMoveX,
		PosY:  p.lastMoveY,
		PosZ:  p.lastMoveZ,
		At:    p.lastMoveAt,
		Known: p.hasLastMove,
	}
}

// RecordMovement 记下这次通过校验的位置，作为下次速度比较的基准。
func (p *Player) RecordMovement(posX float32, posY float32, posZ float32, at time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastMoveX, p.lastMoveY, p.lastMoveZ = posX, posY, posZ
	p.lastMoveAt = at
	p.hasLastMove = true
}

// ResetMovement 清掉移动基准。传送、重生、切世界之后必须调，
// 否则跨越几百格的位置变化会被当成超速。
func (p *Player) ResetMovement() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hasLastMove = false
	p.lastMoveAt = time.Time{}
}
