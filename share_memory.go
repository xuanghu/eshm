// +build linux

package eshm

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const (
	Newest = iota //从最新位置消费
	Last          //从上一次记录的位置开始消费,如果没有上次消费位置,就从最开始消费
)

type ShareMemory struct {
	c     *ShConfig
	addr  uintptr
	queue *Queue
	//status int
}

type ShConfig struct {
	Key    int //共享内存key
	Size   int //内存大小
	Seq    int //消费序列号
	Method int
}

type Queue struct {
	Header *Header
	Data   []byte
}

func NewShareMemory(c *ShConfig) (*ShareMemory, error) {
	if c.Size > MaxCapacity {
		return nil, ErrOutOfCapacity
	}
	if c.Size < MinCapacity {
		return nil, ErrTooSmallSize
	}

	shmId, _, errCode := syscall.Syscall(syscall.SYS_SHMGET, uintptr(c.Key), uintptr(c.Size), ipcCreate|0600)
	if errCode != 0 {
		return nil, fmt.Errorf("syscall hmget error, err: %d\n", errCode)
	}

	shmAddr, _, errCode := syscall.Syscall(syscall.SYS_SHMAT, shmId, 0, 0)
	if errCode != 0 {
		return nil, fmt.Errorf("syscall hmat error, err: %d\n", errCode)
	}

	headerSize := unsafe.Sizeof(Header{})

	var data []byte
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	offset := headerSize + unsafe.Sizeof(sh.Len) + unsafe.Sizeof(sh.Cap) + unsafe.Sizeof(sh.Data)
	sh.Data = shmAddr + offset
	sh.Len = c.Size - int(offset)
	sh.Cap = c.Size - int(offset)

	header := (*Header)(unsafe.Pointer(shmAddr))
	if header.MaxLength == 0 {
		//header.Workers = uint32(c.Workers)
		header.MaxLength = uint32(sh.Cap / 2)
		header.Location = 0
	}

	queue := &Queue{
		Header: header,
		Data:   data,
	}

	return &ShareMemory{
		c:     c,
		queue: queue,
		addr:  shmAddr,
		//status: Writer,
	}, nil
}

func GetShareMemory(c *ShConfig) (*ShareMemory, error) {
	if c.Size > MaxCapacity {
		return nil, ErrOutOfCapacity
	}
	if c.Size < MinCapacity {
		return nil, ErrTooSmallSize
	}

	shmId, _, errCode := syscall.Syscall(syscall.SYS_SHMGET, uintptr(c.Key), uintptr(c.Size), ipcCreate|0600)
	if errCode != 0 {
		return nil, fmt.Errorf("syscall hmget error, err: %d\n", errCode)
	}

	shmAddr, _, errCode := syscall.Syscall(syscall.SYS_SHMAT, shmId, 0, 0)
	if errCode != 0 {
		return nil, fmt.Errorf("syscall hmat error, err: %d\n", errCode)
	}

	headerSize := unsafe.Sizeof(Header{})

	var data []byte
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	offset := headerSize + unsafe.Sizeof(sh.Len) + unsafe.Sizeof(sh.Cap) + unsafe.Sizeof(sh.Data)
	sh.Data = shmAddr + offset
	sh.Len = c.Size - int(offset)
	sh.Cap = c.Size - int(offset)

	header := (*Header)(unsafe.Pointer(shmAddr))

	if c.Seq == 0 {
		var (
			minLoc   int
			minValue = time.Now().Unix()
		)

		for loc, worker := range header.WorkerIndex {
			if worker.LastTime == 0 {
				minLoc = loc
				break
			} else {
				if minValue > worker.LastTime {
					minValue = worker.LastTime
					minLoc = loc
				}
			}
		}

		if abs(int(time.Now().Unix()), int(header.WorkerIndex[minLoc].LastTime)) < 120 { //2分钟未存活
			return nil, ErrNoEnoughWorker
		}

		c.Seq = minLoc
	}

	if c.Seq >= MaxWorker {
		return nil, ErrOutWorkerNumber
	}

	if c.Method == Newest {
		header.WorkerIndex[c.Seq].Location = header.Location
		if header.Location%2 == 0 {
			header.WorkerIndex[c.Seq].ReadIndex = header.EndIndex1
		} else {
			header.WorkerIndex[c.Seq].ReadIndex = header.EndIndex2
		}
		header.WorkerIndex[c.Seq].LastTime = time.Now().Unix()
	}

	//if header.WorkerIndex[c.Seq].Online != 0 {
	//	return nil, ErrHasWorkerConsume
	//}
	//header.WorkerIndex[c.Seq].Online++
	queue := &Queue{
		Header: header,
		Data:   data,
	}

	return &ShareMemory{
		c:     c,
		queue: queue,
		addr:  shmAddr,
		//status: Reader,
	}, nil
}

