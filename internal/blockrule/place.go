package blockrule

// PlaceRequest 描述玩家用手里的方块点击了世界里的某一格。
type PlaceRequest struct {
	// ItemID、ItemMeta 是手持物，以服务端记录为准。
	ItemID   int32
	ItemMeta int32
	// Target 是被点击的方块，Face 是被点击的面（0~5）。
	Target Pos
	Face   int32
	// ClickX/Y/Z 是点击点在被点击方块内的位置，范围 0~1。
	ClickX float32
	ClickY float32
	ClickZ float32
	// Yaw 是玩家偏航角，决定楼梯、箱子、熔炉的朝向。
	Yaw float32
}

// Change 是一格方块的改动。
type Change struct {
	Pos  Pos
	ID   byte
	Meta byte
}

// Place 照参考端 Level::useItemOn 的放置流程，算出这次放置要改哪些格子。
// 它只读 view，不改世界；返回 false 表示这次放不了。
//
// 目前只处理 ID 小于 256 的方块物品。种子、门、床这类物品 ID 大于 255 的，
// 等对应的多格方块和附着规则做完再接进来，现在统一返回 false。
func Place(view View, req PlaceRequest) ([]Change, bool) {
	if req.ItemID <= 0 || req.ItemID > 255 {
		return nil, false
	}
	var hand byte = byte(req.ItemID)
	var handMeta byte = byte(req.ItemMeta) & 15
	var targetID, targetMeta = view.Block(req.Target)
	if targetID == BlockAir {
		return nil, false
	}
	var side, sideOK = req.Target.Side(req.Face)
	if !sideOK || side.PosY < 0 || side.PosY > 255 {
		return nil, false
	}
	// 被点击的方块本身能顶替（高草、水、能合并的半砖）就放在它那格，否则放到它旁边那格。
	var at Pos = req.Target
	if !canBePlacedAt(hand, handMeta, targetID, targetMeta, req, true) {
		at = side
		var sideID, sideMeta = view.Block(side)
		if !canBePlacedAt(hand, handMeta, sideID, sideMeta, req, false) {
			return nil, false
		}
	}
	if isSlab(hand) {
		return placeSlab(view, req, hand, handMeta, at, targetID, targetMeta)
	}
	var meta byte = handMeta
	if isStair(hand) {
		meta = stairMeta(DirectionFromYaw(req.Yaw), req.Face, req.ClickY)
	}
	if hand == BlockLog || hand == BlockWood2 {
		meta = logMeta(handMeta, req.Face)
	}
	if hasFacing(hand) {
		meta = facingMeta(DirectionFromYaw(req.Yaw))
	}
	if hand == BlockTorch {
		var torch, torchOK = torchPlacementMeta(view, req, at, targetID)
		if !torchOK {
			return nil, false
		}
		meta = torch
	}
	return []Change{{Pos: at, ID: hand, Meta: meta}}, true
}

// canBePlacedAt 对应参考端 Block::canBePlacedAt：这一格能不能放下手里的方块。
// 半砖例外：已有同种半砖时，点击位置在对应的一半才允许合并成双层。
func canBePlacedAt(
	hand byte,
	handMeta byte,
	cellID byte,
	cellMeta byte,
	req PlaceRequest,
	isClicked bool,
) bool {
	if IsReplaceable(cellID, cellMeta) {
		return true
	}
	if !isSlab(hand) {
		return false
	}
	if cellID != hand || cellMeta&7 != handMeta&7 {
		return false
	}
	// 下半砖要点上半部分，上半砖要点下半部分，才算往里叠。
	if cellMeta&8 != 0 {
		return req.ClickY <= 0.5 || (!isClicked && req.Face == FaceUp)
	}
	return req.ClickY >= 0.5 || (!isClicked && req.Face == FaceDown)
}
