package storage

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"

	"go-mc-server/internal/world"
)

const (
	gmrMagic   = "GMR1"
	gmrVersion = uint16(1)
	gmrSectors = 4096
	gmrSlots   = 1024
)

// RegionStore maps chunk columns in one dimension directory to r.X.Z.gmr files.
type RegionStore struct {
	dir   string
	mu    sync.Mutex
	files map[uint64]*regionMem
}

type regionMem struct {
	mu    sync.Mutex
	path  string
	slots [gmrSlots][]byte
}

func NewRegionStore(dimDir string) *RegionStore {
	return &RegionStore{
		dir:   dimDir,
		files: make(map[uint64]*regionMem),
	}
}

func (s *RegionStore) LoadColumn(cx, cz int32) (*world.Chunk, error) {
	rx, rz := floorDiv(cx, 32), floorDiv(cz, 32)
	idx := chunkIndex(cx, cz)
	r, err := s.region(rx, rz)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	payload := r.slots[idx]
	r.mu.Unlock()
	if len(payload) == 0 {
		return nil, nil
	}
	raw, err := inflate(payload)
	if err != nil {
		return nil, err
	}
	return world.DecodeDisk(raw)
}

func (s *RegionStore) SaveColumn(cx, cz int32, c *world.Chunk) error {
	raw := c.EncodeDisk()
	comp, err := deflate(raw)
	if err != nil {
		return err
	}
	rx, rz := floorDiv(cx, 32), floorDiv(cz, 32)
	idx := chunkIndex(cx, cz)
	r, err := s.region(rx, rz)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.slots[idx] = comp
	err = r.flush()
	r.mu.Unlock()
	return err
}

func (s *RegionStore) region(rx, rz int32) (*regionMem, error) {
	key := uint64(uint32(rx))<<32 | uint64(uint32(rz))
	s.mu.Lock()
	r, ok := s.files[key]
	if ok {
		s.mu.Unlock()
		return r, nil
	}
	s.mu.Unlock()

	path := RegionPath(s.dir, rx, rz)
	loaded, err := readRegionFile(path)
	if err != nil {
		return nil, err
	}
	loaded.path = path

	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.files[key]; ok {
		return existing, nil
	}
	s.files[key] = loaded
	return loaded, nil
}

func readRegionFile(path string) (*regionMem, error) {
	r := &regionMem{path: path}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, err
	}
	if len(raw) < 8+gmrSlots*4 {
		return nil, fmt.Errorf("region too short: %s", path)
	}
	if string(raw[0:4]) != gmrMagic {
		return nil, fmt.Errorf("bad region magic: %s", path)
	}
	ver := binary.LittleEndian.Uint16(raw[4:6])
	if ver != gmrVersion {
		return nil, fmt.Errorf("unsupported region version %d: %s", ver, path)
	}
	locOff := 8
	for i := 0; i < gmrSlots; i++ {
		loc := binary.LittleEndian.Uint32(raw[locOff+i*4 : locOff+i*4+4])
		if loc == 0 {
			continue
		}
		off := int(loc>>8) * gmrSectors
		n := int(loc&0xff) * gmrSectors
		if off < 0 || n <= 0 || off+n > len(raw) {
			return nil, fmt.Errorf("bad slot %d in %s", i, path)
		}
		blob := raw[off : off+n]
		if len(blob) < 8 {
			continue
		}
		compLen := binary.LittleEndian.Uint32(blob[4:8])
		if 8+int(compLen) > len(blob) {
			return nil, fmt.Errorf("truncated slot %d in %s", i, path)
		}
		p := make([]byte, compLen)
		copy(p, blob[8:8+compLen])
		r.slots[i] = p
	}
	return r, nil
}

func (r *regionMem) flush() error {
	var payload bytes.Buffer
	payload.WriteString(gmrMagic)
	_ = binary.Write(&payload, binary.LittleEndian, gmrVersion)
	_ = binary.Write(&payload, binary.LittleEndian, uint16(0))
	locOff := payload.Len()
	payload.Write(make([]byte, gmrSlots*4))

	headerEnd := payload.Len()
	pad := (gmrSectors - headerEnd%gmrSectors) % gmrSectors
	if pad > 0 {
		payload.Write(make([]byte, pad))
	}

	locs := make([]uint32, gmrSlots)
	for i := 0; i < gmrSlots; i++ {
		comp := r.slots[i]
		if len(comp) == 0 {
			continue
		}
		offSectors := payload.Len() / gmrSectors
		var slot bytes.Buffer
		uncompHint := uint32(0)
		_ = binary.Write(&slot, binary.LittleEndian, uncompHint)
		_ = binary.Write(&slot, binary.LittleEndian, uint32(len(comp)))
		slot.Write(comp)
		nsec := (slot.Len() + gmrSectors - 1) / gmrSectors
		if nsec > 255 {
			return fmt.Errorf("chunk payload too large for region slot")
		}
		need := nsec*gmrSectors - slot.Len()
		if need > 0 {
			slot.Write(make([]byte, need))
		}
		locs[i] = uint32(offSectors<<8) | uint32(nsec)
		payload.Write(slot.Bytes())
	}
	out := payload.Bytes()
	for i := 0; i < gmrSlots; i++ {
		binary.LittleEndian.PutUint32(out[locOff+i*4:], locs[i])
	}

	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}

func chunkIndex(cx, cz int32) int {
	lx := ((cx % 32) + 32) % 32
	lz := ((cz % 32) + 32) % 32
	return int(lx) + int(lz)*32
}

func floorDiv(a, b int32) int32 {
	q := a / b
	if a%b != 0 && a < 0 {
		q--
	}
	return q
}

func deflate(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func inflate(comp []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(comp))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
