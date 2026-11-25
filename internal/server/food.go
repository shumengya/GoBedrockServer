package server

import "time"

import "go-mc-server/internal/protocol"
import "go-mc-server/internal/storage"

// 饥饿数值，取自原版 / 参考端 Human.php。
const maxFoodLevel float32 = 20

// initialSaturation 是新玩家和重生后的饱和度。原版是 5；参考端直接取属性默认值 20，
// 会让饥饿条隔很久才掉第一格，看着像坏了，这里按原版。
const initialSaturation float32 = 5

// exhaustionPerPoint：疲劳度每攒满 4 点，扣 1 点饱和度，饱和度见底才扣饥饿值。
const exhaustionPerPoint float32 = 4

// foodTickPeriod 是回血 / 饿血的结算周期，80 tick = 4 秒。
const foodTickPeriod int32 = 80

// 和平模式：每 10 tick 补 1 点饥饿值，每 20 tick 回 1 点血。
const peacefulFoodTick int32 = 10
const peacefulHealTick int32 = 20

// 饥饿值 >= 18 才自然回血；<= 6 不能疾跑；到 0 开始饿血。
const regenFoodLevel float32 = 18
const sprintBlockedFood float32 = 6

// 各种行为的疲劳度，数值对齐参考端 exhaust() 的调用点。
const exhaustWalkPerBlock float32 = 0.01
const exhaustSprintPerBlock float32 = 0.1
const exhaustJump float32 = 0.2
const exhaustSprintJump float32 = 0.8
const exhaustMining float32 = 0.025
const exhaustAttack float32 = 0.3
const exhaustDamage float32 = 0.3
const exhaustRegen float32 = 3

// maxExhaustMoveStep 限制单次位移计入疲劳的格数。传送、重生、卡顿后补发的大位移
// 不是真的走了那么远，不该被算成疾跑几十格。
const maxExhaustMoveStep float32 = 3

const starvationDamage float32 = 1

// foodState 是玩家的饥饿状态。读写都要在 Player.mu 保护下进行。
type foodState struct {
	Food       float32
	Saturation float32
	Exhaustion float32
	// Timer 在 0..foodTickPeriod 间循环，归零那一刻结算回血 / 饿血。
	Timer     int32
	Sprinting bool
	LastEat   time.Time
}

func newFoodState() foodState {
	return foodState{Food: maxFoodLevel, Saturation: initialSaturation}
}

// setFood 改饥饿值。跨过 17 / 6 / 0 这几个档位时把计时清零，对齐参考端：
// 档位不同回血 / 疾跑 / 饿血的规则就不同，旧档位攒的计时不能带过去。
func (state *foodState) setFood(next float32) {
	var old float32 = state.Food
	state.Food = max(0, min(maxFoodLevel, next))
	var bounds [3]float32 = [3]float32{17, 6, 0}
	var index int = 0
	for index = 0; index < len(bounds); index++ {
		if (old > bounds[index]) != (state.Food > bounds[index]) {
			state.Timer = 0
			return
		}
	}
}

// exhaust 累加疲劳度，返回饥饿值或饱和度是否变了（变了才需要同步给客户端）。
func (state *foodState) exhaust(amount float32) bool {
	var changed bool = false
	state.Exhaustion += amount
	for state.Exhaustion >= exhaustionPerPoint {
		state.Exhaustion -= exhaustionPerPoint
		if state.Saturation > 0 {
			state.Saturation = max(0, state.Saturation-1)
			changed = true
			continue
		}
		if state.Food > 0 {
			state.setFood(state.Food - 1)
			changed = true
		}
	}
	return changed
}

// eat 吃下食物。饱和度不能超过当前饥饿值（原版规则），否则吃饱了还白攒一堆饱和度。
func (state *foodState) eat(restore float32, saturation float32) {
	state.setFood(state.Food + restore)
	state.Saturation = min(state.Food, state.Saturation+saturation)
}

// foodTickEffect 是一个 tick 的结算结果，由调用方去真正回血 / 扣血并同步客户端。
type foodTickEffect struct {
	Heal    float32
	Starve  bool
	Changed bool
}

// starvationApplies 饿血的下限随难度变：简单只饿到 5 颗心，普通饿到半颗心，困难会饿死。
func starvationApplies(health float32, difficulty int32) bool {
	if difficulty == protocol.DifficultyEasy {
		return health > 10
	}
	if difficulty == protocol.DifficultyNormal {
		return health > 1
	}
	return difficulty == protocol.DifficultyHard
}

// tick 推进一个 tick。逻辑逐行对应参考端 Human::doFoodTick。
func (state *foodState) tick(health, healthLimit float32, difficulty int32) foodTickEffect {
	var effect foodTickEffect = foodTickEffect{}
	state.Timer++
	if state.Timer >= foodTickPeriod {
		state.Timer = 0
	}
	if state.Food <= sprintBlockedFood {
		state.Sprinting = false
	}
	if difficulty == protocol.DifficultyPeaceful && state.Timer%peacefulFoodTick == 0 {
		if state.Food < maxFoodLevel {
			state.setFood(state.Food + 1)
			effect.Changed = true
		}
		if state.Timer%peacefulHealTick == 0 && health < healthLimit {
			effect.Heal = 1
		}
	}
	if state.Timer != 0 {
		return effect
	}
	if state.Food >= regenFoodLevel {
		if health < healthLimit && effect.Heal == 0 {
			effect.Heal = 1
			effect.Changed = state.exhaust(exhaustRegen) || effect.Changed
		}
		return effect
	}
	if state.Food <= 0 {
		effect.Starve = starvationApplies(health, difficulty)
	}
	return effect
}

// save / restore 在存档与内存状态之间转换。读档时夹到合法范围，坏档不能让玩家一进服就饿死或溢出。
func (state *foodState) save() *storage.FoodData {
	return &storage.FoodData{
		Level:      state.Food,
		Saturation: state.Saturation,
		Exhaustion: state.Exhaustion,
		Timer:      state.Timer,
	}
}

func (state *foodState) restore(data *storage.FoodData) {
	if data == nil {
		return
	}
	state.Food = max(0, min(maxFoodLevel, data.Level))
	state.Saturation = max(0, min(state.Food, data.Saturation))
	state.Exhaustion = max(0, min(exhaustionPerPoint, data.Exhaustion))
	state.Timer = max(0, min(foodTickPeriod-1, data.Timer))
}
