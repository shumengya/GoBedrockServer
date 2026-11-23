package world

import (
	"math"
	"math/rand/v2"
)

type simplex struct {
	perm   [512]int
	offset [3]float64
}

var gradients = [12][3]int{{1, 1, 0}, {-1, 1, 0}, {1, -1, 0}, {-1, -1, 0}, {1, 0, 1}, {-1, 0, 1}, {1, 0, -1}, {-1, 0, -1}, {0, 1, 1}, {0, -1, 1}, {0, 1, -1}, {0, -1, -1}}

func newSimplex(random *rand.Rand) simplex {
	var noise = simplex{}
	var order = random.Perm(256)
	var index = 0
	for index = 0; index < 512; index++ {
		noise.perm[index] = order[index&255]
	}
	var axis = 0
	for axis = 0; axis < 3; axis++ {
		noise.offset[axis] = random.Float64() * 256
	}
	return noise
}

func (noise *simplex) corner(cellX, cellY, cellZ int, posX, posY, posZ, radius float64) float64 {
	var weight = radius - posX*posX - posY*posY - posZ*posZ
	if weight <= 0 {
		return 0
	}
	var grad = gradients[noise.perm[(cellX&255)+noise.perm[(cellY&255)+noise.perm[cellZ&255]]]%12]
	weight *= weight
	return weight * weight * (float64(grad[0])*posX + float64(grad[1])*posY + float64(grad[2])*posZ)
}

func (noise *simplex) sample2(posX, posZ float64) float64 {
	var root = math.Sqrt(3)
	var skew, unskew = (root - 1) / 2, (3 - root) / 6
	posX += noise.offset[0]
	posZ += noise.offset[1]
	var shift = (posX + posZ) * skew
	var cellX, cellZ = int(math.Floor(posX + shift)), int(math.Floor(posZ + shift))
	var origin = float64(cellX+cellZ) * unskew
	posX, posZ = posX-float64(cellX)+origin, posZ-float64(cellZ)+origin
	var stepX, stepZ = 0, 1
	if posX > posZ {
		stepX, stepZ = 1, 0
	}
	return 70 * (noise.corner(cellX, cellZ, 0, posX, posZ, 0, .5) + noise.corner(cellX+stepX, cellZ+stepZ, 0, posX-float64(stepX)+unskew, posZ-float64(stepZ)+unskew, 0, .5) + noise.corner(cellX+1, cellZ+1, 0, posX-1+2*unskew, posZ-1+2*unskew, 0, .5))
}

func (noise *simplex) sample3(posX, posY, posZ float64) float64 {
	posX += noise.offset[0]
	posY += noise.offset[1]
	posZ += noise.offset[2]
	var shift = (posX + posY + posZ) / 3
	var cellX, cellY, cellZ = int(math.Floor(posX + shift)), int(math.Floor(posY + shift)), int(math.Floor(posZ + shift))
	var origin = float64(cellX+cellY+cellZ) / 6
	posX, posY, posZ = posX-float64(cellX)+origin, posY-float64(cellY)+origin, posZ-float64(cellZ)+origin
	var rank = [3]int{}
	if posX >= posY {
		rank[0]++
	} else {
		rank[1]++
	}
	if posX >= posZ {
		rank[0]++
	} else {
		rank[2]++
	}
	if posY >= posZ {
		rank[1]++
	} else {
		rank[2]++
	}
	var value = noise.corner(cellX, cellY, cellZ, posX, posY, posZ, .6)
	var step, axis = 0, 0
	for step = 1; step <= 3; step++ {
		var delta = [3]int{}
		for axis = 0; axis < 3; axis++ {
			if rank[axis] >= 3-step {
				delta[axis] = 1
			}
		}
		var offset = float64(step) / 6
		value += noise.corner(cellX+delta[0], cellY+delta[1], cellZ+delta[2], posX-float64(delta[0])+offset, posY-float64(delta[1])+offset, posZ-float64(delta[2])+offset, .6)
	}
	return 32 * value
}

func (noise *simplex) climate(posX, posZ float64) float64 {
	var value = (noise.sample2(posX/512, posZ/512) + noise.sample2(posX/256, posZ/256)/16) / (1 + 1.0/16)
	return math.Floor((value+1)*.5*63) / 63
}

func (noise *simplex) density(posX, posY, posZ float64) float64 {
	var value, amplitude, scale = 0.0, 1.0, 1.0 / 32
	var octave = 0
	for octave = 0; octave < 4; octave++ {
		value += noise.sample3(posX*scale, posY*scale, posZ*scale) * amplitude
		amplitude *= .25
		scale *= 2
	}
	return value / (1 + .25 + .0625 + .015625)
}
