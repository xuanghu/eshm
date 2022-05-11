package eshm

type ShareMemory struct {
}

type ShConfig struct {
	Key  int
	Size int
	Seq  int
}

type Queue struct {
	Header *Header
	Data   []byte
}

func NewShareMemory(c *ShConfig) (*ShareMemory, error) {
	return &ShareMemory{}, nil
}

func GetShareMemory(c *ShConfig) (*ShareMemory, error) {
	return &ShareMemory{}, nil
}

func (s *ShareMemory) Close() error {
	return nil
}

func (s *ShareMemory) Append(buf []byte) error {
	return nil
}

func (s *ShareMemory) Get() ([][]byte, error) {
	return s.getByID(0)
}

func (s *ShareMemory) getByID(id int) ([][]byte, error) {
	return nil, nil
}

func abs(x, y int) int {
	if x > y {
		return x - y
	} else {
		return y - x
	}
}