func (s *ShareMemory) Close() error {
	//if s.status == Reader {
	//	s.queue.Header.WorkerIndex[s.c.Seq].Online--
	//}

	_, _, errCode := syscall.Syscall(syscall.SYS_SHMDT, s.addr, 0, 0)
	if errCode != 0 {
		return fmt.Errorf("syscall hmdt error, err: %d\n", errCode)
	}
	return nil
}

func (s *ShareMemory) Append(buf []byte) error {
	data := newBinaryMessage(buf).serialize()

	if len(data) > int(s.queue.Header.MaxLength) {
		return ErrOutOfCapacity
	}

	//max := 0
	for !atomic.CompareAndSwapInt64(&s.queue.Header.WriteLock, 0, 1) {
		//if max > MaxTries {
		//	return ErrQueueLocking
		//}
		//max++
	}

START:
	if s.queue.Header.Location%2 == 0 {
		reserved := s.queue.Header.MaxLength - s.queue.Header.EndIndex1
		if int(reserved) >= len(data) { //空间足够
			for k, bit := range data {
				s.queue.Data[s.queue.Header.EndIndex1+uint32(k)] = bit
			}
			s.queue.Header.EndIndex1 += uint32(len(data))
		} else { //空间不足,换队列
			s.queue.Header.Location++
			s.queue.Header.EndIndex2 = 0
			goto START
		}
	} else {
		reserved := s.queue.Header.MaxLength - s.queue.Header.EndIndex2
		if int(reserved) >= len(data) { //空间足够
			for k, bit := range data {
				s.queue.Data[s.queue.Header.MaxLength+s.queue.Header.EndIndex2+uint32(k)] = bit
			}
			s.queue.Header.EndIndex2 += uint32(len(data))
		} else { //空间不足,换队列
			s.queue.Header.Location++
			s.queue.Header.EndIndex1 = 0
			goto START
		}
	}

	atomic.CompareAndSwapInt64(&s.queue.Header.WriteLock, 1, 0)
	return nil
}

func (s *ShareMemory) Get() ([][]byte, error) {
	return s.getByID(s.c.Seq)
}

func (s *ShareMemory) getByID(id int) ([][]byte, error) {
	worker := &s.queue.Header.WorkerIndex[id]
	//max := 0
	for !atomic.CompareAndSwapInt64(&worker.ReadLock, 0, 1) {
		//if max > MaxTries {
		//	return nil, ErrQueueLocking
		//}
		//max++
	}
	worker.LastTime = time.Now().Unix()

	diff := abs(int(s.queue.Header.Location), int(worker.Location))
	location := s.queue.Header.Location
	if diff >= 2 {
		worker.Location = location - 1
		worker.ReadIndex = 0
	}

	endIndex1 := s.queue.Header.EndIndex1
	endIndex2 := s.queue.Header.EndIndex2

	var retBytes []byte
START:
	if worker.Location == location { //已经最新
		if worker.Location%2 == 0 { //第一条队列
			if worker.ReadIndex < endIndex1 {
				retBytes = append(retBytes, s.queue.Data[int64(worker.ReadIndex):int64(endIndex1)]...)
				worker.ReadIndex = endIndex1
			}
		} else {
			if worker.ReadIndex < endIndex2 {
				readIndex := worker.ReadIndex + s.queue.Header.MaxLength
				endIndex := endIndex2 + s.queue.Header.MaxLength
				retBytes = append(retBytes, s.queue.Data[int64(readIndex):int64(endIndex)]...)
				worker.ReadIndex = endIndex2
			}
		}
	} else { //慢一轮
		if worker.Location%2 == 0 { //第一条队列
			if worker.ReadIndex <= endIndex1 {
				retBytes = append(retBytes, s.queue.Data[int64(worker.ReadIndex):int64(endIndex1)]...)
				worker.ReadIndex = 0
				worker.Location = location
				goto START
			}
		} else {
			if worker.ReadIndex <= endIndex2 {
				readIndex := worker.ReadIndex + s.queue.Header.MaxLength
				endIndex := endIndex2 + s.queue.Header.MaxLength
				retBytes = append(retBytes, s.queue.Data[int64(readIndex):int64(endIndex)]...)
				worker.ReadIndex = 0
				worker.Location = location
				goto START
			}
		}
	}

	atomic.CompareAndSwapInt64(&worker.ReadLock, 1, 0)
	if len(retBytes) == 0 {
		return nil, nil
	}

	binaryMessages, err := deserializeSlice(retBytes)
	if err != nil {
		return nil, err
	}

	var ret [][]byte
	for _, bm := range binaryMessages {
		ret = append(ret, bm.Body)
	}

	return ret, nil
}

func abs(x, y int) int {
	if x > y {
		return x - y
	} else {
		return y - x
	}
}
