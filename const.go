package eshm

import "errors"

const ipcCreate = 00001000

const MaxCapacity = 8 << 30
const MinCapacity = 256

const MaxWorker = 32
const MaxTries = 32

//const (
//	Writer = iota
//	Reader
//)

var (
	ErrOutOfCapacity   = errors.New("out of capacity")
	ErrOutWorkerNumber = errors.New("out of worker number")
	ErrTooSmallSize    = errors.New("size too small")
	ErrNoEnoughWorker  = errors.New("no enough worker")
	ErrQueueLocking    = errors.New("queue has writing")
	//ErrHasWorkerConsume = errors.New("has worker consume")
)

type Header struct {
	//Workers     uint32                  //消费者数量
	MaxLength   uint32                  //单个队列最大长度
	WriteLock   int64                   //写锁
	Location    uint32                  //奇数:正在写第一条队列;偶数：正在写第二条队列
	EndIndex1   uint32                  //数据的终止位置
	EndIndex2   uint32                  //数据的终止位置
	WorkerIndex [MaxWorker]ConsumeIndex //记录每个消费者的位置
}

type ConsumeIndex struct {
	//Online    uint32
	ReadLock  int64 //读锁
	LastTime  int64
	Location  uint32
	ReadIndex uint32
}
