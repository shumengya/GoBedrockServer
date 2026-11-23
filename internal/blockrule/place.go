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
	// Drop 为真表示原来的方块是"自己掉下来"式地消失（比如脚下被挖空的火把）。
	// 服务端要按空手挖掉来结算掉落物和破坏粒子，而不是悄悄变成空气。
	Drop bool
	// Fall 为真表示原来的方块（沙子、沙砾）脚下空了，要变成一个下落中的方块实体，而不是直接消失。
	Fall bool
	// Quiet 只同步这一格的内部状态，不唤醒邻居（如树叶的待检查位）。
	Quiet bool
}

// Place 照参考端 Level::useItemOn 的放置流程，算出这次放置要改哪些格子。
// 它只读 view，不改世界；返回 false 表示这次放不了。
//
// 门、床、双层植物占两格，走 placeMulti。
func Place(view View, req PlaceRequest) ([]Change, bool) {
	var hand, isBlock = BlockForItem(req.ItemID)
	if !isBlock {
		return nil, false
	}
	var handMeta byte = byte(req.ItemMeta) & 15
	if req.ItemID > 255 {
		handMeta = 0
	}
	var targetID, targetMeta = view.Block(req.Target)
	if targetID == BlockAir {
		return nil, false
	}
	var side, sideOK = req.Target.Side(req.Face)
	if !sideOK || side.PosY < 0 || side.PosY > 255 {
		return nil, false
	}
	// 被点击的方块本身能顶替（高草、水、能合并的半砖）就放在它那格，否则放到它旁边那格。
	var spot Pos = req.Target
	if !canBePlacedAt(hand, handMeta, targetID, targetMeta, req, true) {
		spot = side
		var sideID, sideMeta = view.Block(side)
		if !canBePlacedAt(hand, handMeta, sideID, sideMeta, req, false) {
			return nil, false
		}
	}
	if isSlab(hand) {
		return placeSlab(view, req, hand, handMeta, spot, targetID, targetMeta)
	}
	if isMultiCell(hand) {
		return placeMulti(view, req, hand, handMeta, spot)
	}
	if isSign(hand) {
		return placeSign(view, req, spot)
	}
	// 花盆、头颅、物品展示框、漏斗的朝向由被点击的面决定，不走下面按偏航角算 meta 的路。
	switch hand {
	case BlockFlowerPot:
		return placeFlowerPot(view, spot)
	case BlockSkull:
		return placeSkull(view, req, spot)
	case BlockItemFrame:
		return placeItemFrame(view, req, spot)
	case BlockHopper:
		return placeHopper(spot, req.Face)
	}
	if !supportedAt(view, hand, spot) {
		return nil, false
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
	if isInteractive(hand) {
		var interactiveMeta, placeable = interactivePlacementMeta(req, hand, targetID)
		if !placeable {
			return nil, false
		}
		meta = interactiveMeta
	}
	if hand == BlockTorch {
		var torch, torchOK = torchPlacementMeta(view, req, spot, targetID)
		if !torchOK {
			return nil, false
		}
		meta = torch
	}
	if hand == BlockLeaves || hand == BlockLeaves2 {
		meta = meta&3 | 4
	}
	return []Change{{Pos: spot, ID: hand, Meta: meta}}, true
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
