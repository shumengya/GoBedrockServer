package world

// 三个维度都在 4×8×4 的格点采样，区块最外层格点同时属于下一块。
type terrainGrid [5][5][17]float64

func sampleTerrainGrid(noise *simplex, baseX, baseZ, scale float64) terrainGrid {
	var grid = terrainGrid{}
	var col, row, level = 0, 0, 0
	for col = 0; col < 5; col++ {
		for row = 0; row < 5; row++ {
			for level = 0; level < 17; level++ {
				grid[col][row][level] = noise.density((baseX+float64(4*col))*scale, float64(8*level)*scale, (baseZ+float64(4*row))*scale)
			}
		}
	}
	return grid
}

func (grid *terrainGrid) profile(col, row int) [17]float64 {
	var heights = [17]float64{}
	var cellX, cellZ = col / 4, row / 4
	var fracX, fracZ = float64(col%4) / 4, float64(row%4) / 4
	var level = 0
	for level = 0; level < 17; level++ {
		heights[level] = terrainMix(terrainMix(grid[cellX][cellZ][level], grid[cellX+1][cellZ][level], fracX), terrainMix(grid[cellX][cellZ+1][level], grid[cellX+1][cellZ+1][level], fracX), fracZ)
	}
	return heights
}
