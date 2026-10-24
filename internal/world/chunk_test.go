package world

import "testing"

func TestFlatChunkSerialize(t *testing.T) {
	w := New("test")
	data := w.NetworkSerialize(0, 0)
	if len(data) < 1+10240+512+256+2 {
		t.Fatalf("payload too short: %d", len(data))
	}
	if data[0] < 1 {
		t.Fatalf("subchunk count %d", data[0])
	}
	id, _ := w.GetBlock(0, 0, 0)
	if id != BlockBedrock {
		t.Fatalf("y0 want bedrock got %d", id)
	}
	id, _ = w.GetBlock(0, 4, 0)
	if id != BlockGrass {
		t.Fatalf("y4 want grass got %d", id)
	}
	if !w.SetBlock(0, 5, 0, BlockStone, 0) {
		t.Fatal("place stone failed")
	}
	id, _ = w.GetBlock(0, 5, 0)
	if id != BlockStone {
		t.Fatalf("placed stone got %d", id)
	}
	if w.SetBlock(0, 0, 0, BlockAir, 0) {
		t.Fatal("should not replace bedrock")
	}
}

func TestSkyAndOceanGenerators(t *testing.T) {
	sky := New("sky")
	sky.Generator = GeneratorSky
	id, _ := sky.GetBlock(8, 64, 8)
	if id != BlockGrass {
		t.Fatalf("sky (8,64,8) want grass got %d", id)
	}
	id, _ = sky.GetBlock(0, 64, 0)
	if id != BlockGrass {
		t.Fatalf("sky (0,64,0) want grass got %d", id)
	}
	id, _ = sky.GetBlock(15, 64, 15)
	if id != BlockGrass {
		t.Fatalf("sky (15,64,15) want grass got %d", id)
	}
	id, _ = sky.GetBlock(0, 63, 0)
	if id != BlockDirt {
		t.Fatalf("sky (0,63,0) want dirt got %d", id)
	}
	id, _ = sky.GetBlock(16, 64, 0)
	if id != BlockAir {
		t.Fatalf("sky chunk 1,0 want air got %d", id)
	}
	id, _ = sky.GetBlock(-1, 64, 0)
	if id != BlockAir {
		t.Fatalf("sky x=-1 want air got %d", id)
	}
	if !sky.SetBlock(0, 0, 0, BlockStone, 0) {
		t.Fatal("sky y=0 is not bedrock and should be replaceable")
	}

	ocean := New("ocean")
	ocean.Generator = GeneratorOcean
	id, _ = ocean.GetBlock(8, 64, 8)
	if id != BlockGrass {
		t.Fatalf("ocean (8,64,8) want grass got %d", id)
	}
	id, _ = ocean.GetBlock(15, 63, 15)
	if id != BlockDirt {
		t.Fatalf("ocean island pad want dirt got %d", id)
	}
	id, _ = ocean.GetBlock(16, 63, 0)
	if id != BlockStillWater {
		t.Fatalf("ocean y=63 outside island want water got %d", id)
	}
	id, _ = ocean.GetBlock(16, 64, 0)
	if id != BlockAir {
		t.Fatalf("ocean y=64 outside island want air got %d", id)
	}
	id, _ = ocean.GetBlock(16, 10, 0)
	if id != BlockStillWater {
		t.Fatalf("ocean other chunk y=10 want water got %d", id)
	}
	id, _ = ocean.GetBlock(16, 80, 0)
	if id != BlockAir {
		t.Fatalf("ocean other chunk y=80 want air got %d", id)
	}
	if !ocean.SetBlock(16, 0, 0, BlockAir, 0) {
		t.Fatal("ocean y=0 water should be replaceable")
	}
}

func TestNetherAndEndGenerators(t *testing.T) {
	nether := New("nether")
	nether.Generator = GeneratorNether
	id, _ := nether.GetBlock(8, 64, 8)
	if id != BlockNetherrack {
		t.Fatalf("nether (8,64,8) want netherrack got %d", id)
	}
	id, _ = nether.GetBlock(16, 64, 0)
	if id != BlockAir {
		t.Fatalf("nether outside island want air got %d", id)
	}
	if nether.ProtocolDimension() != 1 {
		t.Fatalf("nether dim=%d", nether.ProtocolDimension())
	}

	end := New("end")
	end.Generator = GeneratorEnd
	id, _ = end.GetBlock(0, 64, 0)
	if id != BlockEndStone {
		t.Fatalf("end (0,64,0) want end stone got %d", id)
	}
	id, _ = end.GetBlock(15, 64, 15)
	if id != BlockEndStone {
		t.Fatalf("end (15,64,15) want end stone got %d", id)
	}
	id, _ = end.GetBlock(16, 64, 0)
	if id != BlockAir {
		t.Fatalf("end outside island want air got %d", id)
	}
	if end.ProtocolDimension() != 2 {
		t.Fatalf("end dim=%d", end.ProtocolDimension())
	}
}

func TestEncodeDecodeDisk(t *testing.T) {
	c := newChunk()
	c.fillFlat()
	c.SetBlock(3, 10, 7, BlockStone, 2)
	raw := c.EncodeDisk()
	got, err := DecodeDisk(raw)
	if err != nil {
		t.Fatal(err)
	}
	id, data := got.GetBlock(3, 10, 7)
	if id != BlockStone || data != 2 {
		t.Fatalf("roundtrip stone got id=%d data=%d", id, data)
	}
	id, _ = got.GetBlock(0, 4, 0)
	if id != BlockGrass {
		t.Fatalf("grass lost, got %d", id)
	}
	if got.Dirty() {
		t.Fatal("decoded column should not be dirty")
	}
}

func TestOffsetByFace(t *testing.T) {
	x, y, z, ok := OffsetByFace(10, 5, 10, 1)
	if !ok || y != 6 {
		t.Fatalf("up face: %d %d %d ok=%v", x, y, z, ok)
	}
	_, _, _, ok = OffsetByFace(0, 0, 0, -1)
	if ok {
		t.Fatal("face -1 should fail")
	}
}
